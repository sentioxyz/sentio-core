package supernode

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/sol"
	rg "sentioxyz/sentio-core/common/range"
)

// fakeArchiveStore is a Storage whose only meaningful method is CheckPermission; the rest are never
// called by firstSlot, so the embedded nil interface is fine.
type fakeArchiveStore struct {
	Storage
	permErr error
}

func (f fakeArchiveStore) CheckPermission(context.Context) error { return f.permErr }

type fakeRangeStore struct{ start uint64 }

func (f fakeRangeStore) Get(context.Context) (rg.Range, error) { return rg.Range{Start: f.start}, nil }
func (f fakeRangeStore) Update(context.Context, rg.RangeOperator) (rg.Range, error) {
	return rg.Range{Start: f.start}, nil
}

func TestFirstSlot(t *testing.T) {
	ctx := context.Background()
	const chStart = 1000
	rng := fakeRangeStore{start: chStart}

	t.Run("permitted caller indexes from 0", func(t *testing.T) {
		svc := &RPCService{bqStore: fakeArchiveStore{permErr: nil}, rangeStore: rng}
		got, err := svc.firstSlot(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), got)
	})

	t.Run("clean denial floors at the ClickHouse range start", func(t *testing.T) {
		denied := errors.Wrap(sol.ErrArchiveAccessDenied, "tier FREE not permitted")
		svc := &RPCService{bqStore: fakeArchiveStore{permErr: denied}, rangeStore: rng}
		got, err := svc.firstSlot(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(chStart), got)
	})

	t.Run("transient permission error is propagated, not treated as denial", func(t *testing.T) {
		boom := errors.New("tier db down")
		svc := &RPCService{bqStore: fakeArchiveStore{permErr: boom}, rangeStore: rng}
		_, err := svc.firstSlot(ctx)
		require.ErrorIs(t, err, boom)
	})

	t.Run("no archive tier floors at the ClickHouse range start", func(t *testing.T) {
		svc := &RPCService{bqStore: nil, rangeStore: rng}
		got, err := svc.firstSlot(ctx)
		require.NoError(t, err)
		assert.Equal(t, uint64(chStart), got)
	})
}
