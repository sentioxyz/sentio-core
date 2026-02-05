package wasm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/wasmerio/wasmer-go/wasmer"
)

const (
	MemoryName           = "memory"
	AllocateFunctionName = "allocate"
)

const (
	RTIDString = iota + 1
	RTIDByteArray
	RTIDByteArrayData
	RTIDBaseArray
	RTIDBaseArrayData
	RTIDObjectArray
	RTIDObjectArrayData
	RTIDObject
)

const (
	BlockHeadSize  = 4
	ObjectHeadSize = 16

	memAlignSize = 16
)

type MemoryManager struct {
	// If the total memory in wasm module used more than memHardLimit, we will reset the memory space
	memHardLimit uint32

	allocateFunc wasmer.NativeFunction
	memory       *wasmer.Memory

	memoryUsedInitial uint32
	memoryUsed        uint32
}

func (inst *Instance[DATA]) prepareMemoryManager(memHardLimit uint32) (err error) {
	inst.memoryMgr = &MemoryManager{
		memHardLimit: memHardLimit,
	}
	inst.memoryMgr.allocateFunc, err = inst.instance.Exports.GetFunction(AllocateFunctionName)
	if err != nil {
		return fmt.Errorf("get allocate function %q failed: %w", AllocateFunctionName, err)
	}
	inst.memoryMgr.memory, err = inst.instance.Exports.GetMemory(MemoryName)
	if err != nil {
		return fmt.Errorf("get memory %q failed: %w", MemoryName, err)
	}
	return nil
}

func (mm *MemoryManager) init() {
	mm.setMemoryUsed()
	mm.memoryUsedInitial = mm.memoryUsed
}

func (mm *MemoryManager) setMemoryUsed() {
	mm.memoryUsed = uint32(mm.allocate(0)) - BlockHeadSize + memAlignSize
}

func (mm *MemoryManager) GetMemory() []byte {
	return mm.memory.Data()
}

func (mm *MemoryManager) allocate(size uint32) Pointer {
	mmp, err := mm.allocateFunc(int32(size))
	if err != nil {
		panic(fmt.Errorf("allocate memory with size %d failed: %w", size, err))
	}
	return Pointer(mmp.(int32))
}

// NewMemory more about head see: https://www.assemblyscript.org/runtime.html#memory-layout
// Prioritize the use of reserved memory
func (mm *MemoryManager) NewMemory(rtSize, rtID uint32) Pointer {
	fullSize := rtSize + ObjectHeadSize
	p := mm.allocate(fullSize)
	writeArray(mm.GetMemory()[p:], []any{U32(0), U32(0), U32(rtID), U32(rtSize)}) // write object head
	return p + ObjectHeadSize
}

func buildInvalidFieldError(objType reflect.Type, fieldNum int) error {
	return fmt.Errorf("type of %s.%s is %v, neither a BaseType nor an Object",
		objType.Name(), objType.Field(fieldNum).Name, objType.Field(fieldNum).Type)
}

func (mm *MemoryManager) DumpObject(obj any) Pointer {
	rv := reflect.ValueOf(obj)
	if rv.IsNil() {
		return 0
	}
	rv = rv.Elem()
	rt := rv.Type()
	var buf bytes.Buffer
	for i := 0; i < rt.NumField(); i++ {
		fieldBuf, ok := mm.DumpGoValue(rv.Field(i))
		if !ok {
			panic(buildInvalidFieldError(rt, i))
		}
		offset := extendSize(buf.Len(), len(fieldBuf))
		if offset > buf.Len() {
			buf.Write(make([]byte, offset-buf.Len()))
		}
		buf.Write(fieldBuf)
		//fmt.Printf("Field[%d](%v/%v)[%d:%d] %v\n",
		//	i, rv.Field(i).Type(), rt.Field(i).Type, offset, offset+len(fieldBuf), rv.Field(i).Interface())
	}
	p := mm.NewMemory(uint32(buf.Len()), RTIDObject)
	copy(mm.GetMemory()[p:p+Pointer(buf.Len())], buf.Bytes())
	return p
}

