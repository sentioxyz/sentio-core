package aptos

import (
	"context"
	"fmt"
	"time"

	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/pkg/errors"

	chainaptos "sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	chain "sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/aptos"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"
)

type AptosHandlerAgent interface {
	standard.HandlerAgent[*BlockData]
}

type HandlerController struct {
	*standard.BaseHandlerController[aptos.Client, *BlockData, AptosHandlerAgent]
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *chain.ConfigV2,
	client aptos.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[aptos.Client, *BlockData, AptosHandlerAgent](
			processor, initResult, chainConfig, client, processorClients),
	}
}

func (c *HandlerController) getAddressStart(ctx context.Context, address string, start, latest uint64) (uint64, error) {
	return c.GetAddressStart(
		address,
		start,
		func() (uint64, error) {
			newStart, has, getErr := c.Client.GetAddressStartBlock(ctx, address, start, latest)
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
		normalizedAccountAddr := accountAddress
		var accountAddr aptossdk.AccountAddress
		if accountAddress != "" {
			if err = accountAddr.ParseStringRelaxed(accountAddress); err != nil {
				return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed,
					errors.Wrapf(err, "invalid account address %q", accountAddress))
			}
			normalizedAccountAddr = accountAddr.String()
		}
		dataSource := fmt.Sprintf("APTOS:%s/Account:%s", c.ChainConfig.ChainID, accountAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(accountConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(accountConfig.GetEndBlock()),
		}
		blockRange.StartBlock, err = c.getAddressStart(ctx, normalizedAccountAddr, blockRange.StartBlock, latest)
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		for _, intervalConfig := range accountConfig.GetMoveIntervalConfigs() {
			agent := HandlerAgentInterval{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "interval", intervalConfig.GetIntervalConfig(), blockRange),
			}
			if accountAddress == "" {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("account address cannot be empty for handler %s", agent.GetHandlerID().String()))
			}
			agent.IntervalConfig, err = standard.NewIntervalConfig(intervalConfig.GetIntervalConfig())
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			agent.FetchConfig = aptos.AccountResourceFilter{
				Address:      normalizedAccountAddr,
				ResourceType: nil, // default need all resources of the account
			}
			if resourceType := intervalConfig.GetType(); resourceType != "" {
				agent.FetchConfig.ResourceType = []string{resourceType}
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, changeConfig := range accountConfig.GetMoveResourceChangeConfigs() {
			resTypes := changeConfig.GetTypes()
			agent := HandlerAgentChange{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "change", changeConfig, blockRange),
				Filter: chainaptos.ChangeFilter{
					Address:       set.New[aptossdk.AccountAddress](),
					ResourceTypes: nil,
				},
			}
			agent.Filter.ResourceTypes, err = utils.MapSlice(resTypes, move.BuildType)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			if accountAddress != "" {
				agent.Filter.Address.Add(accountAddr)
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	for dataSourceID, contractConfig := range c.Config.ContractConfigs {
		contractAddress := standard.AdjustAddress(contractConfig.GetContract().GetAddress())
		normalizedContractAddr := contractAddress
		if contractAddress != "" {
			var contractAddr aptossdk.AccountAddress
			if err = contractAddr.ParseStringRelaxed(contractAddress); err != nil {
				return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed,
					errors.Wrapf(err, "invalid contract address %q", contractAddress))
			}
			normalizedContractAddr = contractAddr.String()
		}
		dataSource := fmt.Sprintf("APTOS:%s/Contract:%s", c.ChainConfig.ChainID, contractAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(contractConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(contractConfig.GetEndBlock()),
		}
		blockRange.StartBlock, err = c.getAddressStart(ctx, normalizedContractAddr, blockRange.StartBlock, latest)
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		for _, moveCallConfig := range contractConfig.GetMoveCallConfigs() {
			agent := HandlerAgentFunction{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "call", moveCallConfig, blockRange),
				Filter: chainaptos.TransactionFilter{
					FailedIsOK:      moveCallConfig.GetFetchConfig().GetIncludeFailedTransaction(),
					MultiSigTxnIsOK: moveCallConfig.GetFetchConfig().GetSupportMultisigFunc(),
				},
			}
			agent.Filter.FunctionFilters, err = utils.MapSlice(
				moveCallConfig.GetFilters(),
				func(f *protos.MoveCallFilter) (ff chainaptos.FunctionFilter, err error) {
					var funcName string
					if f.GetFunction() != "" {
						funcName = contractAddress + "::" + f.GetFunction()
					} else if contractAddress != "" {
						funcName = contractAddress + "::" + contractConfig.GetContract().GetName()
					}
					ff.FunctionPattern, err = move.BuildType(funcName)
					if err != nil {
						return ff, errors.Wrapf(err, "invalid call function %q", funcName)
					}
					if f.GetWithTypeArguments() {
						ff.CheckTypeArguments, ff.TypedArguments = true, f.GetTypeArguments()
					}
					if sender := f.GetFromAndToAddress().GetFrom(); sender != "" {
						var senderAddr aptossdk.AccountAddress
						if err = senderAddr.ParseStringRelaxed(sender); err != nil {
							return ff, errors.Wrapf(err, "invalid sender address %q", sender)
						}
						ff.Sender = &senderAddr
					}
					return
				},
			)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "handler %s have invalid filter", agent.GetHandlerID().String()))
			}
			if len(agent.Filter.FunctionFilters) == 0 {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("no filter in handler %s", agent.GetHandlerID().String()))
			}
			agent.FetchConfig.NeedAllEvents = moveCallConfig.GetFetchConfig().GetAllEvents()
			if moveCallConfig.GetFetchConfig().GetResourceChanges() {
				var resType move.Type
				resType, err = move.BuildType(moveCallConfig.GetFetchConfig().GetResourceConfig().GetMoveTypePrefix())
				if err != nil {
					return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
						errors.Wrapf(err, "unexpected config for handler %s: invalid resouce type prefix %q",
							agent.GetHandlerID().String(),
							moveCallConfig.GetFetchConfig().GetResourceConfig().GetMoveTypePrefix()))
				}
				agent.FetchConfig.ChangeResourceTypes = move.TypeSet{resType}
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, eventConfig := range contractConfig.GetMoveEventConfigs() {
			agent := HandlerAgentEvent{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "event", eventConfig, blockRange),
				Filter: chainaptos.TransactionFilter{
					FailedIsOK:      eventConfig.GetFetchConfig().GetIncludeFailedTransaction(),
					MultiSigTxnIsOK: eventConfig.GetFetchConfig().GetSupportMultisigFunc(),
				},
			}
			if contractAddress == "" {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("contract address cannot be empty for handler %s", agent.GetHandlerID().String()))
			}
			agent.Filter.EventFilters, err = utils.MapSlice(
				eventConfig.GetFilters(),
				func(f *protos.MoveEventFilter) (ff chainaptos.EventFilter, err error) {
					eventType := contractAddress + "::" + f.GetType()
					ff.Type, err = move.BuildType(eventType)
					if err != nil {
						return ff, errors.Wrapf(err, "invalid event type %q", eventType)
					}
					if f.GetEventAccount() != "" {
						var addr aptossdk.AccountAddress
						if err = addr.ParseStringRelaxed(f.GetEventAccount()); err != nil {
							return ff, errors.Wrapf(err, "invalid event account %q", f.GetEventAccount())
						}
						ff.GuiAccountAddress = &addr
					}
					if ff.IsEmpty() {
						return ff, errors.New("event filter is empty")
					}
					return ff, nil
				})
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "handler %s have invalid filter", agent.GetHandlerID().String()))
			}
			if len(agent.Filter.EventFilters) == 0 {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("no filter in handler %s", agent.GetHandlerID().String()))
			}
			agent.FetchConfig.NeedAllEvents = eventConfig.GetFetchConfig().GetAllEvents()
			if eventConfig.GetFetchConfig().GetResourceChanges() {
				var resType move.Type
				resType, err = move.BuildType(eventConfig.GetFetchConfig().GetResourceConfig().GetMoveTypePrefix())
				if err != nil {
					return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
						errors.Wrapf(err, "unexpected config for handler %s: invalid resouce type prefix %q",
							agent.GetHandlerID().String(),
							eventConfig.GetFetchConfig().GetResourceConfig().GetMoveTypePrefix()))
				}
				agent.FetchConfig.ChangeResourceTypes = move.TypeSet{resType}
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, intervalConfig := range contractConfig.GetMoveIntervalConfigs() {
			agent := HandlerAgentInterval{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "interval", intervalConfig.GetIntervalConfig(), blockRange),
			}
			agent.IntervalConfig, err = standard.NewIntervalConfig(intervalConfig.GetIntervalConfig())
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			if contractAddress == "" {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("contract address cannot be empty for handler %s", agent.GetHandlerID().String()))
			}
			agent.FetchConfig = aptos.AccountResourceFilter{
				Address:      normalizedContractAddr,
				ResourceType: make([]string, 0), // do not need any resource
			}
			if resourceType := intervalConfig.GetType(); resourceType != "" {
				logger.Warnf("type %q in contract move interval config %s will be ignored", resourceType, agent.HandlerID)
			}
			c.Agents = append(c.Agents, agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	logger.Infof("built %d agents", len(c.Agents))
	return nil
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

	fetchNamePrefix := fmt.Sprintf("APTOS::%s::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		aptos.BuildBlockMainDataFetcher(fetchNamePrefix, req, firstBlockNumber, currentBlockNumber, latest, c.Client),
		latest,
		controller.ProcessConcurrency,
		256*1024*1024, // 256MB
		1000,
		time.Second*10,
		20,
		time.Second,
		func(ctx context.Context, blockNumber uint64, from aptos.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			var err error
			result := BlockData{mainData: from, checkpointData: make(map[string]string)}
			// take the main data and ask the agents what account resources are needed
			var accountResourceFilters []aptos.AccountResourceFilter
			var needFullTx bool
			for _, agent := range c.Agents {
				ag, is := agent.(HandlerAgentInterval)
				if !is || !data.ContainsInterval(from.Intervals, ag.IntervalConfig) {
					continue
				}
				if ag.FetchConfig.NeedNothing() {
					needFullTx = true
				} else {
					accountResourceFilters = append(accountResourceFilters, ag.FetchConfig)
				}
			}
			// actually get the extended data
			result.accountResources, err = c.Client.GetAccountResources(
				ctx, blockNumber, aptos.MergeAccountResourceFilters(accountResourceFilters))
			if err != nil {
				return nil, false, err
			}
			// fetch the transaction
			if needFullTx {
				// Although result.mainData.Txn may already exist, it might be missing some events or changes.
				// So need to re-get the tx here
				var tx chainaptos.Transaction
				if tx, err = c.Client.GetTransaction(ctx, blockNumber); err != nil {
					return nil, false, err
				}
				result.mainData.Txn = &tx
			}
			// always need header, fill result.BlockHeader here
			if result.mainData.Txn != nil {
				result.BlockHeader = (*aptos.Transaction)(result.mainData.Txn)
			} else if result.mainData.SimpleTxn != nil {
				result.BlockHeader = (*aptos.MinimalistTransaction)(result.mainData.SimpleTxn)
			} else if result.BlockHeader, err = c.Client.GetMinimalistTransaction(ctx, blockNumber); err != nil {
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

func (c *HandlerController) getDataRequirement() (dr aptos.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentChange:
			dr.Changes = append(dr.Changes, aptos.ChangeRequirement{
				BlockRange:   ag.Range,
				ChangeFilter: ag.Filter,
			})
		case HandlerAgentEvent:
			dr.Txn = append(dr.Txn, aptos.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentFunction:
			dr.Txn = append(dr.Txn, aptos.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentInterval:
			dr.Interval = append(dr.Interval, data.IntervalRequirement{
				IntervalConfig: ag.IntervalConfig,
				BlockRange:     ag.Range,
			})
		}
	}
	return
}
