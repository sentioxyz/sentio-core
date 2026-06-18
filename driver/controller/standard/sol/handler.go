package sol

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/sol"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"
)

type HandlerController struct {
	*standard.BaseHandlerController[sol.Client, *BlockData, standard.HandlerAgent[*BlockData]]
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *config.ChainConfig,
	client sol.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[sol.Client, *BlockData, standard.HandlerAgent[*BlockData]](
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

	fetchNamePrefix := fmt.Sprintf("SOL::%s::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		sol.BuildBlockMainDataFetcher(fetchNamePrefix, req, currentBlockNumber, latest, c.Client),
		latest,
		controller.ProcessConcurrency,
		256*1024*1024, // 256MB
		100,
		time.Second*30, // may need to get a lot of transaction
		20,
		time.Second,
		func(ctx context.Context, blockNumber uint64, from sol.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			_, logger := log.FromContext(ctx)
			// The main-data fetchers already returned the block header (interval) and the full
			// matching transactions (instruction), so the block data is built without any fetch.
			result := BlockData{
				mainData:       from,
				txJSONDict:     make(map[solana.Signature]string),
				checkpointData: make(map[string]string),
			}
			var err error
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

func (c *HandlerController) buildAgents(ctx context.Context, first, latest uint64) *controller.ExternalError {
	_, logger := log.FromContext(ctx)
	c.Agents = nil
	var err error

	for dataSourceID, contractConfig := range c.Config.ContractConfigs {
		contractAddress := standard.AdjustAddress(contractConfig.GetContract().GetAddress())
		dataSource := standard.BuildDataSource("SOL", c.ChainConfig.ChainID, "Contract", contractAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(contractConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(contractConfig.GetEndBlock()),
		}
		parsedContractAddress, parseContractAddrErr := solana.PublicKeyFromBase58(contractAddress)
		blockRange.StartBlock, err = c.GetAddressStart(
			contractAddress,
			blockRange.StartBlock,
			func() (uint64, error) {
				ns, has, getErr := c.Client.GetContractStartBlock(ctx, parsedContractAddress, blockRange.StartBlock, latest)
				if getErr != nil {
					return 0, getErr
				}
				if has {
					return ns, nil
				}
				return latest + 1, nil
			})
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		if config := contractConfig.GetInstructionConfig(); config != nil {
			if parseContractAddrErr != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig, errors.Wrapf(
					parseContractAddrErr, "contract address %q is invalid for the instruction handler", contractAddress))
			}
			agent := HandlerAgentInstruction{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "instruction", controller.SimpleHandlerConfig{}, blockRange),
				Address:                  parsedContractAddress,
				ProcessInnerInstruction:  config.GetInnerInstruction(),
				ProcessParsedInstruction: config.GetParsedInstruction(),
				ProcessRawInstruction:    config.GetRawDataInstruction(),
				FetchTx:                  config.GetFetchTx(),
			}
			if !agent.ProcessParsedInstruction && !agent.ProcessRawInstruction {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig, errors.Errorf(
					"unexpected config for handler %s: neigher parsed nor raw data is needed", agent.GetHandlerID().String()))
			}
			c.Agents = append(c.Agents, agent)
		}

		for _, intervalConfig := range contractConfig.GetIntervalConfigs() {
			if contractAddress != "" {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("contract %s cannot have interval config", contractAddress))
			}
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
		}
	}

	logger.Infof("built %d agents", len(c.Agents))
	return nil
}

func (c *HandlerController) getDataRequirement() (dr sol.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentInstruction:
			dr.Tx = append(dr.Tx, sol.TransactionRequirement{
				BlockRange: ag.Range,
				Programs:   []solana.PublicKey{ag.Address},
			})
		case HandlerAgentInterval:
			dr.Interval = append(dr.Interval, data.IntervalRequirement{
				BlockRange:     ag.Range,
				IntervalConfig: ag.IntervalConfig,
			})
		}
	}
	return dr
}
