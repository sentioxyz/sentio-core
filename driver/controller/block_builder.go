package controller

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/sparsify"
	"sentioxyz/sentio-core/common/utils"
)

type BlockHeader interface {
	GetBlockNumber() uint64
	GetBlockParentHash() string
	GetBlockHash() string
	GetBlockTime() time.Time
}

func GetBlockSummary(b BlockHeader) string {
	const hashPreviewLen = 8
	hash := b.GetBlockHash()
	if len(hash) == 0 {
		return fmt.Sprintf("%d", b.GetBlockNumber())
	} else if len(hash) < hashPreviewLen {
		return fmt.Sprintf("%d/%s", b.GetBlockNumber(), hash)
	} else {
		return fmt.Sprintf("%d/%s", b.GetBlockNumber(), hash[:hashPreviewLen])
	}
}

func GetBlockFullText[H BlockHeader](b H) string {
	var bf bytes.Buffer
	bf.WriteString(strconv.FormatUint(b.GetBlockNumber(), 10))
	if hash := b.GetBlockHash(); len(hash) > 0 {
		bf.WriteString("/")
		bf.WriteString(hash)
	}
	bf.WriteString("/")
	bf.WriteString(b.GetBlockTime().Format(time.RFC3339Nano))
	if hash := b.GetBlockParentHash(); len(hash) > 0 {
		bf.WriteString("->")
		bf.WriteString(hash)
	}
	return bf.String()
}

type BlockData interface {
	BlockHeader

	GetTaskList() []Task               // BlockData without task may be not a empty BlockData
	CheckpointData() map[string]string // at least should return a empty map
	DataSource() string
	Size() int
}

type TaskIndex struct {
	Global       uint64
	InBlock      int
	TotalInBlock int
}

type Task interface {
	BlockHeader

	Summary() string
	GetHandlerID() HandlerID
	Init(ctx context.Context, index TaskIndex, progressbar ProgressBar)
	Exec(ctx context.Context, checkpointCtrl CheckpointController) *ExternalError
}

type ProgressBar struct {
	LatestBlock    BlockHeader
	FullBlockRange BlockRange
}

type BlockBuilder interface {
	// Start Reset the templates, then reconstruct all HandlerAgent, start constructing blockData
	// after checkpoint.BlockNumber, and return them one by one through Next.
	// If checkpoint is nil, it means starting from the beginning, and the first block to be processed will be
	// obtained through all HandlerAgent.
	Start(
		ctx context.Context,
		checkpoint *Checkpoint,
		templates map[uint64][]TemplateInstance,
	) (agentStat map[string]int, extErr *ExternalError)

	// Next Get the block data of the current block.
	// may be waiting for the Fetcher to fetch data, or waiting for the latest block.
	// If reorg is detected, the returned reorg will not be nil, and its value indicates that all data from
	// that block are invalid.
	Next(ctx context.Context) (blockNumber uint64, blockData BlockData, progressBar ProgressBar, reorg *uint64, err error)

	// Finish End the traversal of the block
	Finish()

	Snapshot() map[string]any
}

type Client interface {
	// GetLatest return err may be ErrInternalNeedUpgrade
	GetLatest(ctx context.Context) (latest BlockHeader, first uint64, err error)
	// Subscribe broken in callback may be ErrInternalNeedUpgrade
	Subscribe(ctx context.Context, from BlockHeader, callback func(latest BlockHeader, broken error))
	// GetHeaderIgnoreCache get the block header without cache, used to check reorg
	GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (BlockHeader, error)
	// ResetCache should remove the cached data from the target block, used when reorg detected or progress passed
	ResetCache(r BlockRange)
	Snapshot() any
}

type FetchTarget interface {
	Size() int
}

type Fetcher[T FetchTarget] interface {
	GetName() string
	GetFullRange() BlockRange
	Snapshot() any
	KeepFetch(ctx context.Context)
	Get(ctx context.Context, blockNumber uint64) (data T, has bool, latest BlockHeader, err error)
	UpdateLatest(latest BlockHeader)
	Broken(err error)
	MoveStart(start uint64)
}

type blockBuilder struct {
	handlerCtrl HandlerController
	client      Client
	checkLink   bool

	dataFetcher Fetcher[BlockData]

	fetchCancel context.CancelFunc
	fetchDone   sync.WaitGroup

	mu sync.RWMutex

	fullBlockRange     BlockRange
	currentBlockNumber uint64

	headerList []BlockHeader
}

func NewBlockBuilder(
	handlerCtrl HandlerController,
	client Client,
	checkLink bool,
) BlockBuilder {
	return &blockBuilder{
		handlerCtrl: handlerCtrl,
		client:      client,
		checkLink:   checkLink,
	}
}

