package wasm

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	"github.com/wasmerio/wasmer-go/wasmer"

	"sentioxyz/sentio-core/common/log"
)

type Instance[DATA fmt.Stringer] struct {
	name         string
	modBytes     []byte
	memHardLimit uint32

	exportDefTable   map[string]reflect.Type
	importDefTable   map[string]map[string]importBox
	debugLevel       int
	initialed        bool
	exportFuncCalled uint

	instance     *wasmer.Instance
	exportedFunc map[string]wasmer.NativeFunction
	memoryMgr    *MemoryManager

	callCtx *CallContext[DATA]
}

func NewInstance[DATA fmt.Stringer](name string, modBytes []byte, memHardLimit uint32) *Instance[DATA] {
	return &Instance[DATA]{
		name:           name,
		modBytes:       modBytes,
		memHardLimit:   memHardLimit,
		exportDefTable: make(map[string]reflect.Type),
		importDefTable: make(map[string]map[string]importBox),
	}
}

func (inst *Instance[DATA]) Name() string {
	return inst.name
}

func (inst *Instance[DATA]) Init(logger *log.SentioLogger) error {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)
	module, err := wasmer.NewModule(store, inst.modBytes)
	if err != nil {
		return fmt.Errorf("new wasm module failed: %w", err)
	}
	inst.instance, err = wasmer.NewInstance(module, inst.prepareImportObject(module, store))
	if err != nil {
		return fmt.Errorf("new wasm instance failed: %w", err)
	}

	// get memory manager
	if err = inst.prepareMemoryManager(inst.memHardLimit); err != nil {
		return err
	}

	// get native export functions
	if err = inst.prepareNativeExportFunctions(); err != nil {
		return err
	}

	// get start function
	start, err := inst.instance.Exports.GetWasiStartFunction()
	if err != nil {
		return fmt.Errorf("get start function failed: %w", err)
	}

	// call start function
	inst.callCtx = &CallContext[DATA]{
		Context: context.Background(),
		stack: callStack[DATA]{
			calls: []*call[DATA]{{
				InstanceName: inst.name,
				CallParams: CallParams[DATA]{
					ExportFuncName: "start",
					Logger:         logger,
				},
			}},
		},
	}
	defer func() {
		inst.callCtx = nil
	}()
	if _, err = start(); err != nil {
		return fmt.Errorf("call start function failed: %w", err)
	}

	// prepare for reserved mem
	inst.memoryMgr.init()

	inst.initialed = true
	inst.exportFuncCalled = 0
	return nil
}

func (inst *Instance[DATA]) Close() {
	if inst.instance != nil {
		inst.instance.Close()
		inst.instance = nil
	}
	inst.memoryMgr = nil
	inst.exportedFunc = nil
	runtime.GC()
}

func (inst *Instance[DATA]) Reset(logger *log.SentioLogger) error {
	if inst.exportFuncCalled == 0 {
		logger.Warnf("want to reset wasm instance %s but no export function called", inst.name)
		return nil
	}
	return inst.reset(logger)
}

func (inst *Instance[DATA]) reset(logger *log.SentioLogger) error {
	inst.Close()
	logger.Infof("will reset wasm instance %s", inst.name)
	return inst.Init(logger)
}

const (
	DebugLevelNone = iota
	DebugLevelTrace
	DebugLevelMem
)

func (inst *Instance[DATA]) SetDebugLevel(level int) *Instance[DATA] {
	inst.debugLevel = level
	return inst
}

func (inst *Instance[DATA]) callImportFuncDebugLog(msg string) {
	if inst.debugLevel >= DebugLevelTrace {
		inst.callCtx.Logger().AddCallerSkip(1).Infof("calling import function: %s", msg)
	}
}

func (inst *Instance[DATA]) callImportFuncErrorLog(err error) {
	inst.callCtx.Logger().AddCallerSkip(1).Warnfe(err, "calling import function failed")
}

func (inst *Instance[DATA]) callExportFuncDebugLog(msg string) {
	if inst.debugLevel >= DebugLevelTrace {
		inst.callCtx.Logger().AddCallerSkip(1).Infof("calling export function: %s", msg)
	}
}

func (inst *Instance[DATA]) callExportFuncErrorLog(err error) {
	inst.callCtx.Logger().AddCallerSkip(1).Warnfe(err, "calling export function failed")
}
