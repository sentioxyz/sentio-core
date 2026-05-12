package aptos

import (
	"context"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	rg "sentioxyz/sentio-core/common/range"
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
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerDimension {
	dim := &ExtServerDimension{client: client}
	// loadBatchSize more than 1 is meaningless
	dim.ExtServerDimension = chain.NewExtServerDimension[*Slot](
		client,
		loadConcurrency,
		1,
		loadRetry,
		validRange,
		fallBehind,
		dim)
	return dim
}

func (d *ExtServerDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	var block api.Block
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlotHeader/%d", sn),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			block, r = cli.GetBlock(ctx, "ext", sn, false)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get header for slot %d (%s) failed", sn, r.ConfigName)
	}
	return (*Slot)(&block), nil
}

func (d *ExtServerDimension) GetSlot(ctx context.Context, sn uint64) (*Slot, error) {
	var block api.Block
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot/%d", sn),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			block, r = cli.GetBlock(ctx, "ext", sn, true)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get slot %d (%s) failed", sn, r.ConfigName)
	}
	return (*Slot)(&block), nil
}

func (d *ExtServerDimension) GetSlots(ctx context.Context, sr rg.Range) ([]*Slot, error) {
	slots := make([]*Slot, 0, *sr.Size())
	for sn := sr.Start; sn <= *sr.End; sn++ {
		st, err := d.GetSlot(ctx, sn)
		if err != nil {
			return nil, err
		}
		slots = append(slots, st)
	}
	return slots, nil
}
