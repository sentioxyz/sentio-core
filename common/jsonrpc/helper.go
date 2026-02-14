package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
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

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, error) {
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	var args []reflect.Value
	tok, err := dec.Token()
	switch {
	case errors.Is(err, io.EOF) || tok == nil && err == nil:
		// "params" is optional and may be empty. Also allow "params":null even though it's
		// not in the spec because our own client used to send it.
	case err != nil:
		return nil, err
	case tok == json.Delim('['):
		// Read argument array.
		if args, err = parseArgumentArray(dec, types); err != nil {
			return nil, err
		}
	default:
		// Only one argument
		switch len(types) {
		case 1:
			// rebuild decoder to get back the first character shaved off by dec.Token
			dec = json.NewDecoder(bytes.NewReader(rawArgs))
			arg, err := parseArgument(dec, 0, types[0])
			if err != nil {
				return nil, err
			}
			return []reflect.Value{arg}, nil
		case 0:
			return nil, errors.New("do not need argument")
		default:
			return nil, errors.New("non-array args")
		}
	}
	// Set any missing args to zero value.
	for i := len(args); i < len(types); i++ {
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

func parseArgumentArray(dec *json.Decoder, types []reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return args, fmt.Errorf("too many arguments, want at most %d", len(types))
		}
		arg, err := parseArgument(dec, i, types[i])
		if err != nil {
			return args, err
		}
		args = append(args, arg)
	}
	// Read end of args array.
	_, err := dec.Token()
	return args, err
}

func parseArgument(dec *json.Decoder, index int, argType reflect.Type) (val reflect.Value, err error) {
	argVal := reflect.New(argType)
	if err = dec.Decode(argVal.Interface()); err != nil {
		return val, fmt.Errorf("invalid argument %d: %w", index, err)
	}
	if argVal.IsNil() && argType.Kind() != reflect.Ptr {
		return val, fmt.Errorf("missing value for required argument %d", index)
	}
	return argVal.Elem(), nil
}
