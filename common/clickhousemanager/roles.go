package ckhmanager

type Role string

const (
	DefaultRole      Role = "default_readonly"
	SmallEngineRole  Role = "small_readonly"
	MediumEngineRole Role = "medium_readonly"
	LargeEngineRole  Role = "large_readonly"
	UltraEngineRole  Role = "ultra_readonly"

	EmptyRole Role = "readonly"
	AdminRole Role = "admin"
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
	SentioNetworkDevnet  DecentralizedNetwork = "devnet"

	NoneNetwork DecentralizedNetwork = ""
)

var decentralizedNetworkDatabase = map[DecentralizedNetwork]string{
	SentioNetworkMainnet: "mainnet",
	SentioNetworkTestnet: "testnet",
	SentioNetworkDevnet:  "devnet",
}
