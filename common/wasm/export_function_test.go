package wasm

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
)

//go:embed testdata/build/main/main.wasm
var testModBytes []byte

const (
	testDebugLevel = DebugLevelNone
	//testDebugLevel = DebugLevelTrace
	//testDebugLevel = DebugLevelMem
)

type testCtxData string

func (s testCtxData) String() string {
	return string(s)
}

func newTestInst(name string, memHardLimit ...uint32) *Instance[testCtxData] {
	var ml uint32 = 3 * 1024 * 1024 * 1024
	if len(memHardLimit) > 0 {
		ml = memHardLimit[0]
	}
	inst := NewInstance[testCtxData](name, testModBytes, ml)
	return inst.
		MustImportFunction("env", "abort",
			func(ctx *CallContext[testCtxData], msg *String, filename *String, lineNum I32, colNum I32) {
				err := fmt.Errorf("[%s:%d:%d] %s", filename, lineNum, colNum, msg)
				ctx.Logger().Warne(err)
				panic(err)
			}).
		MustImportFunction("conversion", "typeConversion.bytesToString",
			func(ctx *CallContext[testCtxData], arg *ByteArray) *String {
				if arg == nil {
					return BuildString("<nil>")
				}
				return BuildStringFromBytes(arg.Data)
			}).
		MustImportFunction("conversion", "typeConversion.bytesToHex",
			func(ctx *CallContext[testCtxData], arg *ByteArray) *String {
				return BuildString(arg.ToHex())
			}).
		MustImportFunction("conversion", "typeConversion.bigIntToString",
			func(*CallContext[testCtxData], *ByteArray) *String {
				panic("not implemented")
			}).
		MustImportFunction("conversion", "typeConversion.bigIntToHex",
			func(*CallContext[testCtxData], *String) *ByteArray {
				panic("not implemented")
			}).
		MustImportFunction("conversion", "typeConversion.stringToH160",
			func(ctx *CallContext[testCtxData], arg *String) *ByteArray {
				arr, buildErr := BuildByteArrayFromHex(arg.String())
				if buildErr != nil {
					panic(buildErr)
				}
				return arr
			}).
		MustImportFunction("numbers", "bigDecimal.toString",
			func(*CallContext[testCtxData], *String) *String {
				panic("not implemented")
			}).
		MustImportFunction("index", "store.set",
			func(*CallContext[testCtxData], *String, *String, *String) {
				panic("not implemented")
			}).
		MustImportFunction("index", "log.log",
			func(ctx *CallContext[testCtxData], level I32, msg *String) {
				ctx.TopParams().Logger.Infof("called 'log.log' [level:%v][msg:%v]", level, msg)
			})
}

func Test_allocateAndReset(t *testing.T) {
	const size = 100 * 1024 * 1024 // 100 MB

	inst := newTestInst("testInst").SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	//const TestRound = 100
	const TestRound = 10
	// 1000MB per round
	for round := 0; round < TestRound; round++ {
		base := inst.memoryMgr.memoryUsed
		for i := 0; i < 10; i++ {
			p := inst.memoryMgr.NewMemory(size, 0)
			m := inst.memoryMgr.GetMemory()
			for j := Pointer(0); j < size; j++ {
				m[p+j] = byte(i + 10)
			}
			//inst.debugShowMemoryReview()
			assert.Equal(t, base+BlockHeadSize+ObjectHeadSize+uint32((size+32)*i), uint32(p))
			log.Infof("ROUND #%d> #%d pointer:%d, mem.Len:%d, mem.Cap:%d", round, i, uint32(p), len(m), cap(m))
		}
		assert.NoError(t, inst.reset(log.With()))
	}
	//time.Sleep(time.Minute)
}

