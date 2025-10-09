package money

import (
	"github.com/shopspring/decimal"
	"google.golang.org/genproto/googleapis/type/money"
)

func MoneyToDecimal(m *money.Money) decimal.Decimal {

	units := decimal.NewFromInt(m.Units)
	nanos := decimal.NewFromInt(int64(m.Nanos)).Div(decimal.NewFromInt(1_000_000_000))
	return units.Add(nanos)
}

func DecimalToMoney(d decimal.Decimal, currency string) *money.Money {
	units := d.IntPart()
	nanos := d.Sub(decimal.NewFromInt(units)).Mul(decimal.NewFromInt(1_000_000_000)).IntPart()

	return &money.Money{
		Units:        units,
		Nanos:        int32(nanos),
		CurrencyCode: currency,
	}
}
