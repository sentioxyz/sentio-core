package chains

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type testCase struct {
	chainID   ChainID
	chainType ChainType
}

func TestChainType(t *testing.T) {
	testCases := []testCase{
		{
			ChainID(EthereumID),
			EthChainType,
		},
		{
			ChainID("167000"),
			EthChainType,
		},
		{
			ChainID(AptosMainnetID),
			AptosChainType,
		},
		{
			ChainID(IotaTestnetID),
			SuiChainType,
		},
		{
			ChainID("sol_testnet"),
			SolanaChainType,
		},
	}

	for _, testCase := range testCases {
		chainType, ok := GetChainType(testCase.chainID)
		assert.True(t, ok)
		assert.Equal(t, testCase.chainType, chainType)
		assert.True(t, IsChainType(testCase.chainID, chainType))
	}

	assert.False(t, IsChainType(ChainID(SuiTestnetID), EthChainType))
}
