package utils

import (
	"fmt"
	"math/big"
)

func ParseBigInt(s string) (x *big.Int, ok bool) {
	return new(big.Int).SetString(s, 0)
}

func MustParseBigInt(s string) *big.Int {
	if x, ok := ParseBigInt(s); ok {
		return x
	}
	return big.NewInt(0)
}

func IsBigIntZero(s string) bool {
	if x, ok := ParseBigInt(s); ok {
		return x.Cmp(big.NewInt(0)) == 0
	}
	return true
}

func FormatBigInt16(x *big.Int) string {
	return fmt.Sprintf("%#x", x)
}

func CmpHex(a, b string) int {
	var ai, bi big.Int
	if _, ok := ai.SetString(a, 0); !ok {
		panic(fmt.Errorf("invalid number %q", a))
	}
	if _, ok := bi.SetString(b, 0); !ok {
		panic(fmt.Errorf("invalid number %q", b))
	}
	return ai.Cmp(&bi)
}

func AddBigInt(x, y *big.Int) *big.Int {
	if x == nil {
		return y
	}
	if y == nil {
		return x
	}
	return new(big.Int).Add(x, y)
}

func SubBigInt(x, y *big.Int) *big.Int {
	return AddBigInt(x, new(big.Int).Neg(y))
}
