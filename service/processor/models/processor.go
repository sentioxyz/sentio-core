package models

import (
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/common/chains"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/common/errors"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/processor/protos"
)

// TODO, should this be outside of models?
type NetworkOverride struct {
	Chain string `json:"chain"`
	Host  string `json:"host"`
}

func BuildNetworkOverrides(data []*protos.NetworkOverride) []NetworkOverride {
	networkOverrides := make([]NetworkOverride, len(data))
	for i, no := range data {
		if len(no.GetHost()) > 0 {
			networkOverrides[i] = NetworkOverride{Chain: no.GetChain(), Host: no.GetHost()}
		}
	}
	return networkOverrides
}

func ParseNetworkOverrides(data []NetworkOverride) []*protos.NetworkOverride {
	networkOverrides := make([]*protos.NetworkOverride, len(data))
	for i, no := range data {
		networkOverrides[i] = &protos.NetworkOverride{Chain: no.Chain, Host: no.Host}
	}
	return networkOverrides
}

func MockProcessorID() string {
	return fmt.Sprintf("placeholder-%s", gonanoid.Must(10))
}

func IsMockProcessorID(processorID string) bool {
	return strings.HasPrefix(processorID, "placeholder-")
}

type SentioProcessorProperties struct {
	CliVersion             string
	SdkVersion             string
	CodeURL                string
	CodeHash               string
	CommitSha              string
	GitURL                 string
	ZipURL                 string
	Debug                  bool
	TimescaleShardingIndex int32
	EntitySchema           string
	ContractID             *string
	Contract               *commonmodels.ProjectContract
	NetworkOverrides       datatypes.JSONSlice[NetworkOverride]
	Warnings               []string `gorm:"type:text[]"`
	Binary                 bool
}

type SubgraphProcessorProperties struct {
	VersionLabel       string
	IpfsHash           string
	DebugFork          string
	GraphQLQueryEngine string `gorm:"default:'v1'"`
}

type ProcessorUpgradeHistory struct {
	ID          string `gorm:"primaryKey"`
	ProcessorID string `gorm:"index"`
	UserID      *string
	UploadedAt  time.Time
	ObsoleteAt  time.Time

	SentioProcessorProperties
	SubgraphProcessorProperties

	ChainStatesJSON datatypes.JSON
	ChainStates     []*ChainState `gorm:"-"`
}

type ReferenceProcessorProperties struct {
	// reference to latest version of project
	ReferenceProjectID string
	// reference to specific processor
	// ReferenceProcessorID *string
}

type SentioNetworkProperties struct {
	ChainID        chains.ChainID `gorm:"column:'sentio_network_chain_id'"` // default to empty string for non-sentio network processors
	RequiredChains []string       `gorm:"column:'sentio_network_required_chains';type:text[]"`
}

func (p *ProcessorUpgradeHistory) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID, err = gonanoid.GenerateID()
	}
	return err
}

func (p *ProcessorUpgradeHistory) AfterFind(tx *gorm.DB) error {
	return json.Unmarshal(p.ChainStatesJSON, &p.ChainStates)
}

func (p *ProcessorUpgradeHistory) ToPB(index int, processor *Processor) (*protos.ProcessorUpgradeHistory, error) {
	snapshot := Processor{
		ID:                          processor.ID,
		ProjectID:                   processor.ProjectID,
		UserID:                      p.UserID,
		Version:                     processor.Version,
		UploadedAt:                  p.UploadedAt,
		VersionState:                processor.VersionState,
		ChainStates:                 p.ChainStates,
		ClickhouseShardingIndex:     processor.ClickhouseShardingIndex,
		SentioProcessorProperties:   p.SentioProcessorProperties,
		SubgraphProcessorProperties: p.SubgraphProcessorProperties,
	}
	sp, err := snapshot.ToPB(nil)
	if err != nil {
		return nil, err
	}
	return &protos.ProcessorUpgradeHistory{
		Index:      int32(index),
		Id:         p.ID,
		Snapshot:   sp,
		UploadedAt: timestamppb.New(p.UploadedAt),
		ObsoleteAt: timestamppb.New(p.ObsoleteAt),
	}, nil
}

