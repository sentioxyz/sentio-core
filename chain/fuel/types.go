package fuel

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/sentioxyz/fuel-go/types"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type GetLatestBlockResponse struct {
	Header     types.Header `json:"latest"`
	APIVersion int          `json:"api_version"`
}

const APIVersion = 0 // api version, if api version increased, all driver client will restart

func (r GetLatestBlockResponse) CheckAPIVersion() error {
	if r.APIVersion <= APIVersion {
		return nil
	}
	return errors.Errorf("remote api version %d is greater than %d", r.APIVersion, APIVersion)
}

type GetTransactionsParam struct {
	StartHeight uint64 `json:"start_height"`
	EndHeight   uint64 `json:"end_height"`

	// filters are linked by OR
	Filters []TransactionFilter `json:"filters"`
}

type CallFilter struct {
	ContractID string
	Function   *uint64
}

func (f CallFilter) Check(receipts []types.Receipt) int {
	for index, receipt := range receipts {
		if receipt.ReceiptType != "CALL" || receipt.To == nil || receipt.Param1 == nil {
			continue
		}
		// this txn is called function `receipt.Param1` of contract `receipt.To`
		if f.ContractID != "" && receipt.To.String() != f.ContractID {
			continue
		}
		if f.Function != nil && *f.Function != uint64(*receipt.Param1) {
			continue
		}
		return index
	}
	return -1 // not found
}

type TransferFilter struct {
	AssetID string
	From    string
	To      string
}

func (f TransferFilter) Check(txn types.Transaction) bool {
	type pair struct {
		assetID string
		owner   string
	}
	var inputs []pair
	var outputs []pair
	contains := func(set []pair, target pair) bool {
		for _, p := range set {
			if target.assetID != "" && target.assetID != p.assetID {
				continue
			}
			if target.owner != "" && target.owner != p.owner {
				continue
			}
			return true
		}
		return false
	}
	for _, input := range txn.Inputs {
		if input.TypeName_ == "InputCoin" {
			inputs = append(inputs, pair{
				assetID: input.InputCoin.AssetId.String(),
				owner:   input.InputCoin.Owner.String(),
			})
		}
	}
	for _, output := range txn.Outputs {
		switch output.TypeName_ {
		case "CoinOutput":
			outputs = append(outputs, pair{
				assetID: output.CoinOutput.AssetId.String(),
				owner:   output.CoinOutput.To.String(),
			})
		case "ChangeOutput":
			outputs = append(outputs, pair{
				assetID: output.ChangeOutput.AssetId.String(),
				owner:   output.ChangeOutput.To.String(),
			})
		case "VariableOutput":
			outputs = append(outputs, pair{
				assetID: output.VariableOutput.AssetId.String(),
				owner:   output.VariableOutput.To.String(),
			})
		}
	}
	if f.AssetID != "" && !contains(inputs, pair{assetID: f.AssetID}) && !contains(outputs, pair{assetID: f.AssetID}) {
		return false
	}
	if f.From != "" && !contains(inputs, pair{assetID: f.AssetID, owner: f.From}) {
		return false
	}
	if f.To != "" && !contains(outputs, pair{assetID: f.AssetID, owner: f.To}) {
		return false
	}
	return true
}

type LogFilter struct {
	ContractID string
	LogRa      *uint64
	LogRb      *uint64
	LogRc      *uint64
	LogRd      *uint64
}

func (f LogFilter) CheckOne(receipt types.Receipt) bool {
	if receipt.ReceiptType != "LOG" && receipt.ReceiptType != "LOG_DATA" {
		return false
	}
	if f.ContractID != "" && (receipt.Id == nil || !strings.EqualFold(f.ContractID, receipt.Id.String())) {
		return false
	}
	if f.LogRa != nil && (receipt.Ra == nil || *f.LogRa != uint64(*receipt.Ra)) {
		return false
	}
	if f.LogRb != nil && (receipt.Rb == nil || *f.LogRb != uint64(*receipt.Rb)) {
		return false
	}
	if f.LogRc != nil && (receipt.Rc == nil || *f.LogRc != uint64(*receipt.Rc)) {
		return false
	}
	if f.LogRd != nil && (receipt.Rd == nil || *f.LogRd != uint64(*receipt.Rd)) {
		return false
	}
	return true
}

