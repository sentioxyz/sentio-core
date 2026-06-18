package controller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type testBlockHeader struct {
	BlockNumber     uint64
	BlockHash       string
	BlockParentHash string
	BlockTime       time.Time
}

func (b testBlockHeader) GetBlockNumber() uint64 {
	return b.BlockNumber
}

func (b testBlockHeader) GetBlockParentHash() string {
	return b.BlockParentHash
}

func (b testBlockHeader) GetBlockHash() string {
	return b.BlockHash
}

func (b testBlockHeader) GetBlockTime() time.Time {
	return b.BlockTime
}

func newTestBlockHeader(blockNumber uint64, blockHash, blockParentHash string) testBlockHeader {
	zeroTime, _ := time.Parse(time.DateTime, "2025-07-01 00:00:00")
	return testBlockHeader{
		BlockNumber:     blockNumber,
		BlockHash:       blockHash,
		BlockParentHash: blockParentHash,
		BlockTime:       zeroTime.Add(time.Second * time.Duration(blockNumber)),
	}
}

type testClient struct {
	mu sync.Mutex

	first             uint64
	latest            uint64
	subscribeCallBack func(latest BlockHeader, broken error)

	fork []uint64

	broken error
}

func newTestClient(first uint64, latest uint64) *testClient {
	return &testClient{first: first, latest: latest}
}

func (c *testClient) GetLatest(ctx context.Context) (latest BlockHeader, first uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buildHeader(c.latest), c.first, nil
}

func (c *testClient) Subscribe(ctx context.Context, from BlockHeader, callback func(latest BlockHeader, broken error)) {
	c.mu.Lock()
	c.subscribeCallBack = callback
	c.mu.Unlock()
	<-ctx.Done()
}

func (c *testClient) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buildHeader(blockNumber), nil
}

func (c *testClient) ResetCache(r BlockRange) {
}

func (c *testClient) Snapshot() any {
	return nil
}

func (c *testClient) buildBlockHash(bn uint64) string {
	var ver int
	for _, fk := range c.fork {
		if fk <= bn {
			ver++
		}
	}
	return fmt.Sprintf("h%d-%d", bn, ver)
}

func (c *testClient) buildHeader(bn uint64) testBlockHeader {
	if bn == 0 {
		return newTestBlockHeader(bn, c.buildBlockHash(0), "")
	}
	return newTestBlockHeader(bn, c.buildBlockHash(bn), c.buildBlockHash(bn-1))
}

func (c *testClient) Fork(bn uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Warnf("fork at %d", bn)
	c.fork = append(c.fork, bn)
}

func (c *testClient) UpdateLatest(bn uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if bn <= c.latest {
		return
	}
	if c.latest < bn {
		c.latest = bn
	}
	c.subscribeCallBack(c.buildHeader(bn), c.broken)
}

func (c *testClient) Broken(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.broken = err
}

type testBlockDataFetcher struct {
	client       *testClient
	taskSleep    time.Duration
	errTaskIndex uint64
	newTplIndex  map[uint64]TemplateInstance

	mu            sync.Mutex
	latest        BlockHeader
	latestChanged chan struct{}
}

func (f *testBlockDataFetcher) GetName() string {
	return "testBlockDataFetcher"
}

func (f *testBlockDataFetcher) GetFullRange() BlockRange {
	return BlockRange{}
}

func (f *testBlockDataFetcher) Snapshot() any {
	return nil
}

func (f *testBlockDataFetcher) KeepFetch(ctx context.Context) {
	<-ctx.Done()
}

func (f *testBlockDataFetcher) Get(ctx context.Context, bn uint64) (
	data BlockData,
	has bool,
	latest BlockHeader,
	err error,
) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for bn > f.latest.GetBlockNumber() {
		changed := f.latestChanged
		f.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			err = ctx.Err()
			f.mu.Lock()
			return
		}
		f.mu.Lock()
	}
	latest = f.latest

	taskLen := bn % 5
	// no data in block 5 15 25 ...
	if taskLen == 0 && bn%2 != 0 {
		return
	}

	header, _ := f.client.GetHeaderIgnoreCache(context.Background(), bn)
	tasks := make([]Task, taskLen)
	for i := uint64(0); i < taskLen; i++ {
		tasks[i] = &testTask{
			BlockHeader: header,
			errIndex:    f.errTaskIndex,
			newTplIndex: f.newTplIndex,
			sleep:       f.taskSleep,
		}
	}
	data = newTestBlockData(header, tasks...)
	has = true
	return
}