type EventlogVersion int32

const (
	EventlogVersion_V1 EventlogVersion = iota + 1
	EventlogVersion_V2
	EventlogVersion_V3
)

func (e EventlogVersion) Int32() int32 {
	return int32(e)
}

func ToEventlogVersion(i int32) EventlogVersion {
	switch i {
	case 1:
		return EventlogVersion_V1
	case 2:
		return EventlogVersion_V2
	case 3:
		return EventlogVersion_V3
	default:
		return EventlogVersion_V2
	}
}

func (e EventlogVersion) String() string {
	return []string{"V1", "V2", "V3"}[e-1]
}

type Processor struct {
	gorm.Model
	ID         string `gorm:"primaryKey"`
	ProjectID  string `gorm:"index"`
	Project    *commonmodels.Project
	UserID     *string
	User       *commonmodels.User
	Version    int32
	UploadedAt time.Time

	// state of the processor
	VersionState int32
	ChainStates  []*ChainState `gorm:"constraint:OnDelete:CASCADE;"`

	ClickhouseShardingIndex int32 `gorm:"default:0"`
	K8sClusterID            int32 `gorm:"default:0"`

	EventlogVersion     int32 `gorm:"default:1"`
	EntitySchemaVersion int32

	DriverVersion int32 `gorm:"default:0"`
	NumWorkers    int32 `gorm:"default:1"`

	Pause       bool
	PauseAt     time.Time
	PauseReason string

	// properties for sentio processor
	SentioProcessorProperties

	// properties for subgraph processor
	SubgraphProcessorProperties

	// properties for reference processor
	ReferenceProcessorProperties

	SentioNetworkProperties
}

func (p Processor) GetProject() *commonmodels.Project {
	return p.Project
}

func (p Processor) IsEventlogV3() bool {
	// all streaming mode and eventlog v3 enabled if driver version is gte than 1
	return p.EventlogVersion == int32(EventlogVersion_V3) || p.DriverVersion >= 1
}

func (p Processor) IsEventlogV2() bool {
	return p.EventlogVersion == int32(EventlogVersion_V2)
}

type Processors []*Processor

func getProcessorsField[T any](processors Processors, getter func(processor *Processor) T) []T {
	result := make([]T, 0, len(processors))
	for _, processor := range processors {
		result = append(result, getter(processor))
	}
	return result
}

func (processors Processors) GetProcessorIDs() []string {
	return getProcessorsField(processors, func(processor *Processor) string {
		return processor.ID
	})
}

func (processors Processors) GetTimescaleShardingIndices() []int32 {
	return getProcessorsField(processors, func(processor *Processor) int32 {
		return processor.TimescaleShardingIndex
	})
}

func (processors Processors) GetClickhouseShardingIndices() []int32 {
	return getProcessorsField(processors, func(processor *Processor) int32 {
		return processor.ClickhouseShardingIndex
	})
}

func (processors Processors) CheckTimescaleShardingIndicesConsistent() bool {
	if len(processors) == 0 {
		return true
	}
	var nowIdx int32 = -1
	indices := processors.GetTimescaleShardingIndices()
	for _, index := range indices {
		if nowIdx < 0 {
			nowIdx = index
		} else if nowIdx != index {
			return false
		}
	}
	return true
}

func (p *Processor) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID, err = gonanoid.GenerateID()
	}

	return err
}

