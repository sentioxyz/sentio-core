package standard

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

type BindingDataInner struct {
	Data         *protos.Data
	DataSize     int
	HandlerType  protos.HandlerType
	TxIndex      int
	TxInnerIndex int
}

type HandlerAgent[BKD controller.BlockHeader] interface {
	controller.HandlerAgent

	BuildBindingDataList(ctx context.Context, bd BKD) ([]BindingDataInner, error)
}

type streamPool chan protos.ProcessorV3_ProcessBindingsStreamClient

type HandlerConfig struct {
	ContractConfigs []*protos.ContractConfig
	AccountConfigs  []*protos.AccountConfig
}

func (c HandlerConfig) String() string {
	return utils.MustJSONMarshal(c)
}

type BaseHandlerController[CLI controller.Client, BKD controller.BlockHeader, HA HandlerAgent[BKD]] struct {
	Processor   *models.Processor
	InitResult  *protos.InitResponse
	ChainConfig *config.ChainConfig
	Client      CLI

	Config HandlerConfig // result of SetTemplates
	Agents []HA          // built from Config

	metricConfigs   map[timeseries.MetaType]map[string]*protos.MetricConfig // load from InitResult
	webhookChannels map[string]string                                       // load from InitResult

	addressStart     map[string]uint64
	addressStartData string

	processorClients []protos.ProcessorV3Client
	processStreams   streamPool
	waiter           *waiter
}

func NewBaseHandlerController[CLI controller.Client, BKD controller.BlockHeader, HA HandlerAgent[BKD]](
	processor *models.Processor,
	initResult *protos.InitResponse,
	chainConfig *config.ChainConfig,
	client CLI,
	processorClients []protos.ProcessorV3Client,
) *BaseHandlerController[CLI, BKD, HA] {
	webhookChannels := make(map[string]string)
	for _, wc := range initResult.GetExportConfigs() {
		webhookChannels[wc.GetName()] = wc.GetChannel()
	}
	return &BaseHandlerController[CLI, BKD, HA]{
		Processor:        processor,
		InitResult:       initResult,
		ChainConfig:      chainConfig,
		Client:           client,
		metricConfigs:    timeseries.BuildMetricConfigs(initResult.GetMetricConfigs()),
		webhookChannels:  webhookChannels,
		processorClients: processorClients,
	}
}

func (c *BaseHandlerController[CLI, BKD, HA]) SetTemplates(
	ctx context.Context,
	templates map[uint64][]controller.TemplateInstance,
) *controller.ExternalError {
	_, logger := log.FromContext(ctx)
	req := &protos.UpdateTemplatesRequest{
		ChainId:           c.ChainConfig.ChainID,
		TemplateInstances: ConvertTemplateInstanceBack(c.ChainConfig.ChainID, templates),
	}

	var confText string
	for i, cli := range c.processorClients {
		// update templates
		if _, err := cli.UpdateTemplates(ctx, req); err != nil {
			return controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
				errors.Errorf("update templates failed: %v", err))
		}
		// get handler config
		var config HandlerConfig
		if resp, err := cli.GetConfig(ctx, &protos.ProcessConfigRequest{}); err != nil {
			return controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
				errors.Errorf("get handler config failed: %v", err))
		} else {
			config = HandlerConfig{
				ContractConfigs: utils.FilterArr(resp.GetContractConfigs(), func(cc *protos.ContractConfig) bool {
					return cc.GetContract().GetChainId() == c.ChainConfig.ChainID
				}),
				AccountConfigs: utils.FilterArr(resp.GetAccountConfigs(), func(cc *protos.AccountConfig) bool {
					return cc.GetChainId() == c.ChainConfig.ChainID
				}),
			}
		}
		// check handler config
		if i == 0 {
			c.Config, confText = config, config.String()
		} else if another := config.String(); confText != another {
			logger.Errorw("configs from different processor has diff", "config1", confText, "config2", another)
			return controller.NewExternalError(controller.ErrCodeProcessorConfigsHasDiff,
				errors.Errorf("configs from different processor has diff"))
		}
	}

	logger.Infow("got config", "config", confText)
	return nil
}

func (c *BaseHandlerController[CLI, BKD, HA]) PrepareExecute(ctx context.Context) *controller.ExternalError {
	clientCount := uint64(len(c.processorClients))
	streamSize := max(controller.ProcessConcurrency, clientCount)
	c.processStreams = make(streamPool, streamSize)
	var opts = []grpc.CallOption{
		grpc.UseCompressor(utils.Select[string](grpcEnableCompress, gzip.Name, "")),
	}
	for i := uint64(0); i < streamSize; i++ {
		stream, err := c.processorClients[i%clientCount].ProcessBindingsStream(ctx, opts...)
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
				errors.Errorf("open stream for process binding failed: %v", err))
		}
		c.processStreams <- stream
	}
	c.waiter = &waiter{
		ready:  concurrency.NewResourceWaiter[uint64](),
		finish: concurrency.NewResourceWaiter[partitionWithIndex](),
	}
	return nil
}

func (c *BaseHandlerController[CLI, BKD, HA]) DisableAgents(ctx context.Context) {
	if len(strings.TrimSpace(disableAgentTypes)) == 0 {
		return
	}
	_, logger := log.FromContext(ctx)
	das := utils.BuildSet(utils.MapSliceNoError(strings.Split(disableAgentTypes, ","), strings.TrimSpace))
	c.Agents = utils.FilterArr(c.Agents, func(a HA) bool {
		typ := fmt.Sprintf("%T", a)
		if das[typ] {
			logger.Warnw("disable agent", "type", typ, "agent", a.Snapshot())
			return false
		}
		return true
	})
}

