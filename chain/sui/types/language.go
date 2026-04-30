package types

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/fardream/go-bcs/bcs"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/sui/types/serde"
	"sentioxyz/sentio-core/common/log"
)

type StructTag struct {
	Address  Address   `json:"address"`
	Module   string    `json:"module,omitempty"`
	Name     string    `json:"name,omitempty"`
	TypeArgs []TypeTag `json:"typeArguments,omitempty"`
}

func (s StructTag) String() string {
	return s.Text(true)
}

func (s StructTag) Text(withArgs bool, prologues ...func(TypeTag) (string, bool)) string {
	var sb strings.Builder
	sb.WriteString(s.Address.ShortString())
	if s.Module != "" {
		sb.WriteString("::")
		sb.WriteString(s.Module)
	}
	if s.Name != "" {
		sb.WriteString("::")
		sb.WriteString(s.Name)
	}
	if withArgs && len(s.TypeArgs) > 0 {
		sb.WriteRune('<')
		for i, t := range s.TypeArgs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(t.Text(true, prologues...))
		}
		sb.WriteRune('>')
	}
	return sb.String()
}

func StructTagFromString(s string) (*StructTag, error) {
	var (
		addressStr  string
		module      string
		name        string
		typeArgsStr []string
	)
	sb := &strings.Builder{}
	parseState := 0
	stack := 0
	i := 0
	for i < len(s) {
		switch parseState {
		case 0:
			if s[i] == ':' && s[i+1] == ':' {
				addressStr = sb.String()
				sb.Reset()
				parseState = 1
				i++
			} else {
				sb.WriteByte(s[i])
			}
		case 1:
			if s[i] == ':' && s[i+1] == ':' {
				module = sb.String()
				sb.Reset()
				parseState = 2
				i++
			} else {
				sb.WriteByte(s[i])
			}
		case 2:
			if s[i] == '<' {
				name = sb.String()
				sb.Reset()
				parseState = 3
				stack = 0
			} else {
				sb.WriteByte(s[i])
			}
		case 3:
			if s[i] == ' ' {
				i++
				continue
			}
			if stack > 0 {
				sb.WriteByte(s[i])
				switch s[i] {
				case '<':
					stack++
				case '>':
					if i >= len(s)-1 || s[i+1] == '>' || s[i+1] == ',' {
						stack--
					}
				}
			} else {
				switch s[i] {
				case '<':
					sb.WriteByte(s[i])
					stack++
				case ',':
					typeArgsStr = append(typeArgsStr, sb.String())
					sb.Reset()
				case '>':
					if i >= len(s)-1 || s[i+1] == '>' || s[i+1] == ',' {
						typeArgsStr = append(typeArgsStr, sb.String())
						sb.Reset()
					} else {
						sb.WriteByte(s[i])
					}
				default:
					sb.WriteByte(s[i])
				}
			}
		}
		i++
	}
	switch parseState {
	case 0:
		addressStr = sb.String()
	case 1:
		module = sb.String()
	case 2:
		name = sb.String()
	case 3:
	default:
		return nil, errors.New("invalid parse state")
	}
	var typeArgs []TypeTag
	address, err := StrToAddress(addressStr)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("address: %s", addressStr))
	}
	for _, s := range typeArgsStr {
		tt, err := TypeTagFromString(s)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("type arg: %s", s))
		}
		typeArgs = append(typeArgs, *tt)
	}
	return &StructTag{
		Address:  address,
		Module:   module,
		Name:     name,
		TypeArgs: typeArgs,
	}, nil
}

func (s *StructTag) UnmarshalJSON(data []byte) error {
	var sts string
	if err := json.Unmarshal(data, &sts); err != nil {
		return err
	}
	st, err := StructTagFromString(sts)
	if err != nil {
		return err
	}
	*s = *st
	return nil
}

func (s StructTag) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *StructTag) Include(other *StructTag) bool {
	if s.Address != other.Address || s.Module != other.Module || s.Name != other.Name {
		return false
	}
	if len(s.TypeArgs) == 0 {
		return true
	}
	if len(s.TypeArgs) != len(other.TypeArgs) {
		return false
	}
	for i := range s.TypeArgs {
		if !s.TypeArgs[i].Include(other.TypeArgs[i]) {
			return false
		}
	}
	return true
}

