package startup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/ClickHouse/clickhouse-go/v2"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"

	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/tracker"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	chain "sentioxyz/sentio-core/driver/controller/config"
	entitychs "sentioxyz/sentio-core/driver/entity/clickhouse"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/exitcode"
	"sentioxyz/sentio-core/driver/timeseries"
	timeserieschs "sentioxyz/sentio-core/driver/timeseries/clickhouse"
	sentioerror "sentioxyz/sentio-core/service/common/errors"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/common/rpc"
	protosregistry "sentioxyz/sentio-core/service/database_registry/protos"
	"sentioxyz/sentio-core/service/processor/models"
	protossvc "sentioxyz/sentio-core/service/processor/protos"
	protousage "sentioxyz/sentio-core/service/usage/protos"
	protoswebhook "sentioxyz/sentio-core/service/webhook/protos"
)

type baseStartupController struct {
	config Config

	processorClient protossvc.ProcessorServiceClient

	usageClient    protousage.UsageServiceClient
	registryClient protosregistry.DatabaseRegistryServiceClient

	processor    *models.Processor
	chainConfigs map[string]*chain.ConfigV2

	pubSubTopic *pubsub.Topic

	// timeseries / entity clickhouse controllers, built by the injected
	// ClickhouseConnector once the processor is loaded (nil => store disabled).
	tsController     *chx.Controller
	entityController *chx.Controller

	ipfsShell *shell.Shell

	timeSeriesStore timeseries.Store
	entityStore     *entitychs.Store

	release []func()
}

func (c *baseStartupController) releaseAll() {
	for i := len(c.release) - 1; i >= 0; i-- {
		c.release[i]()
	}
}

func (c *baseStartupController) connectToProcessorService(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	conn, err := rpc.DialAuto(c.config.ProcessorService, rpc.RetryDialOption)
	if err != nil {
		return errors.Wrapf(err, "dial to processor service %s failed", c.config.ProcessorService)
	}
	c.release = append(c.release, func() {
		_ = conn.Close()
	})

	c.processorClient = protossvc.NewProcessorServiceClient(conn)
	logger.Infof("connected to processor service %s", c.config.ProcessorService)
	return nil
}

func (c *baseStartupController) connectToUsageService(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	if c.config.UsageService == "" {
		logger.Warnf("no usage service so will not connect to usage service")
		return nil
	}
	conn, err := rpc.DialAuto(c.config.UsageService, rpc.RetryDialOption)
	if err != nil {
		return errors.Wrapf(err, "dial to usage service %s failed", c.config.UsageService)
	}
	c.release = append(c.release, func() {
		_ = conn.Close()
	})
	c.usageClient = protousage.NewUsageServiceClient(conn)
	logger.Infof("connected to usage service %s", c.config.UsageService)
	return nil
}

// connectToDBRegistryService dials the gRPC endpoint that hosts
// DatabaseRegistryService and keeps the client stub on baseStartupController.
// The address is logically independent from the billing/usage service address.
// When empty (cloud deployments), this is a no-op and registration is gated
// away later by newEntityProbe and newTimeSeriesProbe based on the processor's TablePattern.
func (c *baseStartupController) connectToDBRegistryService(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	if c.config.DBRegistryService == "" {
		logger.Warnf("no db registry service configured so will not connect to it")
		return nil
	}
	conn, err := rpc.DialAuto(c.config.DBRegistryService, rpc.RetryDialOption)
	if err != nil {
		return errors.Wrapf(err, "dial to db registry service %s failed", c.config.DBRegistryService)
	}
	c.release = append(c.release, func() {
		_ = conn.Close()
	})
	c.registryClient = protosregistry.NewDatabaseRegistryServiceClient(conn)
	logger.Infof("connected to db registry service %s", c.config.DBRegistryService)
	return nil
}

func (c *baseStartupController) loadChainsConfig(ctx context.Context) (err error) {
	_, logger := log.FromContext(ctx)
	c.chainConfigs, err = chain.LoadChainsConfigV2(
		c.config.ChainConfigFile, chain.PatchChainsConfigEnv, c.processor.NetworkOverrides)
	if err == nil {
		logger.Info("loaded chain config")
	}
	return err
}

