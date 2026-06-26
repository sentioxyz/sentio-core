package sol

import (
	"encoding/json"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unixTime(t int64) *solana.UnixTimeSeconds {
	v := solana.UnixTimeSeconds(t)
	return &v
}

func TestGetLatestHeaderResult_JSON(t *testing.T) {
	hash := solana.Hash{1, 2, 3}
	parent := solana.Hash{4, 5, 6}
	resp := GetLatestHeaderResult{
		SimpleBlock: SimpleBlock{Slot: 1234, Blockhash: hash, PreviousBlockhash: parent, BlockTime: unixTime(1700000000)},
		FirstSlot:   1000,
		APIVersion:  APIVersion,
	}

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	// The slot key must be lowercase "slot", consistent with the other header fields (regression: it
	// was once serialized as the untagged, capitalized "Slot").
	var keyed map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keyed))
	assert.Contains(t, keyed, "slot")
	assert.NotContains(t, keyed, "Slot")
	assert.Contains(t, keyed, "firstSlot")
	assert.Contains(t, keyed, "apiVersion")

	var got GetLatestHeaderResult
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, resp, got)

	// SimpleBlock shares Block's JSON keys, so a header is interchangeable between the two shapes.
	var blk Block
	require.NoError(t, json.Unmarshal(raw, &blk))
	assert.Equal(t, uint64(1234), blk.Slot)
	require.NotNil(t, blk.GetBlockResult)
	assert.Equal(t, hash, blk.Blockhash)
	assert.Equal(t, parent, blk.PreviousBlockhash)
}

func TestGetLatestHeaderResult_CheckAPIVersion(t *testing.T) {
	assert.NoError(t, GetLatestHeaderResult{APIVersion: APIVersion}.CheckAPIVersion())
	assert.NoError(t, GetLatestHeaderResult{APIVersion: APIVersion - 1}.CheckAPIVersion())
	assert.Error(t, GetLatestHeaderResult{APIVersion: APIVersion + 1}.CheckAPIVersion())
}

func TestSimpleBlock_BlockHeader(t *testing.T) {
	hash := solana.Hash{7, 8, 9}
	parent := solana.Hash{10, 11, 12}
	b := NewSimpleBlock(&Slot{
		SlotNumber:        42,
		Blockhash:         hash,
		PreviousBlockhash: parent,
		BlockTime:         unixTime(1700000000),
	})
	assert.Equal(t, uint64(42), b.GetBlockNumber())
	assert.Equal(t, hash.String(), b.GetBlockHash())
	assert.Equal(t, parent.String(), b.GetBlockParentHash())
	assert.Equal(t, int64(1700000000), b.GetBlockTime().Unix())
}