func (f *testBlockDataFetcher) UpdateLatest(latest BlockHeader) {
	f.mu.Lock()
	f.latest = latest
	close(f.latestChanged)
	f.latestChanged = make(chan struct{})
	f.mu.Unlock()
}

func (f *testBlockDataFetcher) Broken(err error) {
}

func (f *testBlockDataFetcher) MoveStart(start uint64) {
}

type testHandlerController struct {
	Client       *testClient
	TaskSleep    time.Duration
	ErrTaskIndex uint64
	NewTplIndex  map[uint64]TemplateInstance
	EndBlock     *uint64

	BlockRange
}

func (c *testHandlerController) Prologue(
	ctx context.Context,
	checkpoint *Checkpoint,
	templates map[uint64][]TemplateInstance,
	first uint64,
	latest BlockHeader,
) *ExternalError {
	c.BlockRange = BlockRange{StartBlock: first, EndBlock: c.EndBlock}
	return nil
}

func (c *testHandlerController) Epilogue() {
}

func (c *testHandlerController) GetBlockRange() BlockRange {
	return c.BlockRange
}

func (c *testHandlerController) GetAgentStat() map[string]int {
	return map[string]int{"placeholder": 1}
}

func (c *testHandlerController) BuildBlockDataFetcher(_, _ uint64, latest BlockHeader) Fetcher[BlockData] {
	return &testBlockDataFetcher{
		client:        c.Client,
		taskSleep:     c.TaskSleep,
		errTaskIndex:  c.ErrTaskIndex,
		newTplIndex:   c.NewTplIndex,
		latest:        latest,
		latestChanged: make(chan struct{}),
	}
}

func (c *testHandlerController) Snapshot() any {
	return nil
}

type testTask struct {
	BlockHeader
	index       TaskIndex
	errIndex    uint64
	newTplIndex map[uint64]TemplateInstance
	sleep       time.Duration
}

func (t *testTask) GetHandlerID() HandlerID {
	return HandlerID{}
}

func (t *testTask) Init(ctx context.Context, index TaskIndex, progressbar ProgressBar) {
	t.index = index
}

func (t *testTask) Summary() string {
	return fmt.Sprintf("#%d binding data %d/%d in block %s",
		t.index.Global, t.index.InBlock, t.index.TotalInBlock, GetBlockSummary(t))
}

func (t *testTask) Exec(ctx context.Context, checkpointCtrl CheckpointController) *ExternalError {
	_, logger := log.FromContext(ctx, "block", t.GetBlockNumber(), "index", t.index)
	logger.Debug("task start")
	if t.errIndex == t.index.Global {
		time.Sleep(t.sleep / 2)
		logger.Warnf("task failed")
		return NewExternalError(ErrCodeCallProcessorFailed, errors.Errorf("task %#v fail", t.index))
	}
	if newTpl, has := t.newTplIndex[t.GetBlockNumber()]; has && t.index.InBlock == 0 {
		time.Sleep(t.sleep / 2)
		logger.Warnf("task has new template %s", newTpl)
		return checkpointCtrl.NewTemplateInstance(ctx, t, []TemplateInstance{newTpl})
	}
	select {
	case <-ctx.Done():
		logger.Warnf("task canceled")
		return NewExternalError(ErrCodeCallProcessorFailed, errors.Wrapf(ctx.Err(), "task %#v canceled", t.index))
	case <-time.After(t.sleep):
		logger.Debug("task end")
		return nil
	}
}

