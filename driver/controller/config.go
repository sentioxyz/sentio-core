package controller

import (
	"time"

	"sentioxyz/sentio-core/common/envconf"
)

var (
	ProcessConcurrency = envconf.LoadUInt64("PROCESS_CONCURRENCY", 20,
		envconf.WithMax(200), envconf.WithMin(1))
	SaveCheckpointDelay = envconf.LoadDuration("SENTIO_SAVE_CHECKPOINT_DELAY", 0,
		envconf.WithMinDuration(0), envconf.WithMaxDuration(time.Minute*10))
	SaveCheckpointInterval = envconf.LoadDuration("SENTIO_SAVE_CHECKPOINT_INTERVAL", time.Second*20,
		envconf.WithMinDuration(time.Second))
	MaxKeepCheckpointCount = envconf.LoadUInt64("SENTIO_KEEP_CHECKPOINT_COUNT", 1000000,
		envconf.WithMin(10000))
	SubscribeMinWatchInterval = envconf.LoadDuration("SENTIO_SUBSCRIBE_MIN_WATCH_INTERVAL", time.Second)
	ClientMaxConcurrency      = envconf.LoadUInt64("SENTIO_CLIENT_MAX_CONCURRENCY", 100, envconf.WithMin(10))
	PrintProcessedInterval    = envconf.LoadDuration("SENTIO_PRINT_PROCESSED_INTERVAL", time.Second)
	SkipStartBlockValidation  = envconf.LoadBool("SENTIO_SKIP_START_BLOCK_VALIDATION", false)
)

const (
	// WatchingDelay If the difference between the time of a processed block and the latest block is less than this value,
	// then the block is considered to be in the watching state.
	WatchingDelay = time.Minute * 5

	// RunWaiting If a round got an external error that can be retried, wait this long before starting the next round
	RunWaiting = time.Second * 30
)
