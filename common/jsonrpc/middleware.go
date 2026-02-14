package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"reflect"
	"runtime"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type MethodHandler func(ctx context.Context, method string, params json.RawMessage) (any, error)

type Middleware = func(next MethodHandler) MethodHandler

type MiddlewareChain []Middleware

func (c MiddlewareChain) CallMethod(ctx context.Context, method string, params json.RawMessage) (resp any, err error) {
	_, logger := log.FromContext(ctx)
	defer func() {
		if panicErr := recover(); panicErr != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logger.Warn("RPC method " + method + " crashed: " + fmt.Sprintf("%v\n%s", panicErr, buf))
			err = fmt.Errorf("method handler crashed: %v", panicErr)
		}
	}()

	handler := finalHandler
	for i := len(c) - 1; i >= 0; i-- {
		// m1(m2(m3(... (finalHandler)...))
		handler = c[i](handler)
	}

	return handler(ctx, method, params)
}

func finalHandler(ctx context.Context, method string, params json.RawMessage) (any, error) {
	_, logger := log.FromContext(ctx)
	err := errors.Errorf("reaches final handler, method %s is not handled", method)
	logger.Warne(err)
	return nil, err
}

func MakeServiceAsMiddleware(namespace string, service any, exportMethods ...string) Middleware {
	methods := suitableCallbacks(reflect.ValueOf(service), namespace, exportMethods)
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, methodName string, rawMsg json.RawMessage) (any, error) {
			ctx = context.WithValue(ctx, nextMiddlewareContextKey, next)
			method, ok := methods[strings.ToLower(methodName)]
			if !ok {
				return next(ctx, methodName, rawMsg)
			}
			args, err := parsePositionalArguments(rawMsg, method.argTypes)
			if err != nil {
				_, logger := log.FromContext(ctx)
				logger.Warne(err, "Error in parse arguments")
				return nil, err
			}
			var result any
			result, err = method.call(ctx, args)
			if errors.Is(CallNextMiddleware, err) {
				return next(ctx, methodName, rawMsg)
			}
			return result, err
		}
	}
}

// CallNextMiddleware a special error to indicate that the middleware should call the next middleware instead of return error
var CallNextMiddleware = errors.New("middleware next error")

func NextHandleFromContext(ctx context.Context) (MethodHandler, error) {
	next, ok := ctx.Value(nextMiddlewareContextKey).(MethodHandler)
	if !ok {
		return nil, errors.New("can't find next handle from context")
	}
	return next, nil
}

var nextMiddlewareContextKey struct{}

// callback is a method callback which was registered in the server
type callback struct {
	fn       reflect.Value  // the function
	receiver reflect.Value  // receiver object of method, set if fn is method
	argTypes []reflect.Type // input argument types
	errPos   int            // err return idx, of -1 when method cannot return error
	resPos   int
}

// makeArgTypes composes the argTypes list.
func (c *callback) makeArgTypes() {
	fnType := c.fn.Type()
	firstArg := 0
	if c.receiver.IsValid() {
		firstArg++
	}
	// For ctx
	firstArg++
	// Add all remaining parameters.
	c.argTypes = make([]reflect.Type, fnType.NumIn()-firstArg)
	for i := firstArg; i < fnType.NumIn(); i++ {
		c.argTypes[i-firstArg] = fnType.In(i)
	}
}

// call invokes the callback.
func (c *callback) call(ctx context.Context, args []reflect.Value) (res interface{}, errRes error) {
	// Create the argument slice.
	fullArgs := make([]reflect.Value, 0, 2+len(args))
	if c.receiver.IsValid() {
		fullArgs = append(fullArgs, c.receiver)
	}
	fullArgs = append(fullArgs, reflect.ValueOf(ctx))
	fullArgs = append(fullArgs, args...)

	// Run the callback.
	results := c.fn.Call(fullArgs)
	if len(results) == 0 {
		return nil, nil
	}
	if c.resPos >= 0 {
		res = results[c.resPos].Interface()
	}
	if c.errPos >= 0 && !results[c.errPos].IsNil() {
		// Method has returned non-nil error value.
		errRes = results[c.errPos].Interface().(error)
	}
	return res, errRes
}

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

// Does t satisfy the error interface?
func isErrorType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Implements(errorType)
}

func newCallback(receiver, fn reflect.Value) *callback {
	c := &callback{fn: fn, receiver: receiver, resPos: -1, errPos: -1}
	// Determine parameter types. They must all be exported or builtin types.
	c.makeArgTypes()

	// Verify return types. The function must return at most one error
	// and/or one other non-error value.
	// If an error is returned, it must be the last returned value.
	fnType := fn.Type()
	switch fnType.NumOut() {
	case 0:
	case 1:
		if !isErrorType(fnType.Out(0)) {
			return nil
		}
		c.errPos = 0
	case 2:
		if !isErrorType(fnType.Out(1)) {
			return nil
		}
		c.resPos, c.errPos = 0, 1
	}
	return c
}

func suitableCallbacks(receiver reflect.Value, namespace string, exportMethods []string) map[string]*callback {
	typ := receiver.Type()
	callbacks := make(map[string]*callback)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		if method.PkgPath != "" {
			continue // method not exported
		}
		if len(exportMethods) > 0 && utils.IndexOf(exportMethods, method.Name) < 0 {
			continue // method not exported
		}
		cb := newCallback(receiver, method.Func)
		if cb == nil {
			continue // function invalid
		}
		methodName := strings.ToLower(fmt.Sprintf("%s_%s", namespace, method.Name))
		callbacks[methodName] = cb
	}
	return callbacks
}
