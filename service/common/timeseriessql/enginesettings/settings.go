package enginesettings

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/configmanager"
	"sentioxyz/sentio-core/common/log"

	kyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type Engine interface {
	CPUThreads() int
	MaxMemory() int64
	MaxExecutionTime() int
	Priority() int
	Name() string
	String() string
}

type engine struct {
	name             string
	cpuThreads       int
	maxMemory        int64
	maxExecutionTime int
	priority         int
}

type EngineConfig struct {
	Engines []struct {
		Name             string `yaml:"name"`
		CPUThreads       int    `yaml:"cpuThreads"`
		Memory           string `yaml:"memory"`
		MaxExecutionTime int    `yaml:"maxExecutionTime"`
		Priority         int    `yaml:"priority"`
	} `yaml:"engines"`
}

func NewEngine(name string, cpuThreads int, maxMemory int64, maxExecutionTime, priority int) Engine {
	return &engine{
		name:             name,
		cpuThreads:       cpuThreads,
		maxMemory:        maxMemory,
		maxExecutionTime: maxExecutionTime,
		priority:         priority,
	}
}

func (e *engine) CPUThreads() int {
	return e.cpuThreads
}

func (e *engine) MaxMemory() int64 {
	return e.maxMemory
}

func (e *engine) MaxExecutionTime() int {
	return e.maxExecutionTime
}

func (e *engine) Priority() int {
	return e.priority
}

func (e *engine) Name() string {
	return e.name
}

func (e *engine) String() string {
	return "name:" + e.name +
		", cpuThreads:" + strconv.Itoa(e.cpuThreads) +
		", maxMemory:" + strconv.FormatInt(e.maxMemory, 10) +
		", maxExecutionTime:" + strconv.Itoa(e.maxExecutionTime) +
		", priority:" + strconv.Itoa(e.priority)
}

type Setting interface {
	ToClickhouseSettings(name string) map[string]any
	OverwriteClickhouseSettings(name string, settings map[string]any)
}

type settings struct {
	engines map[string]Engine
	mutex   sync.RWMutex
}

func NewSettings(ctx context.Context, db *gorm.DB) (Setting, error) {
	if db != nil {
		if err := configmanager.Set("analytic_clickhouse_resource", configmanager.NewPgProvider(
			db, configmanager.WithPgKey("analytic_clickhouse_resource_configs")), kyaml.Parser(),
			&configmanager.LoadParams{
				EnableReload: true,
				ReloadPeriod: time.Minute * 10,
			}); err != nil {
			log.Errorf("failed to load clickhouse resource config: %v", err)
			return nil, err
		}
	}

	s := &settings{
		engines: make(map[string]Engine),
	}

	if err := s.watchConfig(); err != nil {
		log.Errorf("failed to watch clickhouse resource config: %v", err)
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			log.Infof("stop watching clickhouse resource config")
			return
		case <-time.After(time.Minute * 7):
			if err := s.watchConfig(); err != nil {
				log.Errorf("failed to watch clickhouse resource config: %v", err)
			}
		}
	}()
	return s, nil
}

func parseMemory(memory string) (int64, error) {
	var multiplier int64
	switch {
	case memory[len(memory)-1] == 'G':
		multiplier = 1024 * 1024 * 1024
	case memory[len(memory)-1] == 'M':
		multiplier = 1024 * 1024
	case memory[len(memory)-1] == 'K':
		multiplier = 1024
	default:
		return 0, errors.Errorf("invalid memory format: %s", memory)
	}
	value, err := strconv.Atoi(memory[:len(memory)-1])
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse memory value: %s", memory)
	}
	return int64(value) * multiplier, nil
}

func (s *settings) watchConfig() error {
	config, ok := configmanager.Get("analytic_clickhouse_resource")
	if !ok {
		log.Errorf("failed to get clickhouse resource config")
		return fmt.Errorf("config not found")
	}
	for _, engineConfig := range config.Raw().Slices("engines") {
		var maxMemory int64
		name := engineConfig.MustString("name")
		cpuThreads := engineConfig.Int("cpuThreads")
		memoryStr := engineConfig.String("memory")
		maxExecutionTime := engineConfig.Int("maxExecutionTime")
		priority := engineConfig.Int("priority")
		if memoryStr != "" {
			var err error
			maxMemory, err = parseMemory(memoryStr)
			if err != nil {
				return errors.Wrapf(err, "failed to parse memory %s", memoryStr)
			}
		}
		e := NewEngine(name, cpuThreads, maxMemory, maxExecutionTime, priority)
		if err := s.RegisterEngine(e); err != nil {
			return errors.Wrapf(err, "failed to register engine %s", name)
		}
	}
	return nil
}

func (s *settings) RegisterEngine(pkg Engine) error {
	if pkg == nil {
		return errors.Errorf("package is nil")
	}
	s.mutex.RLock()
	exists, ok := s.engines[strings.ToLower(pkg.Name())]
	if ok {
		if exists.String() == pkg.String() {
			log.With("settings", pkg.String()).
				Debugf("package %s already registered and equal", pkg.Name())
			s.mutex.RUnlock()
			return nil
		} else {
			log.With("old", exists.String(), "new", pkg.String()).
				Infof("package %s already registered but not equal, will replace with new one", pkg.Name())
		}
	} else {
		log.With("settings", pkg.String()).Infof("register package %s", pkg.Name())
	}
	s.mutex.RUnlock()

	s.mutex.Lock()
	s.engines[strings.ToLower(pkg.Name())] = pkg
	s.mutex.Unlock()
	return nil
}

func (s *settings) ToClickhouseSettings(name string) map[string]any {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var settings = make(map[string]any)
	if engine, ok := s.engines[strings.ToLower(name)]; ok {
		if engine.MaxMemory() > 0 {
			settings["max_memory_usage"] = uint64(engine.MaxMemory())
		}
		if engine.MaxExecutionTime() > 0 {
			settings["max_execution_time"] = engine.MaxExecutionTime()
		}
		if engine.CPUThreads() > 0 {
			settings["max_threads"] = engine.CPUThreads()
		}
		settings["priority"] = engine.Priority()
	}
	return nil
}

func (s *settings) OverwriteClickhouseSettings(name string, settings map[string]any) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if engine, ok := s.engines[strings.ToLower(name)]; ok {
		if engine.MaxMemory() > 0 {
			log.Debugf("overwrite max_memory_usage to %d", engine.MaxMemory())
			settings["max_memory_usage"] = uint64(engine.MaxMemory())
		}
		if engine.MaxExecutionTime() > 0 {
			log.Debugf("overwrite max_execution_time to %d", engine.MaxExecutionTime())
			settings["max_execution_time"] = engine.MaxExecutionTime()
		}
		if engine.CPUThreads() > 0 {
			log.Debugf("overwrite max_threads to %d", engine.CPUThreads())
			settings["max_threads"] = engine.CPUThreads()
		}
		settings["priority"] = engine.Priority()
	}
}
