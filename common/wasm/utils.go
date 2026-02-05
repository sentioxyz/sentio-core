package wasm

import (
	"fmt"
	"math"
)

func readBits(buf []byte, size int) (n uint64) {
	if size > 8 {
		panic(fmt.Errorf("size %d should not more than 8", size))
	}
	for i := 0; i < size; i++ {
		n += uint64(buf[i]) << (8 * i)
	}
	return n
}

func readOne(buf []byte, size int, tpl any) any {
	v := readBits(buf, size)
	switch tpl.(type) {
	case I8:
		return I8(v)
	case I16:
		return I16(v)
	case I32:
		return I32(v)
	case I64:
		return I64(v)
	case U8:
		return U8(v)
	case U16:
		return U16(v)
	case U32:
		return U32(v)
	case U64:
		return U64(v)
	case F32:
		return F32(math.Float32frombits(uint32(v)))
	case F64:
		return F64(math.Float64frombits(v))
	case Bool:
		return Bool(v != 0)
	case Pointer:
		return Pointer(v)
	default:
		panic(fmt.Errorf("unsupport type %T", tpl))
	}
}

func readArray[T BaseType | Pointer](buf []byte, length int) []T {
	var tpl T
	itemSize := sizeof(tpl)
	arr := make([]T, 0, length)
	for i := 0; i < length; i++ {
		it := readOne(buf[i*itemSize:], itemSize, tpl)
		arr = append(arr, it.(T))
	}
	return arr
}

func readRuneArray(buf []byte, length, charSize int) (r []uint16) {
	r = make([]uint16, length)
	for i := 0; i < length; i++ {
		ch := readBits(buf[i*charSize:], charSize)
		r[i] = uint16(ch)
	}
	return r
}

func writeBits(buf []byte, bits uint64, size int) []byte {
	for i := 0; i < size; i++ {
		buf[i] = byte(bits & 0xff)
		bits = bits >> 8
	}
	return buf[size:]
}

func writeOne(buf []byte, data any) []byte {
	switch d := data.(type) {
	case I8:
		return writeBits(buf, uint64(d), 1)
	case I16:
		return writeBits(buf, uint64(d), 2)
	case I32:
		return writeBits(buf, uint64(d), 4)
	case I64:
		return writeBits(buf, uint64(d), 8)
	case U8:
		return writeBits(buf, uint64(d), 1)
	case U16:
		return writeBits(buf, uint64(d), 2)
	case U32:
		return writeBits(buf, uint64(d), 4)
	case U64:
		return writeBits(buf, uint64(d), 8)
	case F32:
		return writeBits(buf, uint64(math.Float32bits(float32(d))), 4)
	case F64:
		return writeBits(buf, math.Float64bits(float64(d)), 8)
	case Bool:
		var v uint64 = 0
		if d {
			v = 1
		}
		return writeBits(buf, v, 1)
	case Pointer:
		return writeBits(buf, uint64(d), PointerSize)
	default:
		panic(fmt.Errorf("unsupport type %T", data))
	}
}

func writeArray[T any](buf []byte, data []T) []byte {
	for i := 0; i < len(data); i++ {
		buf = writeOne(buf, data[i])
	}
	return buf
}

func writeRuneArray(buf []byte, data []uint16, charSize int) []byte {
	for i := 0; i < len(data); i++ {
		buf = writeBits(buf, uint64(data[i]), charSize)
	}
	return buf
}

// extendSize returns the smallest number that is not less than origin and is an integer multiple of size
func extendSize(origin, size int) int {
	return ((origin + size - 1) / size) * size
}
