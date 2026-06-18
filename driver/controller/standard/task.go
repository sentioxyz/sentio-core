package standard

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type partitionWithIndex struct {
	partition string
	index     uint64
}
type waiter struct {
	ready  *concurrency.ResourceWaiter[uint64]
	finish *concurrency.ResourceWaiter[partitionWithIndex]
}

type task struct {
	bindingData

	sp     streamPool
	waiter *waiter

	metricConfigs   map[timeseries.MetaType]map[string]*protos.MetricConfig
	webhookChannels map[string]string
	chainID         string
	processor       *models.Processor

	stream    protos.ProcessorV3_ProcessBindingsStreamClient
	index     controller.TaskIndex
	partition string

	timer *timer.Timer

	logger *log.SentioLogger
}

func (b *task) errLogger() *log.SentioLogger {
	return b.logger.With("binding", b.data.String())
}

func (b *task) Init(ctx context.Context, index controller.TaskIndex, progressbar controller.ProgressBar) {
	b.index = index
	if enableBindingDataPartition {
		b.waiter.ready.NewResource(b.index.Global)
	} else {
		b.waiter.finish.NewResource(partitionWithIndex{index: b.index.Global})
	}
	b.timer = timer.NewTimer()
	_, b.logger = log.FromContext(ctx,
		"block", controller.GetBlockSummary(b),
		"latest", controller.GetBlockSummary(progressbar.LatestBlock),
		"index", index,
		"handler", b.handlerID.String())
}

func (b *task) GetHandlerID() controller.HandlerID {
	return b.handlerID
}

func (b *task) title() string {
	return fmt.Sprintf("#%d binding data %d/%d for handler %s in block %s",
		b.index.Global, b.index.InBlock, b.index.TotalInBlock, b.handlerID, controller.GetBlockSummary(b))
}

func (b *task) Summary() string {
	return b.title()
}

func (b *task) Exec(
	ctx context.Context,
	checkpointCtrl controller.CheckpointController,
) (extErr *controller.ExternalError) {
	b.logger.Debug("task started")
	start := b.timer.Start("ALL")
	hmi := controller.TaskInfo{
		Processor:  b.processor,
		ChainID:    b.chainID,
		Handler:    b.handlerID.Name,
		Category:   b.handlerID.Type,
		DataSource: b.handlerID.DataSource,
	}
	var stat statistic
	ctx = controller.N.BeforeEntityOperation(ctx, hmi) // to pass metric attrs to noticeController of entityController
	defer func() {
		used := start.End()
		usedReport := b.timer.ReportDistribution("ALL", "*")
		b.logger = b.logger.With("used", usedReport, "stat", stat)
		if extErr == nil {
			b.logger.Debugw("task succeed")
			b.waiter.finish.ResourceReady(partitionWithIndex{partition: b.partition, index: b.index.Global})
		} else if errors.Is(extErr.Wrapped(), context.Canceled) {
			b.logger.Warnfe(extErr, "task canceled")
		} else {
			b.errLogger().Errore(extErr, "task failed")
		}
		controller.N.TaskDone(ctx, hmi, extErr == nil, used)
		for eventName, count := range stat.TimeSeries[timeseries.MetaTypeEvent] {
			controller.N.DataEmitted(ctx, hmi, "event", "", eventName, int64(count))
		}
		for counterName, count := range stat.TimeSeries[timeseries.MetaTypeCounter] {
			controller.N.DataEmitted(ctx, hmi, "metric", "counter", counterName, int64(count))
		}
		for gaugeName, count := range stat.TimeSeries[timeseries.MetaTypeGauge] {
			controller.N.DataEmitted(ctx, hmi, "metric", "gauge", gaugeName, int64(count))
		}
		for subtype, st := range stat.Entity {
			for entityName, count := range st {
				controller.N.DataEmitted(ctx, hmi, "entity", subtype, entityName, int64(count))
			}
		}
	}()
	if extErr = b.getStream(ctx); extErr != nil {
		return extErr
	}
	defer b.returnStream()
	if extErr = b.sendBindingData(ctx); extErr != nil {
		return extErr
	}
	if enableBindingDataPartition {
		if extErr = b.recvPartition(ctx); extErr != nil {
			return extErr
		}
		b.waiter.finish.NewResource(partitionWithIndex{partition: b.partition, index: b.index.Global})
		b.waiter.ready.ResourceReady(b.index.Global)
		err := b.waiter.ready.Wait(ctx, func(u uint64) bool {
			return u < b.index.Global
		})
		if err != nil {
			return controller.NewExternalError(controller.ErrCodeSystem,
				errors.Errorf("waiting all previous tasks got partition failed: %v", err))
		}
		if extErr = b.sendStartCommand(ctx); extErr != nil {
			return extErr
		}
	}
	stat, extErr = b.waitProcess(ctx, checkpointCtrl)
	return extErr
}

