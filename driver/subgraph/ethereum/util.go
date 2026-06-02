package ethereum

import (
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/common"
)

// The extractors below take a JSON-decoded value (from a map[string]any parsed out
// of the raw_* fields). A present hex string is built; an absent/null/non-string
// value yields nil.

func MustBuildByteArrayFromHex(value any) *wasm.ByteArray {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	return wasm.MustBuildByteArrayFromHex(s)
}

func MustBuildAddressFromString(value any) *common.Address {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	return common.MustBuildAddressFromString(s)
}

func MustBuildBigIntFromHex(value any) *common.BigInt {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	return common.MustBuildBigInt(s)
}
