package money

import (
	"testing"

	"github.com/shopspring/decimal"
	"google.golang.org/genproto/googleapis/type/money"
)

func TestMoneyToDecimal(t *testing.T) {
	m := &money.Money{
		CurrencyCode: "USD",
		Units:        123,
		Nanos:        450_000_000,
	}
	expected := decimal.RequireFromString("123.45")
	actual := MoneyToDecimal(m)

	if !actual.Equal(expected) {
		t.Errorf("Expected %s, but got %s", expected.StringFixed(2), actual.StringFixed(2))
	}
}

func TestDecimalToMoney(t *testing.T) {
	d := decimal.RequireFromString("67.89")
	expected := &money.Money{
		CurrencyCode: "USD",
		Units:        67,
		Nanos:        890_000_000,
	}
	actual := DecimalToMoney(d, "USD")

	if actual.Units != expected.Units || actual.Nanos != expected.Nanos {
		t.Errorf("Expected %v, but got %v", expected, actual)
	}
}
