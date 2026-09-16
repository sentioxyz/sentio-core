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
)

func Test_processedWindow(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	ctx := log.ToContext(context.Background(), log.FromZap(zap.New(core)))
	cc, err := NewCheckpointController(
		ctx,
		"1",
		0,
		time.Hour,
		10000,
		&testCheckpointStore{},
		EmptyQuotaService{},
		EmptyTimeSeriesController{},
		EmptyEntityController{},
		EmptyWebhookController{},
		nil,
	)
	assert.NoError(t, err)
	defer func(v uint64) { PrintProcessedMaxBindingBlocks = v }(PrintProcessedMaxBindingBlocks)
	PrintProcessedMaxBindingBlocks = 7

	processedLines := func() []string {
		var lines []string
		for _, entry := range logs.TakeAll() {
			if !strings.HasPrefix(entry.Message, "Processed ") {
				continue
			}
			for _, field := range entry.Context {
				assert.NotEqual(t, "user_visible", field.Key, "the Processed line is not for users")
			}
			lines = append(lines, entry.Message)
		}
		return lines
	}
	// Backfill: the blocks are far behind the latest block.
	progressBar := ProgressBar{LatestBlock: newSimpleTestBlockData(100000)}
	process := func(blockNumber, bindings uint64) {
		summary := newSimpleTestBlockDataSummary(blockNumber)
		summary.TaskCount = int(bindings)
		_, extErr := cc.MakeCheckpoint(ctx, summary, progressBar)
		assert.Nil(t, extErr)
	}

	// Blocks 1..20 have 0 1 2 3 4 0 1 2 3 4 ... bindings. Every 7 blocks with bindings make one line, and a
	// segment of a single block is written as [block].
	for blockNumber := uint64(1); blockNumber <= 20; blockNumber++ {
		process(blockNumber, (blockNumber-1)%5)
	}
	lines := processedLines()
	if assert.Len(t, lines, 2) {
		assert.Contains(t, lines[0], "[0/1-9/100000] with 16 bindings in 7 blocks: [2-5][1 2 3 4]+[7-9][1 2 3]")
		assert.Contains(t, lines[1],
			"[0/10-18/100000] with 17 bindings in 7 blocks: [10][4]+[12-15][1 2 3 4]+[17-18][1 2]")
	}

	// The save flushes what is left.
	assert.Nil(t, cc.Save(ctx, true))
	lines = processedLines()
	if assert.Len(t, lines, 1) {
		assert.Contains(t, lines[0], "[0/19-20/100000] with 7 bindings in 2 blocks: [19-20][3 4]")
	}

	// A window without any binding is reported without segments, and an empty window is not reported at all.
	process(21, 0)
	process(22, 0)
	assert.Nil(t, cc.Save(ctx, true))
	assert.Nil(t, cc.Save(ctx, true))
	lines = processedLines()
	if assert.Len(t, lines, 1) {
		assert.True(t, strings.HasSuffix(lines[0], "[0/21-22/100000] with 0 bindings in 0 blocks"), lines[0])
	}

	// A reorg reports the pending window before rolling back, and the restart clears whatever is left so the
	// re-processed blocks are not merged with the rolled back ones.
	process(23, 1)
	process(24, 2)
	assert.Nil(t, cc.CleanCheckpoint(ctx, 25, 24))
	lines = processedLines()
	if assert.Len(t, lines, 1) {
		assert.Contains(t, lines[0], "[0/23-24/100000] with 3 bindings in 2 blocks: [23-24][1 2]")
	}
	assert.Nil(t, cc.Ready(ctx, nil))
	process(24, 5)
	assert.Nil(t, cc.Save(ctx, true))
	lines = processedLines()
	if assert.Len(t, lines, 1) {
		assert.Contains(t, lines[0], "[0/24-24/100000] with 5 bindings in 1 blocks: [24][5]")
	}
}
