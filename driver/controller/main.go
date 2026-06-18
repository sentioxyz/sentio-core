package controller

import (
	"context"
	"sync/atomic"
	"time"

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/pkg/errors"
)

type MainController struct {
	seqMode bool

	blockBuilder   BlockBuilder
	checkpointCtrl CheckpointController
	processor      *models.Processor
	chainID        string

	bindingIndex atomic.Uint64

	analyser
}

func NewMainController(
	blockBuilder BlockBuilder,
	checkpointCtrl CheckpointController,
	seqMode bool,
	processor *models.Processor,
	chainID string,
) *MainController {
	N.DriverCreated(processor, chainID, func() (int64, bool) {
		if cc := checkpointCtrl.GetSavedLatestCheckpoint(); cc != nil {
			return int64(cc.BlockNumber), true
		}
		return 0, false
	})
	return &MainController{
		seqMode:        seqMode,
		blockBuilder:   blockBuilder,
		checkpointCtrl: checkpointCtrl,
		processor:      processor,
		chainID:        chainID,
		analyser:       newAnalyser(),
	}
}

func (c *MainController) Main(ctx context.Context) error {
	const maxDupErrRetryTimes = 10
	lastExtErrCode, lastExtErrRound := 0, 0
	for round := 0; ; round++ {
		runCtx, logger := log.FromContext(ctx, "runID", round)
		err := c.run(runCtx)
		if err == nil {
			logger.UserVisible().Info("chain is done")
			return nil
		}
		switch {
		case errors.Is(err, ErrInternalReorgDetected):
			continue
		case errors.Is(err, ErrInternalHasNewTemplate):
			continue
		}
		var extErr *ExternalError
		if errors.As(err, &extErr) {
			logger.Errorf("run got external error: %+v", extErr)
			saveErr := c.checkpointCtrl.SaveError(ctx, extErr)
			if saveErr != nil {
				logger.Errore(saveErr, "save chain error failed")
			}
			if extErr.IsDriverError() || extErr.IsUserRuntimeError() || saveErr != nil {
				if saveErr == nil && extErr.Code() == lastExtErrCode {
					if dupTimes := round - lastExtErrRound; dupTimes >= maxDupErrRetryTimes {
						logger.Warnf("same error dupped %d times, will exit now", dupTimes)
						return err
					} else {
						logger.Warnf("same error dupped %d times", round-lastExtErrRound)
					}
				} else {
					lastExtErrCode, lastExtErrRound = extErr.Code(), round
				}
				logger.Infof("will retry after %s", RunWaiting.String())
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(RunWaiting):
					continue
				}
			}
		} else {
			logger.Errorf("run got error: %+v", err)
		}
		return err
	}
}

func (c *MainController) Snapshot() any {
	// Mirror the taskProcessConcurrency computed in run(): in sequential mode tasks run on a single
	// worker, otherwise on ProcessConcurrency workers. Surfacing both (plus seqMode) lets a reader tell
	// "handlers are saturated" apart from "handlers are starved" without reverse-engineering the config.
	taskProcessConcurrency := ProcessConcurrency
	if c.seqMode {
		taskProcessConcurrency = 1
	}
	return map[string]any{
		"seqMode":                c.seqMode,
		"taskProcessConcurrency": taskProcessConcurrency,
		"bindingIndex":           c.bindingIndex.Load(),
		"blockBuilder":           c.blockBuilder.Snapshot(),
		"checkpointController":   c.checkpointCtrl.Snapshot(),
		"statistics":             c.analyser.Snapshot(),
	}
}

type BlockDataSummary struct {
	BlockNumber     uint64
	BlockParentHash string
	BlockHash       string
	BlockTime       time.Time

	TaskCount      int
	CheckpointData map[string]string
}

func (s BlockDataSummary) GetBlockNumber() uint64 {
	return s.BlockNumber
}

func (s BlockDataSummary) GetBlockParentHash() string {
	return s.BlockParentHash
}

func (s BlockDataSummary) GetBlockHash() string {
	return s.BlockHash
}

func (s BlockDataSummary) GetBlockTime() time.Time {
	return s.BlockTime
}

type BlockPanel struct {
	BlockNumber uint64
	DataSummary *BlockDataSummary
	TaskCount   int

	ProgressBar
}

type ProgressMessage struct {
	BlockStart    *BlockPanel
	BlockTaskDone uint64
	BlockAllDone  *uint64
}

