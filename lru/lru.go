package lru

import (
	"container/list"
	"fmt"
)

//底层数据存储
type Cache struct {
	maxBytes  int64
	usedBytes int64
	list      *list.List
	data      map[string]*list.Element
	onRemoved func(key string, value string)
}

type entry struct {
	key   string
	value Value
}

type Value interface {
	Len() int
}

func New(maxBytes int64, onRemoved func(key string, value string)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		list:      list.New(),
		data:      make(map[string]*list.Element),
		onRemoved: onRemoved,
	}
}

func (c *Cache) Add(key string, value Value) {
	v := &entry{key: key, value: value}
	if e, ok := c.data[key]; ok {
		//更新
		if e.Value != v {
			kv := e.Value.(*entry)
			//已用内存计算
			c.usedBytes += int64(value.Len() - kv.value.Len())
			e.Value = v
		}
		//lru策略
		c.list.MoveToBack(e)
	} else {
		elemet := c.list.PushBack(v)
		c.data[key] = elemet
		c.usedBytes += int64(len(key) + value.Len())
	}
	//内存不足清理数据
	if c.maxBytes != 0 && c.usedBytes > c.maxBytes {
		c.clearOld()
	}
}

func (c *Cache) Get(key string) (value Value, ok bool) {
	if e, ok := c.data[key]; ok {
		c.list.MoveToBack(e)
		kv := e.Value.(*entry)
		return kv.value, ok
	}
	return nil, ok
}

func (c *Cache) Del(key string) (bool, error) {
	e, ok := c.data[key]
	if !ok {
		return true, fmt.Errorf("data not exists")
	}
	//删除信息
	delete(c.data, key)
	c.list.Remove(e)

	//计算使用内存
	kv := e.Value.(*entry)
	c.usedBytes -= int64(len(kv.key) + kv.value.Len())
	return true, nil
}

func (c *Cache) clearOld() {
	if c.maxBytes == 0 || c.usedBytes == 0 {
		return
	}
	for c.maxBytes < c.usedBytes {
		e := c.list.Front()
		if e == nil {
			panic("usedBytes is not empty but list is nil")
		}
		kv := e.Value.(*entry)
		delete(c.data, kv.key)
		c.list.Remove(e)
		c.usedBytes -= int64(len(kv.key) + kv.value.Len())
	}
}
