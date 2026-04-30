package clickhouse

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type rangeObject struct {
	Left     uint64    `clickhouse:"left"`
	Right    uint64    `clickhouse:"right"`
	CreateAt time.Time `clickhouse:"create_at"`
}

type RangeStore struct {
	ctrl     chx.Controller
	name     chx.FullName
	readOnly bool

	current    *rg.Range
	curValidAt *time.Time
	mu         sync.Mutex
}

func NewReadOnlyRangeStore(connCtrl chx.Controller, tableName string) *RangeStore {
	s, _ := NewRangeStore(
		context.Background(),
		connCtrl,
		tableName,
		0,
		0,
		true)
	return s
}

func NewRangeStore(
	ctx context.Context,
	connCtrl chx.Controller,
	tableName string,
	cleanInterval time.Duration,
	maxKeepDays int,
	readOnly bool,
) (*RangeStore, error) {
	_, logger := log.FromContext(ctx)
	s := &RangeStore{
		ctrl: connCtrl,
		name: chx.FullName{
			Database: connCtrl.GetDatabase(),
			Name:     tableName,
		},
		readOnly: readOnly,
	}
	if !readOnly {
		sch := BuildTable(
			chx.FullName{
				Database: connCtrl.GetDatabase(),
				Name:     tableName,
			},
			&rangeObject{},
			chx.TableConfig{
				Engine:      chx.NewDefaultMergeTreeEngine(connCtrl.GetCluster() != ""),
				PartitionBy: "toYYYYMMDD(create_at)",
				OrderBy:     []string{"create_at"},
				Settings:    nil,
			},
			"",
		)
		pre, has, err := s.ctrl.LoadOne(ctx, sch.Table.FullName)
		if err != nil {
			return nil, errors.Wrapf(err, "load table %s failed", sch.Table.FullName)
		}
		if has {
			err = s.ctrl.Sync(ctx, pre, sch.Table)
		} else {
			err = s.ctrl.Create(ctx, sch.Table)
		}
		if err != nil {
			return nil, err
		}
		logger.Debugf("table for range store is ready")
		go s.keepCleanExpiredPartitions(ctx, cleanInterval, maxKeepDays)
	}
	return s, nil
}

func (s *RangeStore) cleanExpiredPartitions(ctx context.Context, maxKeepDays int) {
	_, logger := log.FromContext(ctx, "table", s.name.String())
	partitions, err := s.listPartitions(ctx)
	if err != nil {
		logger.Errorfe(err, "list partitions for clean expired failed")
	}
	if len(partitions) <= maxKeepDays {
		return
	}
	for _, partition := range partitions[:len(partitions)-maxKeepDays] {
		if err = s.deletePartition(ctx, partition.Partition); err != nil {
			logger.With("partition", partition.Partition).Errorfe(err, "drop expired partition")
		} else {
			logger.With("partition", partition.Partition).Infof("drop expired partition")
		}
	}
}

func (s *RangeStore) keepCleanExpiredPartitions(ctx context.Context, interval time.Duration, maxKeepDays int) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanExpiredPartitions(ctx, maxKeepDays)
		}
	}
}

func (s *RangeStore) setCurrent(cur rg.Range, keep time.Duration) {
	s.current, s.curValidAt = &cur, nil
	if keep > 0 {
		s.curValidAt = utils.WrapPointer(time.Now().Add(keep))
	}
}

func (s *RangeStore) getCurrent() (rg.Range, bool) {
	if s.current == nil {
		return rg.EmptyRange, false
	}
	if s.curValidAt != nil && time.Now().After(*s.curValidAt) {
		return rg.EmptyRange, false
	}
	return *s.current, true
}

func (s *RangeStore) get(ctx context.Context) (r rg.Range, err error) {
	if cur, has := s.getCurrent(); has {
		return cur, nil
	}
	sql := fmt.Sprintf("SELECT left, right FROM %s ORDER BY create_at DESC LIMIT 1", s.name.InSQL())
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var left, right uint64
		if scanErr := rows.Scan(&left, &right); scanErr != nil {
			return scanErr
		}
		r = rg.NewRange(left, right)
		return nil
	}, sql)
	s.setCurrent(r, time.Second)
	return r, nil
}

func (s *RangeStore) set(ctx context.Context, r rg.Range) error {
	sql := fmt.Sprintf("INSERT INTO %s (left, right, create_at) VALUES(%d, %d, NOW64())", s.name.InSQL(), r.Start, *r.End)
	return s.ctrl.Exec(ctx, sql)
}

func (s *RangeStore) Get(ctx context.Context) (rg.Range, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(ctx)
}

func (s *RangeStore) Update(ctx context.Context, operator rg.RangeOperator) (rg.Range, error) {
	if s.readOnly {
		return rg.EmptyRange, errors.New("now is read only mode")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	curRange, err := s.get(ctx)
	if err != nil {
		return rg.EmptyRange, err
	}
	newRange := operator(curRange)
	if newRange.End == nil {
		return rg.EmptyRange, errors.New("new range is infinity")
	}
	if err = s.set(ctx, newRange); err != nil {
		return rg.EmptyRange, err
	}
	s.setCurrent(newRange, 0)
	return newRange, nil
}
