package wasm

import (
	"encoding/json"
	"unicode/utf16"
)

type String string

func BuildString(str string) *String {
	s := String(str)
	return &s
}

func BuildStringFromBytes(b []byte) *String {
	r := []rune(string(b))
	rl := len(r)
	for rl > 0 && r[rl-1] == 0 {
		rl--
	}
	return BuildString(string(r[:rl]))
}

func (s *String) Dump(mm *MemoryManager) Pointer {
	if s == nil {
		return 0
	}
	data := utf16.Encode([]rune(*s))
	strLen := uint32(len(data))
	obj := mm.NewMemory(strLen*CharSize, RTIDString)
	memory := mm.GetMemory()
	writeRuneArray(memory[obj:], data, CharSize)
	return obj
}

func (s *String) MarshalJSON() ([]byte, error) {
	if s == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(s.String())
}

func (s *String) Load(mm *MemoryManager, p Pointer) {
	memory := mm.GetMemory()
	size := readBits(memory[p-4:], 4)
	data := readRuneArray(memory[p:], int(size)/CharSize, CharSize)
	str := string(utf16.Decode(data))
	*s = String(str)
}

func (s *String) String() string {
	if s == nil {
		return ""
	}
	return string(*s)
}
