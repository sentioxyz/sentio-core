package wasm

import (
	"fmt"
)

type I8 int8
type I16 int16
type I32 int32
type I64 int64
type U8 uint8
type U16 uint16
type U32 uint32
type U64 uint64
type F32 float32
type F64 float64
type Bool bool

type Pointer uint32

type BaseType interface {
	I8 | I16 | I32 | I64 | U8 | U16 | U32 | U64 | F32 | F64 | Bool
}

type Object interface {
	// Dump go runtime memory => wasm runtime memory
	// if self is nil, the method should return 0
	Dump(*MemoryManager) Pointer
	// Load go runtime memory <= wasm runtime memory
	// pointer always not 0 and self always not nil
	Load(*MemoryManager, Pointer)
}

/**
 * more see: https://www.assemblyscript.org/runtime.html#memory-layout
 */
const (
	PointerSize = 4

	// CharSize wasm string use 16-bit char code
	CharSize = 2
)

func sizeof(t any) int {
	switch t.(type) {
	case I8, U8, Bool:
		return 1
	case I16, U16:
		return 2
	case I32, U32, F32:
		return 4
	case I64, U64, F64:
		return 8
	case Pointer:
		return PointerSize
	default:
		panic(fmt.Errorf("unsupport type %T for calling wasm.sizeof", t))
	}
}
