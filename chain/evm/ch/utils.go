package ch

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"strings"
)

func AddressToLowerString(addr common.Address) string {
	return strings.ToLower(addr.Hex())
}

func StringToNonce(s string) types.BlockNonce {
	b := hexutil.MustDecode(s)
	var nonce types.BlockNonce
	copy(nonce[:], b[:8])
	return nonce
}

func NonceToString(nonce types.BlockNonce) string {
	return hexutil.Encode(nonce[:])
}
