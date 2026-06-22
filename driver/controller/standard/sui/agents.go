package sui

import (
	"context"
	"strings"

	"sentioxyz/sentio-core/chain/move"
	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/pkg/errors"
)

// BuildSuiAgents turns the parsed processor config into the SUI handler agents,
// emitting each built agent via emit. It is the format-agnostic core shared by
// the json-rpc handler controller (this package) and the grpc one
// (standard/sui/grpc): both build the SAME agents/filters here, and only differ
// in how the agent's BuildBindingDataList reads data and serializes the binding.
// The grpc controller wraps each emitted agent into a grpc agent that embeds it.
func BuildSuiAgents(
	ctx context.Context,
	config standard.HandlerConfig,
	chainConfig *config.ChainConfig,
	sdkVersion string,
	client sui.Client,
	first uint64,
	getAddressStart func(ctx context.Context, address string, start uint64) (uint64, error),
	getPackageHistory func(ctx context.Context, pkgID string) ([]string, error),
	emit func(SuiHandlerAgent),
) *controller.ExternalError {
	_, logger := log.FromContext(ctx)

	processorVersion, err := processor.ParseVersion(sdkVersion)
	if err != nil {
		return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
			errors.Wrapf(err, "parse processor sdk version %q failed", sdkVersion))
	}

	for dataSourceID, accountConfig := range config.AccountConfigs {
		accountAddress := standard.AdjustAddress(accountConfig.GetAddress())
		dataSource := standard.BuildDataSource("SUI", chainConfig.ChainID, "Account", accountAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(accountConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(accountConfig.GetEndBlock()),
		}

		if blockRange.StartBlock, err = getAddressStart(ctx, accountAddress, blockRange.StartBlock); err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		for _, intervalConfig := range accountConfig.GetMoveIntervalConfigs() {
			agent := HandlerAgentInterval{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(
					dataSource, dataSourceID, "interval", intervalConfig.GetIntervalConfig(), blockRange),
				Client: client,
			}
			agent.IntervalConfig, err = standard.NewIntervalConfig(intervalConfig.GetIntervalConfig())
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			switch intervalConfig.GetOwnerType() {
			case protos.MoveOwnerType_ADDRESS, protos.MoveOwnerType_OBJECT, protos.MoveOwnerType_WRAPPED_OBJECT:
				if accountAddress == "" {
					return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
						errors.Errorf("unexpected config for handler %s: address should not be empty because owner type is %s",
							agent.GetHandlerID().String(), intervalConfig.GetOwnerType().String()))
				}
				agent.Filter.OwnerFilter = &chainsui.ObjectChangeOwnerFilter{OwnerID: []string{accountAddress}}
				agent.UnwrapDynamicObject = true
				switch intervalConfig.GetOwnerType() {
				case protos.MoveOwnerType_ADDRESS:
					agent.Filter.OwnerFilter.OwnerType = []string{"address"}
				case protos.MoveOwnerType_OBJECT:
					agent.Filter.OwnerFilter.OwnerType = []string{"object"}
					agent.NeedSelf = true
				case protos.MoveOwnerType_WRAPPED_OBJECT:
					agent.Filter.OwnerFilter.OwnerType = []string{"object"}
				}
				if !intervalConfig.GetResourceFetchConfig().GetOwned() {
					agent.Filter.OwnerFilter.OwnerType = nil
				}
			case protos.MoveOwnerType_TYPE:
				if accountAddress != "" {
					return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig, errors.Errorf(
						"unexpected config for handler %s: address is %s, it should be empty because owner type is %s",
						agent.GetHandlerID().String(), accountAddress, intervalConfig.GetOwnerType().String()))
				}
				var objectType move.Type
				objectType, err = move.BuildType(intervalConfig.GetType())
				if err != nil {
					return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
						errors.Wrapf(err, "unexpected config for handler %s: invalid object type %q",
							agent.GetHandlerID().String(), intervalConfig.GetType()))
				}
				agent.Filter.TypePattern = move.TypeSet{objectType}
				agent.UnwrapDynamicObject = false
			default:
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Errorf("unexpected config for handler %s: unknown owner type %s",
						agent.GetHandlerID().String(), intervalConfig.GetOwnerType().String()))
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, moveCallConfig := range accountConfig.GetMoveCallConfigs() {
			agent, extErr := buildFunctionAgent(dataSource, dataSourceID, accountAddress, moveCallConfig, blockRange)
			if extErr != nil {
				return extErr
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, changeConfig := range accountConfig.GetMoveResourceChangeConfigs() {
			agent := HandlerAgentChange{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "change", changeConfig, blockRange),
			}
			agent.Filter.TypePattern, err = utils.MapSlice(changeConfig.GetTypes(), move.BuildType)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	for dataSourceID, contractConfig := range config.ContractConfigs {
		contractAddress := standard.AdjustAddress(contractConfig.GetContract().GetAddress())
		dataSource := standard.BuildDataSource("SUI", chainConfig.ChainID, "Contract", contractAddress)
		blockRange := controller.BlockRange{
			StartBlock: max(contractConfig.GetStartBlock(), first),
			EndBlock:   standard.AdjustEndBlock(contractConfig.GetEndBlock()),
		}

		if blockRange.StartBlock, err = getAddressStart(ctx, contractAddress, blockRange.StartBlock); err != nil {
			return controller.NewExternalError(controller.ErrCodeGetContractStartBlockFailed, err)
		}

		var pkgHistory []string // used for search real event type
		if contractAddress != "" {
			if !chainConfig.KeepSuiEventTypePackage && len(contractConfig.GetMoveEventConfigs()) > 0 {
				if pkgHistory, err = getPackageHistory(ctx, contractAddress); err != nil {
					return controller.NewExternalError(controller.ErrCodeFetchDataFailed,
						errors.Wrapf(err, "get package history for contract %s failed", contractAddress))
				}
			} else {
				pkgHistory = []string{contractAddress}
			}
		}

		for _, eventConfig := range contractConfig.GetMoveEventConfigs() {
			agent := HandlerAgentEvent{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "event", eventConfig, blockRange),
				Filter: chainsui.TransactionFilter{
					FailedIsOK: eventConfig.GetFetchConfig().GetIncludeFailedTransaction(),
				},
				FetchConfig: chainsui.TransactionFetchConfig{
					NeedInputs:    true,
					NeedEffects:   true,
					NeedAllEvents: eventConfig.GetFetchConfig().GetAllEvents(),
				},
			}
			// sdk before v2.32, includeInputs and includeResourceChanges always be true
			if processorVersion.Major > 2 || (processorVersion.Major == 2 && processorVersion.Minor >= 32) {
				agent.FetchConfig.NeedInputs = eventConfig.GetFetchConfig().GetInputs()
				agent.FetchConfig.NeedEffects = eventConfig.GetFetchConfig().GetResourceChanges()
			}
			for _, f := range eventConfig.GetFilters() {
				var ff chainsui.EventFilterV2
				if sender := f.GetEventAccount(); sender != "" {
					ff.Sender = &sender
				}
				// Address is needed as the packageId to form a complete event type; the address part of the
				// event type maybe the upgraded package id, so iterate the whole package history.
				for _, addr := range pkgHistory {
					var typ move.Type
					typ, err = move.BuildType(addr + "::" + f.GetType())
					if err != nil {
						return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
							errors.Wrapf(err, "unexpected config for handler %s: invalid event type %q",
								agent.GetHandlerID().String(), f.GetType()))
					}
					ff.TypePattern = append(ff.TypePattern, typ)
				}
				agent.Filter.EventFilters = append(agent.Filter.EventFilters, ff)
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, moveCallConfig := range contractConfig.GetMoveCallConfigs() {
			agent, extErr := buildFunctionAgent(dataSource, dataSourceID, contractAddress, moveCallConfig, blockRange)
			if extErr != nil {
				return extErr
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}

		for _, changeConfig := range contractConfig.GetMoveResourceChangeConfigs() {
			agent := HandlerAgentChange{
				BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "change", changeConfig, blockRange),
			}
			agent.Filter.TypePattern, err = utils.MapSlice(changeConfig.GetTypes(), move.BuildType)
			if err != nil {
				return controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(err, "unexpected config for handler %s", agent.GetHandlerID().String()))
			}
			emit(agent)
			logger.Infow("has new agent", "agent", agent.Snapshot())
		}
	}

	return nil
}