func (p *Processor) ToPB(referencedProcessor *Processor) (*protos.Processor, error) {
	var pb *protos.Processor
	var err error
	if referencedProcessor != nil && referencedProcessor.ID != p.ID && p.VersionState == int32(protos.ProcessorVersionState_ACTIVE) {
		pb, err = referencedProcessor.ToPB(nil)
		if err != nil {
			return nil, err
		}

		// keep the original processor id and project id
		pb.ProcessorId = p.ID
		pb.ProjectId = p.ProjectID
		pb.Version = p.Version
		pb.ReferenceProjectId = referencedProcessor.ProjectID
		return pb, nil
	}

	var contractID = ""
	if p.ContractID != nil {
		contractID = *p.ContractID
	}
	networkOverrides := ParseNetworkOverrides(p.NetworkOverrides)

	var states []*protos.ChainState
	if states, err = utils.MapSlice(p.ChainStates, func(cs *ChainState) (*protos.ChainState, error) {
		return cs.ToPB()
	}); err != nil {
		return nil, err
	}
	ret := &protos.Processor{
		ProcessorId: p.ID,
		ProjectId:   p.ProjectID,
		Version:     p.Version,
		// sentio properties
		SdkVersion:             p.SdkVersion,
		CodeUrl:                p.CodeURL,
		Debug:                  p.Debug,
		TimescaleShardingIndex: p.TimescaleShardingIndex,
		ContractId:             contractID,
		NetworkOverrides:       networkOverrides,
		// subgraph properties
		VersionLabel: p.VersionLabel,
		IpfsHash:     p.IpfsHash,
		DebugFork:    p.DebugFork,
		// states
		VersionState:            protos.ProcessorVersionState(p.VersionState),
		ChainStates:             states,
		CreatedAt:               p.CreatedAt.Unix(),
		ClickhouseShardingIndex: p.ClickhouseShardingIndex,
		K8SClusterId:            p.K8sClusterID,
		EntitySchemaVersion:     p.EntitySchemaVersion,
		EventlogVersion:         p.EventlogVersion,
		DriverVersion:           p.DriverVersion,
		NumWorkers:              p.NumWorkers,
		Pause:                   p.Pause,
		PauseAt:                 timestamppb.New(p.PauseAt),
		PauseReason:             p.PauseReason,
		IsBinary:                p.Binary,
		ChainId:                 string(p.ChainID),
		RequiredChains:          p.RequiredChains,
	}

	return ret, nil
}

func (p *Processor) FromPB(processor *protos.Processor) error {
	var err error
	p.ID = processor.ProcessorId
	p.ProjectID = processor.ProjectId
	p.Version = processor.Version
	p.EntitySchemaVersion = processor.EntitySchemaVersion

	// subgraph properties
	p.VersionLabel = processor.VersionLabel
	p.IpfsHash = processor.IpfsHash
	p.DebugFork = processor.DebugFork

	// sentio properties
	p.SdkVersion = processor.SdkVersion
	p.CodeURL = processor.CodeUrl
	p.Debug = processor.Debug
	p.TimescaleShardingIndex = processor.TimescaleShardingIndex
	p.CreatedAt = time.Unix(processor.CreatedAt, 0)
	p.ClickhouseShardingIndex = processor.ClickhouseShardingIndex
	p.K8sClusterID = processor.K8SClusterId
	p.NetworkOverrides = BuildNetworkOverrides(processor.NetworkOverrides)
	p.EventlogVersion = processor.EventlogVersion
	p.DriverVersion = processor.DriverVersion
	p.NumWorkers = processor.NumWorkers
	p.Binary = processor.IsBinary

	// state
	if p.ChainStates, err = utils.MapSlice(
		processor.ChainStates, func(cs *protos.ChainState) (*ChainState, error) {
			chainState := &ChainState{}
			if err = chainState.FromPB(cs, processor.ProcessorId); err != nil {
				return nil, err
			}
			return chainState, nil
		}); err != nil {
		return err
	}
	if processor.ContractId != "" {
		// p.ContractID = new(int64)
		*p.ContractID = processor.ContractId
	}
	p.VersionState = int32(processor.VersionState.Number())
	p.RequiredChains = processor.RequiredChains
	return nil
}

func (p *Processor) IsRunningVersion() bool {
	if p.VersionState == int32(protos.ProcessorVersionState_ACTIVE) {
		return true
	}
	if p.VersionState == int32(protos.ProcessorVersionState_PENDING) {
		return true
	}
	return false
}

type HandlerStat map[string]int // key is handler type

