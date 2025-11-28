package cache

import (
	"time"

	"sentioxyz/sentio-core/common/tokenbucket"
)

const (
	refreshBackgroundOptID = iota
	loaderArgvOptID
	concurrencyControlOptID
	noCacheOptID
	specifyTTL
	specifyRefreshInterval
	noCacheTokenBucketOptID
)

type Option struct {
	optID                    int
	refreshBackground        bool
	concurrencyControl       bool
	noCache                  bool
	force                    bool
	loaderArgv               []any
	specifiedTTL             time.Duration
	specifiedRefreshInterval time.Duration
	tokenBucket              tokenbucket.TokenBucket
	tokenBucketConfig        *tokenbucket.RateLimitConfig
}

func WithRefreshBackground() *Option {
	return &Option{
		optID:             refreshBackgroundOptID,
		refreshBackground: true,
	}
}

func WithLoaderArgv(argv []any) *Option {
	return &Option{
		optID:      loaderArgvOptID,
		loaderArgv: argv,
	}
}

func WithConcurrencyControl() *Option {
	return &Option{
		optID:              concurrencyControlOptID,
		concurrencyControl: true,
	}
}

func WithSpecifiedTTL(t time.Duration) *Option {
	return &Option{
		optID:        specifyTTL,
		specifiedTTL: t,
	}
}

func WithSpecifiedRefreshInterval(t time.Duration) *Option {
	return &Option{
		optID:                    specifyRefreshInterval,
		specifiedRefreshInterval: t,
	}
}

func WithNoCache() *Option {
	return &Option{
		optID:   noCacheOptID,
		noCache: true,
	}
}

func WithNoCacheTokenBucket(tokenBucket tokenbucket.TokenBucket, config *tokenbucket.RateLimitConfig) *Option {
	return &Option{
		optID:             noCacheTokenBucketOptID,
		tokenBucket:       tokenBucket,
		tokenBucketConfig: config,
	}
}

func mergeOptions(options []*Option) *Option {
	option := &Option{}
	for _, o := range options {
		switch o.optID {
		case refreshBackgroundOptID:
			option.refreshBackground = true
		case loaderArgvOptID:
			option.loaderArgv = o.loaderArgv
		case concurrencyControlOptID:
			option.concurrencyControl = true
		case noCacheOptID:
			option.noCache = true
		case specifyTTL:
			option.specifiedTTL = o.specifiedTTL
		case specifyRefreshInterval:
			option.specifiedRefreshInterval = o.specifiedRefreshInterval
		case noCacheTokenBucketOptID:
			option.tokenBucket = o.tokenBucket
			option.tokenBucketConfig = o.tokenBucketConfig
		}
	}
	return option
}
