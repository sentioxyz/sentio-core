package wasm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"strconv"
	"strings"
)

type ByteArray struct {
	Data []byte
}

func BuildByteArrayFromHex(hex string) (*ByteArray, error) {
	hex = strings.TrimPrefix(hex, "0x")
	if len(hex)%2 != 0 {
		return nil, errors.Errorf("invalid length of hex string %q", hex)
	}
	data := make([]byte, len(hex)/2)
	for i := 0; i < len(hex); i += 2 {
		w, err := strconv.ParseUint(hex[i:i+2], 16, 32)
		if err != nil {
			return nil, errors.Errorf("invalid word %q", hex[i:i+2])
		}
		data[i/2] = byte(w)
	}
	return &ByteArray{Data: data}, nil
}

func MustBuildByteArrayFromHex(hex string) *ByteArray {
	arr, err := BuildByteArrayFromHex(hex)
	if err != nil {
		panic(err)
	}
	return arr
}

func (arr *ByteArray) MarshalJSON() ([]byte, error) {
	if arr == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(arr.ToHex())
}

func (arr *ByteArray) String() string {
	return arr.ToHex()
}

func (arr *ByteArray) ToHex() string {
	var buf bytes.Buffer
	buf.WriteString("0x")
	if arr != nil && len(arr.Data) > 0 {
		for _, w := range arr.Data {
			buf.WriteString(fmt.Sprintf("%02x", w))
		}
	}
	return buf.String()
}

func (arr *ByteArray) Dump(mm *MemoryManager) Pointer {
	if arr == nil {
		return 0
	}

	arrLen := uint32(len(arr.Data))
	objectSize := uint32(PointerSize*2 + 4)

	obj := mm.NewMemory(objectSize, RTIDByteArray)
	data := mm.NewMemory(arrLen, RTIDByteArrayData)
	memory := mm.GetMemory()

	// Here is the different with BaseArray and ObjectArray, there is just an length, no total size
	writeArray(memory[obj:], []any{data, data, U32(arrLen)})
	copy(memory[data:data+Pointer(arrLen)], arr.Data)
	return obj
}

func (arr *ByteArray) Load(mm *MemoryManager, p Pointer) {
	memory := mm.GetMemory()
	data := readBits(memory[p:], PointerSize)
	length := readBits(memory[p+PointerSize*2:], 4)
	arr.Data = make([]byte, length)
	copy(arr.Data, memory[data:data+length])
}