func (b *task) getStream(ctx context.Context) *controller.ExternalError {
	select {
	case b.stream = <-b.sp:
		return nil
	case <-ctx.Done():
		return controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
			errors.Errorf("get stream for %s failed: %v", b.title(), ctx.Err()))
	}
}

func (b *task) returnStream() {
	if b.stream != nil {
		b.sp <- b.stream
	}
}

func (b *task) streamSend(
	ctx context.Context,
	req *protos.ProcessStreamRequest,
	what, mark string,
	timeout time.Duration,
) *controller.ExternalError {
	start := b.timer.Start(mark)
	err := timer.Wait(ctx, timeout, time.Minute, func() error {
		return b.stream.Send(req)
	}, func(used time.Duration) {
		b.logger.Warnf("stream send %s already waited %s", what, used.String())
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			b.logger.Warnfe(err, "stream send %s canceled", what)
		} else {
			b.errLogger().Errorfe(err, "stream send %s failed", what)
		}
		return controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
			errors.Wrapf(err, "send %s for %s failed", what, b.title()))
	}
	used := start.End()
	b.logger.With("used", used.String()).Debugf("stream sent %s", what)
	return nil
}

func (b *task) streamReceive(
	ctx context.Context,
	mark string,
	timeout time.Duration,
) (*protos.ProcessStreamResponseV3, *controller.ExternalError) {
	defer b.timer.Start(mark).End()
	var resp *protos.ProcessStreamResponseV3
	err := timer.Wait(ctx, timeout, time.Minute, func() (recvErr error) {
		resp, recvErr = b.stream.Recv()
		return recvErr
	}, func(used time.Duration) {
		b.logger.Warnf("stream receive already waited %s", used.String())
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			b.logger.Warnfe(err, "stream receive canceled")
		} else {
			b.errLogger().Errorfe(err, "stream receive failed")
		}
		return nil, controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
			errors.Wrapf(err, "receive for %s failed", b.title()))
	}
	if uint64(resp.GetProcessId()) != b.index.Global {
		return nil, controller.NewExternalError(controller.ErrCodeCallProcessorFailed,
			errors.Errorf("unexpected ProcessID #%d for %s", resp.GetProcessId(), b.title()))
	}
	return resp, nil
}

func (b *task) sendBindingData(ctx context.Context) *controller.ExternalError {
	return b.streamSend(ctx, &protos.ProcessStreamRequest{
		ProcessId: int32(b.index.Global),
		Value:     &protos.ProcessStreamRequest_Binding{Binding: b.data},
	}, "binding data", "SB", time.Minute*30)
}

func (b *task) recvPartition(ctx context.Context) *controller.ExternalError {
	resp, extErr := b.streamReceive(ctx, "RP", time.Minute*30)
	if extErr != nil {
		return extErr
	}
	for _, p := range resp.GetPartitions().GetPartitions() {
		b.partition = p.GetUserValue()
	}
	return nil
}

func (b *task) sendStartCommand(ctx context.Context) *controller.ExternalError {
	return b.streamSend(ctx, &protos.ProcessStreamRequest{
		ProcessId: int32(b.index.Global),
		Value:     &protos.ProcessStreamRequest_Start{Start: true},
	}, "start command", "SS", time.Minute*30)
}

type statistic struct {
	Get            int
	List           int
	ListEntities   int
	Upsert         int
	UpsertEntities int
	Update         int
	UpdateEntities int
	Delete         int
	DeleteEntities int

	Entity     map[string]map[string]int // Entity[op][name]
	TimeSeries map[timeseries.MetaType]map[string]int
	Export     map[string]int
}

