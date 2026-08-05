package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/clientpool"
)

// fakeFullnode serves just enough of the aptos fullnode REST API for the client pool to boot
// (GET /v1) and for the account-resource probes (GET /v1/accounts/{address}/resources).
type fakeFullnode struct {
	// resourcesAt answers a probe: given the requested address and ledger version, return the
	// resources body, or an error code (e.g. "account_not_found") with its HTTP status
	resourcesAt func(address string, txVersion uint64) (body any, errCode string, status int)
	probeCount  atomic.Int64
}

func (f *fakeFullnode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/v1" || r.URL.Path == "/v1/" {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chain_id":              1,
			"epoch":                 "1",
			"ledger_timestamp":      "1700000000000000",
			"ledger_version":        "1000000",
			"oldest_ledger_version": "0",
			"node_role":             "full_node",
			"block_height":          "500000",
			"oldest_block_height":   "0",
			"git_hash":              "test",
		})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/v1/accounts/") && strings.HasSuffix(r.URL.Path, "/resources") {
		f.probeCount.Add(1)
		address := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/accounts/"), "/resources")
		txVersion, err := strconv.ParseUint(r.URL.Query().Get("ledger_version"), 10, 64)
		if err != nil {
			http.Error(w, `{"message":"bad ledger_version","error_code":"invalid_input"}`, http.StatusBadRequest)
			return
		}
		body, errCode, status := f.resourcesAt(address, txVersion)
		if errCode != "" {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": errCode, "error_code": errCode})
			return
		}
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	http.Error(w, `{"message":"not found","error_code":"web_framework_error"}`, http.StatusNotFound)
}

func newTestServiceV2(t *testing.T, node *fakeFullnode) *RPCServiceV2 {
	svr := httptest.NewServer(node)
	t.Cleanup(svr.Close)
	pool := aptos.NewClientPool("test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch := make(chan clientpool.PoolConfig[aptos.ClientConfig], 1)
	ch <- clientpool.PoolConfig[aptos.ClientConfig]{
		ClientConfigs: []clientpool.ClientConfig[aptos.ClientConfig]{
			{Config: aptos.ClientConfig{Endpoint: svr.URL}},
		},
	}
	go pool.Start(ctx, ch)
	assert.NoError(t, pool.WaitReady(ctx))
	// slotCache is nil: the pool-based search never touches it, and these tests only cover that
	// path (the change-record fallback is exercised by Test_aptosRpc against a live node)
	return NewRPCServiceV2(nil, &mockStorage{}, pool)
}

// accountSince answers probes like a normal account created at firstVersion: resources exist at
// any version >= firstVersion, account_not_found before that.
func accountSince(firstVersion uint64) func(string, uint64) (any, string, int) {
	return func(_ string, txVersion uint64) (any, string, int) {
		if txVersion >= firstVersion {
			return []map[string]any{{"type": "0x1::account::Account", "data": map[string]any{}}}, "", 0
		}
		return nil, "account_not_found", http.StatusNotFound
	}
}

func Test_searchAddressStartTxVersion(t *testing.T) {
	ctx := context.Background()
	const address = "0x0000000000000000000000000000000000000000000000000000000000000123"

	t.Run("found", func(t *testing.T) {
		for _, firstVersion := range []uint64{0, 1, 999, 2_999_999_999, 3_000_000_000} {
			t.Run(fmt.Sprintf("firstVersion=%d", firstVersion), func(t *testing.T) {
				node := &fakeFullnode{resourcesAt: accountSince(firstVersion)}
				svc := newTestServiceV2(t, node)
				ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
				assert.NoError(t, err)
				if assert.NotNil(t, ver) {
					assert.Equal(t, firstVersion, *ver)
				}
				assert.LessOrEqual(t, node.probeCount.Load(), int64(40),
					"binary search should cost O(log n) probes")

				// second call hits the LRU cache without probing
				node.probeCount.Store(0)
				ver, err = svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
				assert.NoError(t, err)
				if assert.NotNil(t, ver) {
					assert.Equal(t, firstVersion, *ver)
				}
				assert.Zero(t, node.probeCount.Load())
			})
		}
	})

	t.Run("no resources up to maxTxVersion", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: func(_ string, _ uint64) (any, string, int) {
			return nil, "account_not_found", http.StatusNotFound
		}}
		svc := newTestServiceV2(t, node)
		ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
		assert.NoError(t, err)
		assert.Nil(t, ver)
		assert.Equal(t, int64(1), node.probeCount.Load(), "one probe at maxTxVersion is enough")
	})

	t.Run("short address form is normalized before probing", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: func(address string, txVersion uint64) (any, string, int) {
			// AIP-40 special address short form
			if address != "0x1" {
				return nil, "account_not_found", http.StatusNotFound
			}
			return accountSince(42)(address, txVersion)
		}}
		svc := newTestServiceV2(t, node)
		ver, err := svc.GetAddressStartTxVersion(ctx, "0x001", 1000)
		assert.NoError(t, err)
		if assert.NotNil(t, ver) {
			assert.Equal(t, uint64(42), *ver)
		}
	})

	t.Run("maxTxVersion zero", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: accountSince(0)}
		svc := newTestServiceV2(t, node)
		ver, found, err := svc.searchAddressStartTxVersion(ctx, address, 0)
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(0), ver)
	})

	t.Run("probe error surfaces from search", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: func(_ string, _ uint64) (any, string, int) {
			return nil, "invalid_input", http.StatusBadRequest
		}}
		svc := newTestServiceV2(t, node)
		_, _, err := svc.searchAddressStartTxVersion(ctx, address, 1000)
		assert.ErrorContains(t, err, "invalid_input")
	})

	t.Run("invalid address surfaces from search", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: accountSince(0)}
		svc := newTestServiceV2(t, node)
		_, _, err := svc.searchAddressStartTxVersion(ctx, "not-an-address", 1000)
		assert.ErrorContains(t, err, "invalid account address")
	})
}