func (b *blockBuilder) Start(
	ctx context.Context,
	checkpoint *Checkpoint,
	templates map[uint64][]TemplateInstance,
) (map[string]int, *ExternalError) {
	latest, first, err := b.client.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, ErrInternalNeedUpgrade) {
			return nil, NewExternalError(ErrCodeNeedUpgrade, err)
		}
		return nil, NewExternalError(ErrCodeFetchDataFailed, err)
	}
	if checkpoint != nil && checkpoint.BlockNumber < first {
		first = checkpoint.BlockNumber
	}
	if extErr := b.handlerCtrl.Prologue(ctx, checkpoint, templates, first, latest); extErr != nil {
		return nil, extErr
	}
	if len(b.handlerCtrl.GetAgentStat()) == 0 {
		return nil, NewExternalError(ErrCodeUnexpectedProcessorConfig, errors.Errorf("no handler"))
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.fullBlockRange = b.handlerCtrl.GetBlockRange()
	b.currentBlockNumber = b.fullBlockRange.StartBlock
	b.headerList = nil
	if checkpoint != nil {
		b.currentBlockNumber = checkpoint.BlockNumber + 1
		if b.checkLink {
			b.headerList = append(b.headerList, checkpoint)
		}
	}

	b.client.ResetCache(BlockRange{StartBlock: b.currentBlockNumber})

	_, logger := log.FromContext(ctx)
	logger.Infow("will start to fetch data",
		"full", b.fullBlockRange.String(),
		"current", b.currentBlockNumber,
		"latest", GetBlockSummary(latest))

	b.dataFetcher = b.handlerCtrl.BuildBlockDataFetcher(first, b.currentBlockNumber, latest)

	var fetchCtx context.Context
	fetchCtx, b.fetchCancel = context.WithCancel(ctx)
	b.fetchDone.Add(2)
	go func() {
		b.client.Subscribe(fetchCtx, latest, func(latest BlockHeader, broken error) {
			if broken != nil {
				b.dataFetcher.Broken(broken)
			} else {
				b.dataFetcher.UpdateLatest(latest)
			}
		})
		b.fetchDone.Done()
	}()
	go func() {
		b.dataFetcher.KeepFetch(fetchCtx)
		b.fetchDone.Done()
	}()
	return b.handlerCtrl.GetAgentStat(), nil
}

func (b *blockBuilder) checkReorg(ctx context.Context, cur BlockHeader) (reorg *uint64, err error) {
	_, logger := log.FromContext(ctx)
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.headerList) == 0 {
		return
	}
	pre := b.headerList[len(b.headerList)-1]
	if pre.GetBlockNumber()+1 == cur.GetBlockNumber() && cur.GetBlockParentHash() == pre.GetBlockHash() {
		return
	}
	for i := len(b.headerList) - 1; i >= 0; i-- {
		var h BlockHeader
		h, err = b.client.GetHeaderIgnoreCache(ctx, b.headerList[i].GetBlockNumber())
		if err != nil {
			return
		}
		if h.GetBlockHash() == b.headerList[i].GetBlockHash() {
			if i < len(b.headerList)-1 {
				logger.Warnf("block %s not changed", GetBlockSummary(h))
				// first different block must in [b.headerList[i].GetBlockNumber()+1, b.headerList[i+1].GetBlockNumber()],
				// be conservative and clear all data starting from b.headerList[i].GetBlockNumber()+1
				reorg = utils.WrapPointer(h.GetBlockNumber() + 1)
			}
			return
		}
		reorg = utils.WrapPointer(b.headerList[i].GetBlockNumber())
		logger.Warnf("block %s changed to %s", GetBlockSummary(b.headerList[i]), GetBlockSummary(h))
	}
	return
}

func (b *blockBuilder) Next(ctx context.Context) (
	blockNumber uint64,
	blockData BlockData,
	progressBar ProgressBar,
	reorg *uint64,
	err error,
) {
	b.mu.RLock()
	blockNumber = b.currentBlockNumber
	progressBar.FullBlockRange = b.fullBlockRange
	b.mu.RUnlock()

	if !b.fullBlockRange.Contains(blockNumber) {
		if blockNumber < b.fullBlockRange.StartBlock {
			// unreachable
			panic(errors.Errorf("current block number %d is to the left of full block range %s",
				blockNumber, b.fullBlockRange))
		}
		// out of range, just return
		return
	}

	var has bool
	blockData, has, progressBar.LatestBlock, err = b.dataFetcher.Get(ctx, blockNumber)
	if err != nil {
		return
	}

	// detect reorg
	if has && b.checkLink && progressBar.LatestBlock.GetBlockTime().Sub(blockData.GetBlockTime()) < WatchingDelay {
		if reorg, err = b.checkReorg(ctx, blockData); reorg != nil || err != nil {
			return
		}
	}

	// progress will go to b.currentBlockNumber + 1, so all cached data in [0,b.currentBlockNumber] are useless now.
	if has && progressBar.LatestBlock.GetBlockTime().Sub(blockData.GetBlockTime()) < WatchingDelay {
		b.client.ResetCache(BlockRange{EndBlock: &b.currentBlockNumber})
	}

	b.mu.Lock()
	if b.checkLink && has {
		b.headerList = append(b.headerList, blockData)
		b.headerList = sparsify.Sparsify(b.headerList, BlockHeader.GetBlockNumber)
	}
	b.currentBlockNumber++
	b.dataFetcher.MoveStart(b.currentBlockNumber)
	b.mu.Unlock()
	return
}

func (b *blockBuilder) Finish() {
	b.fetchCancel()
	b.fetchDone.Wait()
	b.handlerCtrl.Epilogue()
}

func (b *blockBuilder) Snapshot() map[string]any {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sn := map[string]any{
		"client":            b.client.Snapshot(),
		"checkLink":         b.checkLink,
		"handlerController": b.handlerCtrl.Snapshot(),
	}
	if b.dataFetcher != nil {
		// already started
		sn["fetcher"] = b.dataFetcher.Snapshot()
		sn["fullRange"] = b.fullBlockRange.String()
		sn["currentBlockNumber"] = b.currentBlockNumber
		sn["headerList"] = utils.MapSliceNoError(b.headerList, GetBlockSummary)
	}
	return sn
}
