package persistent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// TestChangeHistory_Push verifies that changeHistory.Push maintains
// one entry per block number (later writes in the same block are merged)
// and that operator fields are evaluated against the previous entry.
func TestChangeHistory_Push(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	eType := sch.GetEntity("EntityE1")

	var his changeHistory
	his.Push(eType, &EntityBox{GenBlockNumber: 3, GenBlockHash: "3-1", Data: map[string]any{"propB": int32(1)}})
	his.Push(eType, &EntityBox{
		GenBlockNumber: 3,
		GenBlockHash:   "3-2",
		Data:           map[string]any{},
		Operator: map[string]Operator{
			"propB": {
				NumCalc: &OperatorNumCalc{
					Multi: rsh.NewIntValue(1),
					Add:   rsh.NewIntValue(1234),
				},
			},
		},
	})
	his.Push(eType, &EntityBox{GenBlockNumber: 5, GenBlockHash: "5-1", Data: map[string]any{"propB": int32(3)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 5, GenBlockHash: "5-2", Data: map[string]any{"propB": int32(4)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 1, GenBlockHash: "1-1", Data: map[string]any{"propB": int32(5)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 1, GenBlockHash: "1-2", Data: map[string]any{"propB": int32(6)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 4, GenBlockHash: "4-1", Data: map[string]any{"propB": int32(7)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 4, GenBlockHash: "4-2", Data: map[string]any{"propB": int32(8)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 2, GenBlockHash: "2-1", Data: map[string]any{"propB": int32(9)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 2, GenBlockHash: "2-2", Data: map[string]any{"propB": int32(10)}})

	// One entry per block, keyed by the last hash for that block.
	assert.Equal(t,
		[]string{"1-2", "2-2", "3-2", "4-2", "5-2"},
		utils.MapSliceNoError(his, func(b *EntityBox) string { return b.GenBlockHash }))

	// Block 3's operator (Add 1234) was applied against propB=1 → 1235.
	assert.Equal(t,
		[]map[string]any{
			{"propB": int32(6)},
			{"propB": int32(10)},
			{"propB": int32(1235)},
			{"propB": int32(8)},
			{"propB": int32(4)},
		},
		utils.MapSliceNoError(his, func(b *EntityBox) map[string]any { return b.Data }))
}

// TestChangeHistory_Split verifies that changeHistory.Split correctly
// partitions history at a given block number.
func TestChangeHistory_Split(t *testing.T) {
	make5 := func() changeHistory {
		return changeHistory{
			&EntityBox{GenBlockNumber: 1},
			&EntityBox{GenBlockNumber: 2},
			&EntityBox{GenBlockNumber: 3},
			&EntityBox{GenBlockNumber: 4},
			&EntityBox{GenBlockNumber: 5},
		}
	}

	t.Run("split at 0 moves all to after", func(t *testing.T) {
		his := make5()
		after := his.Split(0)
		assert.Equal(t, make5(), after)
		assert.Equal(t, changeHistory{}, his)
	})

	t.Run("split at 1 keeps first entry", func(t *testing.T) {
		his := make5()
		after := his.Split(1)
		assert.Equal(t, changeHistory{
			&EntityBox{GenBlockNumber: 2},
			&EntityBox{GenBlockNumber: 3},
			&EntityBox{GenBlockNumber: 4},
			&EntityBox{GenBlockNumber: 5},
		}, after)
		assert.Equal(t, changeHistory{
			&EntityBox{GenBlockNumber: 1},
		}, his)
	})

	t.Run("split at 4 keeps first four", func(t *testing.T) {
		his := make5()
		after := his.Split(4)
		assert.Equal(t, changeHistory{
			&EntityBox{GenBlockNumber: 5},
		}, after)
		assert.Equal(t, changeHistory{
			&EntityBox{GenBlockNumber: 1},
			&EntityBox{GenBlockNumber: 2},
			&EntityBox{GenBlockNumber: 3},
			&EntityBox{GenBlockNumber: 4},
		}, his)
	})

	t.Run("split at 5 returns nil (nothing after)", func(t *testing.T) {
		his := make5()
		after := his.Split(5)
		assert.Nil(t, after)
		assert.Equal(t, make5(), his)
	})
}

// TestChangeSet_Split verifies that changeSet.Split correctly partitions
// each entity's history and drops empty entries.
func TestChangeSet_Split(t *testing.T) {
	cs := changeSet{
		"entityA": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"2": {
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
				&EntityBox{GenBlockNumber: 4},
			},
			"3": {
				&EntityBox{GenBlockNumber: 3},
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
			},
			"4": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
		"entityB": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
		},
		"entityC": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
	}

	// Split at block 3: entries with GenBlockNumber > 3 go to the returned set.
	after := cs.Split(3)

	assert.Equal(t, changeSet{
		"entityA": map[string]changeHistory{
			"2": {
				&EntityBox{GenBlockNumber: 4},
			},
			"3": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
			},
			"4": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
		"entityC": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
	}, after)

	// Entries entirely at or before block 3 remain; empty histories are removed.
	assert.Equal(t, changeSet{
		"entityA": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"2": {
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"3": {
				&EntityBox{GenBlockNumber: 3},
			},
		},
		"entityB": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
		},
	}, cs)
}
