package core

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/ylt94/mycache/consistenthash"
)

type service struct {
	addr    string
	mserver *master
}
type master struct {
	addr        string
	nodeGetters map[string]*NodeGetter //注册节点
	mu          sync.RWMutex           //hash 锁
	hash        *consistenthash.Map    //一致性hash
}

func NewService(addr string, mserver *master) *service {
	return &service{
		addr:    addr,
		mserver: mserver,
	}
}

func NewMaster(addr string) *master {
	return &master{
		addr: addr,
		hash: consistenthash.New(1, nil),
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
	nodeGetter.GetByHTTP(w, r)
}

func (m *master) loadNode(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hash.Add(name)
	if m.nodeGetters == nil {
		m.nodeGetters = make(map[string]*NodeGetter)
	}
	m.nodeGetters[name] = &NodeGetter{baseURL: name}
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
	}
	if _, err := m.getNode(name); err != nil {
		w.Write([]byte(fmt.Sprintf("node name:%s is exists", name)))
	}

	//注册节点
	m.loadNode(name)
	//TODO 心跳检测
	w.Write([]byte("success"))
}

func ServiceStart(srv *service) {
	go func() {
		http.Handle("/", srv)
		http.ListenAndServe(srv.addr, nil)
	}()
	go func() {
		http.Handle("/", srv.mserver)
		http.ListenAndServe(srv.mserver.addr, nil)
	}()
}
