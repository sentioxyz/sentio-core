package subgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	chain "sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
	"sentioxyz/sentio-core/service/processor/models"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/pkg/errors"
)

type HandlerAgent interface {
	controller.HandlerAgent

	GetExtendRequirements(context.Context, *BlockData) (evm.BlockExtendRequirement, error)
	BuildTaskDataList(context.Context, *BlockData) ([]taskData, error)
}

const (
	HandlerTypeEvent = "event"
	HandlerTypeBlock = "block"
	HandlerTypeCall  = "call"
)

type HandlerController struct {
	processor    *models.Processor
	chainConfig  *chain.ConfigV2
	client       evm.Client
	ipfsShell    *shell.Shell
	manifest     *manifest.Manifest
	memHardLimit uint32
	debugTrace   bool

	instance *instance // TODO support multi instances

	agents []HandlerAgent // built from Manifest

	addressStart     map[string]uint64
	addressStartData string

	waiter *concurrency.ResourceWaiter[uint64]
}

func NewHandlerController(
	ctx context.Context,
	processor *models.Processor,
	chainConfig *chain.ConfigV2,
	client evm.Client,
	ipfsShell *shell.Shell,
	manifest *manifest.Manifest,
	memHardLimit uint32,
	debugTrace bool,
) (ctrl *HandlerController, err error) {
	ctrl = &HandlerController{
		processor:    processor,
		chainConfig:  chainConfig,
		client:       client,
		ipfsShell:    ipfsShell,
		manifest:     manifest,
		memHardLimit: memHardLimit,
		debugTrace:   debugTrace,
	}
	ctrl.instance, err = ctrl.newInstance(ctx)
	return ctrl, err
}

func (c *HandlerController) chainID() string {
	return c.chainConfig.ChainID
}

func (c *HandlerController) Prologue(
	ctx context.Context,
	checkpoint *controller.Checkpoint,
	templates map[uint64][]controller.TemplateInstance,
	first uint64,
	latest controller.BlockHeader,
) *controller.ExternalError {
	// reset instance
	if err := c.instance.Reset(ctx); err != nil {
		return controller.NewExternalError(controller.ErrCodeResetWasmInstanceFailed, err)
	}
	// load contract start
	if extErr := c.LoadAddressStart(checkpoint); extErr != nil {
		return extErr
	}
	// build agents
	if extErr := c.buildAgents(ctx, first, latest.GetBlockNumber(), templates); extErr != nil {
		return extErr
	}
	// build checkpoint data for contract start
	c.AddressStartReady()
	// reset waiter
	c.waiter = concurrency.NewResourceWaiter[uint64]()
	return nil
}

