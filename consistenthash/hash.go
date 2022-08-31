package consistenthash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

type Hash func(data []byte) uint32

type Map struct {
	hash     Hash
	replicas int
	keys     []int
	hashMap  map[int]string
	mu       *sync.RWMutex
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
		mu:       new(sync.RWMutex),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

func (m *Map) Add(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	hash := int(m.hash([]byte(key)))

	index := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[index%len(m.keys)]]
}

func (m *Map) Delete(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("deleted hash key is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.hashMap {
		if v == key {
			delete(m.hashMap, k)
			index := sort.Search(len(m.keys), func(i int) bool {
				return m.keys[i] == k
			})
			m.keys = append(m.keys[:index], m.keys[index+1:]...)
		}
	}
	sort.Ints(m.keys)
	return nil
}