func (c *baseStartupController) getProcessor(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	req := &protossvc.GetProcessorRequest{ProcessorId: c.config.ProcessorID}
	response, err := c.processorClient.GetProcessorWithProject(ctx, req)
	if err != nil {
		return err
	}
	var p models.Processor
	if err = p.FromPB(response.Processor); err != nil {
		return err
	}
	p.Project = &commonmodels.Project{}
	p.Project.FromPB(response.Project)
	c.processor = &p
	logger.Infof("got processor from processor service")
	return nil
}

func (c *baseStartupController) createWebhookSubscription(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	if c.config.WebhookService == "" {
		logger.Warn("no webhook service so will not create webhook subscription")
		return nil
	}
	conn, err := rpc.DialAuto(c.config.WebhookService, rpc.RetryDialOption)
	if err != nil {
		return errors.Wrapf(err, "dial to webhook service %s failed", c.config.WebhookService)
	}
	c.release = append(c.release, func() {
		_ = conn.Close()
	})
	logger.Infof("connected to webhook service %s", c.config.WebhookService)

	req := &protoswebhook.CreateSubscriptionRequest{ProcessorId: c.config.ProcessorID}
	if _, err = protoswebhook.NewWebhookServiceClient(conn).CreateSubscription(ctx, req); err != nil {
		return errors.Wrapf(err, "create subscription failed")
	}
	logger.Infof("created webhook subscription")
	return nil
}

func (c *baseStartupController) createPubSubTopic(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	if c.config.WebhookTopic == "" || c.config.PubSubProject == "" {
		logger.Warnf("no webhook topic or pubsub project so will not create pubsub topic")
		return nil
	}
	cli, err := pubsub.NewClient(ctx, c.config.PubSubProject)
	if err != nil {
		return errors.Wrapf(err, "create gcp pubsub client failed")
	}
	c.release = append(c.release, func() {
		_ = cli.Close()
	})
	c.pubSubTopic = cli.Topic(c.config.WebhookTopic)
	logger.Infof("created pubsub topic %s in gcp", c.config.WebhookTopic)
	return nil
}

// connectClickhouse asks the injected ClickhouseConnector to build the
// timeseries and entity chx.Controller for the loaded processor. All sharding /
// housegate wiring lives in the connector implementation in the driver binary.
func (c *baseStartupController) connectClickhouse(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	if c.config.ClickhouseConnector == nil {
		logger.Warnf("no clickhouse connector so will not connect to clickhouse")
		return nil
	}
	ts, entity, err := c.config.ClickhouseConnector.Connect(ctx, c.processor)
	if err != nil {
		return errors.Wrap(err, "connect to clickhouse failed")
	}
	c.tsController = ts
	c.entityController = entity
	return nil
}

func (c *baseStartupController) newTimeSeriesProbe() (timeserieschs.Probe, error) {
	if c.processor.TablePattern != models.TablePatternNetworkV1 {
		return nil, nil
	}
	if c.registryClient == nil {
		return nil, errors.Errorf(
			"processor TablePattern=%s requires onchain database registration, "+
				"but no db registry service is configured (set -db-registry-service flag)",
			c.processor.TablePattern,
		)
	}
	return &timeSeriesProbe{
		client:           c.registryClient,
		processorID:      c.processor.ID,
		processorReplica: c.config.ProcessorReplica,
	}, nil
}

func (c *baseStartupController) buildTimeSeriesStore(ctx context.Context) error {
	// build time series clickhouse store, each chain will use it to build time series controller
	_, logger := log.FromContext(ctx)

	if c.tsController == nil {
		logger.Warnf("no clickhouse connection so will not create time series store")
		return nil
	}
	ctrl := *c.tsController

	probe, err := c.newTimeSeriesProbe()
	if err != nil {
		return err
	}

	c.timeSeriesStore = timeserieschs.NewStore(ctrl, timeserieschs.Option{}, probe)
	if err := c.timeSeriesStore.Init(ctx); err != nil {
		return errors.Wrapf(err, "init time series store failed")
	}
	logger.Info("time series store is ready")
	return nil
}

func (c *baseStartupController) newEntityProbe() (entitychs.Probe, error) {
	if c.processor.TablePattern != models.TablePatternNetworkV1 {
		return nil, nil
	}
	if c.registryClient == nil {
		return nil, errors.Errorf(
			"processor TablePattern=%s requires onchain database registration, "+
				"but no db registry service is configured (set -db-registry-service flag)",
			c.processor.TablePattern,
		)
	}
	return &entityProbe{
		client:           c.registryClient,
		processorID:      c.processor.ID,
		processorReplica: c.config.ProcessorReplica,
	}, nil
}

