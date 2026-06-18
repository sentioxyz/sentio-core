package sol

import (
	"context"
	"encoding/json"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentInstruction struct {
	controller.BaseHandlerAgent

	Address solana.PublicKey

	// configures about building binding data
	ProcessInnerInstruction  bool
	ProcessParsedInstruction bool
	ProcessRawInstruction    bool
	FetchTx                  bool
}

// BuildBindingDataList builds the per-instruction binding data for this handler's program.
//
// NOTE on the BigQuery data source: when a transaction is served from the BigQuery tier (archival
// history below the ClickHouse range), it carries ONLY the instructions of the program(s) the
// super node was queried for — not the transaction's full instruction set (this is a deliberate
// cost optimization in the archival-tier transaction lookup, which is clustered by program_id).
// That is fine here: this handler only emits binding data for instructions whose
// ProgramId == a.Address (the program it targets), which the BigQuery query always includes; and
// the raw transaction attached below (via getTxJSON) still has complete transaction-level data
// (account keys, balances, token balances, logs, status/err, fee, compute units). A handler must
// not rely on unrelated programs' instructions being present in the raw transaction.
func (a HandlerAgentInstruction) BuildBindingDataList(
	ctx context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for _, tx := range bd.mainData.Transactions {
		if tx.Transaction == nil {
			continue
		}
		// get instructions
		var instructions []*rpc.ParsedInstruction
		var indexStartOffset int
		if a.ProcessInnerInstruction && tx.Meta != nil {
			for _, innerInstruction := range tx.Meta.InnerInstructions {
				instructions = append(instructions, innerInstruction.Instructions...)
			}
			indexStartOffset = -len(instructions)
		}
		instructions = append(instructions, tx.Transaction.Message.Instructions...)
		// build binding data for each instruction
		for i, instruction := range instructions {
			if instruction.ProgramId != a.Address {
				continue
			}
			if (!a.ProcessParsedInstruction || instruction.Parsed == nil) && !a.ProcessRawInstruction {
				continue // no data
			}
			dataSize := len(instruction.Accounts)*45 + len(instruction.Data)
			// get parsed
			var rawParsed *string
			if a.ProcessParsedInstruction && instruction.Parsed != nil {
				b, marshalErr := json.Marshal(instruction.Parsed)
				if marshalErr != nil {
					return nil, errors.Wrapf(marshalErr,
						"marshal #%d instruction parsed in transaction %d/%s failed", i, bd.GetBlockNumber(), tx.Signature)
				}
				s := string(b)
				rawParsed = &s
				dataSize += len(b)
			}
			// get instructionData
			var instructionData string
			if a.ProcessRawInstruction {
				instructionData = instruction.Data.String()
			}
			// get raw transaction
			var rawTx *string
			if a.FetchTx {
				rawTxJSON, getRawTxErr := bd.getTxJSON(tx)
				if getRawTxErr != nil {
					return nil, getRawTxErr
				}
				rawTx = &rawTxJSON
				dataSize += len(rawTxJSON)
			}
			// build binding data
			data := &protos.Data{
				Value: &protos.Data_SolInstruction_{
					SolInstruction: &protos.Data_SolInstruction{
						Slot:             bd.GetBlockNumber(),
						ProgramAccountId: a.Address.String(),
						Accounts:         utils.MapSliceNoError(instruction.Accounts, solana.PublicKey.String),
						InstructionData:  instructionData,
						RawParsed:        rawParsed,
						RawTransaction:   rawTx,
					},
				},
			}
			// append result
			// calculate the TxInnerIndex of the binding data
			//  - index of inner instruction is in [-len(InnerInstructions), -1]
			//  - index of normal instruction is in [0, len(NormalInstructions)-1]
			result = append(result, standard.BindingDataInner{
				Data:         data,
				DataSize:     dataSize,
				HandlerType:  protos.HandlerType_SOL_INSTRUCTION,
				TxIndex:      int(tx.TransactionIndex),
				TxInnerIndex: i + indexStartOffset,
			})
		}
	}
	return result, nil
}

func (a HandlerAgentInstruction) Snapshot() any {
	return map[string]any{
		"HandlerID":                a.HandlerID,
		"Range":                    a.Range.String(),
		"Address":                  a.Address.String(),
		"ProcessInnerInstruction":  a.ProcessInnerInstruction,
		"ProcessParsedInstruction": a.ProcessParsedInstruction,
		"ProcessRawInstruction":    a.ProcessRawInstruction,
		"FetchTx":                  a.FetchTx,
	}
}
