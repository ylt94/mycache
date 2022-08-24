package core

import (
	"sync"

	"github.com/ylt94/mycache/lru"
)

type cache struct {
	mu         sync.RWMutex
	lru        *lru.Cache
	cacheBytes uint64
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}


func (c *cache) del(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lru == nil {
		return nil
	}

	if _, err := c.lru.Del(key); err != nil {
		return err
	}
	return nil
}