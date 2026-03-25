package wasm

import (
	"context"
	_ "embed"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
)

// Test_globalEngineReuse verifies that all Instance.Init() calls reuse the package-level
// globalEngine rather than creating a new engine each time. This prevents engine memory
// pools from accumulating across reset() calls.
func Test_globalEngineReuse(t *testing.T) {
	inst := newTestInst("testInst")
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))
	assert.Same(t, globalEngine, inst.store.Engine, "after Init, store must use globalEngine")

	assert.NoError(t, inst.reset(log.With()))
	assert.Same(t, globalEngine, inst.store.Engine, "after reset, store must reuse globalEngine")

	assert.NoError(t, inst.reset(log.With()))
	assert.Same(t, globalEngine, inst.store.Engine, "after second reset, store must still reuse globalEngine")
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
