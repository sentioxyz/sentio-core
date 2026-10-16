package supernode

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/jsonrpc"
)

func TestNewSuperNode_GRPCOnlyDoesNotProxy(t *testing.T) {
	// A nil client pool proves the JSON-RPC proxy is never built: building it would dereference
	// the pool.
	chain := jsonrpc.MiddlewareChain(NewSuperNode(nil, nil, true))

	for _, method := range []string{"sui_getCheckpoint", "sui_getObject", "suix_getCoinMetadata"} {
		resp, err := chain.CallMethod(context.Background(), method, json.RawMessage(`[]`))
		assert.Nil(t, resp, method)
		require.Error(t, err, method)
		var rpcErr interface{ ErrorCode() int }
		require.ErrorAs(t, err, &rpcErr, method)
		assert.Equal(t, jsonrpc.MethodNotFoundErrorCode, rpcErr.ErrorCode(), method)
		assert.Contains(t, err.Error(), "gRPC only", method)
	}
}
