package sol

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/driver/controller/data"
)

func Test_getSkippedBlock(t *testing.T) {
	t.Skip("used external endpoint")

	cli, _ := NewClient(
		context.Background(),
		"https://solana-rpc.publicnode.com",
		1,
		10000,
		time.Second,
		0, // driver version < 2 ⇒ native Solana RPC (this endpoint is a plain node)
	)

	blk, err := cli.GetBlock(context.Background(), 392086804)
	assert.NoError(t, err)
	assert.False(t, blk.Skipped())

	blk, err = cli.GetBlock(context.Background(), 392086803)
	assert.NoError(t, err)
	assert.True(t, blk.Skipped())
}

func Test_queryInterval(t *testing.T) {
	t.Skip("used external endpoint")

	cli, _ := NewClient(
		context.Background(),
		"https://solana-rpc.publicnode.com",
		1,
		0,
		time.Second,
		0, // driver version < 2 ⇒ native Solana RPC (this endpoint is a plain node)
	)

	ctx := context.Background()

	latest, first, err := cli.GetLatest(ctx)
	assert.NoError(t, err)

	timeGetter := func(ctx context.Context, blockNumber uint64) (time.Time, error) {
		for n := blockNumber; n >= 0; n-- {
			getCtx, cancel := context.WithTimeout(ctx, time.Second*3)
			h, err := cli.GetBlock(getCtx, n)
			cancel()
			if err != nil {
				return time.Time{}, err
			}
			if !h.Skipped() {
				fmt.Printf("!!! got block %d/%d: %s\n", blockNumber, n, h.GetBlockTime().Format(time.RFC3339))
				return h.GetBlockTime(), nil
			}
			fmt.Printf("!!! got block %d/%d but skipped\n", blockNumber, n)
			// Slot n was skipped, try n-1 next
		}
		// all slot in [0,blockNumber] was skipped, just return zero time
		return time.Time{}, nil
	}
	s := uint64(393722931)
	e := uint64(393825051)

	req := data.IntervalRequirement{
		IntervalConfig: data.IntervalConfig{
			TimeInterval: &data.TimeInterval{
				Backfill: time.Hour * 4,
				Watching: time.Hour,
			},
		},
	}
	bns, err := data.QueryInterval(ctx, s, e, first, latest, req, timeGetter)
	assert.NoError(t, err)
	fmt.Printf("!!! result: %v\n", bns)
}
