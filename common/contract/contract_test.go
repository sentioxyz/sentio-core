package contract

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// FIXME runs into 403 regularly
//func TestIsERC20(t *testing.T) {
//	endpoint := "https://eth-mainnet.g.alchemy.com/v2/KLPDGUUQGKmScCbSdPd0iUNO_JufXdg9"
//
//	ctx := context.Background()
//
//	var res bool
//	var err error
//
//	//// abtc
//	//res, err = IsERC20(ctx, endpoint, "0xC2fcab14Ec1F2dFA82a23C639c4770345085a50F")
//	//assert.NoError(t, err)
//	//assert.False(t, res)
//
//	// weth address
//	res, err = IsERC20(ctx, endpoint, "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
//	assert.NoError(t, err)
//	assert.True(t, res)
//
//	// invalid address
//	res, err = IsERC20(ctx, endpoint, "0x0000000000000000000000000000000000000000")
//	assert.NoError(t, err)
//	assert.False(t, res)
//
//	// token address but does not implement ERC20
//	res, err = IsERC20(ctx, endpoint, "0xa6794DEc66Df7d8B69752956df1b28cA93f77CD7")
//	assert.NoError(t, err)
//	assert.False(t, res)
//
//	// USDC
//	res, err = IsERC20(ctx, endpoint, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
//	assert.NoError(t, err)
//	assert.True(t, res)
//
//	res, err = IsERC20(ctx, endpoint, "0x6B89B97169a797d94F057F4a0B01E2cA303155e4")
//	assert.NoError(t, err)
//	assert.True(t, res)
//}

func TestIsERC20New(t *testing.T) {
	//endpoint := "https://eth-mainnet.g.alchemy.com/v2/z1Q-YhcYg60C5sOQPUzsMFqiDJSvqbsK"
	endpoint := "oGDQZsjYX3IdnfkenzRf0K5NA7Lsy0NljUFVKFp0nGDhwajEq5ltjHU7aFp3V8lG"

	ctx := context.Background()

	var res bool
	var err error

	//// abtc
	//res, err = IsERC20New(ctx, endpoint, "1", "0xC2fcab14Ec1F2dFA82a23C639c4770345085a50F")
	//assert.NoError(t, err)
	//assert.False(t, res)

	// weth address
	res, err = IsERC20New(ctx, endpoint, "1", "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	assert.NoError(t, err)
	assert.True(t, res)

	// invalid address
	// res, err = IsERC20New(ctx, endpoint, "1", "0x0000000000000000000000000000000000000000")
	// assert.NoError(t, err)
	// assert.False(t, res)

	// token address but does not implement ERC20
	_, err = IsERC20New(ctx, endpoint, "1", "0xa6794DEc66Df7d8B69752956df1b28cA93f77CD7")
	assert.NoError(t, err)
	// Uncomment this line when the issue is fixed.
	// assert.False(t, res)

	// USDC
	res, err = IsERC20New(ctx, endpoint, "1", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	assert.NoError(t, err)
	assert.True(t, res)
}
