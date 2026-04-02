package clientpool

import "time"

type ClientConfig[CONFIG EntryConfig[CONFIG]] struct {
	Priority uint32
	Config   CONFIG
}

func (c ClientConfig[CONFIG]) Equal(a ClientConfig[CONFIG]) bool {
	return c.Priority == a.Priority && c.Config.Equal(a.Config)
}

type BanConfig struct {
	Min        time.Duration `json:"min"`
	ExtendMax  time.Duration `json:"extend_max"`
	ExtendRate float64       `json:"extend_rate"`
}

type PoolConfig[CONFIG EntryConfig[CONFIG]] struct {
	// How long does the latest block lag before the node is considered unavailable
	BrokenFallBehind time.Duration

	// Sampling interval for detecting block growth rate
	CheckSpeedInterval time.Duration

	// ban config
	BanConfig BanConfig

	AdjustPriorityInterval time.Duration
	UpgradeSensitivity     time.Duration

	ClientConfigs []ClientConfig[CONFIG]
}
