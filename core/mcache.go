package core

import (
	"errors"
	"log"
	"sync"

	"github.com/ylt94/mycache/proto"
	"github.com/ylt94/mycache/singleflight"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

//用户自定义回调方法
type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type mcache struct {
	id        string
	getter    Getter
	baseCache cache
	peers     PeerPicker
	loader    *singleflight.Group
}

var (
	//TODO 主从切换
	mu sync.RWMutex
	//groups = make(map[string]*Group)
	mainCache *mcache
)

func NewMCache(id string, cacheBytes int64, getter Getter) *mcache {
	g := &mcache{
		id:        id,
		getter:    getter,
		baseCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	//groups[name] = g
	mainCache = g
	return g
}

// func GetGroup(name string) (*Group, bool) {
// 	mu.RLock()
// 	g, ok := groups[name]
// 	mu.RUnlock()
// 	return g, ok
// }

func (g *mcache) Get(key string) ([]byte, error) {
	if key == "" {
		return make([]byte, 0), errors.New("key is required")
	}

	//从底层获取
	if v, ok := g.baseCache.get(key); ok {
		log.Println("[mcache] hit")
		return v.ByteSlice(), nil
	}
	return make([]byte, 0), nil
	//从其他节点获取
	//return g.load(key)
}

func (g *mcache) Set(key string, value string) error {
	if key == "" {
		return errors.New("key is required")
	}

	val := ByteView{b: []byte(value)}
	g.baseCache.add(key, val)
	return nil
	//从其他节点获取
	//return g.load(key)
}

func (g *mcache) Del(key string) error {
	if key == "" {
		return errors.New("key is required")
	}

	if err := g.baseCache.del(key); err != nil {
		return err
	}
	return nil
	//从其他节点获取
	//return g.load(key)
}

func (g *mcache) load(key string) (value ByteView, err error) {
	//从其他节点获取
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err := g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[mCache] Failed to get from peer:", err)
				return value, nil
			}
		}
		//如果其他节点也没有，返回自定义的结果
		return g.getLocally(key)
	})
	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

func (g *mcache) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	if len(bytes) == 0 {
		return ByteView{}, nil
	}

	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *mcache) populateCache(key string, value ByteView) {
	g.baseCache.add(key, value)
}

//注册http-pool
func (g *mcache) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

//从其他节点获取数据
func (g *mcache) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &proto.Request{
		Key: key,
	}
	rsp := &proto.Response{}
	err := peer.Get(req, rsp)
	if err != nil {
		return ByteView{}, err
	}

	return ByteView{b: rsp.Value}, nil
}
