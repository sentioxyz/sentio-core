package wasm

import (
	"encoding/json"
	"reflect"
	"sentioxyz/sentio-core/common/utils"
)

// ========================================
// BaseArray Object
// ----------------------------------------

type BaseArray[T BaseType] struct {
	Data []T
}

func (arr *BaseArray[T]) Dump(mm *MemoryManager) Pointer {
	if arr == nil {
		return 0
	}

	var item T
	itemSize := uint32(sizeof(item))
	arrLen := uint32(len(arr.Data))
	objectSize := uint32(PointerSize*2 + 4*2)

	obj := mm.NewMemory(objectSize, RTIDBaseArray)
	data := mm.NewMemory(itemSize*arrLen, RTIDBaseArrayData)
	memory := mm.GetMemory()

	writeArray(memory[obj:], []any{data, data, U32(itemSize * arrLen), U32(arrLen)})
	writeArray(memory[data:], arr.Data)
	return obj
}

func (arr *BaseArray[T]) Load(mm *MemoryManager, p Pointer) {
	memory := mm.GetMemory()
	data := readBits(memory[p:], PointerSize)
	length := readBits(memory[p+PointerSize*2+4:], 4)
	arr.Data = readArray[T](memory[data:], int(length))
}

// ========================================
// ObjectArray Object
// ----------------------------------------

type ObjectArray[T Object] struct {
	Data []T
}

func (arr *ObjectArray[T]) Dump(mm *MemoryManager) Pointer {
	if arr == nil {
		return 0
	}

	arrLen := uint32(len(arr.Data))
	objectSize := uint32(PointerSize*2 + 4*2)

	obj := mm.NewMemory(objectSize, RTIDObjectArray)
	data := mm.NewMemory(PointerSize*arrLen, RTIDObjectArrayData)
	memory := mm.GetMemory()

	writeArray(memory[obj:], []any{data, data, U32(PointerSize * arrLen), U32(arrLen)})
	for i, item := range arr.Data {
		writeOne(memory[data+Pointer(PointerSize*i):], item.Dump(mm))
	}
	return obj
}

func (arr *ObjectArray[T]) Load(mm *MemoryManager, p Pointer) {
	memory := mm.GetMemory()
	data := readBits(memory[p:], PointerSize)
	length := readBits(memory[p+PointerSize*2+4:], 4)
	itemPointers := readArray[Pointer](memory[data:], int(length))
	arr.Data = make([]T, length)
	var tpl T
	objectType := reflect.ValueOf(tpl).Type().Elem()
	for i := 0; i < int(length); i++ {
		if itemPointers[i] != 0 {
			arr.Data[i] = reflect.New(objectType).Interface().(T)
			arr.Data[i].Load(mm, itemPointers[i])
		}
	}
}

func (arr *ObjectArray[T]) MarshalJSON() ([]byte, error) {
	if arr == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(arr.Data)
}

func (arr *ObjectArray[T]) String() string {
	if arr == nil {
		return "ObjectArray(nil)"
	}
	return "ObjectArray" + utils.ArrSummary(arr.Data, 5)
}