func Test_autoReset(t *testing.T) {
	//export function returnString(n: i32): string {
	//  let s: string = "头";
	//  for (let i: i32 = 0; i < n; i++) {
	//    s = s + 'a'
	//  }
	//  return s
	//}
	inst := newTestInst("testInst", 1024*1024).
		MustExportFunction("returnString", (func(I32) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	var expect bytes.Buffer
	expect.WriteString("头")
	for i := 0; i < 1024; i++ {
		expect.WriteByte('a')
	}

	result, report, err := inst.CallExportFunction(
		NewCallContext[testCtxData](context.Background()),
		CallParams[testCtxData]{
			ExportFuncName: "returnString",
			Logger:         log.With(),
		},
		I32(1024))
	assert.NoError(t, err)
	assert.Equal(t, expect.String(), result.(*String).String())
	log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)

	result, report, err = inst.CallExportFunction(
		NewCallContext[testCtxData](context.Background()),
		CallParams[testCtxData]{
			ExportFuncName: "returnString",
			Logger:         log.With(),
		},
		I32(1024))
	assert.NoError(t, err)
	assert.Equal(t, expect.String(), result.(*String).String())
	log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
}

func Test_add(t *testing.T) {
	//export function add(a: i32, b: i32): i32 {
	//  log.info("a = {}, b = {}", [a.toString(), b.toString()])
	//  return a + b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("add", (func(I32, I32) I32)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "add",
				Logger:         log.With(),
			},
			I32(123), I32(234))
		assert.NoError(t, err)
		assert.Equal(t, I32(357), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "add",
				Logger:         log.With(),
			},
			I32(1111111), I32(2345678))
		assert.NoError(t, err)
		assert.Equal(t, I32(3456789), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
}

func Test_importFuncReturnErr(t *testing.T) {
	//t.Skip("will cause 'double free or corruption' in ci env")
	//export function add(a: i32, b: i32): i32 {
	//  log.info("a = {}, b = {}", [a.toString(), b.toString()])
	//  return a + b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("add", (func(I32, I32) I32)(nil)).
		MustImportFunction("index", "log.log",
			func(ctx *CallContext[testCtxData], level I32, msg *String) {
				if strings.Contains(string(*msg), "999999999") {
					panic(fmt.Errorf("msg contains 999999999"))
				}
				log.Infof("called 'log.log' [level:%v][msg:%v]", level, msg)
			})
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "add",
				Logger:         log.With(),
			},
			I32(999999999), I32(234))
		var errCallingImportFunc *ErrCallingImportFunc
		assert.Nil(t, result)
		assert.True(t, errors.As(err, &errCallingImportFunc))
		assert.Equal(t, "msg contains 999999999", errCallingImportFunc.Err.Error())
		assert.Equal(t, []string{"testInst::add", "index/log.log"}, errCallingImportFunc.Stack)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "add",
				Logger:         log.With(),
			},
			I32(1111111), I32(2345678))
		assert.NoError(t, err)
		assert.Equal(t, I32(3456789), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
}

func Test_floatAdd(t *testing.T) {
	//export function floatAdd(a: f32, b: f32): f64 {
	//  return a + b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("floatAdd", (func(F32, F32) F64)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "floatAdd",
				Logger:         log.With(),
			},
			F32(178.125), F32(20000.125))
		assert.NoError(t, err)
		assert.Equal(t, F64(20178.25), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "floatAdd",
				Logger:         log.With(),
			},
			F32(-178.125), F32(20000.125))
		assert.NoError(t, err)
		assert.Equal(t, F64(19822), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
}

func Test_returnString(t *testing.T) {
	//export function returnString(n: i32): string {
	//  let s: string = "头";
	//  for (let i: i32 = 0; i < n; i++) {
	//    s = s + 'a'
	//  }
	//  return s
	//}
	inst := newTestInst("testInst").
		MustExportFunction("returnString", (func(I32) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		// the pointer returned is in const data segment, so is less than inst.memStartOffset
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnString",
				Logger:         log.With(),
			},
			I32(0))
		assert.NoError(t, err)
		assert.Equal(t, "头", result.(*String).String())
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnString",
				Logger:         log.With(),
			},
			I32(5))
		assert.NoError(t, err)
		assert.Equal(t, "头aaaaa", result.(*String).String())
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
}

func Test_returnStringV2(t *testing.T) {
	//export function returnStringV2(s: string): string {
	//  return s
	//}
	inst := newTestInst("testInst").
		MustExportFunction("returnStringV2", (func(*String) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnStringV2",
				Logger:         log.With(),
			},
			BuildString("good1"))
		assert.NoError(t, err)
		assert.Equal(t, "good1", result.(*String).String())
	}
	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "returnStringV2",
				Logger:         log.With(),
			},
			BuildString(""))
		assert.NoError(t, err)
		assert.Equal(t, "", result.(*String).String())
	}
}

