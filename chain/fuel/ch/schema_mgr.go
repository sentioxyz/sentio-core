package ch

import (
	"context"
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/chx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

type ClickhouseSchemaMgr struct {
	tablesMeta         clickhouse.TablesMeta
	convertConcurrency uint
}

func tableName(database, tableNamePrefix string) chx.FullName {
	return chx.FullName{
		Database: database,
		Name:     tableNamePrefix + ".transactions",
	}
}

func NewClickhouseSchemaMgr(
	cluster string,
	database string,
	tableNamePrefix string,
	blockPartitionSize uint64,
	convertConcurrency uint,
) *ClickhouseSchemaMgr {
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	table := clickhouse.BuildTable(
		tableName(database, tableNamePrefix),
		&ClickhouseTransaction{},
		chx.TableConfig{
			Engine:      chx.NewDefaultMergeTreeEngine(cluster != ""),
			PartitionBy: fmt.Sprintf("intDiv(block_height, %d)", blockPartitionSize),
			OrderBy:     []string{"block_height", "block_time_ms", "transaction_id"},
			Settings:    tableSettings,
		},
		"",
	)
	return &ClickhouseSchemaMgr{
		tablesMeta: clickhouse.TablesMeta{
			Tables:          []clickhouse.TableSchema{table},
			LinkTableIndex:  -1,
			BlockTableIndex: -1,
		},
		convertConcurrency: convertConcurrency,
	}
}

func (m *ClickhouseSchemaMgr) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}

func (m *ClickhouseSchemaMgr) Convert(_ context.Context, st *fuel.Slot) (clickhouse.Chunk, error) {
	slot := ClickhouseSlot{Transactions: make([]ClickhouseTransaction, len(st.Transactions))}
	headerJSON, err := json.Marshal(st.Block.Header)
	if err != nil {
		return clickhouse.Chunk{}, fmt.Errorf("marshal header of block %d/%s failed: %w",
			st.Block.Header.Height, st.Block.Id.String(), err)
	}
	for i, txn := range st.Transactions {
		receipts := fuel.GetTxnReceipt(txn.Status)
		// about call
		callContracts := make([]string, 0, len(receipts))
		callFunctions := make([]uint64, 0, len(receipts))
		for _, receipt := range receipts {
			if receipt.ReceiptType != "CALL" {
				continue
			}
			if receipt.To != nil {
				callContracts = append(callContracts, receipt.To.String())
			}
			if receipt.Param1 != nil {
				callFunctions = append(callFunctions, (uint64)(*receipt.Param1))
			}
		}
		var createdContracts []string
		for _, out := range txn.Outputs {
			if out.TypeName_ == "ContractCreated" {
				createdContracts = append(createdContracts, out.ContractCreated.Contract.String())
			}
		}
		// about asset transfer
		assets := make(map[string]bool)
		assetInputOwners := make(map[string]bool)
		assetOutputOwners := make(map[string]bool)
		for _, input := range txn.Inputs {
			if input.TypeName_ == "InputCoin" {
				assets[input.InputCoin.AssetId.String()] = true
				assetInputOwners[input.InputCoin.Owner.String()] = true
			}
		}
		for _, output := range txn.Outputs {
			switch output.TypeName_ {
			case "CoinOutput":
				assets[output.CoinOutput.AssetId.String()] = true
				assetOutputOwners[output.CoinOutput.To.String()] = true
			case "ChangeOutput":
				assets[output.ChangeOutput.AssetId.String()] = true
				assetOutputOwners[output.ChangeOutput.To.String()] = true
			case "VariableOutput":
				assets[output.VariableOutput.AssetId.String()] = true
				assetOutputOwners[output.VariableOutput.To.String()] = true
			}
		}
		// about log
		var logRaSet []uint64
		var logRbSet []uint64
		var logRcSet []uint64
		var logRdSet []uint64
		for _, receipt := range receipts {
			if receipt.ReceiptType != "LOG" && receipt.ReceiptType != "LOG_DATA" {
				continue
			}
			if receipt.Ra != nil {
				logRaSet = append(logRaSet, uint64(*receipt.Ra))
			}
			if receipt.Rb != nil {
				logRbSet = append(logRbSet, uint64(*receipt.Rb))
			}
			if receipt.Rc != nil {
				logRcSet = append(logRcSet, uint64(*receipt.Rc))
			}
			if receipt.Rd != nil {
				logRdSet = append(logRdSet, uint64(*receipt.Rd))
			}
		}
		// ---
		txnJSON, err := json.Marshal(txn)
		if err != nil {
			return clickhouse.Chunk{}, fmt.Errorf("marshal txn %s in block %d/%s failed: %w",
				txn.Id.String(), st.Block.Header.Height, st.Block.Id.String(), err)
		}
		slot.Transactions[i] = ClickhouseTransaction{
			BlockID:           st.Block.Id.String(),
			BlockHeight:       uint64(st.Block.Height),
			BlockTimeMs:       uint64(st.Block.Header.Time.Time.UnixMilli()),
			BlockTime:         st.Block.Header.Time.Time,
			BlockHeaderJSON:   string(headerJSON),
			TransactionID:     txn.Id.String(),
			TransactionIndex:  uint32(i),
			CallContracts:     callContracts,
			CallFunctions:     callFunctions,
			CreatedContracts:  createdContracts,
			Assets:            utils.GetOrderedMapKeys(assets),
			AssetInputOwners:  utils.GetOrderedMapKeys(assetInputOwners),
			AssetOutputOwners: utils.GetOrderedMapKeys(assetOutputOwners),
			LogRaSet:          logRaSet,
			LogRbSet:          logRbSet,
			LogRcSet:          logRcSet,
			LogRdSet:          logRdSet,
			IsScript:          bool(txn.IsScript),
			IsCreate:          bool(txn.IsCreate),
			IsMint:            bool(txn.IsMint),
			IsUpgrade:         bool(txn.IsUpgrade),
			IsUpload:          bool(txn.IsUpload),
			Status:            txn.Status.TypeName_,
			TransactionJSON:   string(txnJSON),
		}
	}
	return slot.Values(), nil
}

func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return m.convertConcurrency
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return nil
}
