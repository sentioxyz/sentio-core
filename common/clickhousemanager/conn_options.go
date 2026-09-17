package ckhmanager

import (
	"crypto/ecdsa"
	"time"

	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"

	"github.com/ethereum/go-ethereum/crypto"
)

type Options struct {
	settings                                  map[string]any
	readTimeout, dialTimeout, connMaxLifeTime time.Duration
	maxIdleConns, maxOpenConns                int

	// serialization ignored
	privateKeyHex string
	privateKey    *ecdsa.PrivateKey
	payer         string
}

// Serialization renders the options into the string that identifies a cached connection. It carries
// the signing private key, so it must never be logged: use LogSerialization for that.
func (o *Options) Serialization() string {
	return o.serialization(false)
}

// LogSerialization renders the options with the signing private key masked, for logging.
func (o *Options) LogSerialization() string {
	return o.serialization(true)
}

func (o *Options) serialization(maskSecrets bool) string {
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
	if o.connMaxLifeTime > 0 {
		s += "conn_max_life_time=" + o.connMaxLifeTime.String() + ","
	}
	if o.maxIdleConns > 0 {
		s += "max_idle_conns=" + anyutil.ParseString(o.maxIdleConns) + ","
	}
	if o.maxOpenConns > 0 {
		s += "max_open_conns=" + anyutil.ParseString(o.maxOpenConns) + ","
	}
	if o.privateKeyHex != "" {
		privateKey := o.privateKeyHex
		if maskSecrets {
			privateKey = utils.AddSecretMosaic(privateKey)
		}
		s += "private_key=" + privateKey + ","
	}
	if o.payer != "" {
		s += "payer=" + o.payer + ","
	}
	if len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}

func ConnectWithSettings(settings map[string]any) func(o *Options) {
	return func(o *Options) {
		// merge settings if already exists, otherwise set directly
		if o.settings == nil {
			o.settings = make(map[string]any)
		}
		for k, v := range settings {
			o.settings[k] = v
		}
	}
}

func ConnectWithPayer(payer string) func(o *Options) {
	return func(o *Options) {
		o.payer = payer
	}
}

func ConnectWithPrivateKey(privateKeyHex string) func(o *Options) {
	return func(o *Options) {
		o.privateKeyHex = privateKeyHex
		var err error
		o.privateKey, err = crypto.HexToECDSA(privateKeyHex)
		if err != nil {
			log.Errorfe(err, "invalid private key, ignoring")
		}
	}
}

type DialConfig struct {
	ReadTimeout     time.Duration
	DialTimeout     time.Duration
	ConnMaxLifetime time.Duration
	MaxIdleConns    int
	MaxOpenConns    int
}

func ConnectWithDialConfig(config DialConfig) func(o *Options) {
	return func(o *Options) {
		o.readTimeout = config.ReadTimeout
		o.dialTimeout = config.DialTimeout
		o.connMaxLifeTime = config.ConnMaxLifetime
		o.maxIdleConns = config.MaxIdleConns
		o.maxOpenConns = config.MaxOpenConns
	}
}
