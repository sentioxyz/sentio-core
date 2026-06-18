package subgraph

import (
	"context"
	"fmt"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/controller"

	"github.com/pkg/errors"
)

type task struct {
	controller.BlockHeader
	taskData

	handlerCtrl *HandlerController
	instance    *instance

	index  controller.TaskIndex
	timer  *timer.Timer
	logger *log.SentioLogger
}

func (t *task) taskInfo() controller.TaskInfo {
	return controller.TaskInfo{
		Processor:  t.handlerCtrl.processor,
		ChainID:    t.handlerCtrl.chainID(),
		Handler:    t.handlerID.Name,
		Category:   t.handlerID.Type,
		DataSource: t.handlerID.DataSource,
	}
}

func (t *task) GetHandlerID() controller.HandlerID {
	return t.handlerID
}

func (t *task) Init(ctx context.Context, index controller.TaskIndex, progressbar controller.ProgressBar) {
	t.handlerCtrl.waiter.NewResource(index.Global)
	t.instance = t.handlerCtrl.instance // select one
	t.index = index
	t.timer = timer.NewTimer()
	_, t.logger = log.FromContext(ctx,
		"block", controller.GetBlockSummary(t),
		"latest", controller.GetBlockSummary(progressbar.LatestBlock),
		"index", index,
		"handler", t.handlerID.String())
}

func (t *task) errLogger() *log.SentioLogger {
	return t.logger.With("callHandlerArg", utils.MustJSONMarshal(t.callHandlerParam))
}

// taskInfoForCall is like taskInfo but uses the resolved data
// source name (set when the handler is actually invoked).
func (t *task) taskInfoForCall() controller.TaskInfo {
	h := t.taskInfo()
	h.DataSource = t.dataSource.Name
	return h
}

func (t *task) Summary() string {
	return fmt.Sprintf("#%d binding data %d/%d for handler %s in block %s",
		t.index.Global, t.index.InBlock, t.index.TotalInBlock, t.handlerID, controller.GetBlockSummary(t))
}

func (t *task) Exec(ctx context.Context, checkpointCtrl controller.CheckpointController) *controller.ExternalError {
	err := t.handlerCtrl.waiter.Wait(ctx, func(u uint64) bool {
		return u < t.index.Global
	})
	if err != nil {
		return controller.NewExternalError(controller.ErrCodeSystem,
			errors.Errorf("waiting all previous tasks finish failed: %v", err))
	}
	extErr := t.instance.CallHandler(
		wasm.NewCallContext[CtxData](ctx),
		wasm.CallParams[CtxData]{
			ExportFuncName: t.handlerID.Name,
			Logger:         t.logger,
			Data:           CtxData{dataSource: t.dataSource, task: t, checkpointCtrl: checkpointCtrl},
		},
		t.callHandlerParam,
	)
	if extErr != nil {
		return extErr.Wrapf("process %s failed", t.Summary())
	}
	t.handlerCtrl.waiter.ResourceReady(t.index.Global)
	return nil
}
