package progszy

import (
	"io"
	"sync"
)

// MemCache is a simple memory-based cache of HTTP responses, keyed by URL.
type MemCache struct {
	mu          sync.RWMutex
	recordByURL map[string]*cacheRecord
}

// NewMemCache initialises and returns a new MemCache.
func NewMemCache() *MemCache {
	c := MemCache{
		recordByURL: make(map[string]*cacheRecord),
	}
	return &c
}

// Get the cached response for the given URL.
// If the given URL does not exist in the cache,
// error ErrCacheMiss is returned.
func (c *MemCache) Get(uri string) (string, io.ReadCloser, error) {
	nurl, _, err := cacheRecordKey(uri)
	if err != nil {
		return "", nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.recordByURL[nurl]
	if !ok {
		return "", nil, ErrCacheMiss
	}
	return r.ContentType, r.Body(), nil
}

// Put adds the given URL/response pair to the cache.
func (c *MemCache) Put(uri, mime, etag, lastMod string, b []byte, responseTime float64) error {
	r, err := newCacheRecord(uri, mime, etag, lastMod, b, responseTime)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordByURL[r.Key] = r
	return nil
}

func (c *MemCache) CloseAll() error {
	return nil
}
