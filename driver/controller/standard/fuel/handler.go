package fuel

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	chainFuel "sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"
	chain "sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/fuel"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"
)

type FuelHandlerAgent interface {
	standard.HandlerAgent[*BlockData]
}

type HandlerController struct {
	*standard.BaseHandlerController[fuel.Client, *BlockData, FuelHandlerAgent]
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *chain.ConfigV2,
	client fuel.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[fuel.Client, *BlockData, FuelHandlerAgent](
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
	if extErr := c.BaseHandlerController.SetTemplates(ctx, templates); extErr != nil {
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

func (c *HandlerController) getAddressStart(ctx context.Context, address string, start, latest uint64) (uint64, error) {
	return c.GetAddressStart(
		address,
		start,
		func() (uint64, error) {
			newStart, has, getErr := c.Client.GetContractCreateBlockHeight(ctx, address, start)
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

	for dataSourceID, contractConfig := range c.Config.ContractConfigs {
		contractAddress := standard.AdjustAddress(contractConfig.GetContract().GetAddress())
		dataSource := standard.BuildDataSource("FUEL", c.ChainConfig.ChainID, "Contract", contractAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(contractConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(contractConfig.GetEndBlock()),
		}
		if blockRange.StartBlock, err = c.getAddressStart(ctx, contractAddress, blockRange.StartBlock, latest); err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		// interval
		for _, intervalConfig := range contractConfig.IntervalConfigs {
			agent := HandlerAgentInterval{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "interval", intervalConfig, blockRange),
			}
			agent.IntervalConfig, err = standard.NewIntervalConfig(intervalConfig)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		// asset transfer
		for _, assetConfig := range contractConfig.AssetConfigs {
			agent := HandlerAgentTransaction{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "transfer", assetConfig, blockRange),
				Filters:          make([]chainFuel.TransactionFilter, len(assetConfig.Filters)),
			}
			if len(assetConfig.Filters) == 0 {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("no filter for handler %s", agent.GetHandlerID().String()))
			}
			for i, filterConfig := range assetConfig.Filters {
				agent.Filters[i] = chainFuel.TransactionFilter{
					TransferFilter: &chainFuel.TransferFilter{
						AssetID: strings.ToLower(filterConfig.GetAssetId()),
						From:    strings.ToLower(filterConfig.GetFromAddress()),
						To:      strings.ToLower(filterConfig.GetToAddress()),
					},
					ExcludeFailed: true,
				}
				if contractAddress != "" {
					agent.Filters[i].CallFilter = &chainFuel.CallFilter{
						ContractID: contractAddress,
					}
				}
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		// all transactions
		for _, txConfig := range contractConfig.FuelTransactionConfigs {
			agent := HandlerAgentTransaction{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "transaction", txConfig, blockRange),
				Filters: []chainFuel.TransactionFilter{{
					CallFilter: &chainFuel.CallFilter{
						ContractID: contractAddress,
					},
					ExcludeFailed: true,
				}},
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		// receipt
		for _, receiptConfig := range contractConfig.FuelReceiptConfigs {
			if receiptConfig.GetLog() != nil {
				// log
				agent := HandlerAgentTransaction{
					BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "log", receiptConfig, blockRange),
					Filters:          make([]chainFuel.TransactionFilter, len(receiptConfig.GetLog().GetLogIds())),
				}
				for i, logID := range receiptConfig.GetLog().GetLogIds() {
					rb, parseErr := strconv.ParseUint(logID, 0, 64)
					if parseErr != nil {
						return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig, errors.Wrapf(parseErr,
							"parse logId %q to uint failed for handler %s", logID, agent.GetHandlerID().String()))
					}
					agent.Filters[i] = chainFuel.TransactionFilter{
						CallFilter: &chainFuel.CallFilter{
							ContractID: contractAddress,
						},
						LogFilter: &chainFuel.LogFilter{
							ContractID: contractAddress,
							LogRb:      &rb,
						},
						ExcludeFailed: true,
					}
				}
				c.Agents = append(c.Agents, agent)
				logger.Infow("has new agent", "agent", agent.Snapshot())
			}
			if receiptConfig.GetTransfer() != nil {
				// receipt transfer
				agent := HandlerAgentTransaction{
					BaseHandlerAgent: controller.NewBaseHandlerAgent(
						dataSource, dataSourceID, "receiptTransfer", receiptConfig, blockRange),
					Filters: []chainFuel.TransactionFilter{{
						CallFilter: &chainFuel.CallFilter{
							ContractID: contractAddress,
						},
						ReceiptTransferFilter: &chainFuel.ReceiptTransferFilter{
							AssetID: receiptConfig.GetTransfer().GetAssetId(),
							From:    receiptConfig.GetTransfer().GetFrom(),
							To:      receiptConfig.GetTransfer().GetTo(),
						},
						ExcludeFailed: true,
					}},
				}
				c.Agents = append(c.Agents, agent)
				logger.Infow("has new agent", "agent", agent.Snapshot())
			}
		}
	}
	return nil
}

func (c *HandlerController) BuildBlockDataFetcher(
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
) controller.Fetcher[controller.BlockData] {
	req := c.getDataRequirement()
	req.Interval = append(req.Interval, c.BuildReportRequirements(currentBlockNumber)...)

	fetchNamePrefix := fmt.Sprintf("FUEL::%s::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		fuel.BuildBlockMainDataFetcher(fetchNamePrefix, req, firstBlockNumber, currentBlockNumber, latest, c.Client),
		latest,
		controller.ProcessConcurrency,
		256*1024*1024, // 256MB
		100,
		time.Second*3,
		20,
		time.Second,
		func(ctx context.Context, blockNumber uint64, from fuel.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			var err error
			result := BlockData{mainData: from, checkpointData: make(map[string]string)}
			// always need header
			if result.Block, err = c.Client.GetBlock(ctx, blockNumber); err != nil {
				return nil, false, err
			}
			// build binding data
			if result.taskList, result.taskTotalSize, err = c.BuildTaskList(ctx, &result); err != nil {
				return nil, false, err
			}
			c.DumpAddressStart(result.checkpointData)
			return &result, true, nil
		},
	)
}

func (c *HandlerController) getDataRequirement() (dr fuel.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentTransaction:
			dr.Tx = append(dr.Tx, fuel.TransactionRequirement{
				Filters:    ag.Filters,
				BlockRange: ag.Range,
			})
		case HandlerAgentReceipt:
			dr.Tx = append(dr.Tx, fuel.TransactionRequirement{
				Filters:    ag.Filters,
				BlockRange: ag.Range,
			})
		case HandlerAgentInterval:
			dr.Interval = append(dr.Interval, data.IntervalRequirement{
				IntervalConfig: ag.IntervalConfig,
				BlockRange:     ag.Range,
			})
		}
	}
	return dr
}

func (c *HandlerController) Epilogue() {
	c.BaseHandlerController.FinishExecute()
}