func (c *HandlerController) buildAgents(
	ctx context.Context,
	first, latest uint64,
	templates map[uint64][]controller.TemplateInstance,
) (extErr *controller.ExternalError) {
	_, logger := log.FromContext(ctx)
	// collect data sources
	dataSources := c.manifest.DataSources
	for _, tpls := range utils.GetMapValuesOrderByKey(templates) {
		for _, tpl := range tpls {
			if total := len(c.manifest.Templates); tpl.TemplateID < 0 || int(tpl.TemplateID) >= total {
				logger.Warnf("template id %d out of range [0,%d), will be ignored", tpl.TemplateID, total)
				continue
			}
			dataSources = append(dataSources, c.manifest.Templates[tpl.TemplateID].NewDataSource(
				tpl.Address,
				manifest.BuildBigIntFromUint(tpl.StartBlock),
				tpl.TemplateName,
			))
		}
	}
	// build agents
	c.agents = nil
	for dataSourceID, ds := range dataSources {
		blockRange := controller.BlockRange{StartBlock: max(ds.Source.GetStartBlock(), first)}
		if ds.Source.EndBlock != nil {
			blockRange.EndBlock = utils.WrapPointer(ds.Source.GetEndBlock())
		}
		contractRange := blockRange
		var err error
		contractRange.StartBlock, err = c.GetAddressStart(ctx, ds.Source.Address, blockRange.StartBlock, latest)
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}
		for i, eventHandler := range ds.Mapping.EventHandlers {
			if ds.Source.Address == "" {
				return controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
					errors.Errorf("data source #%d %s has event handler but no contract address", dataSourceID, ds.Name))
			}
			agent := HandlerAgentEvent{
				BaseHandlerAgent: controller.BaseHandlerAgent{
					HandlerID: controller.HandlerID{
						DataSource:   ds.Name,
						DataSourceID: dataSourceID,
						Type:         HandlerTypeEvent,
						Name:         eventHandler.Handler,
						ID:           int32(i),
					},
					Range: contractRange,
				},
				DataSource: ds,
				Filter: evm.LogFilter{
					Topics:  [][]string{{eventHandler.Topic0}},
					Address: []string{strings.ToLower(ds.Source.Address)},
				},
			}
			c.agents = append(c.agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
		for i, callHandler := range ds.Mapping.CallHandlers {
			if ds.Source.Address == "" {
				return controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
					errors.Errorf("data source #%d %s has call handler but no contract address", dataSourceID, ds.Name))
			}
			agent := HandlerAgentCall{
				BaseHandlerAgent: controller.BaseHandlerAgent{
					HandlerID: controller.HandlerID{
						DataSource:   ds.Name,
						DataSourceID: dataSourceID,
						Type:         HandlerTypeCall,
						Name:         callHandler.Handler,
						ID:           int32(i),
					},
					Range: contractRange,
				},
				DataSource: ds,
				Filter: evm.TraceFilter{
					Signature: []string{callHandler.Signature},
					Address:   []string{strings.ToLower(ds.Source.Address)},
				},
			}
			c.agents = append(c.agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
		for i, blockHandler := range ds.Mapping.BlockHandlers {
			agent := HandlerAgentBlock{
				BaseHandlerAgent: controller.BaseHandlerAgent{
					HandlerID: controller.HandlerID{
						DataSource:   ds.Name,
						DataSourceID: dataSourceID,
						Type:         HandlerTypeBlock,
						Name:         blockHandler.Handler,
						ID:           int32(i),
					},
					Range: blockRange,
				},
				DataSource: ds,
			}
			switch kind := blockHandler.Filter.GetKind(); kind {
			case "once":
				agent.Once = true
			case "polling":
				interval := blockHandler.Filter.GetEvery()
				if interval <= 0 {
					return controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
						errors.Errorf("every should greater than 0 in data source %s #%d block handler", ds.Name, i))
				}
				agent.IntervalConfig = data.IntervalConfig{
					BlockInterval: &data.BlockInterval{Backfill: uint64(interval), Watching: uint64(interval)},
				}
			default:
				return controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
					errors.Errorf("filter kind %q is not supported in data source %s #%d block handler", kind, ds.Name, i))
			}
			c.agents = append(c.agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}
	return nil
}

func (c *HandlerController) GetBlockRange() controller.BlockRange {
	return controller.GetHandleAgentsBlockRange(c.agents)
}

func (c *HandlerController) GetAgentStat() map[string]int {
	stat := make(map[string]int)
	for _, ag := range c.agents {
		stat[fmt.Sprintf("%T", ag)] += 1
	}
	return stat
}

func (c *HandlerController) buildReportRequirements(
	currentBlockNumber uint64,
) []data.IntervalRequirement {
	endBlock := c.GetBlockRange().EndBlock
	// In the backfill phase, at least one non-empty BlockData is generated for every DAY.
	// In the watching phase, at least one non-empty BlockData is generated for every MINUTE.
	reqs := []data.IntervalRequirement{{
		BlockRange: controller.BlockRange{StartBlock: currentBlockNumber, EndBlock: endBlock},
		IntervalConfig: data.IntervalConfig{TimeInterval: &data.TimeInterval{
			Backfill: time.Hour * 24,
			Watching: time.Minute,
		}},
	}}
	if endBlock != nil {
		reqs = append(reqs, data.IntervalRequirement{
			BlockRange: controller.BlockRange{StartBlock: *endBlock, EndBlock: endBlock},
			IntervalConfig: data.IntervalConfig{BlockInterval: &data.BlockInterval{
				Backfill: 1,
				Watching: 1,
			}},
		})
	}
	return reqs
}

func (c *HandlerController) BuildBlockDataFetcher(
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
) controller.Fetcher[controller.BlockData] {
	req := c.getDataRequirement()
	req.Interval = append(req.Interval, c.buildReportRequirements(currentBlockNumber)...)
	fetchNamePrefix := fmt.Sprintf("EVM::%s::", c.chainID())
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		evm.BuildBlockMainDataFetcher(fetchNamePrefix, req, firstBlockNumber, currentBlockNumber, latest, c.client),
		latest,
		controller.ProcessConcurrency,
		256*1024*1024,
		100,
		time.Second*10,
		20,
		time.Second,
		func(ctx context.Context, blockNumber uint64, from evm.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			var err error
			result := BlockData{mainData: from, checkpointData: make(map[string]string)}
			// always need header
			if result.BlockHeader, err = c.client.GetHeader(ctx, blockNumber); err != nil {
				return nil, false, err
			}
			// check block hash of main data with the header got above
			for _, l := range from.Logs {
				if l.BlockHash.String() != result.GetBlockHash() {
					return nil, false, fetcher.Permanent(errors.Errorf("invalid block hash of the log %s, expected is %s",
						l.BlockHash.String(), controller.GetBlockSummary(result.BlockHeader)))
				}
			}
			for _, t := range from.Traces {
				if t.BlockHash != result.GetBlockHash() {
					return nil, false, fetcher.Permanent(errors.Errorf("invalid block hash of the trace %s, expected is %s",
						t.BlockHash, controller.GetBlockSummary(result.BlockHeader)))
				}
			}
			// take the main data and ask the handler controller what extend data is needed
			var r evm.BlockExtendRequirement
			if r, err = c.getBlockExtendRequirements(ctx, &result); err != nil {
				return nil, false, err
			}
			// actually get the extended data
			if result.extendData, err = c.client.GetBlock(ctx, blockNumber, r); err != nil {
				return nil, false, err
			}
			// build binding data
			if result.taskList, result.taskTotalSize, err = c.BuildTaskList(ctx, &result); err != nil {
				_, logger := log.FromContext(ctx,
					"header", utils.MustJSONMarshal(result.BlockHeader),
					"mainData", utils.MustJSONMarshal(result.mainData),
					"extendData", utils.MustJSONMarshal(result.extendData))
				logger.Warnfe(err, "build task list failed")
				return nil, false, err
			}
			c.DumpAddressStart(result.checkpointData)
			return &result, true, nil
		},
	)
}

