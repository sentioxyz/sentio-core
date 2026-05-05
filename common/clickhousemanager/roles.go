package ckhmanager

type Role string

const (
	DefaultRole      Role = "default_viewer"
	SmallEngineRole  Role = "small_viewer"
	MediumEngineRole Role = "medium_viewer"
	LargeEngineRole  Role = "large_viewer"
	UltraEngineRole  Role = "ultra_viewer"

	EmptyRole Role = "viewer"
	AdminRole Role = ""
)

type Category string

const (
	SentioCategory   Category = "sentio"
	SubgraphCategory Category = "subgraph"
	AllCategory      Category = "all"

	DefaultCategory = SubgraphCategory
)

type DecentralizedNetwork string

const (
	SentioNetworkMainnet DecentralizedNetwork = "mainnet"
	SentioNetworkTestnet DecentralizedNetwork = "testnet"

	NoneNetwork DecentralizedNetwork = ""
)

// decentralizedNetworkDatabase intentionally maps to empty strings so the
// connection's hello.Database stays empty. The decentralized housegate
// rejects hello.Database values that are not registered logical databases
// (forward.Plugin.OnHello) — the physical name (e.g. "testnet") is not in
// that registry. Leaving hello.Database empty makes forward short-circuit
// and the rewriter handles logical→physical translation per query.
var decentralizedNetworkDatabase = map[DecentralizedNetwork]string{
	SentioNetworkMainnet: "",
	SentioNetworkTestnet: "",
}
