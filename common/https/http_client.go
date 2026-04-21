package https

import (
	"net/http"
	"time"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/monitoring"
)

var DefaultClient *http.Client

var defaultMaxIdleConnsPerHost uint64

func init() {
	defaultMaxIdleConnsPerHost = envconf.LoadUInt64("MAX_IDLE_CONNS_PER_HOST", 2000)
	DefaultClient = NewClient()
}

type config struct {
	maxIdleConnsPerHost int
	timeout             time.Duration
}

type Option func(*config)

func NewClient(opts ...Option) *http.Client {
	c := config{
		maxIdleConnsPerHost: int(defaultMaxIdleConnsPerHost),
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &http.Client{
		Transport: monitoring.NewWrappedTraceRoundTripper(&http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			MaxIdleConnsPerHost:   c.maxIdleConnsPerHost,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}),
		Timeout: c.timeout,
	}
}

func WithMaxIdleConnsPerHost(maxIdleConnsPerHost int) Option {
	return func(c *config) {
		c.maxIdleConnsPerHost = maxIdleConnsPerHost
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}
