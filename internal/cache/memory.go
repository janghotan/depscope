// Package cache 提供有界、带过期时间的进程内缓存。
package cache

import (
	"context"
	"sync"
	"time"
)

const defaultMaxEntries = 256

type entry struct {
	value     []byte
	expiresAt time.Time
}

// MemoryCache 是线程安全的有界 TTL 缓存。
// ttl 小于等于零时不会写入条目。
type MemoryCache struct {
	mu         sync.Mutex
	entries    map[string]entry
	maxEntries int
	now        func() time.Time
}

// NewMemoryCache 创建缓存；maxEntries 小于等于零时使用默认上限。
func NewMemoryCache(maxEntries int, now func() time.Time) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	if now == nil {
		now = time.Now
	}
	return &MemoryCache{
		entries:    make(map[string]entry),
		maxEntries: maxEntries,
		now:        now,
	}
}

// Get 返回未过期的缓存值；过期条目会被删除。
func (c *MemoryCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if c == nil {
		return nil, false, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	item, exists := c.entries[key]
	if !exists {
		return nil, false, nil
	}
	if !c.now().Before(item.expiresAt) {
		delete(c.entries, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

// Set 写入条目并执行有界淘汰；ttl 小于等于零时不写入。
func (c *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || ttl <= 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
		c.evictLocked()
	}
	c.entries[key] = entry{
		value:     append([]byte(nil), value...),
		expiresAt: c.now().Add(ttl),
	}
	return nil
}

// Len 返回当前保留的条目数量，主要用于测试和诊断。
func (c *MemoryCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// evictLocked 优先删除过期条目，否则删除最早到期的条目。
func (c *MemoryCache) evictLocked() {
	now := c.now()
	for key, item := range c.entries {
		if !now.Before(item.expiresAt) {
			delete(c.entries, key)
		}
	}
	if len(c.entries) < c.maxEntries {
		return
	}

	var victim string
	var victimExpiry time.Time
	for key, item := range c.entries {
		if victim == "" || item.expiresAt.Before(victimExpiry) ||
			(item.expiresAt.Equal(victimExpiry) && key < victim) {
			victim = key
			victimExpiry = item.expiresAt
		}
	}
	if victim != "" {
		delete(c.entries, victim)
	}
}
