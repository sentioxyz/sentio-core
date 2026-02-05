package wasm

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type CallParams[DATA fmt.Stringer] struct {
	ExportFuncName string
	Logger         *log.SentioLogger
	Data           DATA
}

type CallStat struct {
	ExportFuncCalled   uint
	ImportFuncCalled   uint
	ImportFuncCallUsed time.Duration
}

type call[DATA fmt.Stringer] struct {
	CallParams[DATA]
	InstanceName      string
	EnterInstance     bool
	CallingImportFunc string

	CallStat

	AbortErr *ErrCallingImportFunc
}

func (c call[DATA]) name() string {
	return fmt.Sprintf("%s::%s", c.InstanceName, c.ExportFuncName)
}

func (c call[DATA]) calling() string {
	if c.CallingImportFunc == "" {
		return c.name()
	}
	return fmt.Sprintf("%s -> %s", c.name(), c.CallingImportFunc)
}

type callStack[DATA fmt.Stringer] struct {
	calls []*call[DATA]
}

func (cs *callStack[DATA]) push(c *call[DATA]) {
	cs.calls = append(cs.calls, c)
}

func (cs *callStack[DATA]) pop() *call[DATA] {
	top := cs.top()
	cs.calls = cs.calls[:len(cs.calls)-1]
	return top
}

func (cs *callStack[DATA]) top() *call[DATA] {
	return cs.calls[len(cs.calls)-1]
}

func (cs *callStack[DATA]) deep() int {
	return len(cs.calls)
}

func (cs *callStack[DATA]) isEmpty() bool {
	return cs.deep() == 0
}

func (cs *callStack[DATA]) String() string {
	return strings.Join(cs.Names(), " => ")
}

func (cs *callStack[DATA]) Names() []string {
	return utils.MapSliceNoError(cs.calls, func(c *call[DATA]) string {
		return c.name()
	})
}

type CallContext[DATA fmt.Stringer] struct {
	context.Context

	stack callStack[DATA]
}

func NewCallContext[DATA fmt.Stringer](ctx context.Context) *CallContext[DATA] {
	return &CallContext[DATA]{Context: ctx}
}

func (ctx *CallContext[DATA]) DumpCallStack() string {
	return ctx.stack.String()
}

func (ctx *CallContext[DATA]) CallStackDeep() int {
	return ctx.stack.deep()
}

func (ctx *CallContext[DATA]) TopParams() CallParams[DATA] {
	return ctx.stack.top().CallParams
}

func (ctx *CallContext[DATA]) Logger() *log.SentioLogger {
	top := ctx.stack.top()
	return top.CallParams.Logger.With(
		"calling", top.calling(),
		"stack", ctx.DumpCallStack(),
		"stat", top.CallStat,
		"ctxData", top.Data.String())
}