func (c *MainController) run(ctx context.Context) error {
	_, logger := log.FromContext(ctx)
	logger.Info("run started")
	defer func() {
		logger.Info("run finished")
	}()

	checkpoint, templates := c.checkpointCtrl.GetLatestCheckpoint(), c.checkpointCtrl.GetTemplates()
	agentStat, extErr := c.blockBuilder.Start(ctx, checkpoint, templates)
	if extErr != nil {
		return extErr
	}
	defer c.blockBuilder.Finish()
	if extErr = c.checkpointCtrl.Ready(ctx, agentStat); extErr != nil {
		return extErr
	}
	taskProcessConcurrency := utils.Select(c.seqMode, 1, int(ProcessConcurrency))
	logger.Infow("main stream is ready",
		"checkpoint", utils.NullOrToString(checkpoint),
		"templates", utils.CountMap(templates),
		"taskProcessConcurrency", taskProcessConcurrency)

	N.DriverStarted(ctx, c.processor, c.chainID, CountTemplatesByID(templates))

	g, gctx := errgroup.WithContext(ctx)
	// More capacity to ensure the goroutines that execute tasks and build blockData are less likely to be blocked by this
	progressNotice := make(chan ProgressMessage, 10000)
	// Once all the data for a block is ready, an element will be push to this chan.
	// When a block's checkpoint is constructed, an element will be pop from this chan.
	// So its capacity is the number of blocks that can be processed simultaneously.
	waitingBlocks := make(chan struct{}, 1000)
	concurrency.RunWithProducer(
		g,
		gctx,
		taskProcessConcurrency,
		func(ctx context.Context, taskChan chan<- Task) error {
			logger.Info("keep build block data started")
			defer func() {
				logger.Info("keep build block data finished")
			}()
			for {
				// fetch BlockData, may be waiting some fetcher, may be waiting latest block
				fetchStartAt := time.Now()
				blockNumber, blockData, progressBar, reorg, getErr := c.blockBuilder.Next(ctx)
				c.analyser.fetchWait(time.Since(fetchStartAt))
				if getErr != nil {
					if errors.Is(getErr, ErrInternalNeedUpgrade) {
						return NewExternalError(ErrCodeNeedUpgrade, getErr)
					}
					// even the endpoint was override by user, the fetch data failed error should also be driver error
					return NewExternalError(ErrCodeFetchDataFailed, getErr)
				}
				if reorg != nil {
					if cleanErr := c.checkpointCtrl.CleanCheckpoint(ctx, blockNumber, *reorg); cleanErr != nil {
						return cleanErr
					}
					N.ReorgDetected(ctx, c.processor, c.chainID)
					return ErrInternalReorgDetected
				}
				if !progressBar.FullBlockRange.Contains(blockNumber) {
					// no more data, build block data can finish now,
					// blockData and progressBar.LatestBlock will always be nil here.
					logger.Infow("no more block data", "blockNumber", blockNumber, "full", progressBar.FullBlockRange.String())
					select {
					case progressNotice <- ProgressMessage{BlockAllDone: &blockNumber}:
					case <-ctx.Done():
					}
					return nil
				}
				startAt := time.Now()
				// Using `dataSummary` instead of `blockData` is to release the reference to `blockData`, allowing its memory
				// to be released promptly and preventing subsequent tasks from having their references to `blockData`
				// continuously attached due to a task running for too long.
				var dataSummary *BlockDataSummary
				var taskList []Task
				if blockData != nil {
					taskList = blockData.GetTaskList()
					dataSummary = &BlockDataSummary{
						BlockNumber:     blockData.GetBlockNumber(),
						BlockParentHash: blockData.GetBlockParentHash(),
						BlockHash:       blockData.GetBlockHash(),
						BlockTime:       blockData.GetBlockTime(),
						TaskCount:       len(taskList),
						CheckpointData:  blockData.CheckpointData(),
					}
				}
				// If waitingBlocks is full, it means that the checkpoint goroutine is stuck.
				// This could be because making checkpoint is too time-consuming, or because a task is taking too long.
				select {
				case waitingBlocks <- struct{}{}:
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case progressNotice <- ProgressMessage{
					BlockStart: &BlockPanel{
						BlockNumber: blockNumber,
						DataSummary: dataSummary,
						ProgressBar: progressBar,
						TaskCount:   len(taskList),
					},
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
				for i, task := range taskList {
					index := TaskIndex{
						Global:       c.bindingIndex.Add(1),
						InBlock:      i,
						TotalInBlock: len(taskList),
					}
					task.Init(ctx, index, progressBar)
					select {
					case taskChan <- task:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				c.analyser.taskSent(time.Since(startAt))
			}
		},
		func(ctx context.Context, task Task) error {
			startAt := time.Now()
			if taskErr := task.Exec(ctx, c.checkpointCtrl); taskErr != nil {
				return taskErr
			}
			completeAt := time.Now()
			select {
			case progressNotice <- ProgressMessage{BlockTaskDone: task.GetBlockNumber()}:
			case <-ctx.Done():
			}
			c.analyser.taskComplete(task.GetHandlerID().String(), time.Since(startAt), time.Since(completeAt))
			return nil
		})

	makeCheckpointDone := make(chan struct{})
	g.Go(func() error {
		logger.Info("keep make checkpoint started")
		defer func() {
			logger.Info("keep make checkpoint finished")
		}()
		waiting := make(map[uint64]*BlockPanel) // size will always less than cap(waitingBlocks)
		for {
			var bn uint64
			select {
			case <-gctx.Done():
				return gctx.Err()
			case pm := <-progressNotice:
				if pm.BlockStart != nil {
					bn = pm.BlockStart.BlockNumber
					waiting[bn] = pm.BlockStart
				} else if pm.BlockAllDone != nil {
					bn = *pm.BlockAllDone
					waiting[bn] = nil // bn is the finalize block, will out of full block range
				} else {
					bn = pm.BlockTaskDone
					waiting[bn].TaskCount -= 1
				}
			}
			if bn > 0 && waiting[bn-1] != nil {
				continue
			}
			startAt := time.Now()
			for {
				r, has := waiting[bn]
				if has && r == nil {
					// no more checkpoint need to be make now
					close(makeCheckpointDone)
					return nil
				}
				if !has || r.TaskCount > 0 {
					break
				}
				if r.DataSummary != nil {
					hasNewTpl, makeErr := c.checkpointCtrl.MakeCheckpoint(gctx, *r.DataSummary, r.ProgressBar)
					if makeErr != nil {
						return makeErr
					} else if hasNewTpl {
						return ErrInternalHasNewTemplate
					}
				}
				delete(waiting, bn)
				<-waitingBlocks
				bn++
			}
			c.analyser.makeCheckpoint(time.Since(startAt))
		}
	})

	g.Go(func() error {
		return c.checkpointCtrl.KeepSave(gctx, makeCheckpointDone)
	})

	return g.Wait()
}
