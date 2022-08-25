package core

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

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
	}

	if key == "" {
		w.Write([]byte("key is required"))
	}

	if action == "set" || val == "" {
		w.Write([]byte("val is required"))
	}

	var err error
	//TODO 反射
	if strings.ToLower(action) == "set" { //set 命令
		err = h.mainCache.Set(key, val)
		if err != nil {
			w.Write([]byte("cache set error:" + err.Error()))
		}
	} else if strings.ToLower(action) == "get" { //get 命令
		body, err := h.mainCache.Get(key)
		if err != nil {
			w.Write([]byte("cache get error:" + err.Error()))
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
		h.mainCache.Del(key)
		if err != nil {
			w.Write([]byte("cache del error:" + err.Error()))
		}
	}
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
	u := fmt.Sprintf("%v%v", g.baseURL, url.QueryEscape(r.URL.Query().Get("key")))
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

func ServerStart(srv *NodeServer) {
	http.Handle("/", srv)
	http.ListenAndServe(srv.self, nil)
}

var _ PeerGetter = (*NodeGetter)(nil)