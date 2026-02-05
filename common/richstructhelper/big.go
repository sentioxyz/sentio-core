package richstructhelper

import (
	"github.com/shopspring/decimal"
	"math/big"
	"sentioxyz/sentio-core/service/common/protos"
)

func fromBigInt(d *protos.BigInteger) *big.Int {
	var intValue big.Int
	intValue.SetBytes(d.GetData())
	if d.GetNegative() {
		intValue.Neg(&intValue)
	}
	return &intValue
}

func fromBigDecimal(d *protos.BigDecimal) decimal.Decimal {
	return decimal.NewFromBigInt(fromBigInt(d.GetValue()), d.GetExp())
}

func buildBigInteger(d *big.Int) *protos.BigInteger {
	return &protos.BigInteger{
		Negative: d.Sign() < 0,
		Data:     d.Bytes(),
	}
}

func buildBigDecimal(d decimal.Decimal) *protos.BigDecimal {
	return &protos.BigDecimal{
		Value: buildBigInteger(d.Shift(-d.Exponent()).BigInt()),
		Exp:   d.Exponent(),
	}
}
