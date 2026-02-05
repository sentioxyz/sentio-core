package wasm

import (
	"reflect"

	"github.com/wasmerio/wasmer-go/wasmer"
)

func ConvertType(tpe reflect.Type) (wasmer.ValueKind, bool) {
	if tpe.Kind() != reflect.Pointer {
		// not a pointer, may be an BaseType
		switch reflect.New(tpe).Elem().Interface().(type) {
		case I8, I16, I32, U8, U16, U32, Bool:
			return wasmer.I32, true
		case I64, U64:
			return wasmer.I64, true
		case F32:
			return wasmer.F32, true
		case F64:
			return wasmer.F64, true
		default:
			return wasmer.I32, false
		}
	} else {
		// a pointer, may be an Object
		_, ok := reflect.New(tpe.Elem()).Interface().(Object)
		return wasmer.I32, ok
	}
}

func (mm *MemoryManager) ToGoValue(value wasmer.Value, tpe reflect.Type) (v reflect.Value, ok bool) {
	if tpe.Kind() != reflect.Pointer {
		// not a pointer, may be an BaseType
		v = reflect.New(tpe).Elem()
		ok = true
		switch v.Interface().(type) {
		case Bool:
			v.SetBool(value.I32() != 0)
		case I8, I16, I32:
			v.SetInt(int64(value.I32()))
		case U8, U16, U32:
			v.SetUint(uint64(uint32(value.I32())))
		case I64:
			v.SetInt(value.I64())
		case U64:
			v.SetUint(uint64(value.I64()))
		case F32:
			v.SetFloat(float64(value.F32()))
		case F64:
			v.SetFloat(value.F64())
		default:
			ok = false
		}
	} else {
		// a pointer, may be an Object
		if p := value.I32(); p == 0 {
			return reflect.Zero(tpe), true
		} else {
			v = reflect.New(tpe.Elem())
			var obj Object
			obj, ok = v.Interface().(Object)
			if ok {
				obj.Load(mm, Pointer(p))
			}
		}
	}
	return
}

func (mm *MemoryManager) FromGoValue(value reflect.Value) (wasmer.Value, bool) {
	if value.Type().Kind() != reflect.Pointer {
		// not a pointer, may be an BaseType
		switch v := value.Interface().(type) {
		case Bool:
			if v {
				return wasmer.NewI32(1), true
			} else {
				return wasmer.NewI32(0), true
			}
		case I8:
			return wasmer.NewI32(int32(v)), true
		case I16:
			return wasmer.NewI32(int32(v)), true
		case I32:
			return wasmer.NewI32(int32(v)), true
		case U8:
			return wasmer.NewI32(int32(v)), true
		case U16:
			return wasmer.NewI32(int32(v)), true
		case U32:
			return wasmer.NewI32(int32(v)), true
		case I64:
			return wasmer.NewI64(int64(v)), true
		case U64:
			return wasmer.NewI64(int64(v)), true
		case F32:
			return wasmer.NewF32(float32(v)), true
		case F64:
			return wasmer.NewF64(float64(v)), true
		default:
			return wasmer.NewI32(0), false
		}
	} else {
		// a pointer, may be an Object
		obj, ok := value.Interface().(Object)
		if !ok {
			return wasmer.NewI32(0), false
		}
		if value.IsNil() {
			return wasmer.NewI32(0), true
		}
		p := obj.Dump(mm)
		return wasmer.NewI32(int32(p)), true
	}
}

func (mm *MemoryManager) DumpGoValue(value reflect.Value) ([]byte, bool) {
	if value.Type().Kind() != reflect.Pointer {
		// not a pointer, may be an BaseType
		switch v := value.Interface().(type) {
		case Bool, I8, I16, I32, I64, U8, U16, U32, U64, F32, F64:
			buf := make([]byte, sizeof(v))
			writeOne(buf, v)
			return buf, true
		default:
			return nil, false
		}
	} else {
		// a pointer, may be an Object
		buf := make([]byte, PointerSize)
		if value.IsNil() {
			writeOne(buf, Pointer(0))
			return buf, true
		}
		obj, ok := value.Interface().(Object)
		if !ok {
			return nil, false
		}
		writeOne(buf, obj.Dump(mm))
		return buf, true
	}
}

func (mm *MemoryManager) LoadGoValue(value reflect.Value, memGetter func(size int) []byte) (reflect.Value, int) {
	if value.Type().Kind() != reflect.Pointer {
		// not a pointer, may be an BaseType
		switch v := value.Interface().(type) {
		case Bool, I8, I16, I32, I64, U8, U16, U32, U64, F32, F64:
			size := sizeof(v)
			return reflect.ValueOf(readOne(memGetter(size), size, v)), size
		default:
			return reflect.Zero(value.Type()), 0
		}
	} else {
		// a pointer, may be an Object
		if _, ok := value.Interface().(Object); !ok {
			return reflect.Zero(value.Type()), 0
		}
		p := readOne(memGetter(PointerSize), PointerSize, Pointer(0)).(Pointer)
		if p == 0 {
			return reflect.Zero(value.Type()), PointerSize
		}
		v := reflect.New(value.Type().Elem())
		obj := v.Interface().(Object)
		obj.Load(mm, p)
		return v, PointerSize
	}
}
