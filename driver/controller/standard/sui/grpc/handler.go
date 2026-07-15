package grpc

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	suigrpcdata "sentioxyz/sentio-core/driver/controller/data/sui/grpc"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
)

type GrpcHandlerAgent interface {
	standard.HandlerAgent[*BlockData]
}

type HandlerController struct {
	*standard.BaseHandlerController[suidata.Client, *BlockData, GrpcHandlerAgent]

	objMgr suihandler.ObjectDictSetManager
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *config.ChainConfig,
	client suidata.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[suidata.Client, *BlockData, GrpcHandlerAgent](
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
	if err := c.objMgr.Load(checkpoint); err != nil {
		return controller.NewExternalError(controller.ErrCodeInvalidCheckpointData,
			errors.Wrapf(err, "parse object set from checkpoint data failed"))
	}
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

func (c *HandlerController) getAddressStart(ctx context.Context, address string, start uint64) (uint64, error) {
	return c.GetAddressStart(
		address,
		start,
		func() (uint64, error) {
			newStart, has, getErr := c.Client.GetObjectCreation(ctx, address, start)
			if getErr != nil {
				return 0, getErr
			}
			if has {
				return newStart, nil
			}
			return start, nil
		})
}

func (c *HandlerController) buildAgents(ctx context.Context, first, _ uint64) *controller.ExternalError {
	_, logger := log.FromContext(ctx)
	c.Agents = nil
	extErr := suihandler.BuildSuiAgents(
		ctx, c.Config, c.ChainConfig, c.Client, first, c.getAddressStart,
		c.Client.GetGrpcPackageHistory,
		grpcFilterConvention{},
		func(agent suihandler.SuiHandlerAgent) {
			c.Agents = append(c.Agents, wrapAgent(agent))
		},
	)
	if extErr != nil {
		return extErr
	}
	logger.Infof("built %d grpc agents", len(c.Agents))
	return nil
}

// grpcFilterConvention builds filters in the grpc enum-name conventions the
// grpc data interfaces expect (see the super node's FilterGrpcChangedObjects /
// GetGrpcTransactions contracts).
type grpcFilterConvention struct{}

func (grpcFilterConvention) OwnerType(t protos.MoveOwnerType) string {
	if t == protos.MoveOwnerType_ADDRESS {
		return rpcv2.Owner_ADDRESS.String()
	}
	return rpcv2.Owner_OBJECT.String() // OBJECT / WRAPPED_OBJECT
}

func (grpcFilterConvention) ProgrammableTxKind() string {
	return rpcv2.TransactionKind_PROGRAMMABLE_TRANSACTION.String()
}

// wrapAgent wraps a sui agent (built by the shared BuildSuiAgents with the grpc filter convention)
// into its grpc twin, which reuses the embedded agent's filters but reads grpc data when building
// bindings.
func wrapAgent(agent suihandler.SuiHandlerAgent) GrpcHandlerAgent {
	switch ag := agent.(type) {
	case suihandler.HandlerAgentEvent:
		return HandlerAgentEvent{HandlerAgentEvent: ag}
	case suihandler.HandlerAgentFunction:
		return HandlerAgentFunction{HandlerAgentFunction: ag}
	case suihandler.HandlerAgentChange:
		return HandlerAgentChange{HandlerAgentChange: ag}
	case suihandler.HandlerAgentInterval:
		return HandlerAgentInterval{HandlerAgentInterval: ag}
	default:
		panic(errors.Errorf("unknown sui handler agent type %T", agent))
	}
}

func (c *HandlerController) BuildBlockDataFetcher(
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
) controller.Fetcher[controller.BlockData] {
	req := c.getDataRequirement()
	req.Interval = append(req.Interval, c.BuildReportRequirements(currentBlockNumber)...)
	fetchNamePrefix := fmt.Sprintf("SUI::%s::grpc::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		suigrpcdata.BuildBlockMainDataFetcher(
			fetchNamePrefix,
			req,
			firstBlockNumber,
			currentBlockNumber,
			latest,
			c.Client,
		),
		latest,
		1,
		256*1024*1024,
		1000,
		0,
		20,
		time.Second*10,
		func(ctx context.Context, blockNumber uint64, from suigrpcdata.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			var err error
			bd := BlockData{mainData: from, checkpointData: make(map[string]string)}
			if from.SimpleBlock != nil {
				bd.BlockHeader = *from.SimpleBlock
			} else {
				bd.BlockHeader, err = c.Client.GetSimpleBlock(ctx, blockNumber)
				if err != nil {
					return nil, false, err
				}
			}
			if err = c.pushIntervalAgent(ctx, blockNumber, &bd); err != nil {
				return nil, false, err
			}
			if bd.taskList, bd.taskTotalSize, err = c.BuildTaskList(ctx, &bd); err != nil {
				return nil, false, err
			}
			c.DumpAddressStart(bd.checkpointData)
			return &bd, true, nil
		},
	)
}

func (c *HandlerController) pushIntervalAgent(ctx context.Context, blockNumber uint64, blockData *BlockData) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, agent := range c.Agents {
		ag, is := agent.(HandlerAgentInterval)
		if !is || !data.ContainsInterval(blockData.mainData.Intervals, ag.IntervalConfig) {
			continue
		}
		key := ag.ObjMgrKey()
		g.Go(func() error {
			newDict, err := ag.PushObjectLatestVersion(gctx, blockNumber, c.objMgr.Get(key))
			if err != nil {
				return err
			}
			c.objMgr.Put(key, newDict)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	blockData.objMgr = &c.objMgr
	blockData.checkpointData[suihandler.CheckpointDataKey] = c.objMgr.GetData()
	return nil
}

func (c *HandlerController) getDataRequirement() (dr suidata.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentFunction:
			dr.Txn = append(dr.Txn, suidata.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentEvent:
			dr.Txn = append(dr.Txn, suidata.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentChange:
			dr.ObjectChanges = append(dr.ObjectChanges, suidata.ObjectChangeRequirement{
				BlockRange: ag.Range,
				Filter:     ag.Filter,
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

func (c *HandlerController) Snapshot() any {
	sp := c.BaseHandlerController.Snapshot().(map[string]any)
	sp["objectManager"] = c.objMgr.Snapshot()
	return sp
}