func (c *baseStartupController) buildEntityStore(ctx context.Context, schemaText string) *controller.ExternalError {
	_, logger := log.FromContext(ctx)

	if c.entityController == nil {
		return controller.NewExternalError(controller.ErrCodeSystem,
			errors.Errorf("need clickhouse connection but no clickhouse connector configured"))
	}
	ctrl := *c.entityController

	entityFea := entitychs.BuildFeatures(c.processor.EntitySchemaVersion)
	entitySchema, buildErr := schema.ParseAndVerifySchema(schemaText, entityFea.BuildVerifyOptions()...)
	if buildErr != nil {
		return controller.NewExternalError(controller.ErrCodeInvalidEntitySchema, buildErr)
	}

	probe, err := c.newEntityProbe()
	if err != nil {
		return controller.NewExternalError(controller.ErrCodeSystem, err)
	}

	c.entityStore = entitychs.NewStore(ctrl, entityFea, entitySchema, entitychs.DefaultCreateTableOption, probe)
	if initErr := c.entityStore.InitEntitySchema(ctx); initErr != nil {
		return controller.NewExternalError(controller.ErrCodeInitEntityFailed, initErr)
	}
	logger.Infow("entity store is ready", "schema", schemaText, "feature", entityFea)
	return nil
}

func (c *baseStartupController) buildIpfsShell(ctx context.Context) {
	if c.config.IpfsNodeAddr == "" {
		return
	}
	_, logger := log.FromContext(ctx)
	c.ipfsShell = shell.NewShell(c.config.IpfsNodeAddr)
	c.ipfsShell.SetTimeout(5 * time.Second)
	logger.Infow("ipfs shell is ready")
}

func (c *baseStartupController) getQuotaService(chainID string) controller.QuotaService {
	if c.usageClient == nil {
		return controller.EmptyQuotaService{}
	}
	return newQuotaService(chainID, c.processor, c.usageClient)
}

func (c *baseStartupController) buildCommitCtx(ctx context.Context, chainID string, cur controller.Checkpoint) context.Context {
	sign, _ := json.Marshal(map[string]any{
		"processor_id":         c.config.ProcessorID,
		"processor_replica":    c.config.ProcessorReplica,
		"chain_id":             chainID,
		"watching":             cur.InWatching(),
		"current_block_number": strconv.FormatUint(cur.BlockNumber, 10),
		"current_block_time":   cur.BlockTime.Format(time.RFC3339Nano),
	})
	return ckhmanager.ContextMergeSettings(ctx, clickhouse.Settings{"log_comment": string(sign)})
}

func newErrorRecord(err error) (er sentioerror.ErrorRecord) {
	er.ID, _ = gonanoid.GenerateLongID()
	er.CreatedAt = time.Now()
	er.Namespace = sentioerror.DRIVER
	er.Message = err.Error()
	er.Code = controller.ErrCodeSystem
	var extErr *controller.ExternalError
	if errors.As(err, &extErr) {
		er.Code = int32(extErr.Code())
		if extErr.IsUserError() {
			er.Namespace = sentioerror.PROCESSOR
		}
	}
	return
}

func updateChainState(ctx context.Context, cli protossvc.ProcessorServiceClient, cs models.ChainState) error {
	var name string
	if cs.ChainID == "meta" {
		name = "meta chain state"
	} else {
		name = fmt.Sprintf("chain state of chain %s", cs.ChainID)
	}
	csPb, err := cs.ToPB()
	if err != nil {
		return errors.Wrapf(err, "convert %s to pb failed", name)
	}
	req := protossvc.UpdateChainProcessorStatusRequest{
		Id:         cs.ProcessorID,
		ChainState: csPb,
	}
	// ChainState may be very big, so need to use compressor
	if _, err = cli.UpdateChainProcessorStatus(ctx, &req, grpc.UseCompressor(gzip.Name)); err != nil {
		_, logger := log.FromContext(ctx,
			"IndexerState", utils.StringSummaryV2(string(cs.IndexerState)),
			"MeterState", utils.StringSummaryV2(string(cs.MeterState)),
			"HandlerStat", utils.StringSummaryV2(string(cs.HandlerStat)),
			"Templates", utils.StringSummaryV2(cs.Templates))
		logger.Warnfe(err, "update %s failed", name)
		return errors.Wrapf(err, "update %s failed", name)
	}
	return nil
}

