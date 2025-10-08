package chains

var EthChainIDToInfo = map[ChainID]*EthChainInfo{}
var EthSlugToInfo = map[string]*EthChainInfo{}

var NonEthChainIDToInfo = map[ChainID]*ChainInfo{}
var NonEthSlugToInfo = map[string]*ChainInfo{}

var ChainIDToInfo = map[ChainID]*ChainInfo{}
var SlugToInfo = map[string]*ChainInfo{}

func init() {
	for _, chain := range EthChains {
		Chains = append(Chains, &chain.ChainInfo)

		EthChainIDToInfo[chain.ChainInfo.ChainID] = chain
		EthSlugToInfo[chain.ChainInfo.Slug] = chain
		ChainIDToInfo[chain.ChainInfo.ChainID] = &chain.ChainInfo
		SlugToInfo[chain.ChainInfo.Slug] = &chain.ChainInfo

		for _, additionalSlug := range chain.ChainInfo.AdditionalSlugs {
			EthSlugToInfo[additionalSlug] = chain
			SlugToInfo[additionalSlug] = &chain.ChainInfo
		}
	}

	for _, chainInfo := range NonEthChains {
		Chains = append(Chains, chainInfo)

		NonEthChainIDToInfo[chainInfo.ChainID] = chainInfo
		NonEthSlugToInfo[chainInfo.Slug] = chainInfo
		ChainIDToInfo[chainInfo.ChainID] = chainInfo
		SlugToInfo[chainInfo.Slug] = chainInfo

		for _, additionalSlug := range chainInfo.AdditionalSlugs {
			NonEthSlugToInfo[additionalSlug] = chainInfo
			SlugToInfo[additionalSlug] = chainInfo
		}
	}
}

func GetChainType(chainID ChainID) (ChainType, bool) {
	chainType, ok := ChainIDToType[chainID]
	return chainType, ok
}

func IsChainType(chainID ChainID, targetChainType ChainType) bool {
	chainType, ok := ChainIDToType[chainID]
	if !ok {
		return false
	}
	return chainType == targetChainType
}

// TODO remove the use of all following method in favor of IsChainType
func IsEVMChains(chainID string) bool {
	return IsChainType(ChainID(chainID), EthChainType)
}

func IsSolanaChain(chainID string) bool {
	return IsChainType(ChainID(chainID), SolanaChainType)
}

func IsSuiChain(chainID string) bool {
	return IsChainType(ChainID(chainID), SuiChainType)
}

func IsAptosChain(chainID string) bool {
	return IsChainType(ChainID(chainID), AptosChainType)
}

func IsFuelChain(chainID string) bool {
	return IsChainType(ChainID(chainID), FuelChainType)
}

func IsStarknetChain(chainID string) bool {
	return IsChainType(ChainID(chainID), StarknetChainType)
}
