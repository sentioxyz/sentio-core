package manifest

import (
	"fmt"
	"math/big"
	"strings"
)

type BigInt struct {
	big.Int
}

func BuildBigIntFromUint(x uint64) BigInt {
	var b BigInt
	b.SetUint64(x)
	return b
}

func (b *BigInt) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%#x\"", &b.Int)), nil
}

func (b *BigInt) setString(str string) (ok bool) {
	if strings.HasPrefix(str, "0x") {
		_, ok = b.SetString(str[2:], 16)
	} else {
		_, ok = b.SetString(str, 10)
	}
	return
}

func (b *BigInt) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		b.SetInt64(0)
		return nil
	}
	var str string
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		str = string(p[1 : len(p)-1])
	} else {
		str = string(p)
	}
	if !b.setString(str) {
		return fmt.Errorf("not a valid big integer: %s", p)
	}
	return nil
}

func (b *BigInt) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%#x", &b.Int), nil
}

func (b *BigInt) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var n int64
	var err error
	if err = unmarshal(&n); err == nil {
		b.SetInt64(n)
		return nil
	}
	var s string
	if err = unmarshal(&s); err == nil {
		if !b.setString(s) {
			return fmt.Errorf("not a valid big integer: %q", s)
		}
		return nil
	}
	return err
}