type TypeTag struct {
	Bool, U8, U64, U128, Address, Signer bool
	Vector                               *TypeTag
	Struct                               *StructTag
	U16, U32, U256                       bool
	Any                                  bool
}

var TypeUnresolved = (*TypeTag)(nil)

func TypeTagFromStringMust(s string) TypeTag {
	t, err := TypeTagFromString(s)
	if err != nil {
		panic(err)
	}
	return *t
}

func TypeTagFromStringOrNil(s string) *TypeTag {
	t, err := TypeTagFromString(s)
	if err != nil {
		return nil
	}
	return t
}

func TypeTagFromString(s string) (*TypeTag, error) {
	switch s {
	case "bool":
		return &TypeTag{Bool: true}, nil
	case "u8":
		return &TypeTag{U8: true}, nil
	case "u64":
		return &TypeTag{U64: true}, nil
	case "u128":
		return &TypeTag{U128: true}, nil
	case "address":
		return &TypeTag{Address: true}, nil
	case "signer":
		return &TypeTag{Signer: true}, nil
	case "u16":
		return &TypeTag{U16: true}, nil
	case "u32":
		return &TypeTag{U32: true}, nil
	case "u256":
		return &TypeTag{U256: true}, nil
	case "any":
		return &TypeTag{Any: true}, nil
	default:
		if strings.HasPrefix(s, "vector<") && strings.HasSuffix(s, ">") {
			t, err := TypeTagFromString(s[7 : len(s)-1])
			if err != nil {
				return nil, err
			}
			return &TypeTag{Vector: t}, nil
		}
		if strings.HasPrefix(s, "0x") {
			t, err := StructTagFromString(s)
			if err != nil {
				return nil, err
			}
			return &TypeTag{Struct: t}, nil
		}
		return nil, fmt.Errorf("invalid type tag: %s", s)
	}
}

func (s TypeTag) Text(withStructArgs bool, prologues ...func(TypeTag) (string, bool)) string {
	for _, prologue := range prologues {
		if txt, done := prologue(s); done {
			return txt
		}
	}
	if s.Bool {
		return "bool"
	} else if s.U8 {
		return "u8"
	} else if s.U64 {
		return "u64"
	} else if s.U128 {
		return "u128"
	} else if s.Address {
		return "address"
	} else if s.Signer {
		return "signer"
	} else if s.Vector != nil {
		return fmt.Sprintf("vector<%s>", s.Vector.Text(withStructArgs, prologues...))
	} else if s.Struct != nil {
		return s.Struct.Text(withStructArgs, prologues...)
	} else if s.U16 {
		return "u16"
	} else if s.U32 {
		return "u32"
	} else if s.U256 {
		return "u256"
	} else if s.Any {
		return "any"
	} else {
		return "<invalid TypeTag>"
	}
}

func (s TypeTag) String() string {
	return s.Text(true)
}

func (s *TypeTag) WithoutArgs() *TypeTag {
	if s == nil {
		return nil
	}
	if s.Struct == nil {
		return s
	}
	return &TypeTag{Struct: &StructTag{
		Address: s.Struct.Address,
		Module:  s.Struct.Module,
		Name:    s.Struct.Name,
	}}
}

func (s TypeTag) Include(other TypeTag) bool {
	if s.Any {
		return true
	} else if s.Bool && other.Bool {
		return true
	} else if s.U8 && other.U8 {
		return true
	} else if s.U64 && other.U64 {
		return true
	} else if s.U128 && other.U128 {
		return true
	} else if s.Address && other.Address {
		return true
	} else if s.Signer && other.Signer {
		return true
	} else if s.U16 && other.U16 {
		return true
	} else if s.U32 && other.U32 {
		return true
	} else if s.U256 && other.U256 {
		return true
	} else if s.Struct != nil && other.Struct != nil && s.Struct.Include(other.Struct) {
		return true
	} else if s.Vector != nil && other.Vector != nil && s.Vector.Include(*other.Vector) {
		return true
	}
	return false
}

