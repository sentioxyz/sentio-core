package manifest

import (
	"context"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/utils"
	"strings"

	"github.com/pkg/errors"
)

// all networks which graph-cli supported are here:
// https://github.com/graphprotocol/graph-tooling/blob/main/packages/cli/src/protocols/index.ts#L79
// TODO fill more chain ID
var chainIDMap = map[string]string{
	"mainnet": "1",
	//"rinkeby":               "",
	"goerli": "5",
	//"poa-core":              "",
	//"poa-sokol":             "",
	//"gnosis":                "",
	//"matic":                 "",
	//"mumbai":                "",
	//"fantom":                "",
	//"fantom-testnet":        "",
	//"bsc":                   "",
	//"chapel":                "",
	//"clover":                "",
	//"avalanche":             "",
	//"fuji":                  "",
	//"celo":                  "",
	//"celo-alfajores":        "",
	//"fuse":                  "",
	//"moonbeam":              "",
	//"moonriver":             "",
	//"mbase":                 "",
	//"arbitrum-one":          "",
	//"arbitrum-goerli":       "",
	//"optimism":              "",
	//"optimism-goerli":       "",
	//"aurora":                "",
	//"aurora-testnet":        "",
	//"base-testnet":          "",
	//"zksync-era":            "",
	//"sepolia":               "",
	//"polygon-zkevm-testnet": "",
	//"polygon-zkevm":         "",

	// sentio network
	"arb":                   "42161",
	"arb-mainnet":           "42161",
	"astar":                 "592",
	"astar-mainnet":         "592",
	"astar-zkevm":           "3776",
	"aurora":                "1313161554",
	"aurora-mainnet":        "1313161554",
	"avax-mainnet-c":        "43114",
	"base-goerli":           "84531",
	"base":                  "8453",
	"base-mainnet":          "8453",
	"bevm-canary-mainnet":   "1501",
	"bevm":                  "11501",
	"bevm-mainnet":          "11501",
	"bitlayer":              "200901",
	"bitlayer-mainnet":      "200901",
	"blast":                 "81457",
	"blast-mainnet":         "81457",
	"blast-sepolia":         "168587773",
	"bob":                   "60808",
	"bsc":                   "56",
	"bsc-mainnet":           "56",
	"bsc-testnet":           "97",
	"chiliz":                "88888",
	"chiliz-mainnet":        "88888",
	"conflux":               "1030",
	"conflux-mainnet":       "1030",
	"cronos":                "25",
	"cronos-mainnet":        "25",
	"cronos-testnet":        "338",
	"eth-goerli":            "5",
	"eth-mainnet":           "1",
	"eth-sepolia":           "11155111",
	"fantom":                "250",
	"fantom-mainnet":        "250",
	"kcc":                   "321",
	"kcc-mainnet":           "321",
	"linea":                 "59144",
	"linea-mainnet":         "59144",
	"lumio-testnet":         "9990",
	"manta-pacific-mainnet": "169",
	"mantle":                "5000",
	"mantle-mainnet":        "5000",
	"mode":                  "34443",
	"mode-mainnet":          "34443",
	"moonbase-alpha":        "1287",
	"moonbeam":              "1284",
	"moonbeam-mainnet":      "1284",
	"opt":                   "10",
	"opt-mainnet":           "10",
	"polygon":               "137",
	"polygon-mainnet":       "137",
	"polygon-zk-mainnet":    "1101",
	"scroll":                "534352",
	"scroll-mainnet":        "534352",
	"taiko-katla-testnet":   "167008",
	"xlayer":                "196",
	"xlayer-mainnet":        "196",
	"zircuit":               "48900",
	"zircuit-mainnet":       "48900",
	"zircuit-testnet":       "48899",
	"zksync-mainnet":        "324",
}

func init() {
	// add alias for chainIDMap
	alias := make(map[string]string)
	for network, chainID := range chainIDMap {
		if strings.HasSuffix(network, "-mainnet") {
			alias[strings.TrimSuffix(network, "-mainnet")] = chainID
		}
	}
	for network, chainID := range alias {
		if _, has := chainIDMap[network]; !has {
			chainIDMap[network] = chainID
		}
	}
	// more in chains
	for _, chain := range chains.EthChains {
		chainIDMap[chain.ChainInfo.Slug] = string(chain.ChainInfo.ChainID)

		for _, slug := range chain.ChainInfo.AdditionalSlugs {
			chainIDMap[slug] = string(chain.ChainInfo.ChainID)
		}
	}
}

const (
	CustomizedChainID = "customized"
)

var ErrInvalidCustomizedEndpoint = errors.New("invalid endpoint for the customized evm chain")

// GetChainID return the ChainID of the network, unknown network will return raw value as chainId
func GetChainID(network string, checkCustomizeEndpoint bool) (chainID string, endpoint string, err error) {
	chainID, has := chainIDMap[network]
	if has {
		return chainID, "", nil
	}
	if strings.HasPrefix(network, "http://") || strings.HasPrefix(network, "https://") {
		if checkCustomizeEndpoint {
			if err = evm.CheckArchiveNode(context.Background(), network); err != nil {
				return "", "", errors.Wrapf(ErrInvalidCustomizedEndpoint,
					"check archive node (%s) failed: %s", network, err.Error())
			}
		}
		return CustomizedChainID, network, nil
	}
	for _, c := range chains.EthChains {
		set := utils.MergeArr([]string{string(c.ChainID), c.Name, c.Slug}, c.AdditionalSlugs)
		set = utils.MapSliceNoError(set, strings.ToLower)
		if utils.IndexOf(set, strings.ToLower(network)) >= 0 {
			return string(c.ChainID), "", nil
		}
	}
	return network, "", errors.Errorf("unknown network %q", network)
}
