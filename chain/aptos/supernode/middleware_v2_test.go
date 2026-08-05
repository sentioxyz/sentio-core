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
// (GET /v1) and for the account-state probes (GET /v1/accounts/{address}/resources and
// GET /v1/accounts/{address}/modules).
type fakeFullnode struct {
	// resourcesAt / modulesAt answer a probe: given the requested address and ledger version,
	// return the response body, or an error code (e.g. "account_not_found") with its HTTP
	// status. A nil modulesAt answers account_not_found for every module probe.
	resourcesAt func(address string, txVersion uint64) (body any, errCode string, status int)
	modulesAt   func(address string, txVersion uint64) (body any, errCode string, status int)
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
	if kind, answer := f.route(r.URL.Path); answer != nil {
		f.probeCount.Add(1)
		address := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/accounts/"), "/"+kind)
		txVersion, err := strconv.ParseUint(r.URL.Query().Get("ledger_version"), 10, 64)
		if err != nil {
			http.Error(w, `{"message":"bad ledger_version","error_code":"invalid_input"}`, http.StatusBadRequest)
			return
		}
		body, errCode, status := answer(address, txVersion)
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

func (f *fakeFullnode) route(path string) (string, func(string, uint64) (any, string, int)) {
	if !strings.HasPrefix(path, "/v1/accounts/") {
		return "", nil
	}
	switch {
	case strings.HasSuffix(path, "/resources"):
		return "resources", f.resourcesAt
	case strings.HasSuffix(path, "/modules"):
		if f.modulesAt == nil {
			return "modules", func(string, uint64) (any, string, int) {
				return nil, "account_not_found", http.StatusNotFound
			}
		}
		return "modules", f.modulesAt
	}
	return "", nil
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
				assert.LessOrEqual(t, node.probeCount.Load(), int64(80),
					"binary search should cost O(log n) probes (at most 2 requests per step)")

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

	t.Run("no state up to maxTxVersion", func(t *testing.T) {
		node := &fakeFullnode{resourcesAt: func(_ string, _ uint64) (any, string, int) {
			return nil, "account_not_found", http.StatusNotFound
		}}
		svc := newTestServiceV2(t, node)
		ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
		assert.NoError(t, err)
		assert.Nil(t, ver)
		assert.Equal(t, int64(2), node.probeCount.Load(),
			"one resources + one modules probe at maxTxVersion is enough")
	})

	t.Run("stateless account owning only modules", func(t *testing.T) {
		// a module published through an orderless, fee-sponsored transaction never materializes
		// 0x1::account::Account, so the address owns modules but no resource at all
		node := &fakeFullnode{
			resourcesAt: func(_ string, _ uint64) (any, string, int) {
				return nil, "account_not_found", http.StatusNotFound
			},
			modulesAt: func(_ string, txVersion uint64) (any, string, int) {
				if txVersion >= 777 {
					return []map[string]any{{"bytecode": "0x", "abi": map[string]any{}}}, "", 0
				}
				return nil, "account_not_found", http.StatusNotFound
			},
		}
		svc := newTestServiceV2(t, node)
		ver, err := svc.GetAddressStartTxVersion(ctx, address, 3_000_000_000)
		assert.NoError(t, err)
		if assert.NotNil(t, ver) {
			assert.Equal(t, uint64(777), *ver)
		}
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