func Test_indexOf(t *testing.T) {
	testcases := [][]any{
		{"hello", "ell", I32(1)},
		{"hello", "elx", I32(-1)},
		{"hello", "hell", I32(0)},
		{"hello", "hello", I32(0)},
	}

	//export function indexOf(a: string, b: string): i32 {
	//  return a.indexOf(b)
	//}
	inst := newTestInst("testInst").
		MustExportFunction("indexOf", (func(*String, *String) I32)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	for i := 0; i < len(testcases); i++ {
		ori := testcases[i][0].(string)
		sub := testcases[i][1].(string)
		res := testcases[i][2].(I32)

		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "indexOf",
				Logger:         log.With(),
			},
			BuildString(ori), BuildString(sub))
		assert.NoError(t, err)
		assert.Equal(t, res, result)
	}
}

func Test_foo(t *testing.T) {
	//export function foo(a: string, b: string, c: string): string {
	//  return a.indexOf(c) >= 0 ? a : b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("foo", (func(*String, *String, *String) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "foo",
				Logger:         log.With(),
			},
			BuildString("hello"),
			BuildString("world"),
			BuildString("ell"))
		assert.NoError(t, err)
		assert.Equal(t, "hello", result.(*String).String())
	}
	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "foo",
				Logger:         log.With(),
			},
			BuildString("hello"),
			BuildString("world"),
			BuildString("elx"))
		assert.NoError(t, err)
		assert.Equal(t, "world", result.(*String).String())
	}
}

func Test_bar(t *testing.T) {
	//export function bar(a: string, b: string): string {
	//  return "中文中文中文中文中文" + a + b;
	//}
	inst := newTestInst("testInst").
		MustExportFunction("bar", (func(*String, *String) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "bar",
				Logger:         log.With(),
			},
			BuildString(" hello"),
			BuildString(" world"))
		assert.NoError(t, err)
		assert.Equal(t, "中文中文中文中文中文 hello world", result.(*String).String())
	}
	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "bar",
				Logger:         log.With(),
			},
			BuildString(""),
			BuildString(" xxx"))
		assert.NoError(t, err)
		assert.Equal(t, "中文中文中文中文中文 xxx", result.(*String).String())
	}
}

func Test_getAddr(t *testing.T) {
	//export function getAddr(b: string): string {
	//  return addr.toHexString() + b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("getAddr", (func(*String) *String)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "getAddr",
				Logger:         log.With(),
			},
			BuildString("xxx"))
		assert.NoError(t, err)
		assert.Equal(t, "0x0102030405060708090a0b0c0d0e0f1011121318xxx", result.(*String).String())
	}
	assert.NoError(t, inst.Reset(log.With()))
	{
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "getAddr",
				Logger:         log.With(),
			},
			BuildString("zzz"))
		assert.NoError(t, err)
		assert.Equal(t, "0x0102030405060708090a0b0c0d0e0f1011121318zzz", result.(*String).String())
	}
}

type ethValue struct {
	Kind  U32
	Value any
}

const (
	valueKindAddress = iota
	valueKindFixedBytes
	valueKindBytes
	valueKindInt
	valueKindUint
	valueKindBool
	valueKindString
	valueKindFixedArray
	valueKindArray
	valueKindTuple
)

func (v *ethValue) Dump(mm *MemoryManager) Pointer {
	panic("not implemented")
}

func (v *ethValue) Load(mm *MemoryManager, p Pointer) {
	var val = struct {
		Kind    U32
		Payload U64
	}{}
	mm.LoadObject(p, &val)
	v.Kind = val.Kind
	v.Value = nil
	switch v.Kind {
	case valueKindAddress:
		if val.Payload > 0 {
			var value ByteArray
			value.Load(mm, Pointer(val.Payload))
			v.Value = &value
		}
	case valueKindBool:
		v.Value = Bool(val.Payload != 0)
	case valueKindString:
		if val.Payload > 0 {
			var value String
			value.Load(mm, Pointer(val.Payload))
			v.Value = &value
		}
	default:
		panic("not implemented")
	}
}

type eventParam struct {
	Name  *String
	Value *ethValue
}

func (ep *eventParam) Dump(mm *MemoryManager) Pointer {
	return mm.DumpObject(ep)
}

func (ep *eventParam) Load(mm *MemoryManager, p Pointer) {
	mm.LoadObject(p, ep)
}

