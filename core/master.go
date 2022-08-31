package core

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ylt94/mycache/consistenthash"
)

type service struct {
	addr    string
	mserver *master
}
type master struct {
	addr              string
	nodeGetters       map[string]*NodeGetter //注册节点
	mu                sync.RWMutex           //hash 锁
	hash              *consistenthash.Map    //一致性hash
	heartBeatInterval time.Duration          //心跳检测间隔时间
	dieNodes          chan string            //挂掉节点处理队列
}

var defaultHeartBeatInterval time.Duration = 5
var defaultHeartBeatTimeOut time.Duration = 5 * time.Second

var defaultDieNodeChanCap int = 5

func NewService(addr string, mserver *master) *service {
	return &service{
		addr:    addr,
		mserver: mserver,
	}
}

func NewMaster(addr string) *master {
	return &master{
		addr:              addr,
		hash:              consistenthash.New(1, nil),
		heartBeatInterval: time.Second * defaultHeartBeatInterval,
		dieNodes:          make(chan string, defaultDieNodeChanCap),
	}
}

//对 client端的server
func (m *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	key := values.Get("key")
	if key == "" {
		w.Write([]byte("key is required"))
	}

	nodeGetter, err := m.mserver.getNode(key)
	if err != nil {
		w.Write([]byte("get no err:" + err.Error()))
	}

	err = nodeGetter.GetByHTTP(w, r)
	if err != nil {
		log.Println("node error: " + err.Error())
	}
}

func (m *master) registerNode(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nodeGetters[name]; ok {
		return fmt.Errorf(fmt.Sprintf("node name:%s is exists", name))
	}
	m.hash.Add(name)
	if m.nodeGetters == nil {
		m.nodeGetters = make(map[string]*NodeGetter)
	}
	m.nodeGetters[name] = &NodeGetter{baseURL: name}

	return nil
}

func (m *master) getNode(key string) (*NodeGetter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name := m.hash.Get(key)
	if getter, ok := m.nodeGetters[name]; ok {
		return getter, nil
	}
	return nil, errors.New("no such cache node:" + name)
}

//处理节点注册
func (m *master) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	name := values.Get("name")
	if name == "" {
		w.Write([]byte("node's name is required"))
		return
	}

	//注册节点
	err := m.registerNode(name)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	//心跳检测
	go m.heartBeat(name)

	w.Write([]byte("success"))
}

func (m *master) heartBeat(name string) {
	m.mu.RLock()
	node, ok := m.nodeGetters[name]
	if !ok {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	ticker := time.NewTicker(m.heartBeatInterval)
	for range ticker.C {
		err := node.heartBeat(defaultHeartBeatTimeOut)
		if err != nil {
			//发送给另外一个协程去处理
			m.dieNodes <- name
			break
		}
	}
	ticker.Stop()
}

//处理挂掉节点--单线程处理
func (m *master) dieNodeHandler() {
	for name := range m.dieNodes {
		log.Println("start handle die node:", name)
		node, _ := m.getNode(name)
		if node != nil {
			//再次验证
			err := node.heartBeat(defaultHeartBeatTimeOut)
			if err != nil {
				m.mu.Lock()
				delete(m.nodeGetters, name)
				//删除hash环的节点信息
				m.hash.Delete(name)
				m.mu.Unlock()
			}
		}
	}
}

func ServiceStart(srv *service) {
	go srv.mserver.dieNodeHandler()

	mx := http.NewServeMux()
	mx.Handle("/", srv.mserver)
	go http.ListenAndServe(srv.mserver.addr[7:], mx)

	http.Handle("/", srv)
	http.ListenAndServe(srv.addr[7:], nil)
}
