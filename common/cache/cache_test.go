package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
)

// TestCacheable implements the Cacheable interface for testing
type TestCacheable struct {
	key             string
	value           string
	ttl             time.Duration
	refreshInterval time.Duration
	initFunc        func(ctx context.Context) (string, error)
}

func (tc *TestCacheable) Key() string {
	return tc.key
}

func (tc *TestCacheable) TTL() time.Duration {
	return tc.ttl
}

func (tc *TestCacheable) RefreshInterval() time.Duration {
	return tc.refreshInterval
}

func (tc *TestCacheable) Reload(ctx context.Context) (string, error) {
	if tc.initFunc != nil {
		return tc.initFunc(ctx)
	}
	return tc.value, nil
}

func TestCacheBasicOperations(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	// Test Write and Read
	item := &TestCacheable{
		key:   "test-key",
		value: "test-value",
		ttl:   time.Hour,
	}

	err := cache.Write(item)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	value, found := cache.Read("test-key")
	if !found {
		t.Fatal("Expected to find value")
	}
	if value != "test-value" {
		t.Fatalf("Expected 'test-value', got '%s'", value)
	}

	// Test stats
	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Fatalf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.ItemCount != 1 {
		t.Fatalf("Expected 1 item, got %d", stats.ItemCount)
	}
}

