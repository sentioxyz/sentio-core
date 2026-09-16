package evm

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const guardTestAddress = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"

// The CallContext state guard of a non-archive node must only short-circuit calls whose block
// number is known to be below the start of the state data; everything else goes to the node.
func Test_CallContext_stateGuard(t *testing.T) {
	ctx := context.Background()
	cli, _ := newStateProbeClient(t, 100000, 80001)
	require.Equal(t, uint64(80001), cli.hasStateDataFrom.Load())

	// a block number below the start of the state data is rejected for the task
	r := cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, hexutil.Uint64(100))
	require.Error(t, r.Err)
	assert.Contains(t, r.Err.Error(), "miss state data at block 0x64")
	assert.True(t, r.BrokenForTask)

	// a block number within the state data is served
	r = cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, hexutil.Uint64(90000))
	assert.NoError(t, r.Err)

	// an omitted block parameter means latest and is served, it used to dereference a nil block number
	r = cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress)
	assert.NoError(t, r.Err)

	// so does an explicit null
	r = cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, nil)
	assert.NoError(t, r.Err)

	// a block hash cannot be compared with the boundary and is left to the node
	hash := rpc.BlockNumberOrHashWithHash(common.HexToHash("0x1"), false)
	r = cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, hash)
	assert.NoError(t, r.Err)

	// an unparsable block parameter is still an invalid request
	r = cli.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, "not-a-block")
	require.Error(t, r.Err)
	assert.Contains(t, r.Err.Error(), "invalid block parameter")
	assert.False(t, r.BrokenForTask)

	// an archive node never enters the guard
	archive, _ := newStateProbeClient(t, 100000, 0)
	r = archive.CallContext(ctx, nil, "test", "eth_getBalance", guardTestAddress, hexutil.Uint64(100))
	assert.NoError(t, r.Err)
}
