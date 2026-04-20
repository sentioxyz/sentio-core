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

func (d *ExtServerDimension[SLOT]) LoadHeader(ctx context.Context, sn number.Number) (slot.Header, error) {
	return d.slotGetter.GetSlotHeader(ctx, sn)
}

func (d *ExtServerDimension[SLOT]) tryLoadSector(ctx context.Context, sector number.Range) ([]SLOT, error) {
	// fetch slots
	slots, err := d.slotGetter.GetSlots(ctx, sector)
	if err != nil {
		return nil, err
	}
	// check the data
	if uint64(len(slots)) != sector.Size() {
		return nil, errors.Errorf("incomplete data, sector is %s but only got %d slots", sector, len(slots))
	}
	for i, st := range slots {
		if exp := sector.L() + number.Number(i); exp != st.GetNumber() {
			return nil, errors.Errorf("invalid data, the #%d slot in sector %s is %d not %d", i, sector, st.GetNumber(), exp)
		}
	}
	return slots, nil
}

func (d *ExtServerDimension[SLOT]) loadSector(ctx context.Context, sector number.Range, out chan<- SLOT) error {
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

func (d *ExtServerDimension[SLOT]) Load(ctx context.Context, interval number.Range, slotChan chan<- SLOT) error {
	ctx, logger := log.FromContext(ctx, "interval", interval)
	logger.Debug("load begin")

	g, ctx := errgroup.WithContext(ctx)
	concurrency.MapO2MWithProducer(
		g, ctx, d.loadConcurrency,
		number.RangeCutter{Size: uint64(d.loadBatchSize)}.BuildProducer(interval),
		slotChan,
		func(ctx context.Context, index int, sector number.Range, taskOut chan<- SLOT) error {
			return d.loadSector(ctx, sector, taskOut)
		})

	if err := g.Wait(); err != nil {
		logger.Warne(err, "load failed")
		return err
	}
	logger.Debug("load succeed")
	return nil
}

func (d *ExtServerDimension[SLOT]) GetRange(ctx context.Context) (number.Range, error) {
	latest, err := d.client.Latest(ctx)
	if err != nil {
		return number.Range{}, err
	}
	latestNum := number.Number(latest.Number)
	if d.fallBehind > 0 {
		var blockInterval time.Duration
		if blockInterval, err = node.WaitValidBlockInterval(ctx, d.client); err != nil {
			return number.Range{}, err
		}
		latestNum -= number.Number(d.fallBehind / blockInterval)
	}
	return d.validRange.GetIntersection(number.NewRange(0, latestNum)), nil
}

func (d *ExtServerDimension[SLOT]) Wait(ctx context.Context, blockNumber number.Number) error {
	if !d.validRange.ContainsNumber(blockNumber) {
		_, logger := log.FromContext(ctx)
		logger.Warnf("wait number %d out of valid range %s, will wait forever", blockNumber, d.validRange)
		<-ctx.Done()
		return ctx.Err()
	}
	if d.fallBehind > 0 {
		blockInterval, err := node.WaitValidBlockInterval(ctx, d.client)
		if err != nil {
			return err
		}
		blockNumber += number.Number(d.fallBehind / blockInterval)
	}
	_, err := d.client.WaitBlock(ctx, uint64(blockNumber))
	return err
}

func (d *ExtServerDimension[SLOT]) CheckMissing(
	ctx context.Context,
	interval number.Range,
	missing chan<- number.Range,
) error {
	return nil
}

func (d *ExtServerDimension[SLOT]) Save(ctx context.Context, interval number.Range, slotChan <-chan SLOT) error {
	panic("impossible")
}

func (d *ExtServerDimension[SLOT]) Delete(ctx context.Context, interval number.Range) error {
	panic("impossible")
}
