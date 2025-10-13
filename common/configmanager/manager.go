package configmanager

import (
	"fmt"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"

	"github.com/knadh/koanf/v2"
	"github.com/samber/lo"
)

var ErrReloadNotSupported = fmt.Errorf("provider not support reload")

type LoadParams struct {
	EnableReload bool
	ReloadPeriod time.Duration
	MergeFunc    func(src, desc map[string]any) error
	StrictMode   bool
	Delim        string
}

type Manager interface {
	Get(name string) Config
	Load(name string, provider koanf.Provider, parser koanf.Parser, params *LoadParams) error
	Shutdown() error
}

type Config interface {
	String(path string) string
	MustString(path string) string
	Strings(path string) []string
	MustStrings(path string) []string
	StringMap(path string) map[string]string
	MustStringMap(path string) map[string]string
	Bytes(path string) []byte
	MustBytes(path string) []byte

	Bool(path string) bool
	Bools(path string) []bool
	MustBools(path string) []bool
	BoolMap(path string) map[string]bool
	MustBoolMap(path string) map[string]bool

	Int64(path string) int64
	MustInt64(path string) int64
	Int64s(path string) []int64
	MustInt64s(path string) []int64
	Int64Map(path string) map[string]int64
	MustInt64Map(path string) map[string]int64

	Int(path string) int
	MustInt(path string) int
	Ints(path string) []int
	MustInts(path string) []int
	IntMap(path string) map[string]int
	MustIntMap(path string) map[string]int

	Float64(path string) float64
	MustFloat64(path string) float64
	Float64s(path string) []float64
	MustFloat64s(path string) []float64
	Float64Map(path string) map[string]float64
	MustFloat64Map(path string) map[string]float64

	Duration(path string) time.Duration
	MustDuration(path string) time.Duration
	Time(path, layout string) time.Time
	MustTime(path, layout string) time.Time

	LoadAt() time.Time
	Sprint() string
	Merge(other Config) error
	Raw() *koanf.Koanf
}

type ExtendedProvider interface {
	Watch(period time.Duration, cb func(body any, err error))
	Unwatch()
}

type config struct {
	*koanf.Koanf
	name     string
	provider koanf.Provider
	parser   koanf.Parser
	params   *LoadParams
	loadAt   time.Time
}

func (c *config) LoadAt() time.Time {
	return c.loadAt
}

func (c *config) Merge(other Config) error {
	return c.Koanf.Merge(other.Raw())
}

func (c *config) Raw() *koanf.Koanf {
	return c.Koanf
}

var (
	managerOnce   sync.Once
	globalManager Manager
)

type manager struct {
	configs map[string]*config
	mutex   sync.RWMutex
}

func (m *manager) Get(name string) Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.configs[name]
}

func (m *manager) Load(name string, provider koanf.Provider, parser koanf.Parser, params *LoadParams) error {
	m.mutex.RLock()
	if _, ok := m.configs[name]; ok {
		m.mutex.RUnlock()
		return fmt.Errorf("config %s already loaded", name)
	}
	m.mutex.RUnlock()

	k := koanf.NewWithConf(koanf.Conf{
		Delim:       lo.If(params.Delim == "", ".").Else(params.Delim),
		StrictMerge: params.StrictMode,
	})
	var options []koanf.Option
	if params.MergeFunc != nil {
		options = append(options, koanf.WithMergeFunc(params.MergeFunc))
	}
	if err := k.Load(provider, parser, options...); err != nil {
		return err
	}
	c := &config{
		Koanf:    k,
		name:     name,
		provider: provider,
		parser:   parser,
		params:   params,
		loadAt:   time.Now(),
	}
	if c.params.EnableReload {
		ep, ok := provider.(ExtendedProvider)
		if !ok {
			return ErrReloadNotSupported
		}
		ep.Watch(c.params.ReloadPeriod, func(body any, err error) {
			if err != nil {
				log.Warnf("watch config %s error: %v", name, err)
				return
			}

			log.With("body", body).Infof("config %s changed, Reloading...", name)
			if err := c.Koanf.Load(c.provider, c.parser, options...); err != nil {
				log.Errorf("reload config %s error: %v", name, err)
			}
			c.loadAt = time.Now()
		})
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.configs[name] = c
	return nil
}

func (m *manager) Shutdown() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, c := range m.configs {
		ep, ok := c.provider.(ExtendedProvider)
		if ok {
			ep.Unwatch()
		}
	}
	m.configs = make(map[string]*config)
	return nil
}

func init() {
	managerOnce.Do(func() {
		globalManager = &manager{
			configs: make(map[string]*config),
		}
	})
}

func Get(name string) Config {
	return globalManager.Get(name)
}

func Load(name string, provider koanf.Provider, parser koanf.Parser, params *LoadParams) error {
	return globalManager.Load(name, provider, parser, params)
}

func Shutdown() error {
	return globalManager.Shutdown()
}
