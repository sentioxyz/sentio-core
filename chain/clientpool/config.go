package clientpool

import (
	"encoding/json"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type ClientConfig[CONFIG EntryConfig[CONFIG]] struct {
	Priority uint32 `json:"priority" yaml:"priority"`
	Config   CONFIG `json:",inline" yaml:",inline"`
}

// MarshalJSON flattens Config fields into the same JSON object as Priority.
func (c ClientConfig[CONFIG]) MarshalJSON() ([]byte, error) {
	configBytes, err := json.Marshal(c.Config)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err = json.Unmarshal(configBytes, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]json.RawMessage)
	}
	if m["priority"], err = json.Marshal(c.Priority); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

// UnmarshalJSON reads Priority plus all Config fields from the same JSON object.
func (c *ClientConfig[CONFIG]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Priority uint32 `json:"priority"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.Priority = aux.Priority
	return json.Unmarshal(data, &c.Config)
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
		CheckSpeedInterval: utils.Select(c.CheckSpeedInterval > 0, c.CheckSpeedInterval, time.Minute),
		BanConfig: BanConfig{
			Min:        utils.Select(c.BanConfig.Min > 0, c.BanConfig.Min, time.Second),
			ExtendMax:  utils.Select(c.BanConfig.ExtendMax > 0, c.BanConfig.ExtendMax, time.Minute*5),
			ExtendRate: utils.Select(c.BanConfig.ExtendRate > 0, c.BanConfig.ExtendRate, 0.5),
		},
		AdjustPriorityInterval: utils.Select(c.AdjustPriorityInterval > 0, c.AdjustPriorityInterval, time.Second*30),
		UpgradeSensitivity:     utils.Select(c.UpgradeSensitivity > 0, c.UpgradeSensitivity, time.Minute*3),
		ClientConfigs: utils.MapSliceNoError(c.ClientConfigs, func(cc ClientConfig[CONFIG]) ClientConfig[CONFIG] {
			return ClientConfig[CONFIG]{
				Priority: cc.Priority,
				Config:   cc.Config.Trim(),
			}
		}),
	}
}