func (c *HandlerController) getDataRequirement() (dr evm.DataRequirement) {
	for _, agent := range c.agents {
		switch ag := agent.(type) {
		case HandlerAgentBlock:
			if !ag.Once {
				dr.Interval = append(dr.Interval, data.IntervalRequirement{
					IntervalConfig: ag.IntervalConfig,
					BlockRange:     ag.Range,
				})
			} else {
				dr.Exact = append(dr.Exact, ag.Range.StartBlock)
			}
		case HandlerAgentCall:
			dr.Trace = append(dr.Trace, evm.TraceRequirement{
				TraceFilter: ag.Filter,
				BlockRange:  ag.Range,
			})
		case HandlerAgentEvent:
			dr.Log = append(dr.Log, evm.LogRequirement{
				LogFilter:  ag.Filter,
				BlockRange: ag.Range,
			})
		}
	}
	return dr
}

func (c *HandlerController) getBlockExtendRequirements(
	ctx context.Context,
	blockData *BlockData,
) (req evm.BlockExtendRequirement, err error) {
	var ar evm.BlockExtendRequirement
	for _, agent := range c.agents {
		if ar, err = agent.GetExtendRequirements(ctx, blockData); err != nil {
			return
		}
		req.Merge(ar)
	}
	return
}

func (c *HandlerController) BuildTaskList(
	ctx context.Context,
	bd *BlockData,
) ([]controller.Task, int, error) {
	var taskDatas []taskData
	var taskTotalSize int
	for _, agent := range c.agents { // 15308711065
		if agent.GetRange().Contains(bd.GetBlockNumber()) {
			tds, err := agent.BuildTaskDataList(ctx, bd)
			if err != nil {
				return nil, 0, err
			}
			taskDatas = append(taskDatas, tds...)
			for _, td := range tds {
				taskTotalSize += td.size
			}
		}
	}
	// The purpose of using stable sorting is to ensure that tasks of the same handler remain in the order
	// in which they were generated.
	sort.SliceStable(taskDatas, func(i, j int) bool {
		return taskDatas[i].Cmp(taskDatas[j]) < 0
	})
	var r []controller.Task
	for _, td := range taskDatas {
		r = append(r, &task{
			handlerCtrl: c,
			BlockHeader: bd.BlockHeader,
			taskData:    td,
		})
	}
	return r, taskTotalSize, nil
}

const checkpointDataKeyAddressStart = "AddressStart"

func (c *HandlerController) LoadAddressStart(checkpoint *controller.Checkpoint) *controller.ExternalError {
	c.addressStart = make(map[string]uint64)
	if checkpoint == nil || checkpoint.Data == nil {
		return nil
	}
	raw, has := checkpoint.Data[checkpointDataKeyAddressStart]
	if !has {
		return nil
	}
	err := json.Unmarshal([]byte(raw), &c.addressStart)
	if err != nil {
		return controller.NewExternalError(controller.ErrCodeInvalidCheckpointData,
			errors.Wrapf(err, "load address start failed"))
	}
	return nil
}

func (c *HandlerController) GetAddressStart(
	ctx context.Context,
	address string,
	start uint64,
	latest uint64,
) (uint64, error) {
	if c.chainConfig.SkipStartBlockValidation || controller.SkipStartBlockValidation || address == "" {
		return start, nil
	}
	var has bool
	if start, has = c.addressStart[address]; has {
		return start, nil
	}
	var err error
	start, has, err = c.client.GetContractStartBlock(ctx, address, start, latest)
	if err != nil {
		return 0, errors.Wrapf(err, "get start block for contract %s failed", address)
	}
	if !has {
		start = latest + 1
	}
	c.addressStart[address] = start
	return start, nil
}

func (c *HandlerController) AddressStartReady() {
	b, _ := json.Marshal(c.addressStart)
	c.addressStartData = string(b)
}

func (c *HandlerController) DumpAddressStart(checkpointData map[string]string) {
	checkpointData[checkpointDataKeyAddressStart] = c.addressStartData
}

func (c *HandlerController) Epilogue() {
}

func (c *HandlerController) Snapshot() any {
	return map[string]any{
		"chainConfig":  c.chainConfig,
		"manifest":     c.manifest,
		"memHardLimit": c.memHardLimit,
		"debugTrace":   c.debugTrace,
		"agents": utils.MapSliceNoError(c.agents, func(a HandlerAgent) any {
			return a.Snapshot()
		}),
		"addressStart": c.addressStart,
		"agentStat":    c.GetAgentStat(),
		"wasmInstance": c.instance.Snapshot(),
	}
}