func Test_main_succeed(t *testing.T) {
	//log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 100)
	hc := &testHandlerController{
		Client:      cli,
		TaskSleep:   time.Millisecond * 500,
		NewTplIndex: make(map[uint64]TemplateInstance),
	}
	bb := NewBlockBuilder(hc, cli, false)
	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	mc := NewMainController(bb, cc, false, nil, "")
	err := mc.run(ctx)
	log.Warnf("err: %+v", err)

	last := cs.checkpoints[len(cs.checkpoints)-1]
	assert.Equal(t, uint64(100), last.BlockNumber)
}

func Test_main_succeed_and_end(t *testing.T) {
	//log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 100)
	hc := &testHandlerController{
		Client:      cli,
		TaskSleep:   time.Millisecond * 500,
		NewTplIndex: make(map[uint64]TemplateInstance),
		EndBlock:    utils.WrapPointer[uint64](80), // 80 will build a empty BlockData and make checkpoint, 75 will not
	}
	bb := NewBlockBuilder(hc, cli, false)
	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	mc := NewMainController(bb, cc, false, nil, "")
	err := mc.run(ctx)
	log.Warnf("err: %+v", err)
	assert.NoError(t, err)

	last := cs.checkpoints[len(cs.checkpoints)-1]
	assert.Equal(t, uint64(80), last.BlockNumber)
	assert.True(t, last.AllDone())
}

func Test_main_failed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 100)
	hc := &testHandlerController{
		Client:       cli,
		TaskSleep:    time.Millisecond * 500,
		ErrTaskIndex: 50,
		NewTplIndex:  make(map[uint64]TemplateInstance),
	}
	bb := NewBlockBuilder(hc, cli, false)
	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	mc := NewMainController(bb, cc, false, nil, "")
	err := mc.run(ctx)
	log.Warnf("err: %+v", err)

	var extErr *ExternalError
	assert.True(t, errors.As(err, &extErr))
	assert.Equal(t, ErrCodeCallProcessorFailed, extErr.code)
	assert.Equal(t, "task controller.TaskIndex{Global:0x32, InBlock:3, TotalInBlock:4} fail", extErr.error.Error())
}

func Test_main_reorg(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 30)
	hc := &testHandlerController{
		Client:      cli,
		NewTplIndex: make(map[uint64]TemplateInstance),
	}
	bb := NewBlockBuilder(hc, cli, true)

	go func() {
		time.Sleep(time.Second)
		cli.Fork(26)
		cli.UpdateLatest(60)
		time.Sleep(time.Second * 3)
		cli.Fork(58)
		cli.UpdateLatest(100)
	}()

	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	mc := NewMainController(bb, cc, false, nil, "")
	roundCtx, _ := log.FromContext(ctx, "round", 0)
	err := mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalReorgDetected)

	roundCtx, _ = log.FromContext(ctx, "round", 1)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalReorgDetected)

	roundCtx, _ = log.FromContext(ctx, "round", 2)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	last := cs.checkpoints[len(cs.checkpoints)-1]
	assert.Equal(t, uint64(100), last.BlockNumber)
}

func Test_main_newTpl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 100)
	hc := &testHandlerController{
		Client:    cli,
		TaskSleep: time.Millisecond * 500,
		NewTplIndex: map[uint64]TemplateInstance{
			49: {
				Address:    "0x1111",
				BlockRange: BlockRange{StartBlock: 49},
			},
			51: {
				Address:    "0x2222",
				BlockRange: BlockRange{StartBlock: 70},
			},
			52: { // dup create, will be ignored
				Address:    "0x1111",
				BlockRange: BlockRange{StartBlock: 70},
			},
		},
	}
	bb := NewBlockBuilder(hc, cli, true)
	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	ccc := cc.(*checkpointController)
	mc := NewMainController(bb, cc, false, nil, "")
	roundCtx, _ := log.FromContext(ctx, "round", 0)
	err := mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalHasNewTemplate)
	assert.Equal(t, map[uint64][]TemplateInstance{
		49: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 49},
		}},
	}, ccc.templates)
	assert.Equal(t, map[uint64][]TemplateInstance{}, ccc.unsavedTemplates)
	assert.Equal(t, uint64(48), ccc.checkpoints[len(ccc.checkpoints)-1].BlockNumber)

	roundCtx, _ = log.FromContext(ctx, "round", 1)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalHasNewTemplate)
	assert.Equal(t, map[uint64][]TemplateInstance{
		49: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 49},
		}},
		51: {{
			TemplateID: 0,
			Address:    "0x2222",
			BlockRange: BlockRange{StartBlock: 70},
		}},
	}, ccc.templates)
	assert.Equal(t, map[uint64][]TemplateInstance{}, ccc.unsavedTemplates)
	assert.Equal(t, uint64(51), ccc.checkpoints[len(ccc.checkpoints)-1].BlockNumber)

	roundCtx, _ = log.FromContext(ctx, "round", 2)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.Equal(t, uint64(100), cs.checkpoints[len(cs.checkpoints)-1].BlockNumber)
}

