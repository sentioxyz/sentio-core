package redis

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

const (
	rangeKey      = "range"
	slotKeyPrefix = "slot-"
	slotKeyTpl    = slotKeyPrefix + "%020d"
)

type FixSizeSlotCache[SLOT chain.Slot] struct {
	client        *redis.Client
	keyPrefix     string
	slotStore     *KVStore[SLOT]
	loadBatchSize uint64
	concurrency   uint
	cachedHash    *utils.SafeMap[uint64, string]
}

var _ chain.Dimension[chain.Slot] = (*FixSizeSlotCache[chain.Slot])(nil)

func NewFixSizeSlotCache[SLOT chain.Slot](
	client *redis.Client,
	keyPrefix string,
	loadBatchSize uint64,
	concurrency uint,
) *FixSizeSlotCache[SLOT] {
	return &FixSizeSlotCache[SLOT]{
		client:        client,
		keyPrefix:     keyPrefix,
		slotStore:     NewKVStore[SLOT](client, keyPrefix, 0),
		loadBatchSize: loadBatchSize,
		concurrency:   concurrency,
		cachedHash:    utils.NewSafeMap[uint64, string](),
	}
}

func (d *FixSizeSlotCache[SLOT]) Init(ctx context.Context) error {
	return nil
}

func (d *FixSizeSlotCache[SLOT]) slotKey(n uint64) string {
	return fmt.Sprintf(slotKeyTpl, n)
}

func (d *FixSizeSlotCache[SLOT]) slotNumberFromKey(key string) (uint64, error) {
	if !strings.HasPrefix(key, slotKeyPrefix) {
		return 0, errors.Errorf("invalid key %q: prefix not %s", key, slotKeyPrefix)
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(key, slotKeyPrefix), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "invalid key %q", key)
	}
	return n, nil
}

func (d *FixSizeSlotCache[SLOT]) Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error {
	g, gctx := errgroup.WithContext(ctx)
	concurrency.MapO2MWithProducer(
		g,
		gctx,
		d.concurrency,
		rg.RangeCutter{Size: d.loadBatchSize}.BuildProducer(interval),
		slotChan,
		func(ctx context.Context, index int, task rg.Range, taskOut chan<- SLOT) error {
			keys := make([]string, 0, *task.Size())
			for n := task.Start; n <= *task.End; n++ {
				keys = append(keys, d.slotKey(n))
			}
			slots, err := d.slotStore.Get(ctx, keys...)
			if err != nil {
				return errors.Wrapf(err, "get slots in %s failed", task)
			}
			for n := task.Start; n <= *task.End; n++ {
				key := keys[n-task.Start]
				st, has := slots[key]
				if !has {
					return errors.Errorf("slot %d not found when loading at %s", n, interval)
				}
				select {
				case taskOut <- st:
				case <-ctx.Done():
					return ctx.Err()
				}
				d.cachedHash.Put(n, st.GetHash())
			}
			return nil
		},
	)
	return g.Wait()
}

func (d *FixSizeSlotCache[SLOT]) LoadHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	key := d.slotKey(sn)
	r, err := d.slotStore.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	st, has := r[key]
	if !has {
		return nil, chain.ErrSlotNotFound
	}
	d.cachedHash.Put(sn, st.GetHash())
	return st, nil
}

func (d *FixSizeSlotCache[SLOT]) GetRange(ctx context.Context) (rg.Range, error) {
	raw, err := d.client.Get(ctx, d.keyPrefix+rangeKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return rg.Range{}, nil
		}
		return rg.Range{}, err
	}
	return rg.RangeParser{}.Unmarshal(bytes.NewReader([]byte(raw)))
}

func (d *FixSizeSlotCache[SLOT]) setRange(ctx context.Context, r rg.Range) error {
	var buf bytes.Buffer
	_ = rg.RangeParser{}.Marshal(r, &buf)
	_, err := d.client.Set(ctx, d.keyPrefix+rangeKey, buf.String(), 0).Result()
	return err
}

func (d *FixSizeSlotCache[SLOT]) Wait(ctx context.Context, sn uint64) error {
	return chain.WaitSlot(ctx, d.GetRange, sn)
}

func (d *FixSizeSlotCache[SLOT]) CheckMissing(ctx context.Context, interval rg.Range, missing chan<- rg.Range) error {
	return nil
}

// Save will save slots from slotChan in interval and delete slots not in interval
func (d *FixSizeSlotCache[SLOT]) Save(ctx context.Context, interval rg.Range, slotChan <-chan SLOT) error {
	curRange, err := d.GetRange(ctx)
	if err != nil {
		return errors.Wrapf(err, "get range failed")
	}
	if curRange.IsEmpty() {
		if err = d.Delete(ctx, rg.Range{}); err != nil {
			return errors.Wrapf(err, "clean failed")
		}
	}
	// save slots first
	var ignored atomic.Uint64
	g, gctx := errgroup.WithContext(ctx)
	concurrency.RunWithTaskChan(
		g,
		gctx,
		int(d.concurrency),
		slotChan,
		func(ctx context.Context, st SLOT) error {
			if h, has := d.cachedHash.Get(st.GetNumber()); has && h == st.GetHash() {
				ignored.Add(1)
				return nil
			}
			if setErr := d.slotStore.Set(ctx, map[string]SLOT{d.slotKey(st.GetNumber()): st}); setErr != nil {
				return setErr
			}
			d.cachedHash.Put(st.GetNumber(), st.GetHash())
			return nil
		},
	)
	if err = g.Wait(); err != nil {
		return errors.Wrapf(err, "save slots failed")
	}
	// then save range
	if err = d.setRange(ctx, interval); err != nil {
		return errors.Wrapf(err, "update range failed")
	}
	// clean useless slots
	var toDel []string
	for n := curRange.Start; n <= *curRange.End; n++ {
		if interval.Contains(n) {
			continue
		}
		toDel = append(toDel, d.slotKey(n))
		d.cachedHash.Del(n)
	}
	if len(toDel) > 0 {
		return d.slotStore.Del(ctx, toDel...)
	}
	return nil
}

func (d *FixSizeSlotCache[SLOT]) Delete(ctx context.Context, interval rg.Range) error {
	_, logger := log.FromContext(ctx)
	curRange, err := d.GetRange(ctx)
	if err != nil {
		return errors.Wrapf(err, "get range failed")
	}
	resultRange := curRange.Remove(interval).GetRanges()
	if len(resultRange) > 1 {
		return errors.Errorf("cannot delete range %s in %s", interval, curRange)
	}
	g, gctx := errgroup.WithContext(ctx)
	ch := make(chan string)
	g.Go(func() error {
		defer close(ch)
		return d.slotStore.List(gctx, ch)
	})
	var toDel []string
	g.Go(func() error {
		for key := range ch {
			if key == rangeKey {
				continue
			}
			n, err := d.slotNumberFromKey(key)
			if err != nil {
				logger.Warnfe(err, "has invalid key, will be deleted")
			} else if interval.Contains(n) {
				continue
			}
			toDel = append(toDel, key)
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return errors.Wrapf(err, "list keys failed")
	}
	if len(resultRange) == 0 {
		curRange = rg.EmptyRange
	} else {
		curRange = resultRange[0]
	}
	if err = d.setRange(ctx, curRange); err != nil {
		return errors.Wrapf(err, "update range failed")
	}
	if len(toDel) > 0 {
		if err = d.slotStore.Del(ctx, toDel...); err != nil {
			return errors.Wrapf(err, "delete %d keys failed", len(toDel))
		}
		logger.Infof("deleted %d keys", len(toDel))
	}
	return nil
}
