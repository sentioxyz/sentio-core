package startup

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	evmchain "sentioxyz/sentio-core/chain/evm"
	suitypes "sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	aptosdata "sentioxyz/sentio-core/driver/controller/data/aptos"
	evmdata "sentioxyz/sentio-core/driver/controller/data/evm"
	fueldata "sentioxyz/sentio-core/driver/controller/data/fuel"
	soldata "sentioxyz/sentio-core/driver/controller/data/sol"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/driver/controller/standard/aptos"
	"sentioxyz/sentio-core/driver/controller/standard/evm"
	"sentioxyz/sentio-core/driver/controller/standard/fuel"
	"sentioxyz/sentio-core/driver/controller/standard/sol"
	"sentioxyz/sentio-core/driver/controller/standard/sui"
	suigrpc "sentioxyz/sentio-core/driver/controller/standard/sui/grpc"
	"sentioxyz/sentio-core/driver/exitcode"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/common/rpc"
	protossvc "sentioxyz/sentio-core/service/processor/protos"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type standardStartupController struct {
	baseStartupController

	processorClients []protos.ProcessorV3Client
	initResult       *protos.InitResponse
}

func (c *standardStartupController) buildProcessorUrlList() ([]string, error) {
	processorUrlList := []string{c.config.ProcessorUrl}
	if c.processor.NumWorkers <= 1 {
		return processorUrlList, nil
	}
	host, port, has := strings.Cut(c.config.ProcessorUrl, ":")
	var basePort uint64 = 80
	if has {
		basePort, _ = strconv.ParseUint(port, 10, 64)
	}
	for i := uint64(1); i < uint64(c.processor.NumWorkers); i++ {
		processorUrlList = append(processorUrlList, fmt.Sprintf("%s:%d", host, basePort+i))
	}
	return processorUrlList, nil
}

func (c *standardStartupController) buildMainControllers(ctx context.Context) (
	ctrls map[string]*controller.MainController,
	exitCode exitcode.Code,
	err error,
) {
	_, logger := log.FromContext(ctx)
	ctrls = make(map[string]*controller.MainController)

	// connect to webhook service and create webhook subscription
	if err = c.createWebhookSubscription(ctx); err != nil {
		return ctrls, 0, errors.Wrapf(err, "create webhook subscription failed")
	}

	// create gcp pub sub topic
	if err = c.createPubSubTopic(ctx); err != nil {
		return ctrls, 0, errors.Wrapf(err, "create pubsub topic failed")
	}

	// connect to clickhouse
	if err = c.connectClickhouse(ctx); err != nil {
		return ctrls, 0, errors.Wrapf(err, "connect to clickhouse failed")
	}

	// build time series store
	if err = c.buildTimeSeriesStore(ctx); err != nil {
		return ctrls, exitcode.AlwaysRetry, errors.Wrapf(err, "build time series store failed")
	}

	// load all templates
	var templates []*protos.TemplateInstance
	for _, cs := range c.processor.ChainStates {
		if chainTemplates, err := LoadTemplates(cs); err != nil {
			return ctrls, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeInvalidCheckpointData,
				errors.Wrapf(err, "invaid templates"))
		} else {
			templates = append(templates, standard.ConvertTemplateInstanceBack(cs.ChainID, chainTemplates)...)
		}
	}

	// connect to all processor and init them
	processorUrlList, buildErr := c.buildProcessorUrlList()
	if buildErr != nil {
		return ctrls, exitcode.NeverRetry, buildErr
	}
	c.processorClients = make([]protos.ProcessorV3Client, len(processorUrlList))
	var initResultText string
	for i, processorUrl := range processorUrlList {
		// connect to processor
		conn, connErr := rpc.DialInsecure(processorUrl)
		if connErr != nil {
			return ctrls, 0, errors.Wrapf(connErr, "connect to processor #%d %s failed", i, processorUrl)
		}
		c.release = append(c.release, func() {
			_ = conn.Close()
		})
		c.processorClients[i] = protos.NewProcessorV3Client(conn)
		logger.Infof("connected to processor #%d %s", i, processorUrl)

		// call start function
		for {
			_, startErr := c.processorClients[i].Start(ctx, &protos.StartRequest{TemplateInstances: templates})
			if startErr == nil {
				break
			}
			if status.Code(startErr) == codes.InvalidArgument {
				return ctrls, exitcode.AlwaysRetry, errors.Wrapf(startErr, "call start for processor failed")
			}
			logger.Warnfe(startErr, "call start for processor #%d failed, will retry after %s", i, initRetryInterval)
			if startErr = utils.Sleep(ctx, initRetryInterval); startErr != nil {
				return ctrls, exitcode.AlwaysRetry, errors.Wrapf(startErr, "call start for processor failed")
			}
		}
		logger.Infof("called start for processor #%d", i)

		// get init result
		resp, getConfigErr := c.processorClients[i].GetConfig(ctx, &protos.ProcessConfigRequest{})
		if getConfigErr != nil {
			return ctrls, exitcode.AlwaysRetry, errors.Wrapf(getConfigErr, "get processor config failed")
		}
		chainIDSet := set.New[string]()
		for _, cc := range resp.GetContractConfigs() {
			chainIDSet.Add(cc.GetContract().GetChainId())
		}
		for _, ac := range resp.GetAccountConfigs() {
			chainIDSet.Add(ac.GetChainId())
		}
		chainIDList := chainIDSet.DumpValues()
		sort.Strings(chainIDList)
		initResult := &protos.InitResponse{
			ChainIds:        chainIDList,
			DbSchema:        resp.GetDbSchema(),
			Config:          resp.GetConfig(),
			ExecutionConfig: resp.GetExecutionConfig(),
			MetricConfigs:   resp.GetMetricConfigs(),
			ExportConfigs:   resp.GetExportConfigs(),
			EventLogConfigs: resp.GetEventLogConfigs(),
		}

		// check init result
		if i == 0 {
			c.initResult, initResultText = initResult, string(protojson.MustJSONMarshal(initResult))
		} else if another := string(protojson.MustJSONMarshal(initResult)); another != initResultText {
			logger.With("this", another, "before", initResultText).Errorf("configs from processor #%d was different", i)
			return ctrls, exitcode.AlwaysRetry, errors.Errorf("configs from different processor has diff")
		}
		logger.Infof("got config from processor #%d succeed", i)
	}
	logger.Infow("init processors succeed", "initResult", initResultText)

	// check chain
	if len(c.initResult.GetChainIds()) == 0 {
		return ctrls, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
			errors.Errorf("no chain in processor"))
	}
	for _, chainID := range c.initResult.GetChainIds() {
		if _, has := c.chainConfigs[chainID]; !has {
			return ctrls, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
				errors.Errorf("chain %s is not supported", chainID))
		}
	}

	// build entity store
	if schemeCnt := c.initResult.GetDbSchema().GetGqlSchema(); len(schemeCnt) > 0 {
		if extErr := c.buildEntityStore(ctx, schemeCnt); extErr != nil {
			if extErr.IsUserError() {
				return ctrls, exitcode.NeverRetry, extErr
			}
			return ctrls, exitcode.AlwaysRetry, extErr
		}
		req := &protossvc.SetProcessorEntitySchemaRequest{ProcessorId: c.config.ProcessorID, Schema: schemeCnt}
		if _, err = c.processorClient.SetProcessorEntitySchema(ctx, req); err != nil {
			return ctrls, exitcode.NeverRetry, errors.Wrapf(err, "update processor entity schema failed")
		}
	}

	// build main controller for each chain
	for _, chainID := range c.initResult.GetChainIds() {
		ctrls[chainID], exitCode, err = c.buildMainController(ctx, chainID)
		if err != nil {
			err = errors.Wrapf(err, "build main controller for chain %s failed", chainID)
			return
		}
		logger.Infof("main controller of chain %s is ready", chainID)
	}
	return
}

