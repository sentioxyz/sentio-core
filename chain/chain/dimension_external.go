package chain

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

type ExtServerDimension[SLOT Slot] struct {
	client          NodeClient
	loadConcurrency uint
	loadBatchSize   uint
	loadRetry       int
	validRange      rg.Range
	fallBehind      time.Duration
	slotGetter      SlotGetter[SLOT]
}

func NewExtServerDimension[SLOT Slot](
	client NodeClient,
	loadConcurrency uint,
	loadBatchSize uint,
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
	slotGetter SlotGetter[SLOT],
) *ExtServerDimension[SLOT] {
	return &ExtServerDimension[SLOT]{
		client:          client,
		loadConcurrency: loadConcurrency,
		loadBatchSize:   loadBatchSize,
		loadRetry:       loadRetry,
		validRange:      validRange,
		fallBehind:      fallBehind,
		slotGetter:      slotGetter,
	}
}

func (d *ExtServerDimension[SLOT]) Init(ctx context.Context) error {
	return nil
}

func (d *ExtServerDimension[SLOT]) LoadHeader(ctx context.Context, sn uint64) (Slot, error) {
	return d.slotGetter.GetSlotHeader(ctx, sn)
}

func (d *ExtServerDimension[SLOT]) tryLoadSector(ctx context.Context, sector rg.Range) ([]SLOT, error) {
	// fetch slots
	slots, err := d.slotGetter.GetSlots(ctx, sector)
	if err != nil {
		return nil, err
	}
	// check the data
	if uint64(len(slots)) != *sector.Size() {
		return nil, errors.Errorf("incomplete data, sector is %s but only got %d slots", sector, len(slots))
	}
	for i, st := range slots {
		if exp := sector.Start + uint64(i); exp != st.GetNumber() {
			return nil, errors.Errorf("invalid data, the #%d slot in sector %s is %d not %d", i, sector, st.GetNumber(), exp)
		}
	}
	return slots, nil
}

func (d *ExtServerDimension[SLOT]) loadSector(ctx context.Context, sector rg.Range, out chan<- SLOT) error {
	_, logger := log.FromContext(ctx, "sector", sector)
	const interval = time.Second
	for tried := 0; ; tried++ {
		slots, err := d.tryLoadSector(ctx, sector)
		if err == nil {
			logger.Debug("loaded slots")
			for _, st := range slots {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- st:
				}
			}
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		if d.loadRetry >= 0 && tried >= d.loadRetry {
			logger.Errorfe(err, "load slots failed")
			return err
		}
		logger.Warnfe(err, "load slots failed (%d/%d), will retry after %s", tried, d.loadRetry, interval)
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *ExtServerDimension[SLOT]) Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error {
	ctx, logger := log.FromContext(ctx, "interval", interval)
	logger.Debug("load begin")

	g, ctx := errgroup.WithContext(ctx)
	concurrency.MapO2MWithProducer(
		g, ctx, d.loadConcurrency,
		rg.RangeCutter{Size: uint64(d.loadBatchSize)}.BuildProducer(interval),
		slotChan,
		func(ctx context.Context, index int, sector rg.Range, taskOut chan<- SLOT) error {
			return d.loadSector(ctx, sector, taskOut)
		})

	if err := g.Wait(); err != nil {
		logger.Warne(err, "load failed")
		return err
	}
	logger.Debug("load succeed")
	return nil
}

func (d *ExtServerDimension[SLOT]) GetRange(ctx context.Context) (rg.Range, error) {
	latest, err := d.client.WaitBlock(ctx, 0)
	if err != nil {
		return rg.Range{}, err
	}
	latestNum := latest.Number
	if d.fallBehind > 0 {
		var blockInterval time.Duration
		blockInterval, err = d.client.WaitBlockInterval(ctx)
		if err != nil {
			return rg.Range{}, err
		}
		latestNum -= uint64(d.fallBehind / blockInterval)
	}
	return d.validRange.Intersection(rg.NewRange(0, latestNum)), nil
}

func (d *ExtServerDimension[SLOT]) Wait(ctx context.Context, blockNumber uint64) error {
	if !d.validRange.Contains(blockNumber) {
		_, logger := log.FromContext(ctx)
		logger.Warnf("wait number %d out of valid range %s, will wait forever", blockNumber, d.validRange)
		<-ctx.Done()
		return ctx.Err()
	}
	if d.fallBehind > 0 {
		blockInterval, err := d.client.WaitBlockInterval(ctx)
		if err != nil {
			return err
		}
		blockNumber += uint64(d.fallBehind / blockInterval)
	}
	_, err := d.client.WaitBlock(ctx, blockNumber)
	return err
}

func (d *ExtServerDimension[SLOT]) CheckMissing(
	ctx context.Context,
	interval rg.Range,
	missing chan<- rg.Range,
) error {
	return nil
}

func (d *ExtServerDimension[SLOT]) Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error {
	panic("impossible")
}

func (d *ExtServerDimension[SLOT]) Delete(ctx context.Context, interval rg.Range) error {
	panic("impossible")
}
