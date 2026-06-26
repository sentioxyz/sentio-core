package sol

import (
	"encoding/json"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unixTime(t int64) *solana.UnixTimeSeconds {
	v := solana.UnixTimeSeconds(t)
	return &v
}

// A new GetLatestHeaderResult must decode into the pre-SimpleBlock driver's Block (header fields),
// and an old bare Block response must decode into GetLatestHeaderResult — so a driver/super-node
// rolling upgrade survives in both directions.
func TestGetLatestHeaderResult_WireCompatWithBlock(t *testing.T) {
	hash := solana.Hash{1, 2, 3}
	parent := solana.Hash{4, 5, 6}

	t.Run("new result decodes into old Block", func(t *testing.T) {
		resp := GetLatestHeaderResult{
			SimpleBlock: SimpleBlock{Slot: 1234, Blockhash: hash, PreviousBlockhash: parent, BlockTime: unixTime(1700000000)},
			FirstSlot:   1000,
			APIVersion:  APIVersion,
		}
		raw, err := json.Marshal(resp)
		require.NoError(t, err)

		var blk Block
		require.NoError(t, json.Unmarshal(raw, &blk))
		assert.Equal(t, uint64(1234), blk.Slot)
		require.NotNil(t, blk.GetBlockResult)
		assert.Equal(t, hash, blk.Blockhash)
		assert.Equal(t, parent, blk.PreviousBlockhash)
	})

	t.Run("old bare Block decodes into new result", func(t *testing.T) {
		old := Block{Slot: 1234, GetBlockResult: &rpc.GetBlockResult{
			Blockhash:         hash,
			PreviousBlockhash: parent,
			BlockTime:         unixTime(1700000000),
		}}
		raw, err := json.Marshal(old)
		require.NoError(t, err)

		var resp GetLatestHeaderResult
		require.NoError(t, json.Unmarshal(raw, &resp))
		assert.Equal(t, uint64(1234), resp.Slot)
		assert.Equal(t, hash, resp.Blockhash)
		assert.Equal(t, parent, resp.PreviousBlockhash)
		// An old super node sends neither field; both must default safely.
		assert.Equal(t, uint64(0), resp.FirstSlot)
		assert.Equal(t, 0, resp.APIVersion)
		assert.NoError(t, resp.CheckAPIVersion()) // 0 <= APIVersion ⇒ no forced upgrade
	})
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