func (c *baseStartupController) findChainState(chainID string) (*models.ChainState, bool) {
	for _, chainState := range c.processor.ChainStates {
		if chainState.ChainID == chainID {
			return chainState, true
		}
	}
	return &models.ChainState{
		ID:                   fmt.Sprintf("%s_%s", c.config.ProcessorID, chainID),
		ChainID:              chainID,
		ProcessorID:          c.config.ProcessorID,
		ProcessedBlockNumber: -1,
		ProcessedVersion:     c.processor.Version,
		State:                int32(protossvc.ChainState_Status_CATCHING_UP),
	}, false
}

func (c *baseStartupController) getCheckpointStore(
	ctx context.Context,
	chainID string,
) (controller.CheckpointStore, error) {
	cs, has := c.findChainState(chainID)
	store := newCheckpointStore(c.processor, chainID, c.processorClient, cs)
	if has {
		return store, nil
	}
	// no chain state for the chain, create the initial one
	if err := store.Save(ctx, nil, nil, nil); err != nil {
		return nil, errors.Wrapf(err, "save initial chain state failed")
	}
	return store, nil
}

func (c *baseStartupController) updateMetaState(ctx context.Context, metaErr error) error {
	cs := models.ChainState{
		ID:                   fmt.Sprintf("%s_meta", c.processor.ID),
		ChainID:              "meta",
		ProcessorID:          c.processor.ID,
		ProcessedBlockNumber: -1,
		ProcessedVersion:     -1,
		State:                int32(protossvc.ChainState_Status_PROCESSING_LATEST),
	}
	if metaErr != nil {
		cs.State = int32(protossvc.ChainState_Status_ERROR)
		cs.ErrorRecord = newErrorRecord(metaErr)
	}
	return updateChainState(ctx, c.processorClient, cs)
}

const (
	initRetryInterval = time.Second * 5
)

func main(ctx context.Context, config Config) (exitCode exitcode.Code, err error) {
	_, logger := log.FromContext(ctx)
	logger.Infow("startup now", "startupConfig", config)

	base := baseStartupController{config: config}
	defer base.releaseAll()

	// 1. connect to processor service
	if err = base.connectToProcessorService(ctx); err != nil {
		logger.Errorfe(err, "startup failed")
		return 0, nil
	}

	// 2. get processor from processor service
	if err = base.getProcessor(ctx); err != nil {
		return 0, errors.Wrapf(err, "get processor failed")
	}

	// check if using all streaming mode
	if base.processor.DriverVersion < 1 {
		logger.Warnf("driver version is %d < 1, will not use all stream mode", base.processor.DriverVersion)
		return -1, nil
	}

	// record meta error
	defer func() {
		if err != nil {
			logger.Errorf("startup failed: %+v", err)
			if updateErr := base.updateMetaState(ctx, err); updateErr != nil {
				logger.Errorfe(updateErr, "update meta chain state failed")
			}
		}
	}()

	// 3. connect to usage service
	if err = base.connectToUsageService(ctx); err != nil {
		return 0, err
	}

	// 3b. connect to db registry service (optional, only required for
	// network_v1 processors). The check that the client stub exists when
	// it is actually needed happens in newEntityProbe and newTimeSeriesProbe.
	if err = base.connectToDBRegistryService(ctx); err != nil {
		return 0, err
	}

	// 4. load chain configs
	if err = base.loadChainsConfig(ctx); err != nil {
		return exitcode.AlwaysRetry, errors.Wrapf(err, "load chains config failed")
	}

	// build main controllers
	ctrls := make(map[string]*controller.MainController)
	switch base.processor.Project.Type {
	case commonmodels.ProjectTypeSentio:
		std := standardStartupController{baseStartupController: base}
		if ctrls, exitCode, err = std.buildMainControllers(ctx); err != nil {
			return exitCode, err
		}
	case commonmodels.ProjectTypeSubgraph:
		ss := subgraphStartupController{baseStartupController: base}
		if ctrls, exitCode, err = ss.buildMainControllers(ctx); err != nil {
			return exitCode, err
		}
	default:
		return exitcode.NeverRetry, errors.Errorf("project type %s is not supported", base.processor.Project.Type)
	}

	// update meta chain state
	if updateErr := base.updateMetaState(ctx, nil); updateErr != nil {
		logger.Errorfe(updateErr, "update meta chain state failed")
		return 0, nil
	}
	logger.Info("startup succeed")

	// start all main controllers
	g, gctx := errgroup.WithContext(ctx)
	for chainID_, ctrl_ := range ctrls {
		chainID, ctrl := chainID_, ctrl_
		tracker.AddOrReplaceTrackedObject("MainController::"+chainID, ctrl)
		g.Go(func() error {
			mainCtx, _ := log.FromContext(gctx, "chain_id", chainID)
			return ctrl.Main(mainCtx)
		})
	}
	if err = g.Wait(); err != nil {
		var extErr *controller.ExternalError
		if errors.As(err, &extErr) {
			switch {
			case extErr.IsBillingError():
				return exitcode.OverQuota, extErr
			case extErr.IsUserError():
				return exitcode.NeverRetry, extErr
			}
		}
		return exitcode.AlwaysRetry, err
	}
	logger.Info("all chains are done")
	<-ctx.Done()
	return exitcode.AlwaysRetry, nil
}

