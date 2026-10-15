package ch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Log_ToLog(t *testing.T) {
	// the hashes are arbitrary; the timestamp is that of polygon block 60000123 (2024-07-30T19:44:34Z)
	row := Log{
		BlockIndex: BlockIndex{
			BlockNumber:    60000123,
			BlockHash:      "0x6f0ad1b3b8c8a21e3a12d9f4d7b8f8a2c1e5e3b2a1d0c9b8a7f6e5d4c3b2a190",
			BlockTimestamp: time.Date(2024, 7, 30, 19, 44, 34, 0, time.UTC),
		},
		TxnIndex: TxnIndex{
			TransactionIndex: 3,
			TransactionHash:  "0x1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
		},
		LogIndex: 4,
		Address:  "0x3c499c542cef5e3811e1192ce70d8cc03d5c3359",
		Data:     "0x01",
		Topics:   []string{"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"},
	}
	lg := row.ToLog()
	assert.Equal(t, uint64(60000123), lg.BlockNumber)
	assert.Equal(t, uint64(1722368674), lg.BlockTimestamp)
	assert.Equal(t, uint(3), lg.TxIndex)
	assert.Equal(t, uint(4), lg.Index)

	raw, err := json.Marshal(lg)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"blockTimestamp":"0x66a942a2"`)
}
