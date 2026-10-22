package sui

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sui/types"
	rg "sentioxyz/sentio-core/common/range"
)

// A json-rpc slot whose sui_getCheckpoint call fails must fail as a whole, not be built from an
// empty checkpoint (all-ones digest, zero timestamp, no transactions).
func TestGetSlot_CheckpointLoadFailureIsReturned(t *testing.T) {
	_, grpcEndpoint := startFakeSuiNode(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// the only client is grpc-only, so every sui_getCheckpoint call is refused locally
	pool := NewClientPool("test", nil)
	ch := make(chan clientpool.PoolConfig[ClientConfig], 1)
	ch <- clientpool.PoolConfig[ClientConfig]{
		ClientConfigs: []clientpool.ClientConfig[ClientConfig]{{Config: grpcOnlyConfig(grpcEndpoint)}},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.Start(ctx, ch)
	}()
	defer func() {
		cancel()
		<-done
	}()
	require.NoError(t, pool.WaitReady(ctx))

	dim :=NewExtServerDimension(pool, types.VariationSUI, true, true, false, false, 0, 0, 1, 0, rg.Range{}, 0)
	slot, err := dim.getSlot(ctx, 100)
	require.Error(t, err)
	assert.Nil(t, slot)
	assert.ErrorContains(t, err, "load checkpoint 100 failed")
}
