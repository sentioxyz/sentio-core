package types

import (
	"encoding/base64"
	"io"
	"math/big"
	"sentioxyz/sentio-core/chain/sui/types/serde"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/goccy/go-json"
	"github.com/mr-tron/base58/base58"
	"github.com/pkg/errors"
)

type Base64Data []byte

func NewBase64Data(str string) (Base64Data, error) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, errors.Wrap(err, str)
	}
	return Base64Data(data), nil
}

func (h Base64Data) Data() []byte {
	return h
}

func (h Base64Data) Length() int {
	return len(h)
}

func (h Base64Data) String() string {
	return base64.StdEncoding.EncodeToString(h)
}

func (h Base64Data) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

func (h *Base64Data) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return errors.Wrap(err, string(data))
	}
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return errors.Wrap(err, str)
	}
	*h = decoded
	return nil
}

func (h Base64Data) MarshalBCS() ([]byte, error) {
	return serde.WriteByteSlice(h)
}

func (h *Base64Data) UnmarshalBCS(r io.Reader) (int, error) {
	data, err := serde.ReadByteSlice(r)
	if err != nil {
		return 0, err
	}
	*h = data
	return 0, err
}

type HexData hexutil.Bytes

func (a HexData) MarshalBCS() ([]byte, error) {
	return serde.WriteByteSlice(a)
}

func (a *HexData) UnmarshalBCS(r io.Reader) (int, error) {
	var err error
	data, err := serde.ReadByteSlice(r)
	if err != nil {
		return 0, err
	}
	*a = data
	return 0, nil
}

type Number big.Int

func Int64ToNumber(n int64) Number {
	b := big.NewInt(n)
	return Number(*b)
}

func Uint64ToNumber(n uint64) Number {
	b := big.NewInt(0)
	b.SetUint64(n)
	return Number(*b)
}

func PUint64ToPNumber(n *uint64) *Number {
	if n == nil {
		return nil
	}
	r := Uint64ToNumber(*n)
	return &r
}

func StringToNumber(s string) Number {
	b := big.NewInt(0)
	b.SetString(s, 10)
	return Number(*b)
}

func (n *Number) String() string {
	return (*big.Int)(n).String()
}

func (n Number) MarshalJSON() ([]byte, error) {
	b := big.Int(n)
	return json.Marshal(b.String())
}

func (n *Number) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var str string
	if data[0] == '"' {
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
	} else {
		str = string(data)
	}
	_, ok := (*big.Int)(n).SetString(str, 10)
	if !ok {
		return errors.New("invalid number")
	}
	return nil
}

func (n *Number) Uint64() uint64 {
	return (*big.Int)(n).Uint64()
}

func (n *Number) Uint64Pointer() *uint64 {
	if n == nil {
		return nil
	}
	num := (*big.Int)(n).Uint64()
	return &num
}

func (n Number) MarshalBCS() ([]byte, error) {
	return serde.Marshal(uint64(n.Uint64()))
}

func (n *Number) UnmarshalBCS(r io.Reader) (int, error) {
	var v uint64
	if err := serde.Decode(r, &v); err != nil {
		return 0, err
	}
	*n = Uint64ToNumber(v)
	return 0, nil
}

func (n Number) BigInt() *big.Int {
	return (*big.Int)(&n)
}

type Base58Data []byte

func NewBase58Data(str string) (Base58Data, error) {
	data, err := base58.Decode(str)
	if err != nil {
		return nil, errors.Wrap(err, str)
	}
	return Base58Data(data), nil
}

func (h Base58Data) Data() []byte {
	return h
}

func (h Base58Data) Length() int {
	return len(h)
}

func (h Base58Data) String() string {
	return base58.Encode(h)
}

func (h Base58Data) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

func (h *Base58Data) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return errors.Wrap(err, string(data))
	}
	if len(str) == 0 {
		*h = make([]byte, 0)
		return nil
	}
	decoded, err := base58.Decode(str)
	if err != nil {
		return errors.Wrap(err, str)
	}
	*h = decoded
	return nil
}