// SuiEnableGRPC grpc-format data is supported only for the sui variation (not iota) at DriverVersion >= 2.
func SuiEnableGRPC(chainID string, driverVersion int32) bool {
	return driverVersion >= 2 && suitypes.VariationFromChainID(chains.SuiChainID(chainID)) == suitypes.VariationSUI
}

func (c *standardStartupController) buildMainController(
	ctx context.Context,
	chainID string,
) (ctrl *controller.MainController, exitCode exitcode.Code, err error) {
	chainConfig := c.chainConfigs[chainID]
	if chainConfig.IsCustomizedEndpoint {
		if err = evmchain.CheckArchiveNode(context.Background(), chainConfig.Endpoint); err != nil {
			return nil, exitcode.AlwaysRetry, controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
				errors.Wrapf(err, "invalid customized endpoint %q for chain %s", chainConfig.Endpoint, chainID))
		}
	}
	chainType, hasChain := chains.GetChainType(chains.ChainID(chainID))
	if !hasChain {
		return nil, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeUnexpectedProcessorConfig,
			errors.Errorf("unknown chain id %s", chainID))
	}
	// prepare client and handler controller
	var checkLink bool
	var cli controller.Client
	var handlerCtrl controller.HandlerController
	switch {
	case chainID == manifest.CustomizedChainID || chains.IsEVMChains(chainID):
		checkLink = true
		evmCli, newClientErr := evmdata.NewClient(
			ctx,
			chainConfig.Endpoint,
			int(controller.ClientMaxConcurrency),
			chainConfig.StartBlockOverride,
			chainConfig.ProcessingDelayBlocks,
			controller.SubscribeMinWatchInterval,
			time.Second*3,
		)
		if newClientErr != nil {
			return nil, exitcode.NeverRetry, errors.Wrapf(newClientErr, "build evm client failed")
		}
		handlerCtrl = evm.NewHandlerController(c.processor, c.initResult, chainConfig, evmCli, c.processorClients)
		cli = evmCli
	case chains.IsAptosChain(chainID):
		aptosCli, newClientErr := aptosdata.NewClient(
			ctx,
			chainConfig.Endpoint,
			int(controller.ClientMaxConcurrency),
			chainConfig.StartBlockOverride,
			controller.SubscribeMinWatchInterval,
		)
		if newClientErr != nil {
			return nil, exitcode.NeverRetry, errors.Wrapf(newClientErr, "build aptos client failed")
		}
		handlerCtrl = aptos.NewHandlerController(c.processor, c.initResult, chainConfig, aptosCli, c.processorClients)
		cli = aptosCli
	case chains.IsSuiChain(chainID):
		suiCli, newClientErr := suidata.NewClient(
			ctx,
			chainConfig.Endpoint,
			int(controller.ClientMaxConcurrency),
			chainConfig.StartBlockOverride,
			controller.SubscribeMinWatchInterval,
		)
		if newClientErr != nil {
			return nil, exitcode.NeverRetry, errors.Wrapf(newClientErr, "build sui client failed")
		}
		if SuiEnableGRPC(chainID, c.processor.DriverVersion) {
			handlerCtrl = suigrpc.NewHandlerController(c.processor, c.initResult, chainConfig, suiCli, c.processorClients)
		} else {
			handlerCtrl = sui.NewHandlerController(c.processor, c.initResult, chainConfig, suiCli, c.processorClients)
		}
		cli = suiCli
	case chains.IsFuelChain(chainID):
		fuelCli, newClientErr := fueldata.NewClient(
			ctx,
			chainConfig.Endpoint,
			int(controller.ClientMaxConcurrency),
			chainConfig.StartBlockOverride,
			controller.SubscribeMinWatchInterval,
		)
		if newClientErr != nil {
			return nil, exitcode.NeverRetry, errors.Wrapf(newClientErr, "build fuel client failed")
		}
		handlerCtrl = fuel.NewHandlerController(c.processor, c.initResult, chainConfig, fuelCli, c.processorClients)
		cli = fuelCli
	case chains.IsSolanaChain(chainID):
		solCli, newClientErr := soldata.NewClient(
			ctx,
			chainConfig.Endpoint,
			int(controller.ClientMaxConcurrency),
			chainConfig.StartBlockOverride,
			controller.SubscribeMinWatchInterval,
			c.processor.DriverVersion,
		)
		if newClientErr != nil {
			// A retryable NewClient error (e.g. the super-node probe kept hitting transient HTTP/timeout
			// errors) means the endpoint may simply be temporarily unavailable; restart the pod and try
			// again instead of failing permanently.
			if data.IsNewClientRetryable(newClientErr) {
				return nil, exitcode.AlwaysRetry, errors.Wrapf(newClientErr, "build sol client failed")
			}
			return nil, exitcode.NeverRetry, errors.Wrapf(newClientErr, "build sol client failed")
		}
		handlerCtrl = sol.NewHandlerController(c.processor, c.initResult, chainConfig, solCli, c.processorClients)
		cli = solCli
	default:
		// chainID is OK but the chainType is not supported, so is a system error
		return nil, exitcode.NeverRetry, errors.Errorf("chain type %s is not supported", chainType)
	}
	// block builder
	blockBuilder := controller.NewBlockBuilder(handlerCtrl, cli, checkLink)
	// webhook controller
	var webhookCtrl controller.WebhookController = controller.EmptyWebhookController{}
	if c.pubSubTopic != nil {
		webhookCtrl = newWebhookController(c.processor, c.pubSubTopic)
	}
	// time series controller
	var timeSeriesCtrl controller.TimeSeriesController = controller.EmptyTimeSeriesController{}
	if c.timeSeriesStore != nil {
		timeSeriesCtrl = newTimeSeriesController(chainID, c.timeSeriesStore)
	}
	// entity controller
	var entityCtrl controller.EntityController = controller.EmptyEntityController{}
	if c.entityStore != nil {
		entityCtrl = newEntityController(
			c.entityStore,
			chainID,
			c.config.EntityStoreCacheSize,
			c.config.EntityStoreFullCacheSize,
			c.config.EntityMetricsMonitor)
	}
	// checkpoint store
	var store controller.CheckpointStore
	if store, err = c.getCheckpointStore(ctx, chainID); err != nil {
		return nil, exitcode.AlwaysRetry, controller.NewExternalError(controller.ErrCodeSaveCheckpointFailed, err)
	}
	// checkpoint controller
	var checkpointCtrl controller.CheckpointController
	checkpointCtrl, err = controller.NewCheckpointController(
		ctx,
		chainID,
		controller.SaveCheckpointDelay,
		controller.SaveCheckpointInterval,
		controller.MaxKeepCheckpointCount,
		store,
		c.getQuotaService(chainID),
		timeSeriesCtrl,
		entityCtrl,
		webhookCtrl,
		c.buildCommitCtx,
	)
	if err != nil {
		return nil, exitcode.AlwaysRetry, errors.Wrapf(err, "build checkpoint controller failed")
	}
	// main controller
	seqMode := c.initResult.GetExecutionConfig().GetSequential()
	ctrl = controller.NewMainController(blockBuilder, checkpointCtrl, seqMode, c.processor, chainID)
	return ctrl, exitcode.AlwaysRetry, nil
}
