package core

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ylt94/mycache/consistenthash"
	mproto "github.com/ylt94/mycache/proto"
)

const defaultBasePath = "/node/"
const defaultReplicas = 50

type NodeServer struct {
	self        string
	basePath    string
	mu          sync.Mutex
	peers       *consistenthash.Map
	NodeGetters map[string]*NodeGetter
	mainCache   *mcache
}

type NodeGetter struct {
	baseURL string
}

func NewNodeServer(self string, cache *mcache) *NodeServer {
	return &NodeServer{
		self:      self,            //自己的ip地址端口信息
		basePath:  defaultBasePath, //通讯地址前缀
		mainCache: cache,
	}
}

func (h *NodeServer) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, v...))
}

func (h *NodeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	action := values.Get("action")
	key := values.Get("key")
	val := values.Get("value")
	if action == "" {
		w.Write([]byte("action is required"))
		return
	}
	if action == "ping" {
		w.Write([]byte("pong"))
		return
	}

	if key == "" {
		w.Write([]byte("key is required"))
		return
	}

	if action == "set" && val == "" {
		w.Write([]byte("value is required"))
		return
	}

	var err error
	//TODO 反射
	if strings.ToLower(action) == "set" { //set 命令
		err = h.mainCache.Set(key, val)
		if err != nil {
			w.Write([]byte("cache set error:" + err.Error()))
			return
		}
	} else if strings.ToLower(action) == "get" { //get 命令
		body, err := h.mainCache.Get(key)
		if err != nil {
			w.Write([]byte("cache get error:" + err.Error()))
			return
		}
		//proto 编码
		//body, err := proto.Marshal(&mproto.Response{Value: view.ByteSlice()})
		//if err != nil {
		//	http.Error(w, err.Error(), http.StatusInternalServerError)
		//	return
		//}
		//
		//w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	} else if strings.ToLower(action) == "del" { //del 命令
		err = h.mainCache.Del(key)
		if err != nil {
			w.Write([]byte("cache del error:" + err.Error()))
			return
		}
	}
}

func (h *NodeServer) register(masterAddr string) {
	url := masterAddr + "/mycache?action=register&name=" + h.self
	resp, err := http.Get(url)
	if err != nil {
		panic(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("node register returned:%v", resp.Status))
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Errorf("node register reading response  body: %v", err))
	}

	if string(bytes) != "success" {
		panic("node register returned:" + string(bytes))
	}
	log.Println("node:" + h.self + " register success")
}

// //加载节点地址-一致性hash环
// func (h *NodeServer) Set(peers ...string) {
// 	h.mu.Lock()
// 	defer h.mu.Unlock()

// 	h.peers = consistenthash.New(defaultReplicas, nil)
// 	h.peers.Add(peers...)

// 	h.NodeGetters = make(map[string]*NodeGetter, len(peers))
// 	for _, peer := range peers {
// 		h.NodeGetters[peer] = &NodeGetter{baseURL: peer + h.basePath}
// 	}
// }

// //通过对key的hash，找到节点，返回获取节点getter
// func (h *NodeServer) PickPeer(key string) (PeerGetter, bool) {
// 	h.mu.Lock()
// 	defer h.mu.Unlock()

// 	if peer := h.peers.Get(key); peer != "" && peer != h.self {
// 		h.Log("Pick peer %s", peer)
// 		return h.NodeGetters[peer], true
// 	}
// 	return nil, false
// }

//var _ PeerPicker = (*NodeServer)(nil)

//从节点获取value proto
func (g *NodeGetter) Get(in *mproto.Request, out *mproto.Response) error {
	u := fmt.Sprintf("%v%v", g.baseURL, url.QueryEscape(in.GetKey()))
	log.Println("start get data from", u)
	res, err := http.Get(u)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned:%v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response  body: %v", err)
	}

	err = proto.Unmarshal(bytes, out)
	if err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil
}

func (g *NodeGetter) GetByHTTP(w http.ResponseWriter, r *http.Request) error {
	u := g.baseURL + r.URL.String()
	log.Println("start get data from", u)
	res, err := http.Get(u)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned:%v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response  body: %v", err)
	}
	w.Write(bytes)
	return nil
}

//心跳检测
func (g *NodeGetter) heartBeat(timeOut time.Duration) error {
	u := g.baseURL + "/?action=ping"
	log.Println("start heart beat to", u)

	client := http.Client{Timeout: timeOut}
	res, err := client.Get(u)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("heart beat return error: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	log.Println("node heart beat returned:", string(bytes))
	if err != nil {
		return fmt.Errorf("heart beat reading reponse body error:%v", err)
	}
	if string(bytes) != "pong" {
		return fmt.Errorf("heart beat reading reponse body content error:%v", err)
	}
	return nil
}

func ServerStart(srv *NodeServer, mAddr string) {
	//去master注册
	srv.register(mAddr)
	//请求处理
	http.Handle("/", srv)
	http.ListenAndServe(srv.self[7:], nil)
}

var _ PeerGetter = (*NodeGetter)(nil)
