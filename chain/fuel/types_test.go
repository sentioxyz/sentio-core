package fuel

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/sentioxyz/fuel-go/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_traverse(t *testing.T) {
	a := Slot{
		Block: &types.Block{
			Id: types.BlockId{
				Hash: common.HexToHash("0x1"),
			},
			Header: types.Header{
				Id: types.BlockId{
					Hash: common.HexToHash("0x1"),
				},
				Height: types.U32(123),
			},
			Transactions: []types.Transaction{
				{
					Id: types.TransactionId{
						Hash: common.HexToHash("0x2"),
					},
					Status: &types.TransactionStatus{
						TypeName_: "SuccessStatus",
						SuccessStatus: &types.SuccessStatus{
							TransactionId: types.TransactionId{
								Hash: common.HexToHash("0x2"),
							},
							Receipts: []types.Receipt{
								{
									ReceiptType: "CALL",
								},
							},
						},
					},
				},
			},
		},
	}
	txns := a.GetTransactions()
	txns[0].Status = BuildTransactionStatus(txns[0].Status, a.Header)

	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000",
		a.Transactions[0].Status.SuccessStatus.Block.Id.String())
	assert.Equal(t, types.U32(0), a.Transactions[0].Status.SuccessStatus.Block.Height)
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001",
		txns[0].Status.SuccessStatus.Block.Id.String())
	assert.Equal(t, types.U32(123), txns[0].Status.SuccessStatus.Block.Height)
}
