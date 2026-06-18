package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
