package evm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
)

type EvmHandlerAgent interface {
	standard.HandlerAgent[*BlockData]

	GetExtendRequirements(context.Context, *BlockData) (evm.BlockExtendRequirement, error)
}

type HandlerController struct {
	*standard.BaseHandlerController[evm.Client, *BlockData, EvmHandlerAgent]
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *config.ChainConfig,
	client evm.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[evm.Client, *BlockData, EvmHandlerAgent](
			processor, initResult, chainConfig, client, processorClients),
	}
}

func (c *HandlerController) Prologue(
	ctx context.Context,
	checkpoint *controller.Checkpoint,
	templates map[uint64][]controller.TemplateInstance,
	first uint64,
	latest controller.BlockHeader,
) *controller.ExternalError {
	if extErr := c.SetTemplates(ctx, templates); extErr != nil {
		return extErr
	}
	if extErr := c.LoadAddressStart(checkpoint); extErr != nil {
		return extErr
	}
	if extErr := c.buildAgents(ctx, first, latest.GetBlockNumber()); extErr != nil {
		return extErr
	}
	c.AddressStartReady()
	c.DisableAgents(ctx)
	if extErr := c.PrepareExecute(ctx); extErr != nil {
		return extErr
	}
	return nil
}

func (c *HandlerController) Epilogue() {
	c.BaseHandlerController.FinishExecute()
}

