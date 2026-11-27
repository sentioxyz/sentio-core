package clientpool

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BuildHTTPRequest_URL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		path    string
		params  url.Values
		want    string
	}{
		{"root path keeps endpoint path", "https://node.example/v1/graphql", "/", nil, "https://node.example/v1/graphql"},
		{"empty path keeps endpoint path", "https://node.example/v1/graphql", "", nil, "https://node.example/v1/graphql"},
		{"root path on bare host", "https://node.example", "/", nil, "https://node.example/"},
		{"empty path on bare host", "https://node.example", "", nil, "https://node.example/"},
		{"sub path is joined", "https://node.example", "/v1", nil, "https://node.example/v1"},
		{"sub path under endpoint path", "https://node.example/key", "/v1/accounts", nil,
			"https://node.example/key/v1/accounts"},
		{"explicit trailing slash is kept", "https://node.example/v1", "/accounts/", nil,
			"https://node.example/v1/accounts/"},
		{"params are merged", "https://node.example/v1?a=1", "/", url.Values{"b": {"2"}},
			"https://node.example/v1?a=1&b=2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := BuildHTTPRequest(context.Background(), "POST", c.baseURL, c.path, c.params, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, c.want, req.URL.String())
		})
	}
}
