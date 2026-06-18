package startup

import (
	"context"
	"time"

	evmchain "sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/subgraph"
	"sentioxyz/sentio-core/driver/exitcode"
	"sentioxyz/sentio-core/driver/subgraph/manifest"

	"github.com/pkg/errors"
)

type subgraphStartupController struct {
	baseStartupController
}

func (c *subgraphStartupController) buildMainControllers(ctx context.Context) (
	ctrls map[string]*controller.MainController,
	exitCode exitcode.Code,
	err error,
) {
	ctrls = make(map[string]*controller.MainController)
	// connect to clickhouse
	if err = c.connectClickhouse(ctx); err != nil {
		return ctrls, 0, errors.Wrapf(err, "connect to clickhouse failed")
	}
	// build ipfs client
	c.buildIpfsShell(ctx)
	// manifest
	var mf *manifest.Manifest
	if mf, err = manifest.LoadFromIpfs(c.ipfsShell, c.processor.IpfsHash, true); err != nil {
		if errors.Is(err, manifest.ErrInvalidManifest) || errors.Is(err, manifest.ErrInvalidCustomizedEndpoint) {
			return nil, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest, err)
		}
		return nil, exitcode.AlwaysRetry, errors.Wrapf(err, "load subgraph manifest failed")
	}
	// chainID and chainConfig and client
	chainID, endpoint, _ := manifest.GetChainID(mf.GetNetwork(), false)
	var client evm.Client
	var chainConfig *config.ChainConfig
	if chainID == manifest.CustomizedChainID {
		chainConfig = config.NewCustomizedChainConfig(manifest.CustomizedChainID, endpoint)
	} else if chains.IsEVMChains(chainID) {
		var has bool
		chainConfig, has = c.chainConfigs[chainID]
		if !has {
			return nil, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
				errors.Errorf("unsupported evm chain id %q for the network %q", chainID, mf.GetNetwork()))
		}
		if chainConfig.IsCustomizedEndpoint {
			// chainConfig.Endpoint is not in the manifest file, it is in the NetworkOverrides,
			// so need to check archive node here
			if err = evmchain.CheckArchiveNode(ctx, chainConfig.Endpoint); err != nil {
				return nil, exitcode.AlwaysRetry, errors.Wrapf(err,
					"invalid customized endpoint %q for chain %s", chainConfig.Endpoint, chainID)
			}
		}
	} else {
		return nil, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeInvalidSubgraphManifest,
			errors.Errorf("unknown network %q in the manifest", mf.GetNetwork()))
	}
	client, err = evm.NewClient(
		ctx,
		chainConfig.Endpoint,
		int(controller.ClientMaxConcurrency),
		chainConfig.StartBlockOverride,
		chainConfig.ProcessingDelayBlocks,
		controller.SubscribeMinWatchInterval,
		time.Second*3,
	)
	if err != nil {
		return nil, exitcode.NeverRetry, errors.Wrapf(err, "build evm client failed")
	}
	// handler controller
	var handlerCtrl *subgraph.HandlerController
	handlerCtrl, err = subgraph.NewHandlerController(
		ctx,
		c.processor,
		chainConfig,
		client,
		c.ipfsShell,
		mf,
		uint32(c.config.SubgraphTotalMemSize),
		c.config.SubgraphDebugTrace,
	)
	if err != nil {
		return nil, exitcode.NeverRetry, controller.NewExternalError(controller.ErrCodeWasmInitFailed, err)
	}
	// block builder
	blockBuilder := controller.NewBlockBuilder(handlerCtrl, client, true)
	// c.entityStore
	if extErr := c.buildEntityStore(ctx, mf.Schema.File.GetContent()); extErr != nil {
		if extErr.IsUserError() {
			return nil, exitcode.NeverRetry, extErr
		}
		return nil, exitcode.AlwaysRetry, extErr
	}
	// entity controller
	entityCtrl := newEntityController(c.entityStore, chainID, c.config.EntityStoreCacheSize,
		c.config.EntityStoreFullCacheSize, c.config.EntityMetricsMonitor)
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
		controller.EmptyTimeSeriesController{},
		entityCtrl,
		controller.EmptyWebhookController{},
		c.buildCommitCtx,
	)
	if err != nil {
		return nil, exitcode.AlwaysRetry, errors.Wrapf(err, "build checkpoint controller failed")
	}
	// main controller
	ctrls[chainID] = controller.NewMainController(blockBuilder, checkpointCtrl, false, c.processor, chainID)
	return ctrls, exitcode.AlwaysRetry, nil
}
