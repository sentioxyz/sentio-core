package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timer"
)

type testCheckpointStore struct {
	checkpoints []Checkpoint
	templates   map[uint64][]TemplateInstance
	err         *ExternalError
}

func (cs *testCheckpointStore) Load(ctx context.Context) ([]Checkpoint, map[uint64][]TemplateInstance, error) {
	return cs.checkpoints, cs.templates, nil
}

func (cs *testCheckpointStore) Save(
	ctx context.Context,
	checkpoints []Checkpoint,
	templates map[uint64][]TemplateInstance,
	agentStat map[string]int,
) error {
	cs.checkpoints = checkpoints
	cs.templates = templates
	return nil
}

func (cs *testCheckpointStore) SaveError(ctx context.Context, err *ExternalError) error {
	cs.err = err
	return nil
}

type testBlockData struct {
	BlockHeader
	tasks []Task
}

func (b testBlockData) DataSource() string {
	return "test-data-source"
}

func (b testBlockData) GetTaskList() []Task {
	return b.tasks
}

func (b testBlockData) CheckpointData() map[string]string {
	return nil
}

func (b testBlockData) Size() int {
	return 1
}

func newSimpleTestBlockData(blockNumber uint64) testBlockData {
	return newTestBlockData(newTestBlockHeader(blockNumber, "", ""))
}

func newSimpleTestBlockDataSummary(blockNumber uint64) BlockDataSummary {
	h := newTestBlockHeader(blockNumber, "", "")
	return BlockDataSummary{
		BlockNumber:     h.GetBlockNumber(),
		BlockParentHash: h.GetBlockParentHash(),
		BlockHash:       h.GetBlockHash(),
		BlockTime:       h.GetBlockTime(),
	}
}

func newTestBlockData(header BlockHeader, tasks ...Task) testBlockData {
	return testBlockData{
		BlockHeader: header,
		tasks:       tasks,
	}
}

func Test_save(t *testing.T) {
	cs := &testCheckpointStore{}
	ctx := context.Background()
	cc, err := NewCheckpointController(
		context.Background(),
		"1",
		0,
		time.Hour,
		10000,
		cs,
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	assert.NoError(t, err)

	progressBar := ProgressBar{
		LatestBlock: newSimpleTestBlockData(1000),
	}
	makeCheckpoints := func(bs ...uint64) []Checkpoint {
		r := make([]Checkpoint, len(bs))
		for i, bn := range bs {
			r[i] = Checkpoint{
				BlockNumber:       bn,
				BlockTime:         newSimpleTestBlockData(bn).GetBlockTime(),
				LatestBlockNumber: progressBar.LatestBlock.GetBlockNumber(),
				LatestBlockTime:   progressBar.LatestBlock.GetBlockTime(),
			}
		}
		return r
	}
	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(0), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(1), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 1), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(2), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 1, 2), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(3), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 2, 3), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(4), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 2, 3, 4), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(5), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 2, 4, 5), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(6), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 4, 5, 6), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(7), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 4, 6, 7), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(8), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 4, 6, 7, 8), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(9), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 4, 6, 8, 9), cs.checkpoints)

	_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(10), progressBar)
	assert.Nil(t, err)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Equal(t, makeCheckpoints(0, 4, 8, 9, 10), cs.checkpoints)

	for i := uint64(11); i <= 100; i++ {
		// all even number has non-empty checkpoint
		if i%2 == 0 {
			_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(i), progressBar)
			assert.Nil(t, err)
			assert.Nil(t, cc.Save(ctx, true))
		}
	}
	assert.Equal(t, makeCheckpoints(0, 64, 80, 88, 92, 96, 98, 100), cs.checkpoints)

	for i := uint64(101); i <= 200; i++ {
		_, err = cc.MakeCheckpoint(ctx, newSimpleTestBlockDataSummary(i), progressBar)
		assert.Nil(t, err)
		assert.Nil(t, cc.Save(ctx, true))
	}
	assert.Equal(t, makeCheckpoints(0, 128, 160, 176, 192, 196, 198, 199, 200), cs.checkpoints)
}

func Test_MakeCheckpoint_printProcessed(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	ctx := log.ToContext(context.Background(), log.FromZap(zap.New(core)))
	cc, err := NewCheckpointController(
		ctx,
		"1",
		0,
		time.Hour,
		10,
		&testCheckpointStore{},
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	assert.NoError(t, err)
	// Never let the throttle expire during the test so the output does not depend on timing.
	cc.(*checkpointController).printProcessedExecutor = timer.NewMinimumIntervalExecutor(time.Hour)

	processedLines := func() []string {
		var lines []string
		for _, entry := range logs.TakeAll() {
			if strings.HasPrefix(entry.Message, "Processed ") {
				lines = append(lines, entry.Message)
			}
		}
		return lines
	}
	withBindings := func(blockNumber uint64) BlockDataSummary {
		summary := newSimpleTestBlockDataSummary(blockNumber)
		summary.TaskCount = 3
		return summary
	}

	// Backfill: the blocks are far behind the latest block, so only the first block is reported even though
	// every block has bindings.
	progressBar := ProgressBar{LatestBlock: newSimpleTestBlockData(100000)}
	for blockNumber := uint64(0); blockNumber < 9; blockNumber++ {
		_, extErr := cc.MakeCheckpoint(ctx, withBindings(blockNumber), progressBar)
		assert.Nil(t, extErr)
	}
	lines := processedLines()
	if assert.Len(t, lines, 1) {
		assert.Contains(t, lines[0], "/0/")
		assert.Contains(t, lines[0], "with 3 bindings")
	}

	// The 10th block reaches maxKeepCheckpointCount and triggers a save, so it is reported regardless of the throttle.
	_, extErr := cc.MakeCheckpoint(ctx, withBindings(9), progressBar)
	assert.Nil(t, extErr)
	lines = processedLines()
	if assert.Len(t, lines, 1) {
		assert.Contains(t, lines[0], "/9/")
	}

	// Watching the chain tip: every block is reported.
	progressBar = ProgressBar{LatestBlock: newSimpleTestBlockData(20)}
	for blockNumber := uint64(11); blockNumber < 14; blockNumber++ {
		_, extErr := cc.MakeCheckpoint(ctx, withBindings(blockNumber), progressBar)
		assert.Nil(t, extErr)
	}
	assert.Len(t, processedLines(), 3)
}
