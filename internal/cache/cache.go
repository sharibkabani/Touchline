package cache

import (
	"sync"
	"time"
)

type item[T any] struct {
	value     T
	expiresAt time.Time
}

type Cache[T any] struct {
	mu    sync.RWMutex
	items map[string]item[T]
	ttl   time.Duration
	now   func() time.Time
}

func New[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		items: make(map[string]item[T]),
		ttl:   ttl,
		now:   time.Now,
	}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	cached, ok := c.items[key]
	c.mu.RUnlock()

	var zero T
	if !ok {
		return zero, false
	}

	if !cached.expiresAt.IsZero() && c.now().After(cached.expiresAt) {
		c.Delete(key)
		return zero, false
	}

	return cached.value, true
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = item[T]{
		value:     value,
		expiresAt: c.now().Add(c.ttl),
	}
}

func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]item[T])
}
