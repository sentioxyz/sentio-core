package common

import (
	"encoding/json"
	"github.com/pkg/errors"
	"math/big"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"strings"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/common/numbers.ts

type BigInt struct {
	big.Int
}

func BuildBigInt(x any) (*BigInt, error) {
	var r BigInt
	var err error
	switch b := x.(type) {
	case *big.Int:
		if b == nil {
			return nil, nil
		}
		r.Set(b)
	case string:
		switch b {
		case "", "0x", "-0x", "0x-":
			// will be treated as 0
		default:
			var ok bool
			if strings.HasPrefix(b, "0x") {
				// 0x-1 will be ok
				_, ok = r.SetString(b[2:], 16)
			} else if strings.HasPrefix(b, "-0x") {
				_, ok = r.SetString(b[3:], 16)
				r = *r.Neg()
			} else {
				_, ok = r.SetString(b, 10)
			}
			if !ok {
				err = errors.Errorf("invalid big int %q", b)
			}
		}
	case int64:
		r.SetInt64(b)
	case int32:
		r.SetInt64(int64(b))
	case int16:
		r.SetInt64(int64(b))
	case int8:
		r.SetInt64(int64(b))
	case int:
		r.SetInt64(int64(b))
	case uint64:
		r.SetUint64(b)
	case uint32:
		r.SetUint64(uint64(b))
	case uint16:
		r.SetUint64(uint64(b))
	case uint8:
		r.SetUint64(uint64(b))
	case uint:
		r.SetUint64(uint64(b))
	case []byte:
		r.fromBytes(b)
	default:
		err = errors.Errorf("invalid value %T %v", x, x)
	}
	return &r, err
}

var Zero = &BigInt{}

func MustBuildBigInt(x any) *BigInt {
	return utils.MustReturn(BuildBigInt(x))
}

// calcComplement compute the complement
func calcComplement(x []byte) []byte {
	for i := 0; i < len(x); i++ {
		x[i] = ^x[i]
	}
	end := false
	for i := 0; i < len(x) && !end; i++ {
		if x[i] == 0xff {
			x[i] = 0
		} else {
			x[i]++
			end = true
		}
	}
	return x
}

func trimBytes(x []byte) []byte {
	size := len(x)
	if size == 0 {
		return x
	}
	if x[size-1] > 127 {
		for size >= 2 && x[size-1] == 0xff && x[size-2] > 127 {
			size--
		}
	} else {
		for size >= 2 && x[size-1] == 0 && x[size-2] <= 127 {
			size--
		}
	}
	return x[:size]
}

func (bi *BigInt) toBytes() []byte {
	switch bi.Int.Sign() {
	case -1:
		return trimBytes(calcComplement(append(utils.Reverse(bi.Int.Bytes()), 0)))
	case 1:
		return trimBytes(append(utils.Reverse(bi.Int.Bytes()), 0))
	default: // 0
		return []byte{}
	}
}

func (bi *BigInt) fromBytes(b []byte) {
	buf := make([]byte, len(b))
	copy(buf, b)
	buf = trimBytes(buf)
	if len(b) > 0 && b[len(b)-1] > 127 {
		bi.Int.SetBytes(utils.Reverse(calcComplement(buf)))
		bi.Int.Neg(&bi.Int)
	} else {
		bi.Int.SetBytes(utils.Reverse(buf))
	}
}

func (bi *BigInt) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	var payload wasm.ByteArray
	payload.Data = bi.toBytes()
	return payload.Dump(mm)
}

func (bi *BigInt) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	var payload wasm.ByteArray
	payload.Load(mm, p)
	bi.fromBytes(payload.Data)
}

func (bi *BigInt) MarshalJSON() ([]byte, error) {
	if bi == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(bi.ToHex())
}

func (bi *BigInt) ToBigInt() *big.Int {
	return &bi.Int
}

func (bi *BigInt) ToHex() string {
	if bi.Sign() < 0 {
		return "-0x" + bi.Text(16)[1:]
	} else {
		return "0x" + bi.Text(16)
	}
}

func (bi *BigInt) Cmp(x *BigInt) int {
	return bi.Int.Cmp(&x.Int)
}

// Neg return bi * -1
func (bi *BigInt) Neg() *BigInt {
	var r BigInt
	r.Int.Neg(&bi.Int)
	return &r
}

func (bi *BigInt) Plus(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Add(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) Minus(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Sub(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) Times(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Mul(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) DividedBy(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Div(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) Mod(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Mod(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) Pow(exp wasm.U8) *BigInt {
	var r BigInt
	r.Int.Exp(&bi.Int, big.NewInt(int64(exp)), nil)
	return &r
}

func (bi *BigInt) BitAnd(x *BigInt) *BigInt {
	var r BigInt
	r.Int.And(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) BitOr(x *BigInt) *BigInt {
	var r BigInt
	r.Int.Or(&bi.Int, &x.Int)
	return &r
}

func (bi *BigInt) LeftShift(x wasm.U8) *BigInt {
	var r BigInt
	r.Int.Lsh(&bi.Int, uint(x))
	return &r
}

func (bi *BigInt) RightShift(x wasm.U8) *BigInt {
	var r BigInt
	r.Int.Rsh(&bi.Int, uint(x))
	return &r
}