// ClickhouseConnector builds the timeseries and entity chx.Controller used by a
// processor's stores. Its implementation lives in the driver binary and
// encapsulates the sharding / housegate wiring, so the controller depends only
// on chx, which is in sentio-core.
//
// A nil returned controller means the corresponding store is not available
// (e.g. no clickhouse configured).
type ClickhouseConnector interface {
	Connect(ctx context.Context, processor *models.Processor) (timeseries, entity *chx.Controller, err error)
}

type Config struct {
	ProcessorID              string
	ProcessorReplica         int
	ProcessorService         string
	UsageService             string
	DBRegistryService        string
	WebhookService           string
	WebhookTopic             string
	ProcessorUrl             string
	ChainConfigFile          string
	IpfsNodeAddr             string
	EntityStoreCacheSize     int
	EntityStoreFullCacheSize int
	SubgraphTotalMemSize     uint
	SubgraphDebugTrace       bool
	// PubSubProject is the GCP project used to create the webhook pubsub topic;
	// empty disables pubsub topic creation. Provided by the driver binary.
	PubSubProject string

	// ClickhouseConnector builds the timeseries/entity chx.Controller for a
	// processor. Its implementation lives in the driver binary.
	ClickhouseConnector ClickhouseConnector
	// EntityMetricsMonitor is the metric monitor for the entity store; it carries
	// the otel instrument and is built by the driver binary so the controller
	// does not import the binary's metrics package.
	EntityMetricsMonitor persistent.MetricsMonitor
}

func Main(config Config) {
	const retryInterval = time.Second * 30
	ctx, logger := log.FromContext(concurrency.NewSignalContext(context.Background()), "processorID", config.ProcessorID)
	for {
		exitCode, _ := main(ctx, config)
		if exitCode < 0 {
			logger.Warnf("do not use all streaming mode")
			return
		}
		if exitCode > 0 {
			logger.Infof("EXIT WITH CODE %d", exitCode)
			os.Exit(int(exitCode))
		}

		logger.Infof("startup failed, will retry after %s", retryInterval.String())
		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}

type timeSeriesProbe struct {
	client           protosregistry.DatabaseRegistryServiceClient
	processorID      string
	processorReplica int
}

func (p *timeSeriesProbe) PreCreateTable(ctx context.Context, tv chx.TableOrView) error {
	metaType, _, err := timeseries.CutTableName(tv.GetName())
	if err != nil {
		return err
	}
	_, err = p.client.EnsureTable(ctx, &protosregistry.EnsureTableRequest{
		ProcessorId:  p.processorID,
		ReplicaIndex: uint32(p.processorReplica),
		TableId:      tv.GetName(),
		TableType:    string(metaType),
	})
	return err
}

type entityProbe struct {
	client           protosregistry.DatabaseRegistryServiceClient
	processorID      string
	processorReplica int
}

func (p *entityProbe) PreCreateTable(ctx context.Context, tv chx.TableOrView) error {
	if _, ok := tv.(chx.Table); !ok {
		return nil
	}
	_, err := p.client.EnsureTable(ctx, &protosregistry.EnsureTableRequest{
		ProcessorId:  p.processorID,
		ReplicaIndex: uint32(p.processorReplica),
		TableId:      tv.GetName(),
		TableType:    "entity",
	})
	return err
}