func AnyInclude(types []TypeTag, target *TypeTag) bool {
	if target == nil {
		return false
	}
	for _, t := range types {
		if t.Include(*target) {
			return true
		}
	}
	return false
}

func (s *TypeTag) UnmarshalJSON(data []byte) error {
	var tts string
	if err := json.Unmarshal(data, &tts); err != nil {
		return err
	}
	tt, err := TypeTagFromString(tts)
	if err != nil {
		return err
	}
	*s = *tt
	return nil
}

func (s TypeTag) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s TypeTag) MarshalBCS() ([]byte, error) {
	var enumID int
	buf := bytes.NewBuffer(nil)
	if s.Bool {
		enumID = 0
	} else if s.U8 {
		enumID = 1
	} else if s.U64 {
		enumID = 2
	} else if s.U128 {
		enumID = 3
	} else if s.Address {
		enumID = 4
	} else if s.Signer {
		enumID = 5
	} else if s.Vector != nil {
		enumID = 6
	} else if s.Struct != nil {
		enumID = 7
	} else if s.U16 {
		enumID = 8
	} else if s.U32 {
		enumID = 9
	} else if s.U256 {
		enumID = 10
	} else {
		return nil, fmt.Errorf("invalid TypeTag: %v", s)
	}
	buf.Write(bcs.ULEB128Encode(enumID))
	if s.Vector != nil {
		serde.Encode(buf, s.Vector)
	} else if s.Struct != nil {
		serde.Encode(buf, s.Struct)
	}
	return buf.Bytes(), nil
}

func (s *TypeTag) UnmarshalBCS(r io.Reader) (int, error) {
	enumID, _, err := bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	switch enumID {
	case 0:
		// Bool
		s.Bool = true
	case 1:
		// U8
		s.U8 = true
	case 2:
		// U64
		s.U64 = true
	case 3:
		// U128
		s.U128 = true
	case 4:
		// Address
		s.Address = true
	case 5:
		// Signer
		s.Signer = true
	case 6:
		// Vector
		s.Vector = new(TypeTag)
		err = serde.Decode(r, s.Vector)
	case 7:
		// Struct
		s.Struct = new(StructTag)
		err = serde.Decode(r, s.Struct)
	case 8:
		// U16
		s.U16 = true
	case 9:
		// U32
		s.U32 = true
	case 10:
		// U256
		s.U256 = true
	default:
		return 0, fmt.Errorf("invalid enum id %d", enumID)
	}
	if serde.Trace {
		log.Debugf("TypeTag: %s", s.String())
	}
	return 0, err
}

func bigIntToFixedBytesLE(b *big.Int, n int) []byte {
	buf := make([]byte, n)
	b.FillBytes(buf)
	for i := 0; i < n/2; i++ {
		buf[i], buf[n-1-i] = buf[n-1-i], buf[i]
	}
	return buf
}