type myEvent struct {
	Array1    *BaseArray[Bool]
	Array2    *BaseArray[U8]
	Params    *ObjectArray[*eventParam]
	IntKey10  I8
	IntKey11  I8
	IntKey12  I8
	IntKey2   I16
	IntKey3   I64
	IntKey4   I32
	BoolKey   Bool
	FloatKey1 F32
	FloatKey2 F64
	Address   *ByteArray
	Message   *String
	IntKey20  I8
	IntKey21  I8
	IntKey22  I8
}

func (ev *myEvent) Dump(mm *MemoryManager) Pointer {
	return mm.DumpObject(ev)
}

func (ev *myEvent) Load(mm *MemoryManager, p Pointer) {
	mm.LoadObject(p, ev)
}

func Test_returnMyEvent(t *testing.T) {
	//export function returnMyEvent(): MyEvent {
	//  let arr1: Array<boolean> = new Array<boolean>(5);
	//  for (let i = 0; i < 5; i++) {
	//    arr1[i] = i % 2 == 0
	//  }
	//  let arr2: Array<u8> = new Array<u8>(10);
	//  for (let i: u8 = 0; i < 10; i++) {
	//    arr2[i] = i + 10
	//  }
	//  let params: Array<ethereum.EventParam> = new Array<ethereum.EventParam>();
	//  params.push(new ethereum.EventParam("param1", ethereum.Value.fromBoolean(true)))
	//  params.push(new ethereum.EventParam("param2", ethereum.Value.fromString("value-2")))
	//  params.push(new ethereum.EventParam("param3", ethereum.Value.fromAddress(
	//    Address.fromBytes(Bytes.fromHexString("0102030405060708090a0b0c0d0e0f1011121314")))))
	//  let addr: Address = Address.fromBytes(Bytes.fromHexString("0102030405060708090a0b0c0d0e0f1011121318"))
	//  return new MyEvent(arr1, arr2, params,
	//    111, 112, 113, 114, 115, 116,
	//    true, 178.125, 0.00125, addr, "message12",
	//    117, 118, 119)
	//}
	inst := newTestInst("testInst").MustExportFunction("returnMyEvent", (func() *myEvent)(nil))

	assert.NoError(t, inst.Init(log.With()))

	result, _, err := inst.CallExportFunction(
		NewCallContext[testCtxData](context.Background()),
		CallParams[testCtxData]{
			ExportFuncName: "returnMyEvent",
			Logger:         log.With(),
		})
	assert.NoError(t, err)

	retEvent := result.(*myEvent)
	assert.Equal(t, &BaseArray[Bool]{Data: []Bool{true, false, true, false, true}}, retEvent.Array1)
	assert.Equal(t, &BaseArray[U8]{Data: []U8{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}}, retEvent.Array2)
	assert.Equal(t, &ObjectArray[*eventParam]{Data: []*eventParam{
		{Name: BuildString("param1"), Value: &ethValue{
			Kind:  valueKindBool,
			Value: Bool(true),
		}},
		{Name: BuildString("param2"), Value: &ethValue{
			Kind:  valueKindString,
			Value: BuildString("value-2"),
		}},
		{Name: BuildString("param3"), Value: &ethValue{
			Kind:  valueKindAddress,
			Value: &ByteArray{Data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}},
		}},
	}}, retEvent.Params)
	assert.Equal(t, I8(111), retEvent.IntKey10)
	assert.Equal(t, I8(112), retEvent.IntKey11)
	assert.Equal(t, I8(113), retEvent.IntKey12)
	assert.Equal(t, I16(114), retEvent.IntKey2)
	assert.Equal(t, I64(115), retEvent.IntKey3)
	assert.Equal(t, I32(116), retEvent.IntKey4)
	assert.Equal(t, Bool(true), retEvent.BoolKey)
	assert.Equal(t, F32(178.125), retEvent.FloatKey1)
	assert.Equal(t, F64(0.00125), retEvent.FloatKey2)
	assert.Equal(t, &ByteArray{Data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 24}},
		retEvent.Address)
	assert.Equal(t, "message12", retEvent.Message.String())
	assert.Equal(t, I8(117), retEvent.IntKey20)
	assert.Equal(t, I8(118), retEvent.IntKey21)
	assert.Equal(t, I8(119), retEvent.IntKey22)

	//fmt.Println("result event:", retEvent)
}

