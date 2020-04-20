package core

import (
	"sync"
	"time"
)

type cacheItem struct {
	value        interface{}
	lastAccessed int64
}

// LCache ...
type LCache struct {
	m          map[string]*cacheItem
	l          sync.Mutex
	ttl        int64
	maxItem    int
	refreshTTL bool
}

// NewLCache ...
// maxItem: maximum item
// ttl: time to live (second)
func NewLCache(maxItem int, ttl int) (m *LCache) {
	m = NewLCacheRefreshMode(maxItem, ttl, true)
	return
}

// NewLCacheRefreshMode ...
// maxItem: maximum item
// ttl: time to live (second)
// refreshTTL : if refreshTTL = true, when someone access item, ttl of item will be refresh
func NewLCacheRefreshMode(maxItem int, ttl int, refreshTTL bool) (m *LCache) {
	m = &LCache{m: make(map[string]*cacheItem, maxItem), maxItem: maxItem, refreshTTL: refreshTTL}
	go func() {
		for now := range time.Tick(time.Second) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Unix()-v.lastAccessed > int64(ttl) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
	return
}

// Len ...
func (c *LCache) Len() int {
	return len(c.m)
}

// Put ...
func (c *LCache) Put(k string, v interface{}) {
	c.l.Lock()
	it, ok := c.m[k]
	if !ok {
		it = &cacheItem{value: v}
		c.m[k] = it
	}
	it.lastAccessed = time.Now().Unix()
	c.l.Unlock()
}

// Get ...
func (c *LCache) Get(k string) (v interface{}, ok bool) {
	c.l.Lock()
	if it, okv := c.m[k]; okv {
		ok = okv
		v = it.value
		if c.refreshTTL {
			it.lastAccessed = time.Now().Unix()
		}
	}
	c.l.Unlock()
	return
}

// ContainsKey ...
func (c *LCache) ContainsKey(k string) (ok bool) {
	c.l.Lock()
	it, ok := c.m[k]
	if ok {
		if c.refreshTTL {
			it.lastAccessed = time.Now().Unix()
		}
	}
	c.l.Unlock()
	return
}

// Remove ...
func (c *LCache) Remove(k string) {
	c.l.Lock()
	if _, ok := c.m[k]; ok {
		delete(c.m, k)
	}
	c.l.Unlock()
	return
}

// Cleanup Remove all data
func (c *LCache) Cleanup() {
	c.l.Lock()
	c.m = make(map[string]*cacheItem, c.maxItem)
	c.l.Unlock()
	return
}
