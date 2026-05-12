package types

import (
	"encoding/hex"
	"strings"

	"github.com/goccy/go-json"
)

type Address ObjectID

func StrToAddressMust(str string) Address {
	a, err := StrToAddress(str)
	if err != nil {
		panic(err)
	}
	return a
}

func StrToAddress(str string) (Address, error) {
	tmp, err := StrToObjectID(str)
	if err != nil {
		return Address{}, err
	}
	return Address(tmp), nil
}

func (a Address) String() string {
	return "0x" + hex.EncodeToString(a[:])
}

func (a Address) ShortString() string {
	return "0x" + strings.TrimLeft(hex.EncodeToString(a[:]), "0")
}

func (a Address) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Address) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	tmp, err := StrToObjectID(str)
	if err == nil {
		*a = Address(tmp)
	}
	return err
}