func TestCacheGet(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	item := &TestCacheable{
		key:   "get-key",
		value: "get-value",
		ttl:   time.Hour,
	}

	// Test Get (should load the value)
	value, err := cache.Get(item)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "get-value" {
		t.Fatalf("Expected 'get-value', got '%s'", value)
	}

	// Test Get again (should hit cache)
	value, err = cache.Get(item)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "get-value" {
		t.Fatalf("Expected 'get-value', got '%s'", value)
	}

	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Fatalf("Expected 1 hit, got %d", stats.Hits)
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	item := &TestCacheable{
		key:   "expire-key",
		value: "expire-value",
		ttl:   100 * time.Millisecond,
	}

	err := cache.Write(item)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should be found immediately
	_, found := cache.Read("expire-key")
	if !found {
		t.Fatal("Expected to find value")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not be found after expiration
	_, found = cache.Read("expire-key")
	if found {
		t.Fatal("Expected value to be expired")
	}

	stats := cache.GetStats()
	if stats.Misses != 1 {
		t.Fatalf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestCacheRefresh(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	counter := 0
	item := &TestCacheable{
		key:             "refresh-key",
		ttl:             time.Hour,
		refreshInterval: time.Second * 2,
		initFunc: func(_ context.Context) (string, error) {
			counter++
			return "value-" + string(rune('0'+counter)), nil
		},
	}

	err := cache.Write(item)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Initial value should be "value-1"
	value, found := cache.Read("refresh-key")
	if !found {
		t.Fatal("Expected to find value")
	}
	if value != "value-1" {
		t.Fatalf("Expected 'value-1', got '%s'", value)
	}

	// Wait for refresh
	time.Sleep(time.Second * 3)

	// Value should be refreshed to "value-2"
	value, found = cache.Read("refresh-key")
	if !found {
		t.Fatal("Expected to find value")
	}
	if value != "value-2" {
		t.Fatalf("Expected 'value-2', got '%s'", value)
	}

	stats := cache.GetStats()
	if stats.Refreshes == 0 {
		t.Fatal("Expected at least 1 refresh")
	}
}

func TestCacheEviction(t *testing.T) {
	config := DefaultConfig()
	config.MaxSize = 2
	cache := New[string](context.Background(), config)
	defer cache.Close()

	// Add first item
	item1 := &TestCacheable{
		key:   "key1",
		value: "value1",
		ttl:   time.Hour,
	}
	err := cache.Write(item1)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Add second item
	item2 := &TestCacheable{
		key:   "key2",
		value: "value2",
		ttl:   time.Hour,
	}
	err = cache.Write(item2)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Add third item (should evict first)
	item3 := &TestCacheable{
		key:   "key3",
		value: "value3",
		ttl:   time.Hour,
	}
	err = cache.Write(item3)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// First item should be evicted
	_, found := cache.Read("key1")
	if found {
		t.Fatal("Expected key1 to be evicted")
	}

	// Second and third should still exist
	_, found = cache.Read("key2")
	if !found {
		t.Fatal("Expected to find key2")
	}

	_, found = cache.Read("key3")
	if !found {
		t.Fatal("Expected to find key3")
	}

	if cache.Size() != 2 {
		t.Fatalf("Expected size 2, got %d", cache.Size())
	}
}

func TestCacheError(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	item := &TestCacheable{
		key: "error-key",
		ttl: time.Hour,
		initFunc: func(_ context.Context) (string, error) {
			return "", errors.New("init error")
		},
	}

	err := cache.Write(item)
	if err == nil {
		t.Fatal("Expected Write to fail")
	}

	_, err = cache.Get(item)
	if err == nil {
		t.Fatal("Expected Get to fail")
	}

	stats := cache.GetStats()
	if stats.Errors == 0 {
		t.Fatal("Expected at least 1 error")
	}
}

func TestCacheDelete(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	item := &TestCacheable{
		key:   "delete-key",
		value: "delete-value",
		ttl:   time.Hour,
	}

	err := cache.Write(item)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Should be found
	_, found := cache.Read("delete-key")
	if !found {
		t.Fatal("Expected to find value")
	}

	// Delete
	cache.Delete("delete-key")

	// Should not be found
	_, found = cache.Read("delete-key")
	if found {
		t.Fatal("Expected value to be deleted")
	}

	if cache.Size() != 0 {
		t.Fatalf("Expected size 0, got %d", cache.Size())
	}
}

func TestCacheClear(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	// Add multiple items
	for i := 0; i < 5; i++ {
		item := &TestCacheable{
			key:   "key" + string(rune('0'+i)),
			value: "value" + string(rune('0'+i)),
			ttl:   time.Hour,
		}
		err := cache.Write(item)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	if cache.Size() != 5 {
		t.Fatalf("Expected size 5, got %d", cache.Size())
	}

	// Clear
	cache.Clear()

	if cache.Size() != 0 {
		t.Fatalf("Expected size 0 after clear, got %d", cache.Size())
	}
}

func TestCacheStats(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	item := &TestCacheable{
		key:   "stats-key",
		value: "stats-value",
		ttl:   time.Hour,
	}

	// Write
	err := cache.Write(item)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Hit
	_, found := cache.Read("stats-key")
	if !found {
		t.Fatal("Expected to find value")
	}

	// Miss
	_, found = cache.Read("nonexistent-key")
	if found {
		t.Fatal("Expected not to find value")
	}

	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Fatalf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.ItemCount != 1 {
		t.Fatalf("Expected 1 item, got %d", stats.ItemCount)
	}

	hitRate := stats.HitRate()
	if hitRate != 50.0 {
		t.Fatalf("Expected 50%% hit rate, got %.2f%%", hitRate)
	}
	log.Infof("Hit rate: %f%%", hitRate)
}

func TestCacheConcurrentGetOrLoad(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	loadCount := 0
	var loadMutex sync.Mutex

	item := &TestCacheable{
		key: "concurrent-key",
		ttl: time.Hour,
		initFunc: func(_ context.Context) (string, error) {
			loadMutex.Lock()
			loadCount++
			loadMutex.Unlock()
			// Simulate slow load
			time.Sleep(100 * time.Millisecond)
			return "concurrent-value", nil
		},
	}

	// Launch multiple goroutines trying to Get the same key
	const numGoroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cache.Get(item)
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}

	wg.Wait()
	close(errs)
	close(results)

	// Check for errors
	for err := range errs {
		t.Fatalf("Get failed: %v", err)
	}

	// Check all results are correct
	resultCount := 0
	for value := range results {
		resultCount++
		if value != "concurrent-value" {
			t.Fatalf("Expected 'concurrent-value', got '%s'", value)
		}
	}

	if resultCount != numGoroutines {
		t.Fatalf("Expected %d results, got %d", numGoroutines, resultCount)
	}

	// The key should have been loaded only once due to single-flight
	loadMutex.Lock()
	finalLoadCount := loadCount
	loadMutex.Unlock()

	if finalLoadCount != 1 {
		t.Fatalf("Expected load to be called once, but was called %d times", finalLoadCount)
	}

	// Verify cache stats
	stats := cache.GetStats()
	if stats.ItemCount != 1 {
		t.Fatalf("Expected 1 item in cache, got %d", stats.ItemCount)
	}
}

func TestCacheConcurrentExpiredReload(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	loadCount := 0
	var loadMutex sync.Mutex

	item := &TestCacheable{
		key: "expired-key",
		ttl: 50 * time.Millisecond,
		initFunc: func(_ context.Context) (string, error) {
			loadMutex.Lock()
			count := loadCount
			loadCount++
			loadMutex.Unlock()
			// Simulate slow load
			time.Sleep(100 * time.Millisecond)
			return fmt.Sprintf("value-%d", count), nil
		},
	}

	// First load
	value, err := cache.Get(item)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value-0" {
		t.Fatalf("Expected 'value-0', got '%s'", value)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Launch multiple goroutines trying to Get the expired key
	const numGoroutines = 30
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := cache.Get(item)
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}

	wg.Wait()
	close(errs)
	close(results)

	// Check for errors
	for err := range errs {
		t.Fatalf("Get failed: %v", err)
	}

	// Check all results are the reloaded value
	resultCount := 0
	for value := range results {
		resultCount++
		if value != "value-1" {
			t.Fatalf("Expected 'value-1', got '%s'", value)
		}
	}

	if resultCount != numGoroutines {
		t.Fatalf("Expected %d results, got %d", numGoroutines, resultCount)
	}

	// The reload should have been called only once
	loadMutex.Lock()
	finalLoadCount := loadCount
	loadMutex.Unlock()

	if finalLoadCount != 2 {
		t.Fatalf("Expected load to be called twice (initial + reload), but was called %d times", finalLoadCount)
	}
}

func TestCacheConcurrentGetOrLoadWithErrors(t *testing.T) {
	cache := New[string](context.Background(), DefaultConfig())
	defer cache.Close()

	loadCount := 0
	var loadMutex sync.Mutex

	item := &TestCacheable{
		key: "error-concurrent-key",
		ttl: time.Hour,
		initFunc: func(_ context.Context) (string, error) {
			loadMutex.Lock()
			loadCount++
			loadMutex.Unlock()
			// Simulate slow load that fails
			time.Sleep(50 * time.Millisecond)
			return "", errors.New("load failed")
		},
	}

	// Launch multiple goroutines trying to Get the same key
	const numGoroutines = 20
	var wg sync.WaitGroup
	errorCount := 0
	var errorMutex sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.Get(item)
			if err != nil {
				errorMutex.Lock()
				errorCount++
				errorMutex.Unlock()
			}
		}()
	}

	wg.Wait()

	// All should have received errors
	if errorCount != numGoroutines {
		t.Fatalf("Expected %d errors, got %d", numGoroutines, errorCount)
	}

	// The load should have been called only once
	loadMutex.Lock()
	finalLoadCount := loadCount
	loadMutex.Unlock()

	if finalLoadCount != 1 {
		t.Fatalf("Expected load to be called once, but was called %d times", finalLoadCount)
	}
}