func Test_returnMyEventV2(t *testing.T) {
	//export function returnMyEventV2(event: MyEvent): MyEvent {
	//  event.message = event.message + "-suffix"
	//  event.floatKey1 += 10000
	//  event.floatKey2 += 20000
	//  event.boolKey = !event.boolKey
	//  event.address = Address.fromBytes(Bytes.fromHexString("0102030405060708090a0b0c0d0e0f1011121318"))
	//  event.array2 = null
	//  event.intKey22 = -111
	//  return event
	//}
	inst := newTestInst("testInst").
		MustExportFunction("returnMyEventV2", (func(*myEvent) *myEvent)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	argEvent := myEvent{
		Array1:    &BaseArray[Bool]{Data: []Bool{true, false, true, false, true}},
		Array2:    &BaseArray[U8]{Data: []U8{11, 12, 13, 14, 15, 16}},
		IntKey10:  111,
		IntKey11:  112,
		IntKey12:  113,
		IntKey2:   114,
		IntKey3:   115,
		IntKey4:   116,
		BoolKey:   true,
		FloatKey1: 178.125,
		FloatKey2: 178.125,
		Address:   nil,
		Message:   BuildString("good"),
		IntKey20:  117,
		IntKey21:  118,
		IntKey22:  119,
	}

	result, _, err := inst.CallExportFunction(
		NewCallContext[testCtxData](context.Background()),
		CallParams[testCtxData]{
			ExportFuncName: "returnMyEventV2",
			Logger:         log.With(),
		},
		&argEvent)
	assert.NoError(t, err)

	retEvent := result.(*myEvent)
	assert.Equal(t, argEvent.Array1, retEvent.Array1)
	assert.Zero(t, retEvent.Array2)
	assert.Equal(t, argEvent.IntKey10, retEvent.IntKey10)
	assert.Equal(t, argEvent.IntKey11, retEvent.IntKey11)
	assert.Equal(t, argEvent.IntKey12, retEvent.IntKey12)
	assert.Equal(t, argEvent.IntKey2, retEvent.IntKey2)
	assert.Equal(t, argEvent.IntKey3, retEvent.IntKey3)
	assert.Equal(t, argEvent.IntKey4, retEvent.IntKey4)
	assert.Equal(t, !argEvent.BoolKey, retEvent.BoolKey)
	assert.Equal(t, argEvent.FloatKey1+10000, retEvent.FloatKey1)
	assert.Equal(t, argEvent.FloatKey2+20000, retEvent.FloatKey2)
	assert.Equal(t, &ByteArray{Data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 24}},
		retEvent.Address)
	assert.Equal(t, argEvent.Message.String()+"-suffix", retEvent.Message.String())
	assert.Equal(t, argEvent.IntKey20, retEvent.IntKey20)
	assert.Equal(t, argEvent.IntKey21, retEvent.IntKey21)
	assert.Equal(t, I8(-111), retEvent.IntKey22)

	//fmt.Println("result event:", retEvent)
}

func Test_abort(t *testing.T) {
	//export function testAbort(arr: Array<i32>, i: i32): i32 {
	//  return arr[i]
	//}
	inst := newTestInst("testInst").
		MustExportFunction("testAbort", (func(*ByteArray, I32) U8)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	for i := 0; i < 4; i++ {
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testAbort",
				Logger:         log.With(),
			},
			MustBuildByteArrayFromHex("0x00010203"), I32(i))
		assert.NoError(t, err)
		assert.Equal(t, U8(i), result)
	}
	for _, i := range []int{-1, 4, 5} {
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testAbort",
				Logger:         log.With(),
			},
			MustBuildByteArrayFromHex("0x00010203"), I32(i))
		assert.Nil(t, result)
		assert.NotNil(t, err)
		var errCallingImportFunc *ErrCallingImportFunc
		assert.True(t, errors.As(err, &errCallingImportFunc))
		assert.True(t, strings.Contains(errCallingImportFunc.Err.Error(), "Index out of range"))
		//log.Infof("err:%#v, errCallingImportFunc:%#v", err, errCallingImportFunc.Err)
	}
	runtime.GC()
	for i := 0; i < 4; i++ {
		result, _, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testAbort",
				Logger:         log.With(),
			},
			MustBuildByteArrayFromHex("0x00010203"), I32(i))
		assert.NoError(t, err)
		assert.Equal(t, U8(i), result)
	}
}

