package supernode

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func newTestServiceV2(prober AccountResourceProber) *RPCServiceV2 {
	// slotCache is nil: the prober-based search never touches it, and these tests only cover
	// that path (the change-record fallback is exercised by Test_aptosRpc against a live node)
	return NewRPCServiceV2(nil, &mockStorage{}, prober)
}

func Test_searchAddressStartTxVersion(t *testing.T) {
	ctx := context.Background()
	const address = "0x0000000000000000000000000000000000000000000000000000000000000123"

	t.Run("found", func(t *testing.T) {
		for _, firstVersion := range []uint64{0, 1, 999, 1_000_000, 2_999_999_999, 3_000_000_000} {
			probes := 0
			svc := newTestServiceV2(func(_ context.Context, addr string, txVersion uint64) (bool, error) {
				assert.Equal(t, address, addr)
				probes++
				return txVersion >= firstVersion, nil
			})
			ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
			assert.NoError(t, err)
			if assert.NotNil(t, ver) {
				assert.Equal(t, firstVersion, *ver)
			}
			assert.LessOrEqual(t, probes, 40, "binary search should cost O(log n) probes")

			// second call hits the cache without probing
			probes = 0
			ver, err = svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
			assert.NoError(t, err)
			if assert.NotNil(t, ver) {
				assert.Equal(t, firstVersion, *ver)
			}
			assert.Zero(t, probes)
		}
	})

	t.Run("no resources up to maxTxVersion", func(t *testing.T) {
		svc := newTestServiceV2(func(_ context.Context, _ string, _ uint64) (bool, error) {
			return false, nil
		})
		ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
		assert.NoError(t, err)
		assert.Nil(t, ver)
	})

	t.Run("short address form is normalized before probing", func(t *testing.T) {
		svc := newTestServiceV2(func(_ context.Context, addr string, txVersion uint64) (bool, error) {
			assert.Equal(t, "0x1", addr) // AIP-40 special address short form
			return txVersion >= 42, nil
		})
		ver, err := svc.GetAddressStartTxVersion(ctx, "0x001", 1000)
		assert.NoError(t, err)
		if assert.NotNil(t, ver) {
			assert.Equal(t, uint64(42), *ver)
		}
	})

	t.Run("maxTxVersion zero", func(t *testing.T) {
		svc := newTestServiceV2(func(_ context.Context, _ string, _ uint64) (bool, error) {
			return true, nil
		})
		ver, found, err := svc.searchAddressStartTxVersion(ctx, address, 0)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(0), ver)
	})

	t.Run("prober error surfaces from search", func(t *testing.T) {
		svc := newTestServiceV2(func(_ context.Context, _ string, _ uint64) (bool, error) {
			return false, errors.New("all endpoints pruned")
		})
		_, _, err := svc.searchAddressStartTxVersion(ctx, address, 1000)
		assert.ErrorContains(t, err, "all endpoints pruned")
	})

	t.Run("invalid address surfaces from search", func(t *testing.T) {
		svc := newTestServiceV2(func(_ context.Context, _ string, _ uint64) (bool, error) {
			return true, nil
		})
		_, _, err := svc.searchAddressStartTxVersion(ctx, "not-an-address", 1000)
		assert.ErrorContains(t, err, "invalid account address")
	})
}
