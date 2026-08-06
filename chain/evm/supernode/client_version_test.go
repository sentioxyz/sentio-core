package supernode

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestClientVersionMiddleware(t *testing.T) {
	handler := NewClientVersionMiddleware()(
		func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			return nil, errors.New("passed through")
		})

	result, err := handler(context.Background(), "web3_clientVersion", nil)
	assert.NoError(t, err)
	identity, isString := result.(string)
	assert.True(t, isString)
	assert.True(t, strings.HasPrefix(identity, SuperNodeClientName+"/"), identity)

	_, err = handler(context.Background(), "eth_call", nil)
	assert.EqualError(t, err, "passed through")
}