type ChainState struct {
	gorm.Model
	ID                         string `gorm:"primaryKey"`
	ChainID                    string
	ProcessorID                string `gorm:"index"`
	Processor                  *Processor
	ProcessedBlockNumber       int64
	ProcessedTimestampMicros   int64
	ProcessedBlockHash         string
	InitialStartBlockNumber    int64
	EstimatedLatestBlockNumber int64

	// Trackers should be replaced by MeterState in the future.
	Trackers datatypes.JSON

	// The state for the meter.
	MeterState datatypes.JSON

	// The state for the indexer.
	// is a driver/common/CheckpointState object
	IndexerState datatypes.JSON

	// is a HandlerStat object
	HandlerStat datatypes.JSON

	ProcessedVersion int32
	// Use processor_service.proto ENUM value.
	State int32
	// Use set ErrorMessage if state is ERROR.
	ErrorRecord errors.ErrorRecord `gorm:"embedded;embeddedPrefix:error_"`

	// Used for checkpoint dynamic template creation.
	Templates string
}

func (cs *ChainState) ToPB() (*protos.ChainState, error) {
	var trackersBytes, indexerStateBytes, meterStateBytes, handlerStatBytes []byte
	var err error
	if trackersBytes, err = cs.Trackers.MarshalJSON(); err != nil {
		return nil, err
	}
	if meterStateBytes, err = cs.MeterState.MarshalJSON(); err != nil {
		return nil, err
	}
	if indexerStateBytes, err = cs.IndexerState.MarshalJSON(); err != nil {
		return nil, err
	}
	if handlerStatBytes, err = cs.HandlerStat.MarshalJSON(); err != nil {
		return nil, err
	}
	return &protos.ChainState{
		ChainId:                  cs.ChainID,
		ProcessedBlockNumber:     cs.ProcessedBlockNumber,
		ProcessedTimestampMicros: cs.ProcessedTimestampMicros,
		ProcessedBlockHash:       cs.ProcessedBlockHash,
		Trackers:                 string(trackersBytes),
		ProcessedVersion:         cs.ProcessedVersion,
		IndexerState:             string(indexerStateBytes),
		MeterState:               string(meterStateBytes),
		HandlerStat:              string(handlerStatBytes),
		Status: &protos.ChainState_Status{
			State:       protos.ChainState_Status_State(cs.State),
			ErrorRecord: cs.ErrorRecord.ToPB(),
		},
		UpdatedAt:                  timestamppb.New(cs.UpdatedAt),
		Templates:                  cs.Templates,
		InitialStartBlockNumber:    cs.InitialStartBlockNumber,
		EstimatedLatestBlockNumber: cs.EstimatedLatestBlockNumber,
	}, nil
}

func (cs *ChainState) FromPB(chainState *protos.ChainState, processorID string) error {
	if err := cs.Trackers.UnmarshalJSON([]byte(chainState.Trackers)); err != nil {
		return err
	}
	if err := cs.IndexerState.UnmarshalJSON([]byte(chainState.IndexerState)); err != nil {
		return err
	}
	if err := cs.MeterState.UnmarshalJSON([]byte(chainState.MeterState)); err != nil {
		return err
	}
	if err := cs.HandlerStat.UnmarshalJSON([]byte(chainState.HandlerStat)); err != nil {
		return err
	}
	cs.ID = fmt.Sprintf("%s_%s", processorID, chainState.ChainId)
	cs.ProcessorID = processorID
	cs.ChainID = chainState.ChainId
	cs.ProcessedBlockNumber = chainState.ProcessedBlockNumber
	cs.ProcessedTimestampMicros = chainState.ProcessedTimestampMicros
	cs.ProcessedBlockHash = chainState.ProcessedBlockHash
	cs.ProcessedVersion = chainState.ProcessedVersion
	cs.State = int32(chainState.Status.State)
	cs.ErrorRecord.FromPB(chainState.Status.ErrorRecord)
	cs.UpdatedAt = chainState.UpdatedAt.AsTime()
	cs.Templates = chainState.Templates
	cs.InitialStartBlockNumber = chainState.InitialStartBlockNumber
	cs.EstimatedLatestBlockNumber = chainState.EstimatedLatestBlockNumber
	return nil
}