func (f LogFilter) Check(receipts []types.Receipt) []int {
	var indexes []int
	for index, receipt := range receipts {
		if f.CheckOne(receipt) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

type ReceiptTransferFilter struct {
	AssetID string
	From    string
	To      string
}

func (f ReceiptTransferFilter) CheckOne(receipt types.Receipt) bool {
	// TRANSFER_OUT is used to transfer assets to an off-chain address, so just find the TRANSFER receipt
	if receipt.ReceiptType != "TRANSFER" {
		return false
	}
	if f.AssetID != "" && (receipt.AssetId == nil || !strings.EqualFold(f.AssetID, receipt.AssetId.String())) {
		return false
	}
	if f.From != "" && (receipt.Id == nil || !strings.EqualFold(f.From, receipt.Id.String())) {
		return false
	}
	if f.To != "" && (receipt.To == nil || !strings.EqualFold(f.To, receipt.To.String())) {
		return false
	}
	return true
}

func (f ReceiptTransferFilter) Check(receipts []types.Receipt) []int {
	var indexes []int
	for index, receipt := range receipts {
		if f.CheckOne(receipt) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

type TransactionFilter struct {
	// all filters below are linked with AND
	*CallFilter
	*TransferFilter
	*LogFilter
	*ReceiptTransferFilter

	ExcludeFailed bool
}

type TransactionFilterPayload struct {
	// call filter conditions
	CallContractID string  `json:"call_contract_id"`
	CallFunction   *uint64 `json:"call_function"`

	// transfer filter conditions
	TransferAssetID string `json:"transfer_asset_id"`
	TransferFrom    string `json:"transfer_from"`
	TransferTo      string `json:"transfer_to"`

	// log filter condition
	LogContractID string  `json:"log_contract_id"`
	LogRa         *uint64 `json:"log_ra"`
	LogRb         *uint64 `json:"log_rb"`
	LogRc         *uint64 `json:"log_rc"`
	LogRd         *uint64 `json:"log_rd"`

	// receipt transfer condition
	ReceiptTransferAssetID string `json:"receipt_transfer_asset_id"`
	ReceiptTransferFrom    string `json:"receipt_transfer_from"`
	ReceiptTransferTo      string `json:"receipt_transfer_to"`

	ExcludeFailed bool `json:"exclude_failed"`
}

func (f TransactionFilter) MarshalJSON() ([]byte, error) {
	payload := TransactionFilterPayload{
		ExcludeFailed: f.ExcludeFailed,
	}
	if f.CallFilter != nil {
		payload.CallFunction = f.Function
		payload.CallContractID = f.CallFilter.ContractID
	}
	if f.TransferFilter != nil {
		payload.TransferAssetID = f.TransferFilter.AssetID
		payload.TransferFrom = f.TransferFilter.From
		payload.TransferTo = f.TransferFilter.To
	}
	if f.LogFilter != nil {
		payload.LogContractID = f.LogFilter.ContractID
		payload.LogRa = f.LogRa
		payload.LogRb = f.LogRb
		payload.LogRc = f.LogRc
		payload.LogRd = f.LogRd
	}
	if f.ReceiptTransferFilter != nil {
		payload.ReceiptTransferAssetID = f.ReceiptTransferFilter.AssetID
		payload.ReceiptTransferFrom = f.ReceiptTransferFilter.From
		payload.ReceiptTransferTo = f.ReceiptTransferFilter.To
	}
	return json.Marshal(payload)
}

func (f *TransactionFilter) UnmarshalJSON(data []byte) error {
	var payload TransactionFilterPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	f.ExcludeFailed = payload.ExcludeFailed
	if payload.CallContractID != "" || payload.CallFunction != nil {
		f.CallFilter = &CallFilter{
			ContractID: payload.CallContractID,
			Function:   payload.CallFunction,
		}
	}
	if payload.TransferAssetID != "" || payload.TransferFrom != "" || payload.TransferTo != "" {
		f.TransferFilter = &TransferFilter{
			AssetID: payload.TransferAssetID,
			From:    payload.TransferFrom,
			To:      payload.TransferTo,
		}
	}
	if payload.LogContractID != "" ||
		payload.LogRa != nil ||
		payload.LogRb != nil ||
		payload.LogRc != nil ||
		payload.LogRd != nil {
		f.LogFilter = &LogFilter{
			ContractID: payload.LogContractID,
			LogRa:      payload.LogRa,
			LogRb:      payload.LogRb,
			LogRc:      payload.LogRc,
			LogRd:      payload.LogRd,
		}
	}
	if payload.ReceiptTransferAssetID != "" || payload.ReceiptTransferFrom != "" || payload.ReceiptTransferTo != "" {
		f.ReceiptTransferFilter = &ReceiptTransferFilter{
			AssetID: payload.ReceiptTransferAssetID,
			From:    payload.ReceiptTransferFrom,
			To:      payload.ReceiptTransferTo,
		}
	}
	return nil
}

func (f TransactionFilter) IsEmpty() bool {
	return f.CallFilter == nil && f.TransferFilter == nil && f.LogFilter == nil && !f.ExcludeFailed
}

func (f TransactionFilter) Check(txn types.Transaction) bool {
	receipts := GetTxnReceipt(txn.Status)
	if f.ExcludeFailed && txn.Status.TypeName_ != "SuccessStatus" {
		return false
	}
	if f.CallFilter != nil && f.CallFilter.Check(receipts) < 0 {
		return false
	}
	if f.TransferFilter != nil && !f.TransferFilter.Check(txn) {
		return false
	}
	if f.LogFilter != nil && len(f.LogFilter.Check(receipts)) == 0 {
		return false
	}
	if f.ReceiptTransferFilter != nil && len(f.ReceiptTransferFilter.Check(receipts)) == 0 {
		return false
	}
	return true
}

type WrappedTransaction struct {
	BlockHeight      uint64
	TransactionIndex uint64
	types.Transaction
}

func (w WrappedTransaction) GetBlockHeader() *types.Header {
	switch w.Status.TypeName_ {
	case "SuccessStatus":
		return &w.Status.SuccessStatus.Block.Header
	case "FailureStatus":
		return &w.Status.FailureStatus.Block.Header
	default:
		return nil
	}
}

// CheckTransaction filters are linked by OR
func CheckTransaction(tx WrappedTransaction, filters []TransactionFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f.Check(tx.Transaction) {
			return true
		}
	}
	return false
}

// FilterTransactions filters are linked by OR
func FilterTransactions(txns []WrappedTransaction, filters []TransactionFilter) []WrappedTransaction {
	return utils.FilterArr(txns, func(tx WrappedTransaction) bool {
		return CheckTransaction(tx, filters)
	})
}
