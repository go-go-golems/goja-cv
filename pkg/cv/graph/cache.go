package graph

import (
	"container/list"
	"sync"
)

// Signature is an evaluation cache key: node hash + quantized scale + the
// values of the params the node depends on (through its inputs).
type Signature [32]byte

// Cache is a byte-capped LRU of evaluated values.
type Cache struct {
	mu        sync.Mutex
	capBytes  int64
	used      int64
	ll        *list.List
	items     map[Signature]*list.Element
	hits      int64
	misses    int64
	evictions int64
}

type cacheEntry struct {
	key   Signature
	val   Value
	bytes int64
}

// CacheStats is a snapshot of cache counters.
type CacheStats struct {
	Entries   int   `json:"entries"`
	Bytes     int64 `json:"bytes"`
	CapBytes  int64 `json:"capBytes"`
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
}

// NewCache returns a cache holding at most capBytes of values.
func NewCache(capBytes int64) *Cache {
	return &Cache{capBytes: capBytes, ll: list.New(), items: map[Signature]*list.Element{}}
}

// Get returns a cached value and marks it recently used.
func (c *Cache) Get(k Signature) (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		c.ll.MoveToFront(el)
		c.hits++
		return el.Value.(*cacheEntry).val, true
	}
	c.misses++
	return nil, false
}

// Contains reports presence without touching recency or counters.
func (c *Cache) Contains(k Signature) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[k]
	return ok
}

// Put stores v, evicting least recently used entries until the total fits.
// Values larger than the whole cache are not stored.
func (c *Cache) Put(k Signature, v Value) {
	n := v.Bytes()
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > c.capBytes {
		return
	}
	if el, ok := c.items[k]; ok {
		e := el.Value.(*cacheEntry)
		c.used += n - e.bytes
		e.val, e.bytes = v, n
		c.ll.MoveToFront(el)
	} else {
		c.items[k] = c.ll.PushFront(&cacheEntry{key: k, val: v, bytes: n})
		c.used += n
	}
	for c.used > c.capBytes {
		el := c.ll.Back()
		e := el.Value.(*cacheEntry)
		c.ll.Remove(el)
		delete(c.items, e.key)
		c.used -= e.bytes
		c.evictions++
	}
}

// Stats returns a snapshot of the counters.
func (c *Cache) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{Entries: len(c.items), Bytes: c.used, CapBytes: c.capBytes, Hits: c.hits, Misses: c.misses, Evictions: c.evictions}
}

// Clear drops every entry.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	c.items = map[Signature]*list.Element{}
	c.used = 0
}
