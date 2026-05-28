package cache

import (
	"sync"
	"time"
)

// MetricsCache is a simple TTL cache for a single metrics payload.
// Prevents N browser tabs from all hammering the Temporal Cloud metrics
// endpoint and CloudWatch simultaneously.
type MetricsCache struct {
	mu        sync.Mutex
	data      []byte
	expiresAt time.Time
	ttl       time.Duration
}

func NewMetricsCache(ttl time.Duration) *MetricsCache {
	return &MetricsCache{ttl: ttl}
}

// Get returns cached data and true if the cache is still valid.
func (c *MetricsCache) Get() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().Before(c.expiresAt) {
		return c.data, true
	}
	return nil, false
}

// Set stores data in the cache and resets the TTL.
func (c *MetricsCache) Set(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = data
	c.expiresAt = time.Now().Add(c.ttl)
}
