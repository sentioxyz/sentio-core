package wasm

import (
	"fmt"
	"github.com/wasmerio/wasmer-go/wasmer"
	"reflect"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type importBox struct {
	funcType  *wasmer.FunctionType
	implement func([]wasmer.Value) ([]wasmer.Value, error)
}

func (inst *Instance[DATA]) prepareImportObject(module *wasmer.Module, store *wasmer.Store) *wasmer.ImportObject {
	obj := wasmer.NewImportObject()
	for _, importType := range module.Imports() {
		namespace, name := importType.Module(), importType.Name()
		box, has := utils.GetFromK2Map(inst.importDefTable, namespace, name)
		if !has {
			box, has = utils.GetFromK2Map(inst.importDefTable, "*", name)
		}
		if !has {
			continue // will return error while new wasm instance using obj
		}
		f := wasmer.NewFunction(store, box.funcType, box.implement)
		obj.Register(namespace, map[string]wasmer.IntoExtern{name: f})
	}
	return obj
}

func (inst *Instance[DATA]) newImportBox(namespace, name string, fn any) (box importBox, err error) {
	ft := reflect.TypeOf(fn)
	fv := reflect.ValueOf(fn)

	if ft.Kind() != reflect.Func {
		panic(fmt.Errorf("fn is not an function"))
	}

	// first arg of fn must be *CallContext[DATA]
	if ft.NumIn() < 1 {
		return box, fmt.Errorf("import function must have at least 1 parameter")
	}
	if ft.In(0) != reflect.TypeOf(&CallContext[DATA]{}) {
		return box, fmt.Errorf("first arg of import function must be %T", &CallContext[DATA]{})
	}
	var ok bool
	ins := make([]wasmer.ValueKind, ft.NumIn()-1)
	for i := 1; i < ft.NumIn(); i++ {
		ins[i-1], ok = ConvertType(ft.In(i))
		if !ok {
			return box, fmt.Errorf("parameter #%d: invalid type %v", i, ft.In(i))
		}
	}
	outs := make([]wasmer.ValueKind, ft.NumOut())
	for i := 0; i < ft.NumOut(); i++ {
		outs[i], ok = ConvertType(ft.Out(i))
		if !ok {
			return box, fmt.Errorf("return #%d: invalid type %v", i, ft.Out(i))
		}
	}
	funcType := wasmer.NewFunctionType(
		wasmer.NewValueTypes(ins...),
		wasmer.NewValueTypes(outs...),
	)

	fullName := fmt.Sprintf("%s/%s", namespace, name)
	wrapped := func(args []wasmer.Value) (returns []wasmer.Value, err error) {
		if inst.callCtx == nil {
			return nil, fmt.Errorf("calling %s failed: wasm instance is not ready", fullName)
		}
		top := inst.callCtx.stack.top()
		top.CallingImportFunc = fullName
		inst.callImportFuncDebugLog("begin")
		start := time.Now()
		defer func() {
			top.ImportFuncCalled++
			top.ImportFuncCallUsed += time.Since(start)
			if e := recover(); e != nil {
				var is bool
				if err, is = e.(error); !is {
					err = fmt.Errorf("%v", e)
				}
				inst.callImportFuncErrorLog(err)
				top.AbortErr = &ErrCallingImportFunc{
					Err:   err,
					Stack: append(inst.callCtx.stack.Names(), fullName),
				}
				err = top.AbortErr
			} else {
				inst.callImportFuncDebugLog("succeed")
			}
			top.CallingImportFunc = ""
		}()
		callArgs := make([]reflect.Value, ft.NumIn())
		callArgs[0] = reflect.ValueOf(inst.callCtx) // first arg of fn must be *CallContext[DATA]
		for i := 1; i < ft.NumIn(); i++ {
			callArgs[i], _ = inst.memoryMgr.ToGoValue(args[i-1], ft.In(i))
			inst.callImportFuncDebugLog(fmt.Sprintf("arg[%d]: %v", i-1, callArgs[i]))
		}
		callReturns := fv.Call(callArgs)
		returns = make([]wasmer.Value, ft.NumOut())
		for i, callReturn := range callReturns {
			inst.callImportFuncDebugLog(fmt.Sprintf("ret[%d]: %v", i, callReturn))
			returns[i], _ = inst.memoryMgr.FromGoValue(callReturn)
		}
		return returns, err
	}

	return importBox{funcType: funcType, implement: wrapped}, nil
}

func (inst *Instance[DATA]) ImportFunction(namespace, name string, fn any) error {
	if inst.initialed {
		return fmt.Errorf("should import function before init")
	}

	box, err := inst.newImportBox(namespace, name, fn)
	if err != nil {
		return err
	}
	utils.PutIntoK2Map(inst.importDefTable, namespace, name, box)
	utils.PutIntoK2Map(inst.importDefTable, "*", name, box)
	return nil
}

func (inst *Instance[DATA]) MustImportFunction(namespace, name string, fn any) *Instance[DATA] {
	err := inst.ImportFunction(namespace, name, fn)
	if err != nil {
		panic(err)
	}
	return inst
}

type ErrCallingImportFunc struct {
	Err   error
	Stack []string
}

func (e ErrCallingImportFunc) Error() string {
	return fmt.Sprintf("abort in %s: %v, callstack: %s",
		e.Stack[len(e.Stack)-1], e.Err.Error(), strings.Join(e.Stack, " => "))
}