func (c *HandlerController) BuildBlockDataFetcher(
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
) controller.Fetcher[controller.BlockData] {
	req := c.getDataRequirement()
	req.Interval = append(req.Interval, c.BuildReportRequirements(currentBlockNumber)...)

	fetchNamePrefix := fmt.Sprintf("EVM::%s::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		evm.BuildBlockMainDataFetcher(fetchNamePrefix, req, firstBlockNumber, currentBlockNumber, latest, c.Client),
		latest,
		controller.ProcessConcurrency,
		256*1024*1024, // 256MB
		100,
		time.Second*10,
		20,
		time.Second,
		func(ctx context.Context, blockNumber uint64, from evm.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			_, logger := log.FromContext(ctx)
			logger.Debugf("will build block data in block #%d with %d logs %d traces %d intervals in main data",
				blockNumber, len(from.Logs), len(from.Traces), len(from.Intervals))
			var err error
			result := BlockData{mainData: from, checkpointData: make(map[string]string)}
			// always need header
			if result.BlockHeader, err = c.Client.GetHeader(ctx, blockNumber); err != nil {
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
			if result.extendData, err = c.Client.GetBlock(ctx, blockNumber, r); err != nil {
				return nil, false, err
			}
			// build binding data
			if result.taskList, result.taskTotalSize, err = c.BuildTaskList(ctx, &result); err != nil {
				return nil, false, err
			}
			logger.Debugf("built %d task in block #%d with handlerIDs %v",
				len(result.taskList),
				blockNumber,
				utils.Stat(utils.MapSliceNoError(
					utils.MapSliceNoError(result.taskList, controller.Task.GetHandlerID),
					controller.HandlerID.String,
				)))
			c.DumpAddressStart(result.checkpointData)
			return &result, true, nil
		},
	)
}

func (c *HandlerController) getAddressStart(ctx context.Context, address string, start, latest uint64) (uint64, error) {
	return c.GetAddressStart(
		address,
		start,
		func() (uint64, error) {
			newStart, has, getErr := c.Client.GetContractStartBlock(ctx, address, start, latest)
			if getErr != nil {
				return 0, getErr
			}
			if has {
				return newStart, nil
			}
			return latest + 1, nil
		})
}

func (c *HandlerController) buildAgents(ctx context.Context, first, latest uint64) *controller.ExternalError {
	_, logger := log.FromContext(ctx)
	c.Agents = nil
	var err error

	for dataSourceID, accountConfig := range c.Config.AccountConfigs {
		accountAddress := standard.AdjustAddress(accountConfig.GetAddress())
		dataSource := standard.BuildDataSource("EVM", c.ChainConfig.ChainID, "Account", accountAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(accountConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(accountConfig.GetEndBlock()),
		}
		for _, logConfig := range accountConfig.GetLogConfigs() {
			agent := HandlerAgentLog{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "log", logConfig, blockRange),
				Client:           c.Client,
				FetchConfig:      logConfig.GetFetchConfig(),
			}
			agent.Filters, err = NewLogFilters(logConfig.GetFilters(), true, "")
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	var hasTxnHandler bool
	for dataSourceID, contractConfig := range c.Config.ContractConfigs {
		contractAddress := standard.AdjustAddress(contractConfig.GetContract().GetAddress())
		dataSource := standard.BuildDataSource("EVM", c.ChainConfig.ChainID, "Contract", contractAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(contractConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(contractConfig.GetEndBlock()),
		}
		if blockRange.StartBlock, err = c.getAddressStart(ctx, contractAddress, blockRange.StartBlock, latest); err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		for _, logConfig := range contractConfig.GetLogConfigs() {
			agent := HandlerAgentLog{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "log", logConfig, blockRange),
				Client:           c.Client,
				FetchConfig:      logConfig.GetFetchConfig(),
			}
			agent.Filters, err = NewLogFilters(logConfig.GetFilters(), false, contractAddress)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, traceConfig := range contractConfig.GetTraceConfigs() {
			agent := HandlerAgentTrace{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "trace", traceConfig, blockRange),
				FetchConfig:      traceConfig.GetFetchConfig(),
			}
			if contractAddress != "" {
				agent.Filter.Address = []string{strings.ToLower(contractAddress)}
			}
			if traceConfig.GetSignature() != "" {
				agent.Filter.Signature = []string{traceConfig.GetSignature()}
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		if len(contractConfig.GetTransactionConfig()) > 0 {
			if contractAddress != "" {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("transaction handler only support global processor"))
			}
			if len(contractConfig.GetTransactionConfig()) > 1 || hasTxnHandler {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("there can only be one transaction handler"))
			}
			txnConfig := contractConfig.GetTransactionConfig()[0]
			agent := HandlerAgentTransaction{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "transaction", txnConfig, blockRange),
				FetchConfig: txnConfig.GetFetchConfig(),
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
			hasTxnHandler = true
		}

		for _, intervalConfig := range contractConfig.GetIntervalConfigs() {
			agent := HandlerAgentInterval{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "interval", intervalConfig, blockRange),
				FetchConfig: intervalConfig.GetFetchConfig(),
			}
			agent.IntervalConfig, err = standard.NewIntervalConfig(intervalConfig)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	logger.Infof("built %d agents", len(c.Agents))
	return nil
}

func (c *HandlerController) getDataRequirement() (dr evm.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentTransaction:
			dr.Interval = append(dr.Interval, data.IntervalRequirement{
				IntervalConfig: data.IntervalConfig{
					BlockInterval: &data.BlockInterval{Backfill: 1, Watching: 1},
				},
				BlockRange: ag.Range,
			})
		case HandlerAgentInterval:
			dr.Interval = append(dr.Interval, data.IntervalRequirement{
				IntervalConfig: ag.IntervalConfig,
				BlockRange:     ag.Range,
			})
		case HandlerAgentTrace:
			dr.Trace = append(dr.Trace, evm.TraceRequirement{
				TraceFilter: ag.Filter,
				BlockRange:  ag.Range,
			})
		case HandlerAgentLog:
			dr.Log = append(dr.Log, evm.LogRequirement{
				LogFilter:  utils.Reduce(ag.Filters, evm.LogFilter.Merge),
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
	for _, agent := range c.Agents {
		if ar, err = agent.GetExtendRequirements(ctx, blockData); err != nil {
			return
		}
		req.Merge(ar)
	}
	return
}
