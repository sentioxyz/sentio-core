package wasm

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
)

// processRSSBytes returns the current resident set size (RSS) of the process in bytes.
// It uses `ps` which is available on both macOS and Linux.
func processRSSBytes() (int64, error) {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return 0, fmt.Errorf("ps failed: %w", err)
	}
	rssKB, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse rss: %w", err)
	}
	return rssKB * 1024, nil
}

// Test_storeAndModuleReuse verifies that Store and Module objects are created only once
// and reused across reset() calls. Only the Instance is recreated on each reset.
// Reusing the Store avoids store-level memory pool churn; reusing the Module avoids
// redundant JIT recompilation of the Wasm bytecode.
func Test_storeAndModuleReuse(t *testing.T) {
	inst := newTestInst("testInst")
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))
	assert.Same(t, globalEngine, inst.store.Engine, "store must use globalEngine")

	store0 := inst.store
	module0 := inst.module
	instance0 := inst.instance

	assert.NoError(t, inst.reset(log.With()))
	assert.Same(t, store0, inst.store, "store must be reused after reset")
	assert.Same(t, module0, inst.module, "module must be reused after reset")
	assert.NotSame(t, instance0, inst.instance, "instance must be recreated after reset")

	store1 := inst.store
	module1 := inst.module

	assert.NoError(t, inst.reset(log.With()))
	assert.Same(t, store1, inst.store, "store must be reused after second reset")
	assert.Same(t, module1, inst.module, "module must be reused after second reset")
}

// Test_resetDoesNotLeakMemory verifies that repeated reset() calls do not cause the process
// heap to grow without bound. Before the fix, each reset() created a new wasmer.Engine
// whose C resources were only freed by a GC finalizer, causing unbounded accumulation.
// After the fix (global engine), the Go heap growth across many resets should be negligible.
func Test_resetDoesNotLeakMemory(t *testing.T) {
	const numResets = 30

	inst := newTestInst("testInst")
	defer inst.Close()
	assert.NoError(t, inst.Init(log.With()))

	// Force a full GC cycle and measure baseline heap.
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < numResets; i++ {
		assert.NoError(t, inst.reset(log.With()))
	}

	// Give finalizers a chance to run, then measure again.
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// HeapInuse should not grow significantly. Allow a generous 20 MB buffer to
	// account for test framework allocations and normal GC fluctuations.
	const maxGrowthBytes = 20 * 1024 * 1024
	heapGrowth := int64(after.HeapInuse) - int64(before.HeapInuse)
	t.Logf("HeapInuse before=%d after=%d growth=%d bytes across %d resets",
		before.HeapInuse, after.HeapInuse, heapGrowth, numResets)
	assert.Less(t, heapGrowth, int64(maxGrowthBytes),
		"Go heap must not grow unboundedly across resets")
}

// Test_autoResetWithEngineReuse verifies the full auto-reset path triggered by
// CallExportFunction when memoryUsed exceeds memHardLimit still works correctly
// with the global engine.
func Test_autoResetWithEngineReuse(t *testing.T) {
	const memHardLimit = 512 * 1024 // 512 KB — forces frequent resets

	inst := newTestInst("testInst", memHardLimit).
		MustExportFunction("returnString", (func(I32) *String)(nil)).
		SetDebugLevel(DebugLevelNone)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	for i := 0; i < 10; i++ {
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnString",
				Logger:         log.With(),
			},
			I32(1024),
		)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Engine must always be the global engine, even after auto-reset.
		assert.Same(t, globalEngine, inst.store.Engine)
	}
	t.Logf("resetCounter=%d", inst.resetCounter)
	assert.Greater(t, inst.resetCounter, uint(0), "at least one auto-reset should have occurred")
}

// Test_memoryUsed verifies that process RSS stays below 512 MB across many reset cycles.
func Test_memoryUsed(t *testing.T) {
	const (
		memHardLimit = 512 * 1024 // 512 KB — forces a reset on every call
		iterations   = 10
		maxRSSBytes  = 512 * 1024 * 1024 // 512 MB
	)

	inst := newTestInst("testInst", memHardLimit).
		MustExportFunction("returnString", (func(I32) *String)(nil)).
		SetDebugLevel(DebugLevelNone)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	for i := 0; i < iterations; i++ {
		// NOTE on argument size: returnString(n) builds a string by appending 'a' in a loop,
		// creating intermediate strings of lengths 1..n+1. AssemblyScript's bump allocator
		// advances its pointer for every allocation even if the old string is RC-freed, so
		// wasmMemUsed after a single call is proportional to n*(n+1)/2 in bytes:
		_, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnString",
				Logger:         log.With(),
			},
			I32(1024*10), // will use 100 MB memory
		)
		assert.NoError(t, err)

		assert.Equal(t, uint(i), inst.resetCounter)
		rss, rssErr := processRSSBytes()
		if assert.NoError(t, rssErr) {
			log.Infof("#%d wasmMemUsed=%d/%d resetCounter=%d processRSS=%d MB",
				i, inst.memoryMgr.memoryUsed, len(inst.memoryMgr.memory.Data()), inst.resetCounter, rss/1024/1024)
			assert.Less(t, rss, int64(maxRSSBytes),
				"#%d: process RSS %d bytes exceeded %d bytes limit", i, rss, maxRSSBytes)
		}
		time.Sleep(3 * time.Second)
	}
}
