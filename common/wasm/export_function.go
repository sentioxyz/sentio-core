package wasm

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/wasmerio/wasmer-go/wasmer"
)

func (inst *Instance[DATA]) prepareNativeExportFunctions() error {
	inst.exportedFunc = make(map[string]wasmer.NativeFunction)
	for name := range inst.exportDefTable {
		nativeFunc, err := inst.instance.Exports.GetFunction(name)
		if err != nil {
			return fmt.Errorf("get export function %q failed: %w", name, err)
		}
		inst.exportedFunc[name] = nativeFunc
	}
	return nil
}

func (inst *Instance[DATA]) ExportFunction(name string, fn any) error {
	if inst.initialed {
		return fmt.Errorf("should export function before init")
	}

	ft := reflect.TypeOf(fn)

	if ft.Kind() != reflect.Func {
		return fmt.Errorf("fn is not an function")
	}
	// check inputs
	for i := 0; i < ft.NumIn(); i++ {
		if _, ok := ConvertType(ft.In(i)); !ok {
			return fmt.Errorf("fn has invalid args #%d type %v", i, ft.In(i))
		}
	}
	// check outputs
	if ft.NumOut() > 1 {
		return fmt.Errorf("fn cannot return more than one value")
	}
	if ft.NumOut() == 1 {
		if _, ok := ConvertType(ft.Out(0)); !ok {
			return fmt.Errorf("fn has an invalid return type %v", ft.Out(0))
		}
	}

	inst.exportDefTable[name] = ft
	return nil
}

func (inst *Instance[DATA]) MustExportFunction(name string, fn any) *Instance[DATA] {
	err := inst.ExportFunction(name, fn)
	if err != nil {
		panic(err)
	}
	return inst
}

type CallResult struct {
	TimeUsed           time.Duration
	ExportFuncCalled   uint
	ImportFuncCalled   uint
	ImportFuncCallUsed time.Duration
	MemoryUsed         uint32 // Include only the main module's memory usage
}

var (
	ErrPrepareCallExportFunc = errors.New("prepare call export function failed")
	ErrPanic                 = errors.New("panic in wasm")
)

// CallExportFunction error returned may be an ErrCallingImportFunc or wrapped ErrPrepareCallExportFunc or ErrPanic
func (inst *Instance[DATA]) CallExportFunction(
	ctx *CallContext[DATA],
	params CallParams[DATA],
	args ...any,
) (ret any, result CallResult, err error) {
	fullName := fmt.Sprintf("%s::%s", inst.name, params.ExportFuncName)
	// find the export function
	ft, has := inst.exportDefTable[params.ExportFuncName]
	if !has {
		err = fmt.Errorf("%w: %s not exist", ErrPrepareCallExportFunc, fullName)
		return
	}

	// check call context
	if inst.callCtx != nil && inst.callCtx != ctx {
		err = fmt.Errorf("%w: unexpected call context", ErrPrepareCallExportFunc)
		return
	}
	if inst.callCtx == nil && inst.memoryMgr.memoryUsed > inst.memoryMgr.memHardLimit {
		if err = inst.reset(params.Logger); err != nil {
			return
		}
	}
	initMemoryUsed := inst.memoryMgr.memoryUsed
	startTime := time.Now()

	// prepare native objects
	nativeArgs := make([]any, len(args))
	for i := 0; i < ft.NumIn(); i++ {
		goArg := reflect.ValueOf(args[i])
		if goArg.Type() != ft.In(i) {
			err = fmt.Errorf("%w: invalid args #%d for %s, type is %v not %v",
				ErrPrepareCallExportFunc, i, fullName, goArg.Type(), ft.In(i))
			return
		}
		v, ok := inst.memoryMgr.FromGoValue(goArg)
		if !ok {
			err = fmt.Errorf("%w: invalid args #%d for %s type %T", ErrPrepareCallExportFunc, i, fullName, args[i])
			return
		}
		nativeArgs[i] = v.Unwrap()
	}
	nativeFunc := inst.exportedFunc[params.ExportFuncName]

	// push call stack
	calling := &call[DATA]{
		InstanceName:  inst.name,
		EnterInstance: inst.callCtx == nil,
		CallParams:    params,
	}
	ctx.stack.push(calling)
	inst.callCtx = ctx

	inst.callExportFuncDebugLog(fmt.Sprintf("arg:%v", nativeArgs))
	inst.debugShowMemoryReview()

	// call export function
	var nativeReturn any
	nativeReturn, err = nativeFunc(nativeArgs...)
	if err != nil {
		var trapErr *wasmer.TrapError
		if calling.AbortErr != nil {
			err = calling.AbortErr
		} else if errors.As(err, &trapErr) {
			err = fmt.Errorf("%w: %s", ErrPanic, trapErr.Error())
		}
	}
	inst.memoryMgr.setMemoryUsed()

	// prepare return objects
	if ft.NumOut() > 0 && err == nil {
		var rv wasmer.Value
		switch k, _ := ConvertType(ft.Out(0)); k {
		case wasmer.I32:
			rv = wasmer.NewI32(nativeReturn)
		case wasmer.I64:
			rv = wasmer.NewI64(nativeReturn)
		case wasmer.F32:
			rv = wasmer.NewF32(nativeReturn)
		case wasmer.F64:
			rv = wasmer.NewF64(nativeReturn)
		}
		retVal, _ := inst.memoryMgr.ToGoValue(rv, ft.Out(0))
		ret = retVal.Interface()
	}

	// build result
	result.TimeUsed = time.Since(startTime)
	result.ExportFuncCalled = calling.ExportFuncCalled
	result.ImportFuncCalled = calling.ImportFuncCalled
	result.ImportFuncCallUsed = calling.ImportFuncCallUsed
	result.MemoryUsed = inst.memoryMgr.memoryUsed - initMemoryUsed
	inst.exportFuncCalled++

	if err == nil {
		inst.callExportFuncDebugLog(fmt.Sprintf("ret:%v", nativeReturn))
	} else {
		inst.callExportFuncErrorLog(err)
	}
	inst.debugShowMemoryReview()

	// pop call stack
	if ctx.stack.pop().EnterInstance {
		inst.callCtx = nil
	}

	if !ctx.stack.isEmpty() {
		top := ctx.stack.top()
		top.ExportFuncCalled += calling.ExportFuncCalled + 1
		top.ImportFuncCalled += calling.ImportFuncCalled
		top.ImportFuncCallUsed += calling.ImportFuncCallUsed
	}

	return
}
