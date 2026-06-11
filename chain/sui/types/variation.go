package types

import "strings"

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

// VariationFromNetwork resolves the chain variation from a network name. IOTA
// networks (e.g. "iota-mainnet", "iota-testnet") are VariationIOTA; every other
// sui-chain-type network defaults to VariationSUI. This is the single source of
// truth for the sui/iota distinction — callers (e.g. the launcher) should use it
// rather than re-checking the network name.
func VariationFromNetwork(network string) Variation {
	if strings.HasPrefix(network, "iota") {
		return VariationIOTA
	}
	return VariationSUI
}