func (s *TypeTag) SerializeBCS(v json.RawMessage) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	decodeString := func() string {
		var s string
		err := json.Unmarshal(v, &s)
		if err != nil {
			panic(err)
		}
		return s
	}
	if s.Bool {
		b := bytes.Equal(v, []byte("true"))
		serde.Encode(buf, b)
	} else if s.U8 {
		// u8 is stored as a number in JSON
		i, err := strconv.ParseUint(string(v), 10, 8)
		if err != nil {
			return nil, err
		}
		serde.Encode(buf, uint8(i))
	} else if s.U64 {
		i, err := strconv.ParseUint(decodeString(), 10, 64)
		if err != nil {
			return nil, err
		}
		serde.Encode(buf, uint64(i))
	} else if s.U128 {
		b := big.NewInt(0)
		_, ok := b.SetString(decodeString(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid u128 value: %s", v)
		}
		buf.Write(bigIntToFixedBytesLE(b, 128/8))
	} else if s.Address {
		addr, err := StrToAddress(decodeString())
		if err != nil {
			return nil, err
		}
		serde.Encode(buf, &addr)
	} else if s.Signer {
		panic("not implelemented Signer decode: " + decodeString())
	} else if s.Vector != nil {
		if s.Vector.U8 && len(v) > 0 && v[0] == '"' {
			// special case: json may be a string
			serde.Encode(buf, []byte(decodeString()))
		} else {
			var elems []json.RawMessage
			err := json.Unmarshal(v, &elems)
			if err != nil {
				return nil, err
			}
			buf.Write(bcs.ULEB128Encode[int](len(elems)))
			for _, elem := range elems {
				serializedElem, err := s.Vector.SerializeBCS(elem)
				if err != nil {
					return nil, err
				}
				buf.Write(serializedElem)
			}
		}
	} else if s.Struct != nil {
		panic("not supported Struct decode: " + decodeString())
	} else if s.U16 {
		i, err := strconv.ParseUint(string(v), 10, 16)
		if err != nil {
			return nil, err
		}
		serde.Encode(buf, uint16(i))
	} else if s.U32 {
		i, err := strconv.ParseUint(string(v), 10, 32)
		if err != nil {
			return nil, err
		}
		serde.Encode(buf, uint32(i))
	} else if s.U256 {
		b := big.NewInt(0)
		_, ok := b.SetString(decodeString(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid u256 value: %s", v)
		}
		buf.Write(bigIntToFixedBytesLE(b, 256/8))
	} else {
		return nil, fmt.Errorf("invalid TypeTag: %v", s)
	}
	return buf.Bytes(), nil
}

func ContainsAnyType(types ...TypeTag) bool {
	for _, typ := range types {
		if typ.Any {
			return true
		}
		if typ.Vector != nil && ContainsAnyType(*typ.Vector) {
			return true
		}
		if typ.Struct != nil && ContainsAnyType(typ.Struct.TypeArgs...) {
			return true
		}
	}
	return false
}

type PureValue struct {
	Value []byte
	json  *pureValueJSON `bcs:"-"`
}

type pureValueJSON struct {
	Type      string          `json:"type"`
	ValueType *TypeTag        `json:"valueType"`
	Value     json.RawMessage `json:"value"`
}

func PureValueFromJSON(value json.RawMessage, valueType *TypeTag) *PureValue {
	return &PureValue{
		json: &pureValueJSON{
			Type:      "pure",
			ValueType: valueType,
			Value:     value,
		},
	}
}

func (s *PureValue) ValueType() *TypeTag {
	if s.json == nil {
		panic("no type information provided")
	}
	return s.json.ValueType
}

func (s *PureValue) ValueJSON() json.RawMessage {
	if s.json == nil {
		panic("no type information provided")
	}
	return s.json.Value
}

func (s *PureValue) RawBytes() ([]byte, error) {
	if s.Value != nil {
		return s.Value, nil
	}
	if s.json == nil {
		return nil, fmt.Errorf("no type information provided")
	}
	var err error
	if s.json.ValueType == TypeUnresolved {
		var bytes []uint8
		err = json.Unmarshal(s.json.Value, &bytes)
		s.Value = bytes
	} else {
		s.Value, err = s.json.ValueType.SerializeBCS(s.json.Value)
	}
	return s.Value, err
}

func (s *PureValue) UnmarshalJSON(data []byte) error {
	var j pureValueJSON
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	if j.Type != "pure" {
		return fmt.Errorf("invalid pure value %s", j.Type)
	}
	s.json = &j
	return nil
}

func (s PureValue) MarshalJSON() ([]byte, error) {
	if s.json != nil {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				log.Errorf("marshal PureValue failed (%v), json:%#v", panicErr, s.json)
				panic(panicErr)
			}
		}()
		return json.Marshal(s.json)
	}
	bytes := make([]uint8, len(s.Value))
	copy(bytes, s.Value)
	j, err := json.Marshal(bytes)
	if err != nil {
		return nil, err
	}
	return json.Marshal(pureValueJSON{
		Type:      "pure",
		ValueType: TypeUnresolved,
		Value:     j,
	})
}

func (s PureValue) MarshalBCS() ([]byte, error) {
	if s.Value == nil {
		if s.json == nil {
			return nil, fmt.Errorf("no type information provided")
		}
		var err error
		s.Value, err = s.json.ValueType.SerializeBCS(s.json.Value)
		if err != nil {
			return nil, err
		}
	}
	return serde.WriteByteSlice(s.Value)
}
