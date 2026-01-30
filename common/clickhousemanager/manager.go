package ckhmanager

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/network/state"
	"sentioxyz/sentio-core/service/common/models"
	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

type ShardingIndex uint64

func (s ShardingIndex) String() string {
	return fmt.Sprintf("%d", s)
}

func (s ShardingIndex) Int64() int64 {
	if s > math.MaxInt64 {
		log.Errorf("sharding index overflow: %d", s)
	}
	return int64(s)
}

func (s ShardingIndex) Uint32() uint32 {
	if s > math.MaxUint32 {
		log.Errorf("sharding index overflow: %d", s)
	}
	return uint32(s)
}

func (s ShardingIndex) Int32() int32 {
	if s > math.MaxInt32 {
		log.Errorf("sharding index overflow: %d", s)
	}
	return int32(s)
}

type PickOptions interface {
	GetProject() *models.Project
}

type Manager interface {
	GetShardByIndex(i ShardingIndex) Sharding
	GetShardByName(name string) Sharding
	All() []Sharding
	DefaultIndex() ShardingIndex
	Pick(PickOptions) (ShardingIndex, Sharding)
	Reload(Config) error
	NewShardByStateIndexer(indexerInfo state.IndexerInfo) Sharding
}

type ShardingStrategy struct {
	ProjectsMapping      map[string][]ShardingIndex
	OrganizationsMapping map[string][]ShardingIndex
	TiersMapping         map[protoscommon.Tier][]ShardingIndex
	PickStrategy         string
}

type ShardingConfig struct {
	Index              ShardingIndex     `yaml:"index" json:"index"`
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
	PickLbStrategy     string                `yaml:"pick_lb_strategy" json:"pick_lb_strategy"`

	Shards []ShardingConfig `yaml:"shards"`
}

