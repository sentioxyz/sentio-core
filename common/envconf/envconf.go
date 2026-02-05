package envconf

import (
	"os"
	"sentioxyz/sentio-core/common/log"
	"strconv"
	"time"
)

func Load[V any](
	key string,
	defaultValue V,
	parseFunc func(string) (V, error),
	trimmers ...func(V) V,
) (v V) {
	var err error
	raw, has := os.LookupEnv(key)
	if has {
		if v, err = parseFunc(raw); err != nil {
			log.Warnfe(err, "parse env config %s with raw value %q failed", key, raw)
		}
	}
	if !has || err != nil {
		v = defaultValue
	}
	for _, trimmer := range trimmers {
		v = trimmer(v)
	}
	if has {
		log.Infof("loaded env config %s with raw value %q: %v", key, raw, v)
	} else {
		log.Infof("loaded env config %s with default value: %v", key, v)
	}
	return v
}

func LoadString(key string, defaultValue string, trimmers ...func(string) string) string {
	return Load(key, defaultValue, func(s string) (string, error) {
		return s, nil
	}, trimmers...)
}

func LoadBool(key string, defaultValue bool) bool {
	return Load(key, defaultValue, strconv.ParseBool)
}

func LoadUInt64(key string, defaultValue uint64, trimmers ...func(uint64) uint64) uint64 {
	return Load(key, defaultValue, func(s string) (uint64, error) {
		return strconv.ParseUint(s, 0, 64)
	}, trimmers...)
}

func LoadDuration(key string, defaultValue time.Duration, trimmers ...func(time.Duration) time.Duration) time.Duration {
	return Load(key, defaultValue, time.ParseDuration, trimmers...)
}

func WithMax(maxValue uint64) func(uint64) uint64 {
	return func(v uint64) uint64 {
		return min(v, maxValue)
	}
}

func WithMin(minValue uint64) func(uint64) uint64 {
	return func(v uint64) uint64 {
		return max(v, minValue)
	}
}

func WithMinDuration(minDuration time.Duration) func(d time.Duration) time.Duration {
	return func(d time.Duration) time.Duration {
		return max(minDuration, d)
	}
}

func WithMaxDuration(maxDuration time.Duration) func(d time.Duration) time.Duration {
	return func(d time.Duration) time.Duration {
		return min(maxDuration, d)
	}
}
