package ckhmanager

import (
	"hash/fnv"
	"math/rand"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/models"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
)

type PickOptions interface {
	GetProject() *models.Project
}

type Manager interface {
	GetShardByIndex(i int32) Sharding
	GetShardByName(name string) Sharding
	All() []Sharding
	Pick(PickOptions) (int32, Sharding)
}

type ShardingStrategy struct {
	ProjectsMapping      map[string][]int32
	OrganizationsMapping map[string][]int32
	TiersMapping         map[protoscommon.Tier][]int32
	PickStrategy         string
}

type ShardingConfig struct {
	Index              int32             `yaml:"index"`
	Name               string            `yaml:"name"`
	AllowTiers         []int32           `yaml:"allow_tiers"`
	AllowOrganizations []string          `yaml:"allow_organizations"`
	AllowProjects      []string          `yaml:"allow_projects"`
	Addresses          map[string]string `yaml:"addresses"`
}

type Config struct {
	ReadTimeout        time.Duration         `yaml:"read_timeout"`
	DialTimeout        time.Duration         `yaml:"dial_timeout"`
	MaxIdleConnections int                   `yaml:"max_idle_connections"`
	MaxOpenConnections int                   `yaml:"max_open_connections"`
	ConnSettings       map[string]any        `yaml:"conn_settings"`
	Credential         map[string]Credential `yaml:"credential"`

	Shards []ShardingConfig `yaml:"shards"`
}

type manager struct {
	shards       map[int32]Sharding
	strategies   ShardingStrategy
	defaultIndex int32

	shardIndex        map[string]int32
	shardReverseIndex map[int32]string
}

func initShard(credential map[string]Credential, shardingConfig ShardingConfig, connOptions []func(*Options)) Sharding {
	return NewSharding(
		shardingConfig.Index,
		shardingConfig.Name,
		credential,
		shardingConfig.Addresses,
		connOptions...)
}

func (m *manager) addTierMapping(index int32, tiers []int32) {
	for _, tier := range tiers {
		if _, ok := protoscommon.Tier_name[tier]; ok {
			m.strategies.TiersMapping[protoscommon.Tier(tier)] = append(m.strategies.TiersMapping[protoscommon.Tier(tier)], index)
		} else {
			log.Warnf("invalid tier enum: %d, will ignore", tier)
		}
	}
}

func (m *manager) addOrganizationMapping(index int32, organizations []string) {
	for _, org := range organizations {
		m.strategies.OrganizationsMapping[org] = append(m.strategies.OrganizationsMapping[org], index)
	}
}

func (m *manager) addProjectMapping(index int32, projects []string) {
	for _, project := range projects {
		m.strategies.ProjectsMapping[project] = append(m.strategies.ProjectsMapping[project], index)
	}
}

func (m *manager) verify() {
	if len(m.strategies.TiersMapping[protoscommon.Tier_FREE]) == 0 {
		panic("there is no available sharding for free tier, please check your config")
	}
	m.defaultIndex = m.strategies.TiersMapping[protoscommon.Tier_FREE][0]
	for idx := range protoscommon.Tier_name {
		if len(m.strategies.TiersMapping[protoscommon.Tier(idx)]) == 0 {
			log.Infof("no sharding instance for tier: %s, will use free tier instead", protoscommon.Tier_name[idx])
		}
	}
}

func NewManager(config Config) Manager {
	manager := &manager{
		shards: make(map[int32]Sharding),
		strategies: ShardingStrategy{
			ProjectsMapping:      make(map[string][]int32),
			OrganizationsMapping: make(map[string][]int32),
			TiersMapping:         make(map[protoscommon.Tier][]int32),
		},
		shardIndex:        make(map[string]int32),
		shardReverseIndex: make(map[int32]string),
	}

	var connOptions []func(*Options)
	connOptions = append(connOptions, WithDialConfig(dialConfig{
		readTimeout:  config.ReadTimeout,
		dialTimeout:  config.DialTimeout,
		maxIdleConns: config.MaxIdleConnections,
		maxOpenConns: config.MaxOpenConnections,
	}))
	connOptions = append(connOptions, WithSettings(config.ConnSettings))

	for _, shard := range config.Shards {
		_, exists := manager.shardIndex[shard.Name]
		if exists {
			log.Errorf("duplicate shard name: " + shard.Name)
			continue
		}
		_, exists = manager.shardReverseIndex[shard.Index]
		if exists {
			log.Errorf("duplicate shard index: " + shard.Name)
			continue
		}

		sharding := initShard(config.Credential, shard, connOptions)
		manager.shards[shard.Index] = sharding
		manager.shardIndex[shard.Name] = shard.Index
		manager.shardReverseIndex[shard.Index] = shard.Name

		manager.addTierMapping(shard.Index, shard.AllowTiers)
		manager.addOrganizationMapping(shard.Index, shard.AllowOrganizations)
		manager.addProjectMapping(shard.Index, shard.AllowProjects)
	}

	manager.verify()
	return manager
}

func (m *manager) GetShardByIndex(i int32) Sharding {
	return m.shards[i]
}

func (m *manager) GetShardByName(name string) Sharding {
	return m.shards[m.shardIndex[name]]
}

func (m *manager) All() []Sharding {
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
