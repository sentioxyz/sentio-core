package common

import (
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"strings"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/common/numbers.ts

type Address struct {
	wasm.ByteArray
}

func BuildAddressFromBytes(b []byte) *Address {
	return &Address{ByteArray: wasm.ByteArray{Data: b}}
}

func BuildAddressFromString(addr string) (*Address, error) {
	addr = strings.TrimPrefix(addr, "0x")
	if len(addr) != 40 {
		return nil, errors.Errorf("length of address %q should be 40", addr)
	}
	b, err := wasm.BuildByteArrayFromHex(addr)
	if err != nil {
		return nil, err
	}
	return &Address{ByteArray: *b}, nil
}

func MustBuildAddressFromString(addr string) *Address {
	return utils.MustReturn(BuildAddressFromString(addr))
}