func (b *task) waitProcess(
	ctx context.Context,
	checkpointCtrl controller.CheckpointController,
) (statistic, *controller.ExternalError) {
	var stat statistic
	stat.Entity = make(map[string]map[string]int)
	stat.TimeSeries = make(map[timeseries.MetaType]map[string]int)
	stat.Export = make(map[string]int)
	for {
		resp, extErr := b.streamReceive(ctx, "RR", time.Minute*30)
		if extErr != nil {
			return stat, extErr
		}
		if resp.GetResult() != nil {
			if errMsg := resp.GetResult().GetStates().GetError(); errMsg != "" {
				b.errLogger().Errorf("got error in final result: %s", errMsg)
				return stat, controller.NewExternalError(controller.ErrCodeProcessFailed,
					errors.Errorf("got error in %s: %s", b.title(), errMsg))
			}
			// Event v2 and metric (gauge/counter) v2 data are no longer supported by
			// the streaming driver (driver v3+). A processor that still emits them is
			// running an unsupported SDK and must be rejected instead of silently
			// dropping the data.
			if len(resp.GetResult().GetEvents()) > 0 {
				return stat, controller.NewExternalError(controller.ErrCodeProcessFailed,
					errors.Errorf("event v2 data is no longer supported in %s, please upgrade the processor SDK", b.title()))
			}
			if len(resp.GetResult().GetGauges()) > 0 || len(resp.GetResult().GetCounters()) > 0 {
				return stat, controller.NewExternalError(controller.ErrCodeProcessFailed,
					errors.Errorf("metric v2 data is no longer supported in %s, please upgrade the processor SDK", b.title()))
			}
			if tsr := resp.GetResult().GetTimeseriesResult(); len(tsr) > 0 {
				b.logger.Debugf("got %d time series data in final process result", len(tsr))
				if data, convertErr := b.ConvertTimeSeriesData(tsr); convertErr != nil {
					b.errLogger().Errorfe(convertErr, "convert time series data failed")
					return stat, convertErr.Wrapf("invalid time series data in %s", b.title())
				} else {
					checkpointCtrl.InsertTimeSeriesData(b.GetBlockNumber(), b.index, data)
					timeseries.Statistic(data, stat.TimeSeries)
				}
			}
			if epr := resp.GetResult().GetExports(); len(epr) > 0 {
				b.logger.Debugf("got %d export data in final process result", len(epr))
				data := b.ConvertExportData(epr)
				checkpointCtrl.InsertWebhookData(b.GetBlockNumber(), b.index, data)
				controller.StatisticWebhookMessages(data, stat.Export)
			}
			return stat, nil
		}

		if req := resp.GetTplRequest(); req != nil {
			b.logger.Debugf("got %d templates", len(req.GetTemplates()))
			extErr = checkpointCtrl.NewTemplateInstance(ctx, b, ConvertTemplateInstance(req.GetTemplates(), req.GetRemove()))
			if extErr != nil {
				return stat, extErr.Wrapf("add template instance in %s failed", b.title())
			}
			continue
		}
		if req := resp.GetTsRequest(); req != nil {
			b.logger.Debugf("got %d time series data", len(req.GetData()))
			if data, convertErr := b.ConvertTimeSeriesData(req.GetData()); convertErr != nil {
				b.errLogger().Errorfe(convertErr, "convert time series data failed: %v", req.GetData())
				return stat, convertErr.Wrapf("invalid time series data in %s", b.title())
			} else {
				checkpointCtrl.InsertTimeSeriesData(b.GetBlockNumber(), b.index, data)
				timeseries.Statistic(data, stat.TimeSeries)
			}
			continue
		}

		// must be an db request
		dbReq := resp.GetDbRequest()
		dbResp := protos.DBResponse{OpId: dbReq.GetOpId()}
		reqLogger := b.logger.With("opid", dbReq.GetOpId())
		reqErrLogger := func() *log.SentioLogger {
			return reqLogger.With("binding", b.data.String(), "dbreq", dbReq.String())
		}

		// wait resource
		if dbReq.GetGet() != nil || dbReq.GetList() != nil {
			wr := b.timer.Start("WR")
			err := b.waiter.finish.Wait(ctx, func(p partitionWithIndex) bool {
				return p.partition == b.partition && p.index < b.index.Global
			})
			if err != nil {
				return stat, controller.NewExternalError(controller.ErrCodeSystem,
					errors.Wrapf(err, "wait previous task finish for %s failed", b.title()))
			}
			waitUsed := wr.End()
			reqLogger = reqLogger.With("waitUsed", waitUsed.String())
		}

		// do the db operation
		var what string
		start := b.timer.Start("DE")
		if dbReq.GetGet() != nil {
			entity, id := dbReq.GetGet().GetEntity(), dbReq.GetGet().GetId()
			reqLogger = reqLogger.With("dbop", "get", "entity", entity, "id", id)
			entityType := checkpointCtrl.GetEntityOrInterfaceType(entity)
			if entityType == nil {
				reqErrLogger().Errorf("get unknown entity")
				return stat, controller.NewExternalError(controller.ErrCodeGetUnknownEntity,
					errors.Errorf("get unknown entity %q for %s", entity, b.title()))
			}
			box, getErr := checkpointCtrl.GetEntity(ctx, entityType, id, b.GetBlockNumber())
			if getErr != nil {
				reqErrLogger().Errorfe(getErr, "get entity failed")
				return stat, getErr.Wrapf("get entity %s/%s in %s failed", entity, id, b.title())
			}
			if box != nil && box.Data != nil {
				data, convertErr := box.ToRichStruct(entityType)
				if convertErr != nil {
					reqErrLogger().Errorfe(getErr, "convert entity to RichStruct failed")
					return stat, controller.NewExternalError(controller.ErrCodeInvalidEntityData,
						errors.Wrapf(convertErr, "convert entity %s/%s in %s failed", entity, id, b.title()))
				}
				dbResp.Value = &protos.DBResponse_EntityList{
					EntityList: &protos.EntityList{
						Entities: []*protos.Entity{{
							Entity:         box.Entity,
							GenBlockNumber: box.GenBlockNumber,
							GenBlockTime:   timestamppb.New(box.GenBlockTime),
							GenBlockChain:  b.chainID,
							Data:           data,
						}},
					},
				}
			} else {
				// not created or deleted, return an empty list
				dbResp.Value = &protos.DBResponse_EntityList{
					EntityList: &protos.EntityList{},
				}
			}
			stat.Get++
			what = fmt.Sprintf("entity get response #%d", stat.Get)
		}
		if dbReq.GetList() != nil {
			const defaultPageSize = 10000
			entity := dbReq.GetList().GetEntity()
			pageSize := int(dbReq.GetList().GetPageSize())
			if pageSize <= 0 {
				pageSize = defaultPageSize
			}
			reqLogger = reqLogger.With(
				"dbop", "list",
				"entity", entity,
				"cursor", dbReq.GetList().GetCursor(),
				"pageSize", pageSize,
				"filters", dbReq.GetList().GetFilters())
			entityType := checkpointCtrl.GetEntityType(entity)
			if entityType == nil {
				reqErrLogger().Errorf("list unknown entity")
				return stat, controller.NewExternalError(controller.ErrCodeListUnknownEntity,
					errors.Errorf("list unknown entity %q for %s", entity, b.title()))
			}
			filters := make([]persistent.EntityFilter, len(dbReq.GetList().GetFilters()))
			for fi, ft := range dbReq.GetList().GetFilters() {
				field := entityType.GetFieldByName(ft.GetField())
				if field == nil {
					reqErrLogger().Errorf("field %s.%s in entity list filter is not exist", entityType.Name, ft.GetField())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
						errors.Errorf("field %s.%s in entity list filter is not exist for %s",
							entityType.Name, ft.GetField(), b.title()))
				}
				if entityType.GetForeignKeyFieldByName(ft.GetField()).IsReverseField() {
					reqErrLogger().Errorf("field %s.%s in entity list filter is a reverse foreign key",
						entityType.Name, ft.GetField())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
						errors.Errorf("field %s.%s in entity list filter is a reverse foreign key for %s",
							entityType.Name, ft.GetField(), b.title()))
				}
				fieldTitle := fmt.Sprintf("%s.%s %s", entityType.Name, ft.GetField(), field.Type.String())
				filters[fi] = persistent.EntityFilter{Field: field}
				switch ft.GetOp() {
				case protos.DBRequest_EQ:
					filters[fi].Op = persistent.EntityFilterOpEq
				case protos.DBRequest_NE:
					filters[fi].Op = persistent.EntityFilterOpNe
				case protos.DBRequest_GT:
					filters[fi].Op = persistent.EntityFilterOpGt
				case protos.DBRequest_GE:
					filters[fi].Op = persistent.EntityFilterOpGe
				case protos.DBRequest_LT:
					filters[fi].Op = persistent.EntityFilterOpLt
				case protos.DBRequest_LE:
					filters[fi].Op = persistent.EntityFilterOpLe
				case protos.DBRequest_IN:
					filters[fi].Op = persistent.EntityFilterOpIn
				case protos.DBRequest_NOT_IN:
					filters[fi].Op = persistent.EntityFilterOpNotIn
				case protos.DBRequest_LIKE:
					filters[fi].Op = persistent.EntityFilterOpLike
				case protos.DBRequest_NOT_LIKE:
					filters[fi].Op = persistent.EntityFilterOpNotLike
				case protos.DBRequest_HAS_ALL:
					filters[fi].Op = persistent.EntityFilterOpHasAll
				case protos.DBRequest_HAS_ANY:
					filters[fi].Op = persistent.EntityFilterOpHasAny
				default:
					reqErrLogger().Errorf("unknown list filter op %s", ft.GetOp())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
						errors.Errorf("unknown list filter operator %v for %s", ft.GetOp(), b.title()))
				}
				filterValueType := field.Type
				if ft.GetOp() == protos.DBRequest_HAS_ALL || ft.GetOp() == protos.DBRequest_HAS_ANY {
					fieldType := schema.BreakType(field.Type)
					if fieldType.CountListLayer() != 1 {
						reqErrLogger().Errorf("field %s cannot use op %s", fieldTitle, ft.GetOp().String())
						return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
							errors.Errorf("field %s cannot use op %s for %s", fieldTitle, ft.GetOp().String(), b.title()))
					}
					filterValueType = schema.BreakType(field.Type).SkipListLayer(1).Join()
				}
				for _, val := range ft.GetValue().GetValues() {
					value, convertErr := persistent.FromRichValue(val, filterValueType)
					if convertErr != nil {
						reqErrLogger().Errorfe(convertErr, "convert filter value %v for field %s with op %s to go value failed",
							val, fieldTitle, ft.GetOp().String())
						return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
							errors.Wrapf(convertErr, "convert filter value %v for field %s with op %s to go value for %s failed",
								val, fieldTitle, ft.GetOp().String(), b.title()))
					}
					filters[fi].Value = append(filters[fi].Value, value)
				}
				if initErr := filters[fi].Init(); initErr != nil {
					reqErrLogger().Errorfe(initErr, "init filter for field %s with op %s", fieldTitle, ft.GetOp().String())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter,
						errors.Wrapf(initErr, "init filter for field %s with op %s for %s failed",
							fieldTitle, ft.GetOp().String(), b.title()))
				}
			}
			boxes, nextCursor, listErr := checkpointCtrl.ListEntity(
				ctx, entityType, filters, dbReq.GetList().GetCursor(), pageSize, b.GetBlockNumber())
			if listErr != nil {
				reqErrLogger().Errorfe(listErr, "list entity failed")
				return stat, listErr.Wrapf("list entity for %s failed", b.title())
			}
			data := &protos.EntityList{Entities: make([]*protos.Entity, len(boxes))}
			for k, box := range boxes {
				one, convertErr := box.ToRichStruct(entityType)
				if convertErr != nil {
					reqErrLogger().With("box", box.String()).Errorfe(convertErr, "convert entity to RichStruct failed")
					return stat, controller.NewExternalError(controller.ErrCodeInvalidEntityData,
						errors.Wrapf(convertErr, "convert entity %s to RichStruct for %s failed ", box.String(), b.title()))
				}
				data.Entities[k] = &protos.Entity{
					Entity:         box.Entity,
					GenBlockNumber: box.GenBlockNumber,
					GenBlockTime:   timestamppb.New(box.GenBlockTime),
					GenBlockChain:  b.chainID,
					Data:           one,
				}
			}
			dbResp.Value = &protos.DBResponse_EntityList{EntityList: data}
			dbResp.NextCursor = nextCursor
			stat.List++
			stat.ListEntities += len(boxes)
			what = fmt.Sprintf("entity list response #%d/%d", stat.List, stat.ListEntities)
		}
		if dbReq.GetUpsert() != nil {
			reqLogger = reqLogger.With("dbop", "upsert", "count", len(dbReq.GetUpsert().GetId()))
			entities := dbReq.GetUpsert().GetEntity()
			ids := dbReq.GetUpsert().GetId()
			datas := dbReq.GetUpsert().GetEntityData()
			if len(ids) != len(datas) || len(ids) != len(entities) {
				reqErrLogger().Errorf("len(entity),length(id),length(data) = %d,%d,%d must be equal",
					len(entities), len(ids), len(datas))
				return stat, controller.NewExternalError(controller.ErrCodeInvalidUpsertEntityRequest,
					errors.Errorf("len(entity),length(id),length(data) = %d,%d,%d must be equal in db upsert request for %s",
						len(entities), len(ids), len(datas), b.title()))
			}
			for k := range ids {
				entity, id, data := entities[k], ids[k], datas[k]
				summary := fmt.Sprintf("%d/%d %s/%s", k+1, len(ids), entity, id)
				entityType := checkpointCtrl.GetEntityType(entity)
				if entityType == nil {
					reqErrLogger().Errorf("set unknown entity %s failed", summary)
					return stat, controller.NewExternalError(controller.ErrCodeUpsertUnknownEntity,
						errors.Errorf("set unknown entity %s for %s", summary, b.title()))
				}
				box := persistent.UncommittedEntityBox{EntityBox: persistent.EntityBox{
					ID:             id,
					GenBlockNumber: b.GetBlockNumber(),
					GenBlockTime:   b.GetBlockTime(),
					GenBlockHash:   b.GetBlockHash(),
				}}
				if convertErr := box.FromRichStruct(entityType, data); convertErr != nil {
					reqErrLogger().Errorfe(convertErr, "convert entity %s/%s failed", summary, data.String())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidUpsertEntityRequest,
						errors.Wrapf(convertErr, "convert entity %s/%s failed for %s failed", summary, data.String(), b.title()))
				}
				if setErr := checkpointCtrl.SetEntity(ctx, entityType, box); setErr != nil {
					reqErrLogger().Errorfe(setErr, "set entity %s %s failed", summary, box.String())
					return stat, setErr.Wrapf("set entity %s %s for %s failed", summary, box.String(), b.title())
				}
				reqLogger.Debugf("set entity %s completed", summary)
				utils.IncrK2Map(stat.Entity, "upsert", entity, 1)
			}
			stat.Upsert++
			stat.UpsertEntities += len(ids)
			what = fmt.Sprintf("entity upsert response #%d/%d", stat.Upsert, stat.UpsertEntities)
		}
		if dbReq.GetUpdate() != nil {
			reqLogger = reqLogger.With("dbop", "update", "count", len(dbReq.GetUpdate().GetId()))
			entities := dbReq.GetUpdate().GetEntity()
			ids := dbReq.GetUpdate().GetId()
			datas := dbReq.GetUpdate().GetEntityData()
			if len(ids) != len(datas) || len(ids) != len(entities) {
				reqErrLogger().Errorf("len(entity),length(id),length(data) = %d,%d,%d must be equal",
					len(entities), len(ids), len(datas))
				return stat, controller.NewExternalError(controller.ErrCodeInvalidUpdateEntityRequest,
					errors.Errorf("len(entity),length(id),length(data) = %d,%d,%d must be equal in db update request for %s",
						len(entities), len(ids), len(datas), b.title()))
			}
			for k := range ids {
				entity, id, data := entities[k], ids[k], datas[k]
				summary := fmt.Sprintf("%d/%d %s/%s", k+1, len(ids), entity, id)
				entityType := checkpointCtrl.GetEntityType(entity)
				if entityType == nil {
					reqErrLogger().Errorf("set unknown entity %s failed", summary)
					return stat, controller.NewExternalError(controller.ErrCodeUpdateUnknownEntity,
						errors.Errorf("set unknown entity %s for %s", summary, b.title()))
				}
				box := persistent.UncommittedEntityBox{EntityBox: persistent.EntityBox{
					ID:             id,
					GenBlockNumber: b.GetBlockNumber(),
					GenBlockTime:   b.GetBlockTime(),
					GenBlockHash:   b.GetBlockHash(),
				}}
				if convertErr := box.FromEntityUpdateData(entityType, data); convertErr != nil {
					reqErrLogger().Errorfe(convertErr, "convert entity %s/%s failed", summary, data.String())
					return stat, controller.NewExternalError(controller.ErrCodeInvalidUpdateEntityRequest,
						errors.Wrapf(convertErr, "convert entity %s/%s failed for %s failed", summary, data.String(), b.title()))
				}
				if setErr := checkpointCtrl.SetEntity(ctx, entityType, box); setErr != nil {
					reqErrLogger().Errorfe(setErr, "set entity %s %s failed", summary, box.String())
					return stat, setErr.Wrapf("set entity %s %s for %s failed", summary, box.String(), b.title())
				}
				reqLogger.Debugf("set entity %s completed", summary)
				utils.IncrK2Map(stat.Entity, "update", entity, 1)
			}
			stat.Update++
			stat.UpdateEntities += len(ids)
			what = fmt.Sprintf("entity update response #%d/%d", stat.Update, stat.UpdateEntities)
		}
		if dbReq.GetDelete() != nil {
			reqLogger = reqLogger.With("dbop", "delete", "count", len(dbReq.GetDelete().GetId()))
			entities, ids := dbReq.GetDelete().GetEntity(), dbReq.GetDelete().GetId()
			if len(ids) != len(entities) {
				reqErrLogger().Errorf("len(entity),length(id) = %d,%d must be equal", len(entities), len(ids))
				return stat, controller.NewExternalError(controller.ErrCodeInvalidDeleteEntityRequest,
					errors.Errorf("len(entity),length(id) = %d,%d must be equal in db delete request for %s",
						len(entities), len(ids), b.title()))
			}
			for k := range ids {
				entity, id := entities[k], ids[k]
				summary := fmt.Sprintf("%d/%d %s/%s", k+1, len(ids), entity, id)
				entityType := checkpointCtrl.GetEntityType(entity)
				if entityType == nil {
					reqErrLogger().Errorf("delete unknown entity %s failed", summary)
					return stat, controller.NewExternalError(controller.ErrCodeDeleteUnknownEntity,
						errors.Errorf("delete unknown entity %s for %s", summary, b.title()))
				}
				box := persistent.UncommittedEntityBox{EntityBox: persistent.EntityBox{
					ID:             id,
					GenBlockNumber: b.GetBlockNumber(),
					GenBlockTime:   b.GetBlockTime(),
					GenBlockHash:   b.GetBlockHash(),
				}}
				if delErr := checkpointCtrl.SetEntity(ctx, entityType, box); delErr != nil {
					reqErrLogger().Errorfe(delErr, "delete entity %s failed", summary)
					return stat, delErr.Wrapf("delete entity %s for %s failed", summary, b.title())
				}
				reqLogger.Debugf("delete entity %s completed", summary)
				utils.IncrK2Map(stat.Entity, "delete", entity, 1)
			}
			stat.Delete++
			stat.DeleteEntities += len(ids)
			what = fmt.Sprintf("entity delete response #%d/%d", stat.Delete, stat.DeleteEntities)
		}
		reqLogger.With("used", start.End().String()).Debugf("%s is ready", what)

		// send db result
		req := &protos.ProcessStreamRequest{
			ProcessId: resp.GetProcessId(),
			Value:     &protos.ProcessStreamRequest_DbResult{DbResult: &dbResp},
		}
		if sendErr := b.streamSend(ctx, req, what, "SE", time.Minute*30); sendErr != nil {
			return stat, sendErr
		}
	}
}
