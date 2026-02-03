package state

import (
	"context"
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresStore struct {
	db       *gorm.DB
	stateKey string
}

type StateRow struct {
	gorm.Model
	StateKey  string `gorm:"uniqueIndex:state_key_unique;column:state_key"`
	LastBlock uint64 `gorm:"not null;column:last_block"`
}

func (StateRow) TableName() string { return "sentio_node_state" }

type IndexerInfoRow struct {
	gorm.Model
	StateKey            string `gorm:"uniqueIndex:indexer_info_state_key_indexer_id_unique;column:state_key"`
	IndexerId           uint64 `gorm:"uniqueIndex:indexer_info_state_key_indexer_id_unique;column:indexer_id"`
	IndexerUrl          string `gorm:"not null;column:indexer_url"`
	ComputeNodeRpcPort  uint16 `gorm:"not null;column:compute_node_rpc_port"`
	StorageNodeRpcPort  uint16 `gorm:"not null;column:storage_node_rpc_port"`
	ClickhouseProxyPort uint16 `gorm:"not null;column:clickhouse_proxy_port"`
}

func (IndexerInfoRow) TableName() string { return "sentio_node_indexer_infos" }

type ProcessorAllocationRow struct {
	gorm.Model
	StateKey    string `gorm:"uniqueIndex:processor_allocation_state_key_processor_id_indexer_id_unique;column:state_key"`
	ProcessorId string `gorm:"uniqueIndex:processor_allocation_state_key_processor_id_indexer_id_unique;column:processor_id"`
	IndexerId   uint64 `gorm:"uniqueIndex:processor_allocation_state_key_processor_id_indexer_id_unique;column:indexer_id"`
}

func (ProcessorAllocationRow) TableName() string { return "sentio_node_processor_allocations" }

type ProcessorInfoRow struct {
	gorm.Model
	StateKey     string `gorm:"uniqueIndex:processor_info_state_key_processor_id_unique;column:state_key"`
	ProcessorId  string `gorm:"uniqueIndex:processor_info_state_key_processor_id_unique;column:processor_id"`
	EntitySchema string `gorm:"not null;column:entity_schema"`
}

func (ProcessorInfoRow) TableName() string { return "sentio_node_processor_infos" }

type HostedProcessorRow struct {
	gorm.Model
	StateKey    string `gorm:"uniqueIndex:hosted_processor_state_key_processor_id_unique;column:state_key"`
	ProcessorId string `gorm:"uniqueIndex:hosted_processor_state_key_processor_id_unique;column:processor_id"`
}

func (HostedProcessorRow) TableName() string { return "sentio_node_hosted_processors" }

func NewPostgresStore(dsn string, stateKey string) (*PostgresStore, error) {
	if dsn == "" {
		return nil, errors.New("postgres dsn is required")
	}
	if stateKey == "" {
		return nil, errors.New("stateKey is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&StateRow{}, &IndexerInfoRow{}, &ProcessorAllocationRow{}, &ProcessorInfoRow{}, &HostedProcessorRow{}); err != nil {
		return nil, err
	}
	return &PostgresStore{db: db, stateKey: stateKey}, nil
}

func (s *PostgresStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *PostgresStore) Load(ctx context.Context) (*PlainState, error) {
	st := &PlainState{
		ProcessorAllocations: map[string]map[uint64]ProcessorAllocation{},
		ProcessorInfos:       map[string]ProcessorInfo{},
		IndexerInfos:         map[uint64]IndexerInfo{},
		HostedProcessors:     map[string]bool{},
	}

	var stateRow StateRow
	err := s.db.WithContext(ctx).
		Where("state_key = ?", s.stateKey).
		First(&stateRow).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	st.LastBlock = stateRow.LastBlock

	var indexerInfos []IndexerInfoRow
	if err := s.db.WithContext(ctx).
		Where("state_key = ?", s.stateKey).
		Find(&indexerInfos).Error; err != nil {
		return nil, err
	}
	for _, r := range indexerInfos {
		st.IndexerInfos[r.IndexerId] = IndexerInfo{
			IndexerId:           r.IndexerId,
			IndexerUrl:          r.IndexerUrl,
			ComputeNodeRpcPort:  r.ComputeNodeRpcPort,
			StorageNodeRpcPort:  r.StorageNodeRpcPort,
			ClickhouseProxyPort: r.ClickhouseProxyPort,
		}
	}

	var processorInfos []ProcessorInfoRow
	if err := s.db.WithContext(ctx).
		Where("state_key = ?", s.stateKey).
		Find(&processorInfos).Error; err != nil {
		return nil, err
	}
	for _, r := range processorInfos {
		st.ProcessorInfos[r.ProcessorId] = ProcessorInfo{
			ProcessorId:  r.ProcessorId,
			EntitySchema: r.EntitySchema,
		}
	}

	var allocs []ProcessorAllocationRow
	if err := s.db.WithContext(ctx).
		Where("state_key = ?", s.stateKey).
		Find(&allocs).Error; err != nil {
		return nil, err
	}
	for _, r := range allocs {
		m := st.ProcessorAllocations[r.ProcessorId]
		if m == nil {
			m = make(map[uint64]ProcessorAllocation)
			st.ProcessorAllocations[r.ProcessorId] = m
		}
		m[r.IndexerId] = ProcessorAllocation{
			ProcessorId: r.ProcessorId,
			IndexerId:   r.IndexerId,
		}
	}

	var hostedProcessors []HostedProcessorRow
	if err := s.db.WithContext(ctx).
		Where("state_key = ?", s.stateKey).
		Find(&hostedProcessors).Error; err != nil {
		return nil, err
	}
	for _, r := range hostedProcessors {
		st.HostedProcessors[r.ProcessorId] = true
	}
	return st, nil
}

func (s *PostgresStore) Save(ctx context.Context, state State) error {
	return s.db.WithContext(ctx).Unscoped().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("state_key = ?", s.stateKey).Delete(&StateRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("state_key = ?", s.stateKey).Delete(&IndexerInfoRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("state_key = ?", s.stateKey).Delete(&ProcessorAllocationRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("state_key = ?", s.stateKey).Delete(&ProcessorInfoRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("state_key = ?", s.stateKey).Delete(&HostedProcessorRow{}).Error; err != nil {
			return err
		}

		stateRow := &StateRow{
			StateKey:  s.stateKey,
			LastBlock: state.GetLastBlock(),
		}
		if err := tx.Create(stateRow).Error; err != nil {
			return err
		}

		{
			var rows []IndexerInfoRow
			for _, info := range state.GetIndexerInfos() {
				rows = append(rows, IndexerInfoRow{
					StateKey:            s.stateKey,
					IndexerId:           info.IndexerId,
					IndexerUrl:          info.IndexerUrl,
					ComputeNodeRpcPort:  info.ComputeNodeRpcPort,
					StorageNodeRpcPort:  info.StorageNodeRpcPort,
					ClickhouseProxyPort: info.ClickhouseProxyPort,
				})
			}
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}

		{
			var rows []ProcessorAllocationRow
			for processorId, m := range state.GetProcessorAllocations() {
				for _, alloc := range m {
					rows = append(rows, ProcessorAllocationRow{
						StateKey:    s.stateKey,
						ProcessorId: processorId,
						IndexerId:   alloc.IndexerId,
					})
				}
			}
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}

		{
			var rows []ProcessorInfoRow
			for _, info := range state.GetProcessorInfos() {
				rows = append(rows, ProcessorInfoRow{
					StateKey:     s.stateKey,
					ProcessorId:  info.ProcessorId,
					EntitySchema: info.EntitySchema,
				})
			}
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}

		{
			var rows []HostedProcessorRow
			for processorId := range state.GetHostedProcessors() {
				rows = append(rows, HostedProcessorRow{
					StateKey:    s.stateKey,
					ProcessorId: processorId,
				})
			}
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}
