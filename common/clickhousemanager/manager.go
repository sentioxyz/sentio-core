package ckhmanager

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/models"
	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"gopkg.in/yaml.v3"
)

type PickOptions interface {
	GetProject() *models.Project
}

type Manager interface {
	GetShardByIndex(i int32) Sharding
	GetShardByName(name string) Sharding
	All() []Sharding
	Pick(PickOptions) (int32, Sharding)
	Reload(Config) error
	DefaultIndex() int32
}

type ShardingStrategy struct {
	ProjectsMapping      map[string][]int32
	OrganizationsMapping map[string][]int32
	TiersMapping         map[protoscommon.Tier][]int32
	PickStrategy         string
}

type ShardingConfig struct {
	Index              int32             `yaml:"index" json:"index"`
	Name               string            `yaml:"name" json:"name"`
	AllowTiers         []int32           `yaml:"allow_tiers" json:"allow_tiers"`
	AllowOrganizations []string          `yaml:"allow_organizations" json:"allow_organizations"`
	AllowProjects      []string          `yaml:"allow_projects" json:"allow_projects"`
	Addresses          map[string]string `yaml:"addresses" json:"addresses"`
}

type Config struct {
	ReadTimeout        time.Duration         `yaml:"read_timeout" json:"read_timeout"`
	DialTimeout        time.Duration         `yaml:"dial_timeout" json:"dial_timeout"`
	MaxIdleConnections int                   `yaml:"max_idle_connections" json:"max_idle_connections"`
	MaxOpenConnections int                   `yaml:"max_open_connections" json:"max_open_connections"`
	Settings           map[string]any        `yaml:"settings" json:"settings"`
	Credential         map[string]Credential `yaml:"credential" json:"credential"`

	Shards []ShardingConfig `yaml:"shards"`
}

type manager struct {
	shards       map[int32]Sharding
	strategies   ShardingStrategy
	defaultIndex int32

	shardIndex        map[string]int32
	shardReverseIndex map[int32]string
	mutex             sync.RWMutex
}

func initShard(credential map[string]Credential, shardingConfig ShardingConfig, connOptions []func(*Options)) Sharding {
	return NewSharding(
		shardingConfig.Index,
		shardingConfig.Name,
		credential,
		shardingConfig.Addresses,
		connOptions...)
}

func (m *manager) GetShardByIndex(i int32) Sharding {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.shards[i]
}

func (m *manager) GetShardByName(name string) Sharding {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.shards[m.shardIndex[name]]
}

func (m *manager) All() []Sharding {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	shards := make([]Sharding, 0, len(m.shards))
	for _, shard := range m.shards {
		shards = append(shards, shard)
	}
	return shards
}

func (m *manager) pickByStrategy(key string, indexes []int32) int32 {
	if len(indexes) == 0 {
		return m.defaultIndex
	}
	switch m.strategies.PickStrategy {
	case "random":
		return indexes[rand.Int31n(int32(len(indexes)))]
	case "hash":
		f := fnv.New64a()
		f.Write([]byte(key))
		return int32(f.Sum64()) % int32(len(indexes))
	}
	return indexes[0]
}

func (m *manager) Pick(options PickOptions) (int32, Sharding) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.strategies.PickStrategy == "" {
		return m.defaultIndex, m.GetShardByIndex(m.defaultIndex)
	}
	if project := options.GetProject(); project != nil {
		if indexes, ok := m.strategies.ProjectsMapping[project.FullName()]; ok {
			if len(indexes) > 0 {
				i := m.pickByStrategy(project.FullName(), indexes)
				log.Infof("clickhouse manager use project mappings, project: %s, index: %d", project.FullName(), i)
				return i, m.GetShardByIndex(i)
			}
		}
		if org := project.OwnerAsOrg; org != nil {
			if indexes, ok := m.strategies.OrganizationsMapping[org.Name]; ok {
				if len(indexes) > 0 {
					i := m.pickByStrategy(project.FullName(), indexes)
					log.Infof("clickhouse manager use organization mappings"+
						", project: %s, organization: %s, index: %d", project.FullName(), org.Name, i)
					return i, m.GetShardByIndex(i)
				}
			}
		}
		var tier int32 = -1
		switch project.OwnerType {
		case models.ProjectOwnerTypeOrg:
			if org := project.OwnerAsOrg; org != nil {
				tier = org.Tier
			}
		case models.ProjectOwnerTypeUser:
			if user := project.OwnerAsUser; user != nil {
				tier = user.Tier
			}
		}
		if tier != -1 {
			log.Infof("clickhouse manager check tier mappings, project: %s, tier: %s", project.FullName(), protoscommon.Tier(tier).String())
			if _, ok := protoscommon.Tier_name[tier]; ok {
				if indexes, ok := m.strategies.TiersMapping[protoscommon.Tier(tier)]; ok {
					if len(indexes) > 0 {
						i := m.pickByStrategy(project.FullName(), indexes)
						log.Infof("clickhouse manager use tier mappings, project: %s, tier: %s, index: %d", project.FullName(), protoscommon.Tier(tier).String(), i)
						return i, m.GetShardByIndex(i)
					}
				}
			}
		}
	}
	i := m.pickByStrategy("", m.strategies.TiersMapping[protoscommon.Tier_FREE])
	log.Infof("clickhouse manager fallback to: %d", i)
	return i, m.GetShardByIndex(i)
}

