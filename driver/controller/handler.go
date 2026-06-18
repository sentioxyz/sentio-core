package controller

import (
	"context"
	"fmt"
)

type HandlerID struct {
	DataSource   string
	DataSourceID int
	Type         string
	Name         string
	ID           int32
}

func (h HandlerID) String() string {
	return fmt.Sprintf("%d#%s/%s/%s/%d", h.DataSourceID, h.DataSource, h.Type, h.Name, h.ID)
}

type HandlerConfig interface {
	GetHandlerName() string
	GetHandlerId() int32
}

type SimpleHandlerConfig struct {
	Name string
	ID   int32
}

func (h SimpleHandlerConfig) GetHandlerName() string {
	return h.Name
}

func (h SimpleHandlerConfig) GetHandlerId() int32 {
	return h.ID
}

func BuildHandlerID(dataSource string, dataSourceID int, handlerType string, handlerConfig HandlerConfig) HandlerID {
	return HandlerID{
		DataSource:   dataSource,
		DataSourceID: dataSourceID,
		Type:         handlerType,
		Name:         handlerConfig.GetHandlerName(),
		ID:           handlerConfig.GetHandlerId(),
	}
}

type HandlerAgent interface {
	GetHandlerID() HandlerID
	GetRange() BlockRange
	Snapshot() any
}

func GetHandleAgentsBlockRange[HA HandlerAgent](agents []HA) BlockRange {
	r := EmptyBlockRange
	for _, ag := range agents {
		r = r.Cover(ag.GetRange())
	}
	return r
}

type BaseHandlerAgent struct {
	HandlerID HandlerID
	Range     BlockRange
}

func (h BaseHandlerAgent) GetHandlerID() HandlerID {
	return h.HandlerID
}

func (h BaseHandlerAgent) GetRange() BlockRange {
	return h.Range
}

func NewBaseHandlerAgent(
	dataSource string,
	dataSourceID int,
	handlerType string,
	handlerConfig HandlerConfig,
	blockRange BlockRange,
) BaseHandlerAgent {
	return BaseHandlerAgent{
		HandlerID: BuildHandlerID(dataSource, dataSourceID, handlerType, handlerConfig),
		Range:     blockRange,
	}
}

type HandlerController interface {
	Prologue(
		ctx context.Context,
		checkpoint *Checkpoint,
		templates map[uint64][]TemplateInstance,
		first uint64,
		latest BlockHeader,
	) *ExternalError
	GetBlockRange() BlockRange
	GetAgentStat() map[string]int
	BuildBlockDataFetcher(firstBlockNumber uint64, currentBlockNumber uint64, latest BlockHeader) Fetcher[BlockData]
	Epilogue()

	Snapshot() any
}
