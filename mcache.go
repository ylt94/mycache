package mcache

import (
	"errors"
	"log"
	"sync"

	"github.com/ylt94/mcache/proto"
	"github.com/ylt94/mcache/singleflight"
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
	mainCache cache
	peers     PeerPicker
	loader    *singleflight.Group
}

var (
	//TODO 主从切换
	mu sync.RWMutex
	//groups = make(map[string]*Group)
	mainCache *mcache
)

func NewMCache(id string, cacheBytes uint64, getter Getter) *mcache {
	g := &mcache{
		id:        id,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
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

func (g *mcache) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, errors.New("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[mcache] hit")
		return v, nil
	}

	//从其他节点获取
	return g.load(key)
}

func (g *mcache) load(key string) (value ByteView, err error) {
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err := g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[mCache] Failed to get from peer:", err)
			}
		}
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

	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *mcache) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
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
