package ch

import (
	"github.com/ethereum/go-ethereum/common"
	"strings"
)

func AddressToLowerString(addr common.Address) string {
	return strings.ToLower(addr.Hex())
}
