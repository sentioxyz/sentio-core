package types

import (
	"strings"

	"sentioxyz/sentio-core/common/chains"
)

// Variation identifies a variation of the `sui` chain type. Sui and IOTA share
// most wire formats but differ on some BCS enum layouts (different variant
// indices / payloads for the same conceptual variant). The variation is the
// selector threaded into the serde decoder/encoder so per-chain `bcs` enum tags
// (`enumNum[sui]=..,enumNum[iota]=..`) resolve correctly.
//
// See bcs_enum_selector_design.md and CLAUDE.md in this package.
type Variation string

const (
	VariationSUI  Variation = "sui"
	VariationIOTA Variation = "iota"
)

// String returns the serde selector for this variation.
func (v Variation) String() string { return string(v) }

// VariationFromChainID resolves the chain variation from a sui-chain-type chain
// id. IOTA chain ids map to VariationIOTA; everything else (sui mainnet/testnet)
// defaults to VariationSUI.
func VariationFromChainID(chainID chains.SuiChainID) Variation {
	switch chainID {
	case chains.IotaMainnetID, chains.IotaTestnetID:
		return VariationIOTA
	default:
		return VariationSUI
	}
}

// SpecialMethodPrefix is the json-rpc method-name prefix for this variation. The
// base sui methods start with "sui" (e.g. "sui_*" and "suix_*"); IOTA serves the
// same methods with the leading "sui" replaced by "iota" (e.g. "iota_*" and
// "iotax_*"), so SUI has an empty prefix (no rewrite) and IOTA's is "iota".
func (v Variation) SpecialMethodPrefix() string {
	if v == VariationIOTA {
		return "iota"
	}
	return ""
}

// RPCMethod maps a base "sui"-prefixed json-rpc method name to this variation's
// actual method name by replacing the leading "sui" with the variation's prefix:
// e.g. for IOTA "sui_getCheckpoint" -> "iota_getCheckpoint" and
// "suix_queryEvents" -> "iotax_queryEvents". Names that don't start with "sui",
// or variations with no prefix (SUI), are returned unchanged.
func (v Variation) RPCMethod(baseSuiMethod string) string {
	prefix := v.SpecialMethodPrefix()
	if prefix == "" || !strings.HasPrefix(baseSuiMethod, "sui") {
		return baseSuiMethod
	}
	return prefix + strings.TrimPrefix(baseSuiMethod, "sui")
}