// buildFunctionAgent builds a move-call (function) agent; shared by both the account and contract loops.
func buildFunctionAgent(
	dataSource string,
	dataSourceID int,
	address string,
	moveCallConfig *protos.MoveCallHandlerConfig,
	blockRange controller.BlockRange,
) (HandlerAgentFunction, *controller.ExternalError) {
	agent := HandlerAgentFunction{
		BaseHandlerAgent: controller.NewBaseHandlerAgent(dataSource, dataSourceID, "call", moveCallConfig, blockRange),
		FetchConfig: chainsui.TransactionFetchConfig{
			NeedInputs:    true,
			NeedEffects:   true,
			NeedAllEvents: true,
		},
	}
	for _, f := range moveCallConfig.GetFilters() {
		var ff chainsui.FunctionFilter
		ff.Kind = utils.WrapPointer("ProgrammableTransaction")
		ff.CommandFilter = &chainsui.CommandFilter{}
		if address != "" {
			packageID, parseErr := types.StrToObjectID(address)
			if parseErr != nil {
				return agent, controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
					errors.Wrapf(parseErr, "unexpected config for handler %s: address %q is not a valid object id",
						agent.GetHandlerID().String(), address))
			}
			ff.CommandFilter.CallPackage = utils.WrapPointer(packageID.String())
		}
		callModule, callFunc, _ := strings.Cut(f.GetFunction(), "::")
		if callModule != "" && callModule != "*" {
			ff.CommandFilter.CallModule = &callModule
		}
		if callFunc != "" && callFunc != "*" {
			ff.CommandFilter.CallFunction = &callFunc
		}
		if ff.CommandFilter.IsEmpty() {
			ff.CommandFilter = nil
		}
		if prefix := f.GetPublicKeyPrefix(); prefix != "" {
			if !strings.HasPrefix(prefix, "0x") {
				prefix = "0x" + prefix
			}
			ff.MultiSigPublicKeyPrefix = &prefix
		}
		if from := f.GetFromAndToAddress().GetFrom(); from != "" {
			ff.Sender = &from
		}
		if to := f.GetFromAndToAddress().GetTo(); to != "" {
			ff.Receiver = &to
		}
		ff.FailedIsOK = f.GetIncludeFailed() && moveCallConfig.GetFetchConfig().GetIncludeFailedTransaction()
		agent.Filter.FunctionFilters = append(agent.Filter.FunctionFilters, ff)
	}
	agent.Filter.FailedIsOK = moveCallConfig.GetFetchConfig().GetIncludeFailedTransaction()
	if len(agent.Filter.FunctionFilters) > 0 {
		agent.Filter.FailedIsOK = utils.Reduce(
			utils.MapSliceNoError(agent.Filter.FunctionFilters, func(ff chainsui.FunctionFilter) bool {
				return ff.FailedIsOK
			}),
			func(a, b bool) bool { return a || b },
		)
	}
	return agent, nil
}
