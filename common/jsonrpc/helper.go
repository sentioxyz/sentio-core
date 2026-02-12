package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sentioxyz/sentio-core/common/utils"
)

func CallMethod(method any, ctx context.Context, params json.RawMessage) (r any, err error) {
	// base check
	mtd := reflect.ValueOf(method)
	if mtd.Kind() != reflect.Func {
		panic(fmt.Errorf("method %v is not an function", method))
	}
	mt := mtd.Type()
	if mt.NumIn() <= 0 {
		panic(fmt.Errorf("method %v should has one context argument at least", method))
	}
	if !mt.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		panic(fmt.Errorf("method %v the first argument is %v, not a context.Context", method, mt.In(0)))
	}
	// prepare arguments
	argTypes := make([]reflect.Type, mt.NumIn()-1)
	for i := 1; i < mt.NumIn(); i++ {
		argTypes[i-1] = mt.In(i)
	}
	var args []reflect.Value
	args, err = parsePositionalArguments(params, argTypes)
	if err != nil {
		return nil, err
	}
	args = utils.Prepend(args, reflect.ValueOf(ctx))
	// actually call
	rets := mtd.Call(args)
	// check the returned values
	var errVal reflect.Value
	switch len(rets) {
	case 0:
		return nil, nil
	case 1:
		errVal = rets[0]
	case 2:
		r = rets[0].Interface()
		errVal = rets[1]
	default:
		panic(fmt.Errorf("method %v returned %d values, more than 2", method, len(rets)))
	}
	if errVal.IsNil() {
		return r, nil
	}
	return r, errVal.Interface().(error)
}
