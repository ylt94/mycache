package core

import (
	"errors"
	"log"

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
	loader    *singleflight.Group
}

var (
//TODO 主从切换
//mu sync.RWMutex
//groups = make(map[string]*Group)
)

func NewMCache(id string, cacheBytes int64, getter Getter) *mcache {
	g := &mcache{
		id:        id,
		getter:    getter,
		baseCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}
	return g
}

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
}

func (g *mcache) Set(key string, value string) error {
	if key == "" {
		return errors.New("key is required")
	}

	val := ByteView{b: []byte(value)}
	g.baseCache.add(key, val)
	return nil
}

func (g *mcache) Del(key string) error {
	if key == "" {
		return errors.New("key is required")
	}

	if err := g.baseCache.del(key); err != nil {
		return err
	}
	return nil
}
