package clientpool

import (
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type ClientConfig[CONFIG EntryConfig[CONFIG]] struct {
	Priority uint32
	Config   CONFIG
}

func (c ClientConfig[CONFIG]) Equal(a ClientConfig[CONFIG]) bool {
	return c.Priority == a.Priority && c.Config.Equal(a.Config)
}

type BanConfig struct {
	Min        time.Duration `json:"min"         yaml:"min"`
	ExtendMax  time.Duration `json:"extend_max"  yaml:"extend_max"`
	ExtendRate float64       `json:"extend_rate" yaml:"extend_rate"`
}

type PoolConfig[CONFIG EntryConfig[CONFIG]] struct {
	// How long does the latest block lag before the node is considered unavailable
	BrokenFallBehind time.Duration `json:"broken_fall_behind" yaml:"broken_fall_behind"`

	// Sampling interval for detecting block growth rate
	CheckSpeedInterval time.Duration `json:"check_speed_interval" yaml:"check_speed_interval"`

	// ban config
	BanConfig BanConfig `json:"ban" yaml:"ban"`

	AdjustPriorityInterval time.Duration `json:"adjust_priority_interval" yaml:"adjust_priority_interval"`
	UpgradeSensitivity     time.Duration `json:"upgrade_sensitivity" yaml:"upgrade_sensitivity"`

	ClientConfigs []ClientConfig[CONFIG] `json:"endpoints" yaml:"endpoints"`
}

func (c PoolConfig[CONFIG]) Trim() PoolConfig[CONFIG] {
	return PoolConfig[CONFIG]{
		BrokenFallBehind:   max(c.BrokenFallBehind, 0),
		CheckSpeedInterval: max(c.CheckSpeedInterval, time.Minute),
		BanConfig: BanConfig{
			Min:        max(c.BanConfig.Min, time.Second),
			ExtendMax:  max(c.BanConfig.ExtendMax, time.Minute*5),
			ExtendRate: max(c.BanConfig.ExtendRate, 0.5),
		},
		AdjustPriorityInterval: max(c.AdjustPriorityInterval, time.Second*30),
		UpgradeSensitivity:     max(c.UpgradeSensitivity, time.Minute*3),
		ClientConfigs: utils.MapSliceNoError(c.ClientConfigs, func(cc ClientConfig[CONFIG]) ClientConfig[CONFIG] {
			return ClientConfig[CONFIG]{
				Priority: cc.Priority,
				Config:   cc.Config.Trim(),
			}
		}),
	}
}
