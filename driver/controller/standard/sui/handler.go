package sui

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
)

type SuiHandlerAgent interface {
	standard.HandlerAgent[*BlockData]
}

type HandlerController struct {
	*standard.BaseHandlerController[sui.Client, *BlockData, SuiHandlerAgent]

	objMgr ObjectDictSetManager
}

func NewHandlerController(
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *config.ChainConfig,
	client sui.Client,
	processorClients []protos.ProcessorV3Client,
) *HandlerController {
	return &HandlerController{
		BaseHandlerController: standard.NewBaseHandlerController[sui.Client, *BlockData, SuiHandlerAgent](
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
			// if has is false, object with id `address` may not exist, in this situation keep the user's StartBlock
			return start, nil
		})
}

func (c *HandlerController) buildAgents(ctx context.Context, first, _ uint64) *controller.ExternalError {
	_, logger := log.FromContext(ctx)
	c.Agents = nil
	extErr := BuildSuiAgents(
		ctx, c.Config, c.ChainConfig, c.Client, first, c.getAddressStart,
		c.Client.GetPackageHistory,
		JSONRPCFilterConvention,
		func(agent SuiHandlerAgent) {
			c.Agents = append(c.Agents, agent)
		},
	)
	if extErr != nil {
		return extErr
	}
	logger.Infof("built %d agents", len(c.Agents))
	return nil
}

func (c *HandlerController) BuildBlockDataFetcher(
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
) controller.Fetcher[controller.BlockData] {
	req := c.getDataRequirement()
	req.Interval = append(req.Interval, c.BuildReportRequirements(currentBlockNumber)...)
	fetchNamePrefix := fmt.Sprintf("SUI::%s::", c.ChainConfig.ChainID)
	return fetcher.TransferFetcher(
		fetchNamePrefix+"BlockDataFetcher",
		sui.BuildBlockMainDataFetcher(
			fetchNamePrefix,
			req,
			firstBlockNumber,
			currentBlockNumber,
			latest,
			c.Client,
		),
		latest,
		1, // The transfer process must be performed strictly in block order，so it cannot be performed concurrently.
		256*1024*1024,
		1000,
		0, // The push process may take a long time, so no timeout is set.
		20,
		time.Second*10,
		func(ctx context.Context, blockNumber uint64, from sui.BlockMainData) (controller.BlockData, bool, error) {
			if from.IsEmpty() {
				return nil, false, nil
			}
			var err error
			bd := BlockData{mainData: from, checkpointData: make(map[string]string)}
			// Always need the header. Prefer the one the data fetchers prefetched concurrently (off this
			// strictly block-ordered path); only fall back to a serial RPC if it's missing, which keeps
			// the order-independent sui_getSimpleCheckpoint out of the throughput-limiting path.
			if from.SimpleBlock != nil {
				bd.BlockHeader = *from.SimpleBlock
			} else {
				bd.BlockHeader, err = c.Client.GetSimpleBlock(ctx, blockNumber)
				if err != nil {
					return nil, false, err
				}
			}
			// interval agent should push object latest version
			if err = c.pushIntervalAgent(ctx, blockNumber, &bd); err != nil {
				return nil, false, err
			}
			// build binding data
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
			if newDict, err := ag.PushObjectLatestVersion(gctx, blockNumber, c.objMgr.Get(key)); err != nil {
				return err
			} else {
				c.objMgr.Put(key, newDict)
				return nil
			}
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	blockData.objMgr = &c.objMgr
	blockData.checkpointData[CheckpointDataKey] = c.objMgr.GetData()
	return nil
}

func (c *HandlerController) getDataRequirement() (dr sui.DataRequirement) {
	for _, agent := range c.Agents {
		switch ag := agent.(type) {
		case HandlerAgentFunction:
			dr.Txn = append(dr.Txn, sui.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentEvent:
			dr.Txn = append(dr.Txn, sui.TransactionRequirement{
				BlockRange:  ag.Range,
				Filter:      ag.Filter,
				FetchConfig: ag.FetchConfig,
			})
		case HandlerAgentChange:
			dr.ObjectChanges = append(dr.ObjectChanges, sui.ObjectChangeRequirement{
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
