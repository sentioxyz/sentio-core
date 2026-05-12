package fuel

import "github.com/sentioxyz/fuel-go/types"

func BuildTransactionStatus(raw *types.TransactionStatus, header types.Header) *types.TransactionStatus {
	block := types.Block{
		Id:     header.Id,
		Height: header.Height,
		Header: header,
	}
	switch raw.TypeName_ {
	case "SuccessStatus":
		succ := *raw.SuccessStatus
		succ.Block = block
		return &types.TransactionStatus{
			TypeName_:     raw.TypeName_,
			SuccessStatus: &succ,
		}
	case "FailureStatus":
		fail := *raw.FailureStatus
		fail.Block = block
		return &types.TransactionStatus{
			TypeName_:     raw.TypeName_,
			FailureStatus: &fail,
		}
	}
	return raw
}

func GetTxnReceipt(status *types.TransactionStatus) []types.Receipt {
	if status.SuccessStatus != nil {
		return status.SuccessStatus.Receipts
	} else if status.FailureStatus != nil {
		return status.FailureStatus.Receipts
	}
	return nil
}