func Test_main_removeTpl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	cs := &testCheckpointStore{}
	cli := newTestClient(0, 100)
	hc := &testHandlerController{
		Client:    cli,
		TaskSleep: time.Millisecond * 500,
		NewTplIndex: map[uint64]TemplateInstance{
			49: {
				Address:    "0x1111",
				BlockRange: BlockRange{StartBlock: 49},
			},
			51: {
				Address:    "0x2222",
				BlockRange: BlockRange{StartBlock: 70},
			},
			52: { // dup create, will be ignored
				Address:    "0x1111",
				BlockRange: BlockRange{StartBlock: 70},
			},
			53: { // remove 0::0x1111
				Address:    "0x1111",
				BlockRange: BlockRange{StartBlock: 70},
				Removed:    true,
			},
		},
	}
	bb := NewBlockBuilder(hc, cli, true)
	cc, _ := NewCheckpointController(
		ctx,
		"1",
		time.Second,
		time.Second*2,
		100000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	ccc := cc.(*checkpointController)
	mc := NewMainController(bb, cc, false, nil, "")
	roundCtx, _ := log.FromContext(ctx, "round", 0)
	err := mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalHasNewTemplate)
	assert.Equal(t, map[uint64][]TemplateInstance{
		49: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 49},
		}},
	}, ccc.templates)
	assert.Equal(t, map[uint64][]TemplateInstance{}, ccc.unsavedTemplates)
	assert.Equal(t, uint64(48), ccc.checkpoints[len(ccc.checkpoints)-1].BlockNumber)

	roundCtx, _ = log.FromContext(ctx, "round", 1)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalHasNewTemplate)
	assert.Equal(t, map[uint64][]TemplateInstance{
		49: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 49},
		}},
		51: {{
			TemplateID: 0,
			Address:    "0x2222",
			BlockRange: BlockRange{StartBlock: 70},
		}},
	}, ccc.templates)
	assert.Equal(t, map[uint64][]TemplateInstance{}, ccc.unsavedTemplates)
	assert.Equal(t, uint64(51), ccc.checkpoints[len(ccc.checkpoints)-1].BlockNumber)

	roundCtx, _ = log.FromContext(ctx, "round", 2)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	assert.ErrorIs(t, err, ErrInternalHasNewTemplate)
	assert.Equal(t, map[uint64][]TemplateInstance{
		49: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 49},
		}},
		51: {{
			TemplateID: 0,
			Address:    "0x2222",
			BlockRange: BlockRange{StartBlock: 70},
		}},
		53: {{
			TemplateID: 0,
			Address:    "0x1111",
			BlockRange: BlockRange{StartBlock: 70},
			Removed:    true,
		}},
	}, ccc.templates)
	assert.Equal(t, map[uint64][]TemplateInstance{}, ccc.unsavedTemplates)
	assert.Equal(t, uint64(53), ccc.checkpoints[len(ccc.checkpoints)-1].BlockNumber)

	roundCtx, _ = log.FromContext(ctx, "round", 3)
	err = mc.run(roundCtx)
	log.Warnf("err: %+v", err)
	last := cs.checkpoints[len(cs.checkpoints)-1]
	assert.Equal(t, uint64(100), last.BlockNumber)
}
