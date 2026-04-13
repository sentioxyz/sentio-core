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

var decentralizedNetworkDatabase = map[DecentralizedNetwork]string{
	SentioNetworkMainnet: "mainnet",
	SentioNetworkTestnet: "testnet",
}