func (mm *MemoryManager) LoadObject(p Pointer, obj any) {
	memory := mm.GetMemory()
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		panic(fmt.Errorf("load object failed: obj must be a non-null pointer"))
	}
	rv = rv.Elem()
	rt := reflect.TypeOf(obj).Elem()
	var offset int
	for i := 0; i < rt.NumField(); i++ {
		v, fieldSize := mm.LoadGoValue(rv.Field(i), func(fieldSize int) []byte {
			return memory[p+Pointer(extendSize(offset, fieldSize)):]
		})
		if fieldSize == 0 {
			panic(buildInvalidFieldError(rt, i))
		}
		rv.Field(i).Set(v)
		offset = extendSize(offset, fieldSize)
		//fmt.Printf("Field[%d](%v/%v)[%d:%d] %v\n",
		//	i, rv.Field(i).Type(), rt.Field(i).Type, offset, offset+fieldSize, rv.Field(i).Interface())
		offset += fieldSize
	}
}

func (inst *Instance[DATA]) debugShowMemoryReview(out ...io.Writer) {
	if inst.debugLevel < DebugLevelMem {
		return
	}
	var w io.Writer = os.Stdout
	if len(out) > 0 {
		w = out[0]
	}
	mem := inst.memoryMgr.GetMemory()
	printBytes := func(b []byte) string {
		var buf bytes.Buffer
		for i := 0; i < len(b); i++ {
			if i >= 4096 {
				buf.WriteString(fmt.Sprintf(" ignored %d bytes", len(b)-4096))
				break
			}
			buf.WriteString(fmt.Sprintf(" %3d", b[i]))
		}
		return buf.String()
	}
	printBlock := func(blockStart int) int {
		if blockStart >= len(mem) {
			_, _ = fmt.Fprintf(w, "!!! blockStart > len(mem) : %d > %d\n", blockStart, len(mem))
			return -1
		}
		blockSize := int(readBits(mem[blockStart:], BlockHeadSize))
		if blockSize == 0 {
			return -1
		}
		if blockStart+BlockHeadSize+blockSize >= len(mem) {
			_, _ = fmt.Fprintf(w, "!!! blockStart + BlockHeadSize + blockSize >= len(mem) : %d + %d + %d >= %d\n",
				blockStart, BlockHeadSize, blockSize, len(mem))
			return -1
		}
		if blockSize+BlockHeadSize == memAlignSize {
			// empty block, no object
			_, _ = fmt.Fprintf(w, "[%05d..%05d.........%05d]:%s |%s {empty block}\n",
				blockStart,
				blockStart+BlockHeadSize,
				blockStart+BlockHeadSize+blockSize,
				printBytes(mem[blockStart:blockStart+BlockHeadSize]),
				printBytes(mem[blockStart+BlockHeadSize:blockStart+BlockHeadSize+blockSize]),
			)
		} else {
			//_, _ = fmt.Fprintf(w, "blockStart:%d, blockSize:%d\n", blockStart, blockSize)
			_, _ = fmt.Fprintf(w, "[%05d..%05d..%05d..%05d]:%s |%s |%s\n",
				blockStart,
				blockStart+BlockHeadSize,
				blockStart+BlockHeadSize+ObjectHeadSize,
				blockStart+BlockHeadSize+blockSize,
				printBytes(mem[blockStart:blockStart+BlockHeadSize]),
				printBytes(mem[blockStart+BlockHeadSize:blockStart+BlockHeadSize+ObjectHeadSize]),
				printBytes(mem[blockStart+BlockHeadSize+ObjectHeadSize:blockStart+BlockHeadSize+blockSize]),
			)
		}
		return blockStart + BlockHeadSize + blockSize
	}

	_, _ = fmt.Fprintf(w, "============================================================\n")
	_, _ = fmt.Fprintf(w, "MEM LEN / CAP : %d / %d\n", len(mem), cap(mem))
	_, _ = fmt.Fprintf(w, "--- NORMAL MEM [%d .. ]\n", inst.memoryMgr.memoryUsedInitial)
	for s := int(inst.memoryMgr.memoryUsedInitial); s >= 0; {
		s = printBlock(s)
	}
	_, _ = fmt.Fprintf(w, "============================================================\n")
}