func (c *BaseHandlerController[CLI, BKD, HA]) FinishExecute() {
	close(c.processStreams)
	for stream := range c.processStreams {
		_ = stream.CloseSend()
	}
}

func (c *BaseHandlerController[CLI, BKD, HA]) BuildTaskList(
	ctx context.Context,
	d BKD,
) ([]controller.Task, int, error) {
	var result []bindingData
	var totalSize int
	for _, agent := range c.Agents {
		if !agent.GetRange().Contains(d.GetBlockNumber()) {
			continue
		}
		if inners, err := agent.BuildBindingDataList(ctx, d); err != nil {
			return nil, 0, err
		} else {
			for _, inner := range inners {
				result = append(result, bindingData{
					BlockHeader: d,
					handlerID:   agent.GetHandlerID(),
					data: &protos.DataBinding{
						Data:        inner.Data,
						HandlerType: inner.HandlerType,
						HandlerIds:  []int32{agent.GetHandlerID().ID},
						ChainId:     c.ChainConfig.ChainID,
					},
					txIndex:      inner.TxIndex,
					txInnerIndex: inner.TxInnerIndex,
				})
				totalSize += inner.DataSize
			}
		}
	}
	// The purpose of using stable sorting is to ensure that tasks of the same handler remain in the order
	// in which they were generated.
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Cmp(result[j], c.InitResult.ExecutionConfig.GetHandlerOrderInsideTransaction()) < 0
	})
	var r []controller.Task
	for _, bd := range result {
		r = append(r, &task{
			bindingData:     bd,
			sp:              c.processStreams,
			waiter:          c.waiter,
			chainID:         c.ChainConfig.ChainID,
			processor:       c.Processor,
			metricConfigs:   c.metricConfigs,
			webhookChannels: c.webhookChannels,
		})
	}
	return r, totalSize, nil
}

func (c *BaseHandlerController[CLI, BKD, HA]) BuildReportRequirements(
	currentBlockNumber uint64,
) []data.IntervalRequirement {
	endBlock := c.GetBlockRange().EndBlock
	// In the backfill phase, at least one non-empty BlockData is generated for every DAY.
	// In the watching phase, at least one non-empty BlockData is generated for every MINUTE.
	reqs := []data.IntervalRequirement{{
		BlockRange: controller.BlockRange{StartBlock: currentBlockNumber, EndBlock: endBlock},
		IntervalConfig: data.IntervalConfig{TimeInterval: &data.TimeInterval{
			Backfill: time.Hour * 24,
			Watching: time.Minute,
		}},
	}}
	if endBlock != nil {
		reqs = append(reqs, data.IntervalRequirement{
			BlockRange: controller.BlockRange{StartBlock: *endBlock, EndBlock: endBlock},
			IntervalConfig: data.IntervalConfig{BlockInterval: &data.BlockInterval{
				Backfill: 1,
				Watching: 1,
			}},
		})
	}
	return reqs
}

func (c *BaseHandlerController[CLI, BKD, HA]) GetBlockRange() controller.BlockRange {
	return controller.GetHandleAgentsBlockRange(c.Agents)
}

func (c *BaseHandlerController[CLI, BKD, HA]) GetAgentStat() map[string]int {
	stat := make(map[string]int)
	for _, ag := range c.Agents {
		stat[fmt.Sprintf("%T", ag)] += 1
	}
	return stat
}

const checkpointDataKeyAddressStart = "AddressStart"

func (c *BaseHandlerController[CLI, BKD, HA]) LoadAddressStart(checkpoint *controller.Checkpoint) *controller.ExternalError {
	c.addressStart = make(map[string]uint64)
	if checkpoint == nil || checkpoint.Data == nil {
		return nil
	}
	raw, has := checkpoint.Data[checkpointDataKeyAddressStart]
	if !has {
		return nil
	}
	err := json.Unmarshal([]byte(raw), &c.addressStart)
	if err != nil {
		return controller.NewExternalError(controller.ErrCodeInvalidCheckpointData,
			errors.Wrapf(err, "load address start failed"))
	}
	return nil
}

func (c *BaseHandlerController[CLI, BKD, HA]) GetAddressStart(
	address string,
	start uint64,
	loader func() (uint64, error),
) (uint64, error) {
	if c.InitResult.GetExecutionConfig().GetSkipStartBlockValidation() ||
		c.ChainConfig.SkipStartBlockValidation || controller.SkipStartBlockValidation {
		return start, nil
	}
	if address == "" {
		return start, nil
	}
	var has bool
	if start, has = c.addressStart[address]; has {
		return start, nil
	}
	var err error
	start, err = loader()
	if err != nil {
		return 0, errors.Wrapf(err, "get start block for %s failed", address)
	}
	c.addressStart[address] = start
	return start, nil
}

func (c *BaseHandlerController[CLI, BKD, HA]) AddressStartReady() {
	b, _ := json.Marshal(c.addressStart)
	c.addressStartData = string(b)
}

func (c *BaseHandlerController[CLI, BKD, HA]) DumpAddressStart(checkpointData map[string]string) {
	checkpointData[checkpointDataKeyAddressStart] = c.addressStartData
}

func (c *BaseHandlerController[CLI, BKD, HA]) Snapshot() any {
	return map[string]any{
		"initResult":     c.InitResult,
		"chainConfig":    c.ChainConfig,
		"handlerConfig":  c.Config,
		"agents":         utils.MapSliceNoError(c.Agents, HA.Snapshot),
		"agentStat":      c.GetAgentStat(),
		"addressStart":   c.addressStart,
		"processorCount": len(c.processorClients),
		"streamCount":    cap(c.processStreams),
	}
}