func (m *manager) Reload(config Config) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	ok, shards, strategies, defaultIndex, shardIndex, shardReverseIndex := loadShardingConfig(config, true)
	if !ok {
		return fmt.Errorf("failed to reload sharding config")
	}
	m.shardIndex = shardIndex
	m.shardReverseIndex = shardReverseIndex
	m.shards = shards
	m.strategies = strategies
	m.defaultIndex = defaultIndex
	return nil
}

func (m *manager) DefaultIndex() int32 {
	return m.defaultIndex
}

func addTierMapping(strategies *ShardingStrategy, index int32, tiers []int32) {
	for _, tier := range tiers {
		if _, ok := protoscommon.Tier_name[tier]; ok {
			strategies.TiersMapping[protoscommon.Tier(tier)] = append(strategies.TiersMapping[protoscommon.Tier(tier)], index)
		} else {
			log.Warnf("invalid tier enum: %d, will ignore", tier)
		}
	}
}

func addOrganizationMapping(strategies *ShardingStrategy, index int32, organizations []string) {
	for _, org := range organizations {
		strategies.OrganizationsMapping[org] = append(strategies.OrganizationsMapping[org], index)
	}
}

func addProjectMapping(strategies *ShardingStrategy, index int32, projects []string) {
	for _, project := range projects {
		strategies.ProjectsMapping[project] = append(strategies.ProjectsMapping[project], index)
	}
}

func verify(strategies *ShardingStrategy, allowPanic bool) bool {
	if len(strategies.TiersMapping[protoscommon.Tier_FREE]) == 0 {
		if allowPanic {
			panic("there is no available sharding for free tier, please check your config")
		}
		return false
	}
	for idx := range protoscommon.Tier_name {
		if len(strategies.TiersMapping[protoscommon.Tier(idx)]) == 0 {
			log.Infof("no sharding instance for tier: %s, will use free tier instead", protoscommon.Tier_name[idx])
		}
	}
	return true
}

func loadShardingConfig(config Config, allowPanic bool) (ok bool, shards map[int32]Sharding, strategies ShardingStrategy, defaultIndex int32,
	shardIndex map[string]int32, shardReverseIndex map[int32]string) {
	shards = make(map[int32]Sharding)
	strategies = ShardingStrategy{
		ProjectsMapping:      make(map[string][]int32),
		OrganizationsMapping: make(map[string][]int32),
		TiersMapping:         make(map[protoscommon.Tier][]int32),
	}
	shardIndex = make(map[string]int32)
	shardReverseIndex = make(map[int32]string)
	defaultIndex = 0

	var connOptions []func(*Options)
	connOptions = append(connOptions, ConnectWithDialConfig(dialConfig{
		readTimeout:  config.ReadTimeout,
		dialTimeout:  config.DialTimeout,
		maxIdleConns: config.MaxIdleConnections,
		maxOpenConns: config.MaxOpenConnections,
	}))
	connOptions = append(connOptions, ConnectWithSettings(config.Settings))

	for _, shard := range config.Shards {
		_, exists := shardIndex[shard.Name]
		if exists {
			log.Errorf("duplicate shard name: " + shard.Name)
			continue
		}
		_, exists = shardReverseIndex[shard.Index]
		if exists {
			log.Errorf("duplicate shard index: " + shard.Name)
			continue
		}

		sharding := initShard(config.Credential, shard, connOptions)
		shards[shard.Index] = sharding
		shardIndex[shard.Name] = shard.Index
		shardReverseIndex[shard.Index] = shard.Name

		addTierMapping(&strategies, shard.Index, shard.AllowTiers)
		addOrganizationMapping(&strategies, shard.Index, shard.AllowOrganizations)
		addProjectMapping(&strategies, shard.Index, shard.AllowProjects)
	}

	ok = verify(&strategies, allowPanic)
	defaultIndex = strategies.TiersMapping[protoscommon.Tier_FREE][0]
	return
}

func NewManager(config Config) Manager {
	_, shards, strategies, defaultIndex, shardIndex, shardReverseIndex := loadShardingConfig(config, true)
	manager := &manager{
		shards:            shards,
		strategies:        strategies,
		shardIndex:        shardIndex,
		shardReverseIndex: shardReverseIndex,
		defaultIndex:      defaultIndex,
	}
	return manager
}

func loadConfig(configPath string) Config {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Errorf("failed to read file: %v", err)
		panic(err)
	}

	var config Config
	switch {
	case strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml"):
		err = yaml.Unmarshal(data, &config)
	case strings.HasSuffix(configPath, ".json"):
		err = json.Unmarshal(data, &config)
	default:
		log.Errorf("unsupported config file type: %s", configPath)
		panic("unsupported config file type")
	}
	if err != nil {
		log.Errorf("failed to unmarshal config file: %v", err)
		panic(err)
	}
	return config
}

func LoadManager(configPath string) Manager {
	if configPath == "" {
		return nil
	}
	return NewManager(loadConfig(configPath))
}
