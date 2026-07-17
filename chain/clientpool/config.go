package clientpool

import (
	"encoding/json"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type ClientConfig[CONFIG EntryConfig[CONFIG]] struct {
	// Index is the position of this entry in the original ClientConfigs slice, assigned during Trim.
	// Excluded from serialization; passed to Notifier so callers can identify which entry triggered the notification.
	Index    uint32 `json:"-" yaml:"-"`
	Priority uint32 `json:"priority" yaml:"priority"`
	// MethodAuthority marks this entry as defining the pool's supported method set (typically the
	// chain's own full nodes). When any method-authority entry reports a method as not supported,
	// the pool rejects that method outright instead of probing other endpoints for it — except
	// for methods disabled by the entry's own method black/white list (see MethodACL), for which
	// the entry abstains.
	MethodAuthority bool   `json:"method_authority,omitempty" yaml:"method_authority,omitempty"`
	Config          CONFIG `json:",inline" yaml:",inline"`
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
	if c.MethodAuthority {
		if m["method_authority"], err = json.Marshal(c.MethodAuthority); err != nil {
			return nil, err
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON reads Priority plus all Config fields from the same JSON object.
func (c *ClientConfig[CONFIG]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Priority        uint32 `json:"priority"`
		MethodAuthority bool   `json:"method_authority"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.Priority = aux.Priority
	c.MethodAuthority = aux.MethodAuthority
	return json.Unmarshal(data, &c.Config)
}

func (c ClientConfig[CONFIG]) Equal(a ClientConfig[CONFIG]) bool {
	return c.Priority == a.Priority && c.MethodAuthority == a.MethodAuthority && c.Config.Equal(a.Config)
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

	TagDuration time.Duration `json:"tag_duration" yaml:"tag_duration"`

	ConsumerMaxWait time.Duration `json:"consumer_max_wait" yaml:"consumer_max_wait"`

	ClientConfigs []ClientConfig[CONFIG] `json:"endpoints" yaml:"endpoints"`
}

func (c PoolConfig[CONFIG]) Trim(configModifiers []ConfigModifier[CONFIG]) PoolConfig[CONFIG] {
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
		TagDuration:            utils.Select(c.TagDuration > 0, c.TagDuration, time.Minute*30),
		ConsumerMaxWait:        utils.Select(c.ConsumerMaxWait > 0, c.ConsumerMaxWait, time.Minute*2),
		ClientConfigs: utils.MapSliceNoErrWithIndex(c.ClientConfigs, func(index int, cc ClientConfig[CONFIG]) (ClientConfig[CONFIG], bool) {
			ccc := cc.Config
			for _, m := range configModifiers {
				ccc = m(ccc)
			}
			return ClientConfig[CONFIG]{
				Index:           uint32(index),
				Priority:        cc.Priority,
				MethodAuthority: cc.MethodAuthority,
				Config:          ccc,
			}, true
		}),
	}
}
