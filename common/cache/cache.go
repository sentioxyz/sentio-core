package cache

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Policy defines the eviction policy for the cache
type Policy int

const (
	PolicyLRU Policy = iota
	PolicyLFU
	PolicyTinyLFU
)

// Stats holds cache statistics
type Stats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Refreshes int64
	Errors    int64
	ItemCount int64
	LoadTime  int64 // Total time spent loading values (nanoseconds)
}

// HitRate returns the cache hit rate as a percentage
func (s *Stats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// entry represents a cache entry with metadata
// add field in entry[T]
type entry[T any] struct {
	key          string
	value        T
	item         Cacheable[T]
	expireTime   time.Time
	lastAccess   int64
	accessCount  int64
	refreshTimer *time.Timer
	loading      int32 // atomic flag
	ready        chan struct{}
	loadErr      error // stores load error to propagate to waiters
	mu           sync.RWMutex
}

// isExpired checks if the entry has expired
func (e *entry[T]) isExpired() bool {
	return time.Now().After(e.expireTime)
}

// touch updates the access metadata
func (e *entry[T]) touch() {
	atomic.StoreInt64(&e.lastAccess, time.Now().UnixNano())
	atomic.AddInt64(&e.accessCount, 1)
}

// Cache is a high-performance, thread-safe cache with TTL and auto-refresh capabilities
type Cache[T any] struct {
	entries map[string]*entry[T]
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc

	// Configuration
	maxSize         int
	policy          Policy
	cleanupInterval time.Duration

	// Statistics
	stats Stats

	// Channels for async operations
	refreshChan chan string
	evictChan   chan string
}

// Config holds cache configuration options
type Config struct {
	MaxSize         int           // Maximum number of entries (0 = unlimited)
	Policy          Policy        // Eviction policy
	CleanupInterval time.Duration // How often to run cleanup (default: 5 minutes)
	RefreshWorkers  int           // Number of refresh workers (default: runtime.NumCPU())
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxSize:         1000,
		Policy:          PolicyLRU,
		CleanupInterval: 5 * time.Minute,
		RefreshWorkers:  runtime.NumCPU(),
	}
}

// New creates a new Cache with the given configuration
func New[T any](ctx context.Context, config Config) *Cache[T] {
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}
	if config.RefreshWorkers <= 0 {
		config.RefreshWorkers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(ctx)
	cache := &Cache[T]{
		entries:         make(map[string]*entry[T]),
		ctx:             ctx,
		cancel:          cancel,
		maxSize:         config.MaxSize,
		policy:          config.Policy,
		cleanupInterval: config.CleanupInterval,
		refreshChan:     make(chan string, 1000),
		evictChan:       make(chan string, 1000),
	}

	// Start background workers
	go cache.cleanupWorker()
	for i := 0; i < config.RefreshWorkers; i++ {
		go cache.refreshWorker()
	}
	go cache.evictionWorker()

	return cache
}

// Read retrieves a value from the cache
func (c *Cache[T]) Read(key string) (T, bool) {
	c.mu.RLock()
	e, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&c.stats.Misses, 1)
		var zero T
		return zero, false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check if expired
	if e.isExpired() {
		atomic.AddInt64(&c.stats.Misses, 1)
		// Do not schedule eviction here to avoid racing with concurrent reloads.
		// Cleanup worker or explicit Delete will handle removal if needed.
		var zero T
		return zero, false
	}

	// Update access metadata
	e.touch()
	atomic.AddInt64(&c.stats.Hits, 1)

	return e.value, true
}

// Write stores a Cacheable item in the cache
func (c *Cache[T]) Write(item Cacheable[T]) error {
	key := item.Key()

	// Load the value
	start := time.Now()
	value, err := item.Reload(c.ctx)
	loadTime := time.Since(start).Nanoseconds()
	atomic.AddInt64(&c.stats.LoadTime, loadTime)

	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return fmt.Errorf("failed to initialize cache item: %w", err)
	}

	ttl := item.TTL()
	refreshInterval := item.RefreshInterval()

	e := &entry[T]{
		key:         key,
		value:       value,
		item:        item,
		expireTime:  time.Now().Add(ttl),
		lastAccess:  time.Now().UnixNano(),
		accessCount: 0,
	}

	// Set up refresh timer if needed
	if refreshInterval > 0 {
		e.refreshTimer = time.AfterFunc(refreshInterval, func() {
			select {
			case c.refreshChan <- key:
			default:
			}
		})
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up existing entry
	if existing, exists := c.entries[key]; exists {
		c.cleanupEntry(existing)
	} else {
		atomic.AddInt64(&c.stats.ItemCount, 1)
	}

	c.entries[key] = e

	// Check if we need to evict due to size limit
	if c.maxSize > 0 && len(c.entries) > c.maxSize {
		c.evictOldest()
	}

	return nil
}