type manager struct {
	config Config

	shards            map[ShardingIndex]Sharding
	strategies        ShardingStrategy
	defaultIndex      ShardingIndex
	singleSharding    *ShardingIndex
	shardIndex        map[string]ShardingIndex
	shardReverseIndex map[ShardingIndex]string
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

func (m *manager) GetShardByIndex(i ShardingIndex) Sharding {
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

func (m *manager) pickByStrategy(key string, indexes []ShardingIndex) ShardingIndex {
	if len(indexes) == 0 {
		return m.defaultIndex
	}
	switch m.strategies.PickStrategy {
	case "random":
		return indexes[rand.Int63n(int64(len(indexes)))]
	case "hash":
		f := fnv.New64a()
		f.Write([]byte(key))
		return indexes[f.Sum64()%uint64(len(indexes))]
	}
	return indexes[0]
}

func (m *manager) Pick(options PickOptions) (ShardingIndex, Sharding) {
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

	m.config = config
	ok, shards, strategies, defaultIndex, shardIndex, shardReverseIndex := loadShardingConfig(config, true, m.singleSharding)
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

func (m *manager) DefaultIndex() ShardingIndex {
	return m.defaultIndex
}

func (m *manager) NewShardByStateIndexer(indexerInfo state.IndexerInfo) Sharding {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	connOptions := loadConnOptions(m.config)
	sharding := initShard(m.config.Credential, ShardingConfig{
		Index: ShardingIndex(indexerInfo.IndexerId),
		Name:  fmt.Sprintf("indexer-%d", indexerInfo.IndexerId),
		Addresses: map[string]string{
			ExternalTcpProxyField: fmt.Sprintf("clickhouse://%s:%d", indexerInfo.IndexerUrl, indexerInfo.ClickhouseProxyPort),
		},
	}, connOptions)
	return sharding
}

func addTierMapping(strategies *ShardingStrategy, index ShardingIndex, tiers []int32) {
	for _, tier := range tiers {
		if _, ok := protoscommon.Tier_name[tier]; ok {
			strategies.TiersMapping[protoscommon.Tier(tier)] = append(strategies.TiersMapping[protoscommon.Tier(tier)], index)
		} else {
			log.Warnf("invalid tier enum: %d, will ignore", tier)
		}
	}
}

func addOrganizationMapping(strategies *ShardingStrategy, index ShardingIndex, organizations []string) {
	for _, org := range organizations {
		strategies.OrganizationsMapping[org] = append(strategies.OrganizationsMapping[org], index)
	}
}

func addProjectMapping(strategies *ShardingStrategy, index ShardingIndex, projects []string) {
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

func loadConnOptions(config Config) []func(*Options) {
	var connOptions []func(*Options)
	var dialConfig = dialConfig{
		readTimeout:  lo.If(config.ReadTimeout > 0, config.ReadTimeout).ElseIfF(*ReadTimeout > 0, func() time.Duration { return time.Duration(*ReadTimeout) * time.Second }).Else(time.Duration(0)),
		dialTimeout:  lo.If(config.DialTimeout > 0, config.DialTimeout).ElseIfF(*DialTimeout > 0, func() time.Duration { return time.Duration(*DialTimeout) * time.Second }).Else(time.Duration(0)),
		maxIdleConns: lo.If(config.MaxIdleConnections > 0, config.MaxIdleConnections).ElseIf(*MaxIdleConns > 0, *MaxIdleConns).Else(0),
		maxOpenConns: lo.If(config.MaxOpenConnections > 0, config.MaxOpenConnections).ElseIf(*MaxOpenConns > 0, *MaxOpenConns).Else(0),
	}
	connOptions = append(connOptions, ConnectWithDialConfig(dialConfig))
	var settings = make(map[string]any)
	for k, v := range NewConnSettingsMacro() {
		settings[k] = v
	}
	for k, v := range config.Settings {
		settings[k] = v
	}
	connOptions = append(connOptions, ConnectWithSettings(settings))
	log.Infof("clickhouse conn options dump, dial config: %+v, settings: %+v", dialConfig, settings)
	return connOptions
}

func loadShardingConfig(config Config, allowPanic bool, singleSharding *ShardingIndex) (
	ok bool, shards map[ShardingIndex]Sharding, strategies ShardingStrategy, defaultIndex ShardingIndex,
	shardIndex map[string]ShardingIndex, shardReverseIndex map[ShardingIndex]string) {
	shards = make(map[ShardingIndex]Sharding)
	strategies = ShardingStrategy{
		ProjectsMapping:      make(map[string][]ShardingIndex),
		OrganizationsMapping: make(map[string][]ShardingIndex),
		TiersMapping:         make(map[protoscommon.Tier][]ShardingIndex),
		PickStrategy:         config.PickLbStrategy,
	}
	shardIndex = make(map[string]ShardingIndex)
	shardReverseIndex = make(map[ShardingIndex]string)
	defaultIndex = 0

	connOptions := loadConnOptions(config)
	for _, shard := range config.Shards {
		if singleSharding != nil && shard.Index != *singleSharding {
			log.Infof("skip shard %d, not match single sharding %d", shard.Index, *singleSharding)
			continue
		}
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

	if singleSharding != nil {
		defaultIndex = *singleSharding
		return
	}

	ok = verify(&strategies, allowPanic)
	defaultIndex = strategies.TiersMapping[protoscommon.Tier_FREE][0]
	return
}

func NewManager(config Config, singleSharding *ShardingIndex) Manager {
	_, shards, strategies, defaultIndex, shardIndex, shardReverseIndex := loadShardingConfig(config, true, singleSharding)
	manager := &manager{
		config:            config,
		shards:            shards,
		strategies:        strategies,
		shardIndex:        shardIndex,
		shardReverseIndex: shardReverseIndex,
		defaultIndex:      defaultIndex,
		singleSharding:    singleSharding,
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
	return NewManager(loadConfig(configPath), nil)
}

func LoadManagerWithSingleSharding(configPath string, shardingIndex ShardingIndex) (Manager, error) {
	if configPath == "" {
		return nil, fmt.Errorf("empty config path")
	}
	config := loadConfig(configPath)
	if len(config.Shards) == 0 {
		return nil, fmt.Errorf("no shards found in config")
	}
	return NewManager(config, &shardingIndex), nil
}
