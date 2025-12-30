package ckhmanager

import (
	"time"

	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/utils"
)

type Options struct {
	settings                   map[string]any
	readTimeout, dialTimeout   time.Duration
	maxIdleConns, maxOpenConns int
}

func (o *Options) Serialization() string {
	var s string
	for _, k := range utils.GetOrderedMapKeys(o.settings) {
		s += k + "=" + anyutil.ToString(o.settings[k]) + ","
	}
	if o.readTimeout > 0 {
		s += "read_timeout=" + o.readTimeout.String() + ","
	}
	if o.dialTimeout > 0 {
		s += "dial_timeout=" + o.dialTimeout.String() + ","
	}
	if o.maxIdleConns > 0 {
		s += "max_idle_conns=" + anyutil.ParseString(o.maxIdleConns) + ","
	}
	if o.maxOpenConns > 0 {
		s += "max_open_conns=" + anyutil.ParseString(o.maxOpenConns) + ","
	}
	if len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}

func WithSettings(settings map[string]any) func(o *Options) {
	return func(o *Options) {
		o.settings = settings
	}
}

func WithDialConfig(config struct {
	readTimeout  time.Duration
	dialTimeout  time.Duration
	maxIdleConns int
	maxOpenConns int
}) func(o *Options) {
	return func(o *Options) {
		o.readTimeout = config.readTimeout
		o.dialTimeout = config.dialTimeout
		o.maxIdleConns = config.maxIdleConns
		o.maxOpenConns = config.maxOpenConns
	}
}