func Test_divZero(t *testing.T) {
	//export function testDivZero(a: i32, b: i32): i32 {
	//  return a / b
	//}
	inst := newTestInst("testInst").
		MustExportFunction("testDivZero", (func(I32, I32) I32)(nil)).
		SetDebugLevel(testDebugLevel)
	defer inst.Close()

	assert.NoError(t, inst.Init(log.With()))

	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testDivZero",
				Logger:         log.With(),
			},
			I32(123), I32(12))
		assert.NoError(t, err)
		assert.Equal(t, I32(10), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testDivZero",
				Logger:         log.With(),
			},
			I32(123), I32(0))
		assert.Nil(t, result)
		assert.NotNil(t, err)
		assert.True(t, errors.Is(err, ErrPanic))
		log.Infof("err: %#v", err)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
	runtime.GC()
	{
		result, report, err := inst.CallExportFunction(
			NewCallContext[testCtxData](context.Background()),
			CallParams[testCtxData]{
				ExportFuncName: "testDivZero",
				Logger:         log.With(),
			},
			I32(321), I32(32))
		assert.NoError(t, err)
		assert.Equal(t, I32(10), result)
		log.Infof("report:%#v, memUsed:%d", report, inst.memoryMgr.memoryUsed)
	}
}

func Test_recursion(t *testing.T) {
	const maxDeep = 5
	//export function testRecursion() {
	//  log.info("call testRecursion", [])
	//}
	inst1 := newTestInst("testInst1").MustExportFunction("testRecursion", (func())(nil))
	inst2 := newTestInst("testInst2").MustExportFunction("testRecursion", (func())(nil))
	inst1.MustImportFunction("index", "log.log",
		func(ctx *CallContext[testCtxData], level I32, msg *String) {
			if strings.Contains(string(*msg), "call testRecursion") {
				top := ctx.TopParams()
				deep := ctx.CallStackDeep()
				ctx.Logger().Infof("called 'log.log' [level:%v][msg:%v] now recursion deep = %d", level, msg, deep)
				if deep < maxDeep {
					_, _, err := inst2.CallExportFunction(ctx, CallParams[testCtxData]{
						ExportFuncName: "testRecursion",
						Logger:         top.Logger,
						Data:           testCtxData(fmt.Sprintf("%s/%d", top.Data, deep)),
					})
					if err != nil {
						panic(err)
					}
				}
				return
			}
			ctx.Logger().Infof("called 'log.log' [level:%v][msg:%v]", level, msg)
		})
	inst2.MustImportFunction("index", "log.log",
		func(ctx *CallContext[testCtxData], level I32, msg *String) {
			if strings.Contains(string(*msg), "call testRecursion") {
				top := ctx.TopParams()
				deep := ctx.CallStackDeep()
				ctx.Logger().Infof("called 'log.log' [level:%v][msg:%v] now recursion deep = %d", level, msg, deep)
				if deep < maxDeep {
					_, _, err := inst1.CallExportFunction(ctx, CallParams[testCtxData]{
						ExportFuncName: "testRecursion",
						Logger:         top.Logger,
						Data:           testCtxData(fmt.Sprintf("%s/%d", top.Data, deep)),
					})
					if err != nil {
						panic(err)
					}
				}
				return
			}
			ctx.Logger().Infof("called 'log.log' [level:%v][msg:%v]", level, msg)
		})

	inst1.SetDebugLevel(testDebugLevel)
	inst2.SetDebugLevel(testDebugLevel)
	defer inst1.Close()
	defer inst2.Close()
	assert.NoError(t, inst1.Init(log.With()))
	assert.NoError(t, inst2.Init(log.With()))

	_, report, err := inst1.CallExportFunction(
		NewCallContext[testCtxData](context.Background()),
		CallParams[testCtxData]{
			ExportFuncName: "testRecursion",
			Logger:         log.With(),
			Data:           "init",
		})
	assert.NoError(t, err)
	assert.Equal(t, uint(4), report.ExportFuncCalled)
	assert.Equal(t, uint(5), report.ImportFuncCalled)
	assert.True(t, report.ImportFuncCallUsed > report.TimeUsed) // because recursion
	assert.Nil(t, inst1.callCtx)
	assert.Nil(t, inst2.callCtx)
	log.Infof("report: %#v", report)
}