// Get retrieves a value or loads it if not present (similar to Otter's GetOrLoad)
func (c *Cache[T]) Get(item Cacheable[T]) (T, error) {
	key := item.Key()

	// Try to get from cache first
	if value, found := c.Read(key); found {
		return value, nil
	}

	// Not found, try to load it
	return c.getOrLoad(item)
}

// getOrLoad implements single-flight loading to prevent duplicate loads
func (c *Cache[T]) getOrLoad(item Cacheable[T]) (T, error) {
	key := item.Key()

	for {
		c.mu.Lock()
		e, exists := c.entries[key]

		if exists {
			// Someone else loading? Wait.
			if atomic.LoadInt32(&e.loading) == 1 {
				e.mu.RLock()
				ready := e.ready
				e.mu.RUnlock()

				c.mu.Unlock()
				if ready != nil {
					<-ready
				}
				// After wake-up, retry from the beginning
				continue
			}

			// Check validity (need entry RLock for expireTime consistency).
			e.mu.RLock()
			notExpired := !e.isExpired()
			val := e.value
			loadErr := e.loadErr
			e.mu.RUnlock()

			// If there's a stored error, return it
			if loadErr != nil {
				c.mu.Unlock()
				var zero T
				return zero, loadErr
			}

			if notExpired {
				// Another goroutine populated it after our initial Read miss.
				c.mu.Unlock()
				e.touch() // maintain LRU/LFU metadata (do not increment hits again).
				return val, nil
			}

			// Entry exists but is expired. Try to atomically become the loader.
			// Use CAS to ensure only one goroutine becomes the loader
			if !atomic.CompareAndSwapInt32(&e.loading, 0, 1) {
				// Someone else just became the loader, retry
				c.mu.Unlock()
				continue
			}

			// We successfully became the loader for reload
			e.mu.Lock()
			if e.ready == nil {
				e.ready = make(chan struct{})
			}
			ready := e.ready
			e.loadErr = nil // Clear any previous error
			e.mu.Unlock()
			c.mu.Unlock()

			// Perform load
			return c.performLoad(e, ready, item)
		}

		// Entry doesn't exist, create it
		e = &entry[T]{
			key:     key,
			item:    item,
			loading: 1,
			ready:   make(chan struct{}),
		}
		ready := e.ready
		c.entries[key] = e
		atomic.AddInt64(&c.stats.ItemCount, 1)
		// Enforce max size if needed.
		if c.maxSize > 0 && len(c.entries) > c.maxSize {
			c.evictOldest()
		}
		c.mu.Unlock()

		// We created the entry, so we're the loader
		return c.performLoad(e, ready, item)
	}
}

// performLoad executes the actual load operation and updates the entry
func (c *Cache[T]) performLoad(e *entry[T], ready chan struct{}, item Cacheable[T]) (T, error) {
	key := item.Key()

	// Perform load outside locks.
	start := time.Now()
	value, err := item.Reload(c.ctx)
	loadTime := time.Since(start).Nanoseconds()
	atomic.AddInt64(&c.stats.LoadTime, loadTime)

	e.mu.Lock()
	if err != nil {
		// Store the error BEFORE clearing loading flag so waiters see it
		e.loadErr = err
		// Close ready channel to wake up waiters BEFORE clearing loading
		if ready != nil && e.ready == ready {
			close(ready)
			e.ready = nil
		}
		// Clear loading state AFTER storing error and waking waiters
		atomic.StoreInt32(&e.loading, 0)
		e.mu.Unlock()

		atomic.AddInt64(&c.stats.Errors, 1)

		var zero T
		return zero, err
	}

	// Clear any previous error
	e.loadErr = nil

	e.value = value
	e.item = item
	e.expireTime = time.Now().Add(item.TTL())
	e.lastAccess = time.Now().UnixNano()
	e.accessCount = 1

	// (Re)start refresh timer.
	if e.refreshTimer != nil {
		e.refreshTimer.Stop()
	}
	if ri := item.RefreshInterval(); ri > 0 {
		e.refreshTimer = time.AfterFunc(ri, func() {
			select {
			case c.refreshChan <- key:
			default:
			}
		})
	}

	// Clear loading state and wake up waiters.
	atomic.StoreInt32(&e.loading, 0)
	// Only close if this is still the same ready channel we created.
	if ready != nil && e.ready == ready {
		close(ready)
		e.ready = nil
	}
	e.mu.Unlock()

	return value, nil
}

// Delete removes an entry from the cache
func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, exists := c.entries[key]; exists {
		c.cleanupEntry(e)
		delete(c.entries, key)
		atomic.AddInt64(&c.stats.ItemCount, -1)
	}
}

