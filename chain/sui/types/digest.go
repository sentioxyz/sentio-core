package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/goccy/go-json"
	"github.com/mr-tron/base58/base58"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

const DigestLength = 32

type Digest [DigestLength]byte

func decodeDigestData(str string) ([]byte, error) {
	data, err := base58.Decode(str)
	if err != nil {
		return nil, err
	}
	if len(data) != DigestLength {
		return nil, errors.New("invalid digest length")
	}
	return data, nil
}

func StrToDigest(str string) (Digest, error) {
	data, err := decodeDigestData(str)
	if err != nil {
		return Digest{}, err
	}
	d := Digest{}
	copy(d[:], data)
	return d, nil
}

func StrToDigestMust(str string) Digest {
	data, err := decodeDigestData(str)
	if err != nil {
		panic(err)
	}
	d := Digest{}
	copy(d[:], data)
	return d
}

func StrToDigestOrEmptyMust(str string) Digest {
	if str == "" {
		return Digest{}
	}
	return StrToDigestMust(str)
}

func StrToDigestPointerMust(str string) *Digest {
	if len(str) == 0 {
		return nil
	}
	d := StrToDigestMust(str)
	return &d
}

func (d *Digest) String() string {
	return base58.Encode(d[:])
}

func (d Digest) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Digest) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	data, err = decodeDigestData(str)
	if err != nil {
		return err
	}
	copy(d[:], data)
	return nil
}

func (d Digest) MarshalBCS() ([]byte, error) {
	return serde.WriteByteSlice(d[:])
}

func (d *Digest) UnmarshalBCS(r io.Reader) (int, error) {
	data, err := serde.ReadByteSlice(r)
	if err != nil {
		return 0, err
	}
	if len(data) != DigestLength {
		return 0, fmt.Errorf("invalid digest length, expect %d, got %d", DigestLength, len(data))
	}
	copy(d[:], data)
	return 0, nil
}
