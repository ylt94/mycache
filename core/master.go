package core

import (
	"errors"
	"fmt"
	"github.com/ylt94/mycache/consistenthash"
	"net/http"
	"strings"
	"sync"
)
type service struct {
	addr string
	mserver *master
}
type master struct {
	addr string
	nodes map[string]*mcache
	mu sync.RWMutex
	hash *consistenthash.Map
}

func NewService(addr string, mserver *master) *service {
	return &service{
		addr: addr,
		mserver: mserver,
	}
}

func NewMaster(addr string) *master{
	return &master{
		addr: addr,
		hash: consistenthash.New(1, nil),
	}
}

func (m *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	action := values.Get("action")
	key    := values.Get("key")
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
	if strings.ToLower(action) == "set" {

	} else if strings.ToLower(action) == "get" {

	} else if strings.ToLower(action) == "del" {
	}

}

func (m *master) loadNode(name string, cache *mcache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hash.Add(name)
	if m.nodes == nil {
		m.nodes = make(map[string]*mcache)
	}
	m.nodes[name] = cache
}

func (m *master) getNode(key string) (*mcache, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name := m.hash.Get(key)
	if cache, ok := m.nodes[name]; ok {
		return cache,nil
	}
	return nil, errors.New("no such cache node:" + name)
}

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
	cache := NewMCache(name, 2 << 10, nil)
	m.loadNode(name, cache)
	//TODO 心跳检测
	w.Write([]byte("success"))
}