// Clear removes all entries from the cache
func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, e := range c.entries {
		c.cleanupEntry(e)
	}
	c.entries = make(map[string]*entry[T])
	atomic.StoreInt64(&c.stats.ItemCount, 0)
}

// Stats returns a copy of the current cache statistics
func (c *Cache[T]) GetStats() Stats {
	return Stats{
		Hits:      atomic.LoadInt64(&c.stats.Hits),
		Misses:    atomic.LoadInt64(&c.stats.Misses),
		Evictions: atomic.LoadInt64(&c.stats.Evictions),
		Refreshes: atomic.LoadInt64(&c.stats.Refreshes),
		Errors:    atomic.LoadInt64(&c.stats.Errors),
		ItemCount: atomic.LoadInt64(&c.stats.ItemCount),
		LoadTime:  atomic.LoadInt64(&c.stats.LoadTime),
	}
}

// Size returns the number of entries in the cache
func (c *Cache[T]) Size() int {
	return int(atomic.LoadInt64(&c.stats.ItemCount))
}

// Close stops the cache and cleans up resources
func (c *Cache[T]) Close() {
	c.cancel()
	c.Clear()
	close(c.refreshChan)
	close(c.evictChan)
}

// cleanupEntry cleans up an entry's resources
func (c *Cache[T]) cleanupEntry(e *entry[T]) {
	if e.refreshTimer != nil {
		e.refreshTimer.Stop()
	}
}

// cleanupWorker periodically removes expired entries
func (c *Cache[T]) cleanupWorker() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.evictExpired()
		}
	}
}

// refreshWorker handles refresh requests
func (c *Cache[T]) refreshWorker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case key := <-c.refreshChan:
			c.refreshEntry(key)
		}
	}
}

// evictionWorker handles eviction requests
func (c *Cache[T]) evictionWorker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case key := <-c.evictChan:
			c.Delete(key)
		}
	}
}

// refreshEntry refreshes a specific cache entry
func (c *Cache[T]) refreshEntry(key string) {
	c.mu.RLock()
	e, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return
	}

	e.mu.RLock()
	item := e.item
	refreshInterval := item.RefreshInterval()
	e.mu.RUnlock()

	// Refresh the value
	start := time.Now()
	newValue, err := item.Reload(c.ctx)
	loadTime := time.Since(start).Nanoseconds()
	atomic.AddInt64(&c.stats.LoadTime, loadTime)

	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		// Schedule retry
		if refreshInterval > 0 {
			time.AfterFunc(refreshInterval, func() {
				select {
				case c.refreshChan <- key:
				default:
				}
			})
		}
		return
	}

	e.mu.Lock()
	e.value = newValue
	e.expireTime = time.Now().Add(item.TTL())
	e.mu.Unlock()

	atomic.AddInt64(&c.stats.Refreshes, 1)

	// Schedule next refresh
	if refreshInterval > 0 {
		e.refreshTimer = time.AfterFunc(refreshInterval, func() {
			select {
			case c.refreshChan <- key:
			default:
			}
		})
	}
}

// evictExpired removes all expired entries
func (c *Cache[T]) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, e := range c.entries {
		if now.After(e.expireTime) {
			c.cleanupEntry(e)
			delete(c.entries, key)
			atomic.AddInt64(&c.stats.ItemCount, -1)
			atomic.AddInt64(&c.stats.Evictions, 1)
		}
	}
}

// evictOldest removes the oldest entry based on the eviction policy
func (c *Cache[T]) evictOldest() {
	if len(c.entries) == 0 {
		return
	}

	var victimKey string
	var victimScore int64

	switch c.policy {
	case PolicyLRU:
		victimScore = time.Now().UnixNano()
		for key, e := range c.entries {
			if atomic.LoadInt64(&e.lastAccess) < victimScore {
				victimScore = atomic.LoadInt64(&e.lastAccess)
				victimKey = key
			}
		}
	case PolicyLFU:
		victimScore = int64(^uint64(0) >> 1) // Max int64
		for key, e := range c.entries {
			if atomic.LoadInt64(&e.accessCount) < victimScore {
				victimScore = atomic.LoadInt64(&e.accessCount)
				victimKey = key
			}
		}
	default: // PolicyTinyLFU - simplified version
		// For simplicity, fall back to LRU
		victimScore = time.Now().UnixNano()
		for key, e := range c.entries {
			if atomic.LoadInt64(&e.lastAccess) < victimScore {
				victimScore = atomic.LoadInt64(&e.lastAccess)
				victimKey = key
			}
		}
	}

	if victimKey != "" {
		if e, exists := c.entries[victimKey]; exists {
			c.cleanupEntry(e)
			delete(c.entries, victimKey)
			atomic.AddInt64(&c.stats.ItemCount, -1)
			atomic.AddInt64(&c.stats.Evictions, 1)
		}
	}
}
