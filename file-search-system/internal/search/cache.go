package search

import (
	"sync"
	"time"
)

// Cache implements a simple in-memory cache for search results
type Cache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	ttl     time.Duration
	maxSize int
}

type cacheItem struct {
	response     *Response
	expiresAt    time.Time
	accessCount  int
	lastAccessed time.Time
}

// NewCache creates a new search cache
func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		items:   make(map[string]*cacheItem),
		ttl:     ttl,
		maxSize: 1000, // Maximum number of cached queries
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached search response
func (c *Cache) Get(key string) *Response {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(item.expiresAt) {
		c.Delete(key)
		return nil
	}

	// Update access statistics
	c.mu.Lock()
	item.accessCount++
	item.lastAccessed = time.Now()
	c.mu.Unlock()

	// Return a copy to prevent mutation
	responseCopy := *item.response
	return &responseCopy
}

// Set stores a search response in the cache
func (c *Cache) Set(key string, response *Response) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache size and evict if necessary
	if len(c.items) >= c.maxSize {
		c.evictLRU()
	}

	c.items[key] = &cacheItem{
		response:     response,
		expiresAt:    time.Now().Add(c.ttl),
		accessCount:  1,
		lastAccessed: time.Now(),
	}
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// Size returns the number of items in the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// evictLRU removes the least recently used item
func (c *Cache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.lastAccessed.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.lastAccessed
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// cleanupLoop periodically removes expired items
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		Size:       len(c.items),
		MaxSize:    c.maxSize,
		TTL:        c.ttl,
		TotalHits:  0,
		AvgHitRate: 0,
	}

	totalAccess := 0
	for _, item := range c.items {
		totalAccess += item.accessCount
		stats.TotalHits += item.accessCount - 1 // Subtract initial set
	}

	if len(c.items) > 0 {
		stats.AvgHitRate = float64(stats.TotalHits) / float64(len(c.items))
	}

	return stats
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size       int           `json:"size"`
	MaxSize    int           `json:"max_size"`
	TTL        time.Duration `json:"ttl"`
	TotalHits  int           `json:"total_hits"`
	AvgHitRate float64       `json:"avg_hit_rate"`
}
