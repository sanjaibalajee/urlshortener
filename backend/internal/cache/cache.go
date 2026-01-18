package cache

import (
	"container/list"
	"sync"
	"time"
)

// entry represents a cache entry with expiration
type entry[K comparable, V any] struct {
	key       K
	value     V
	expiresAt time.Time
}

// LRU is a generic thread-safe LRU cache with TTL support
type LRU[K comparable, V any] struct {
	capacity int
	ttl      time.Duration
	mu       sync.RWMutex
	items    map[K]*list.Element
	order    *list.List
}

// NewLRU creates a new LRU cache with the given capacity and TTL
func NewLRU[K comparable, V any](capacity int, ttl time.Duration) *LRU[K, V] {
	return &LRU[K, V]{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[K]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves a value from the cache
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero V
	elem, exists := c.items[key]
	if !exists {
		return zero, false
	}

	ent := elem.Value.(*entry[K, V])

	// Check if expired
	if time.Now().After(ent.expiresAt) {
		c.removeElement(elem)
		return zero, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	return ent.value, true
}

// Set adds or updates a value in the cache
func (c *LRU[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, exists := c.items[key]; exists {
		c.order.MoveToFront(elem)
		ent := elem.Value.(*entry[K, V])
		ent.value = value
		ent.expiresAt = time.Now().Add(c.ttl)
		return
	}

	// Evict if at capacity
	if c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	// Add new entry
	ent := &entry[K, V]{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	elem := c.order.PushFront(ent)
	c.items[key] = elem
}

// Delete removes a key from the cache
func (c *LRU[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.removeElement(elem)
	}
}

// Len returns the number of items in the cache
func (c *LRU[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// evictOldest removes the least recently used entry
func (c *LRU[K, V]) evictOldest() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes an element from the cache
func (c *LRU[K, V]) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	ent := elem.Value.(*entry[K, V])
	delete(c.items, ent.key)
}
