package mcache

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/ylt94/mcache/consistenthash"
	mproto "github.com/ylt94/mcache/proto"
)

const defaultBasePath = "/_mcache_/"
const defaultReplicas = 50

type HTTPServer struct {
	self        string
	basePath    string
	mu          sync.Mutex
	peers       *consistenthash.Map
	httpGetters map[string]*httpGetter
}

type httpGetter struct {
	baseURL string
}

func NewHTTPServer(self string) *HTTPServer {
	return &HTTPServer{
		self:     self,            //自己的ip地址端口信息
		basePath: defaultBasePath, //通讯地址前缀
	}
}

func (h *HTTPServer) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, v...))
}

func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//TODO
	if !strings.HasPrefix(r.URL.Path, h.basePath) {
		panic("HTTPPool serving unexpected path:" + r.URL.Path)
	}

	h.Log("%s %s", r.Method, r.URL.Path)

	parts := strings.SplitN(r.URL.Path[len(h.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	if mainCache == nil {
		http.Error(w, "no such group:"+groupName, http.StatusNotFound)
		return
	}

	view, err := mainCache.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//proto 编码
	body, err := proto.Marshal(&mproto.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}

//加载节点地址-一致性hash环
func (h *HTTPServer) Set(peers ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.peers = consistenthash.New(defaultReplicas, nil)
	h.peers.Add(peers...)

	h.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		h.httpGetters[peer] = &httpGetter{baseURL: peer + h.basePath}
	}
}

//通过对key的hash，找到节点，返回获取节点getter
func (h *HTTPServer) PickPeer(key string) (PeerGetter, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if peer := h.peers.Get(key); peer != "" && peer != h.self {
		h.Log("Pick peer %s", peer)
		return h.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPServer)(nil)

//从节点获取value
func (g *httpGetter) Get(in *mproto.Request, out *mproto.Response) error {
	u := fmt.Sprintf("%v%v", g.baseURL, url.QueryEscape(in.GetKey()))

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

func ServerStart() {

}

var _ PeerGetter = (*httpGetter)(nil)
