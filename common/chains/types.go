package chains

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
)

type ExplorerAPIType string

const (
	ExplorerAPITypeEtherscan   ExplorerAPIType = "etherscan"
	ExplorerAPITypeEtherscanV2 ExplorerAPIType = "etherscan_v2"
	ExplorerAPITypeBlockscout  ExplorerAPIType = "blockscout"
	ExplorerAPITypeOkLink      ExplorerAPIType = "oklink"
	ExplorerAPITypeL2Scan      ExplorerAPIType = "l2scan"
	ExplorerAPITypeUnknown     ExplorerAPIType = "unknown"
)

type EthVariation int

const (
	EthVariationDefault      EthVariation = 0
	EthVariationArbitrum     EthVariation = 1
	EthVariationOptimism     EthVariation = 2
	EthVariationZkSync       EthVariation = 3
	EthVariationPolygonZkEVM EthVariation = 4
	EthVariationSubstrate    EthVariation = 5
	EthVariationTron         EthVariation = 6
)

func (e EthVariation) String() string {
	switch e {
	case EthVariationDefault:
		return "default"
	case EthVariationArbitrum:
		return "arbitrum"
	case EthVariationOptimism:
		return "optimism"
	case EthVariationZkSync:
		return "zksync"
	case EthVariationPolygonZkEVM:
		return "polygonzkevm"
	case EthVariationSubstrate:
		return "substrate"
	case EthVariationTron:
		return "tron"
	default:
		return "unknown"
	}
}

func (e EthVariation) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

type EthChainInfo struct {
	ChainInfo
	Variation EthVariation `json:"variation"`

	TokenAddress  common.Address `json:"token_address"`
	TokenSymbol   string         `json:"token_symbol"`
	TokenDecimals int            `json:"token_decimals"`

	PriceTokenAddress   common.Address `json:"price_token_address"`
	WrappedTokenAddress common.Address `json:"wrapped_token_address"`

	ExplorerAPI     string          `json:"explorer_api"`
	ExplorerAPIType ExplorerAPIType `json:"explorer_api_type"`
	BlockScoutUrl   string          `json:"block_Scout_Url"`
}

type ChainInfo struct {
	ChainID         ChainID  `json:"chain_id"`
	MainnetChainID  ChainID  `json:"mainnet_chain_id"`
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	AdditionalSlugs []string `json:"additional_slugs"`
	ExplorerURL     string   `json:"explorerUrl"`
}
