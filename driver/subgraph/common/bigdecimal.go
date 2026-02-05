package common

import (
	"bytes"
	"math/big"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"

	"github.com/shopspring/decimal"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/common/numbers.ts
// https://thegraph.com/docs/en/developing/assemblyscript-api/#bigdecimal

// DecimalMaxPrecision official precision is 34, 128 is much higher than it.
// https://thegraph.com/docs/en/developing/creating-a-subgraph/#graphql-supported-scalars
const DecimalMaxPrecision = 128

// BigDecimal = Digits * 10 ^ Exp
type BigDecimal struct {
	Digits *BigInt
	Exp    *BigInt
}

func BuildBigDecimal(d decimal.Decimal) *BigDecimal {
	exp := d.Exponent()
	return &BigDecimal{
		Digits: MustBuildBigInt(d.Shift(-exp).BigInt()),
		Exp:    MustBuildBigInt(int64(exp)),
	}
}

func BuildBigDecimalFromBigInt(digits *BigInt, exp int64) *BigDecimal {
	return &BigDecimal{
		Digits: digits,
		Exp:    MustBuildBigInt(exp),
	}
}

func BuildBigDecimalFromString(text string) (*BigDecimal, error) {
	if text == "" || text == "." {
		return &BigDecimal{Digits: MustBuildBigInt(""), Exp: MustBuildBigInt("")}, nil
	}
	dec, err := decimal.NewFromString(text)
	if err != nil {
		return nil, err
	}
	exp := dec.Exponent()
	dec = dec.Shift(-dec.Exponent())
	digits := dec.BigInt()
	// if digits part has tail zero, we can raise the exponent and reduce the length of the digits part
	s := digits.Text(10)
	n := len(s)
	for n > 0 && s[n-1] == '0' {
		n--
	}
	if n < len(s) {
		p := int32(len(s) - n)
		exp += p
		digits.SetString(s[:n], 10)
	}
	return &BigDecimal{Digits: MustBuildBigInt(digits), Exp: MustBuildBigInt(exp)}, nil
}

func MustBuildBigDecimalFromString(text string) *BigDecimal {
	return utils.MustReturn(BuildBigDecimalFromString(text))
}

func (d *BigDecimal) ToDecimal() decimal.Decimal {
	return decimal.NewFromBigInt(&d.Digits.Int, int32(d.Exp.Int64()))
}

func (d *BigDecimal) String() string {
	digits := d.Digits.String()
	if exp := int(d.Exp.Int64()); exp > 0 {
		var buf bytes.Buffer
		buf.WriteString(digits)
		for i := 0; i < exp; i++ {
			buf.WriteRune('0')
		}
		return buf.String()
	} else if exp < 0 {
		var buf bytes.Buffer
		if len(digits) <= -exp {
			buf.WriteString("0.")
			for len(digits) < -exp {
				buf.WriteRune('0')
				exp++
			}
			buf.WriteString(digits)
		} else {
			buf.WriteString(digits[:len(digits)+exp])
			buf.WriteRune('.')
			buf.WriteString(digits[len(digits)+exp:])
		}
		return buf.String()
	} else {
		return digits
	}
}

func (d *BigDecimal) TruncateDigits() *BigDecimal {
	ds := d.Digits.Text(10)
	var fullLen = int64(len(ds))
	var cutLen int64
	var plus bool
	var digits BigInt
	if fullLen > DecimalMaxPrecision {
		cutLen = fullLen - DecimalMaxPrecision
		plus = ds[fullLen-cutLen] >= '5'
	}
	var ignore uint8 = '0'
	if plus {
		ignore = '9'
	}
	for cutLen < fullLen && ds[fullLen-cutLen-1] == ignore {
		cutLen++
	}
	if cutLen == 0 {
		return d
	}
	if cutLen == fullLen {
		if plus {
			digits.SetInt64(1)
			return BuildBigDecimalFromBigInt(&digits, d.Exp.Int64()+cutLen)
		} else {
			// zero
			return BuildBigDecimalFromBigInt(&digits, 0)
		}
	}
	// p = 10 ^ cutLen
	var p big.Int
	p.Exp(big.NewInt(10), big.NewInt(cutLen), nil)
	// digits = d.Digits / 10 ^ cutLen
	digits.Div(&d.Digits.Int, &p)
	// rounding
	if plus {
		digits.Add(&digits.Int, big.NewInt(1))
	}
	return BuildBigDecimalFromBigInt(&digits, d.Exp.Int64()+cutLen)
}

func (d *BigDecimal) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(d)
}

func (d *BigDecimal) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, d)
}

func (d *BigDecimal) Plus(x *BigDecimal) *BigDecimal {
	return BuildBigDecimal(d.ToDecimal().Add(x.ToDecimal())).TruncateDigits()
}

func (d *BigDecimal) Minus(x *BigDecimal) *BigDecimal {
	return BuildBigDecimal(d.ToDecimal().Sub(x.ToDecimal())).TruncateDigits()
}

func (d *BigDecimal) Times(x *BigDecimal) *BigDecimal {
	return BuildBigDecimal(d.ToDecimal().Mul(x.ToDecimal())).TruncateDigits()
}

func (d *BigDecimal) DividedBy(x *BigDecimal) *BigDecimal {
	return BuildBigDecimal(d.ToDecimal().DivRound(x.ToDecimal(), DecimalMaxPrecision)).TruncateDigits()
}

func (d *BigDecimal) Equals(x *BigDecimal) bool {
	return d.ToDecimal().Equal(x.ToDecimal())
}
