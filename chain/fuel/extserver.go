package fuel

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	fuelGo "github.com/sentioxyz/fuel-go"
	"github.com/sentioxyz/fuel-go/types"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type ExtServerDimension struct {
	client *ClientPool

	*chain.ExtServerDimension[*Slot]
}

var _ chain.Dimension[*Slot] = (*ExtServerDimension)(nil)

func NewExtServerDimension(
	client *ClientPool,
	loadConcurrency uint,
	loadBatchSize uint,
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerDimension {
	dim := &ExtServerDimension{client: client}
	dim.ExtServerDimension = chain.NewExtServerDimension[*Slot](
		client,
		loadConcurrency,
		loadBatchSize,
		loadRetry,
		validRange,
		fallBehind,
		dim)
	return dim
}

func (d *ExtServerDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	var block *types.Block
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlotHeader/%d", sn),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			block, r = cli.GetBlock(ctx, "ext", sn, fuelGo.GetBlockOption{})
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get header for block %d (%s) failed", sn, r.ConfigName)
	}
	return NewSlot(block), nil
}

func (d *ExtServerDimension) GetSlots(ctx context.Context, sr rg.Range) ([]*Slot, error) {
	opt := fuelGo.GetBlockOption{
		WithHeader:              true,
		WithTransactions:        true,
		WithTransactionDetail:   true,
		WithTransactionReceipts: true,
	}
	blockNumbers := make([]uint64, 0, *sr.Size())
	for sn := sr.Start; sn <= *sr.End; sn++ {
		blockNumbers = append(blockNumbers, sn)
	}
	var blocks []*types.Block
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlots/%s", sr),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			blocks, r = cli.GetBlocks(ctx, "ext", blockNumbers, opt)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get blocks %s (%s) failed", sr, r.ConfigName)
	}
	return utils.MapSliceNoError(blocks, NewSlot), nil
}
