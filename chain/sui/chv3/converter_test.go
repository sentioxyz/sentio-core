package chv3

import (
	"encoding/json"
	"os"
	"testing"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// iota testnet tx H15Gqr4pLZL1479EKdqxpkdbHA1y8exu4b3Ny4aS9M8g in checkpoint 283601808 burns an NFT, and the
// json-rpc reply lists it as a "deleted" object change
func TestConvertTxnSkipsDeletedObjectChanges(t *testing.T) {
	raw, err := os.ReadFile("testdata/iota-testnet-deleted-nft-tx.json")
	require.NoError(t, err)
	var tx types.TransactionResponseV1
	require.NoError(t, json.Unmarshal(raw, &tx))
	require.Len(t, tx.ObjectChanges, 2)
	require.EqualValues(t, types.ObjectChangeTypeDeleted, tx.ObjectChanges[1].Type)

	var m SlotConverter
	cp := &sui.SlotCheckpointInfo{SequenceNumber: 283601808, TimestampMs: tx.TimestampMs}
	txn, _, _, _, objectChanges, err := m.ConvertTxn(cp, &tx, 0)
	require.NoError(t, err)
	assert.EqualValues(t, 1, txn.DeletedCount)
	require.Len(t, objectChanges, 1)
	assert.Equal(t, string(types.ObjectChangeTypeMutated), objectChanges[0].Type)
	assert.Equal(t, "0x2::coin::Coin<0x2::iota::IOTA>", *objectChanges[0].ObjectType)
}
