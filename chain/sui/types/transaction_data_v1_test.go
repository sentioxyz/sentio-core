package types

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/kinbiko/jsonassert"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"sentioxyz/sentio-core/chain/sui/types/serde"
	"sentioxyz/sentio-core/common/log"
)

// transactionBundleFile is a curated corpus of 60 real sui-mainnet
// TransactionResponseV1 replies (mostly diverse ProgrammableTransactions, plus a
// few system txs and one errored tx). It drives the JSON/structpb/BCS coverage
// tests that need breadth a single per-kind sample can't provide:
// TestDecodeV1TransactionFromBCSToJSON, TestDecodeV1TransactionFromJSONToBCS
// (here), TestTransactionResponseV1JSON (rpc_types_test.go) and
// TestTransactionResponseV1Structpb (structpb_test.go). Per-kind byte-exact
// round-trips live in transaction_kind_roundtrip_test.go instead.
var transactionBundleFile = "testdata/sui/transactions-bundle.json"

func unresolvePureValueTypes(j string) string {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(j), &m)
	if err != nil {
		panic(err)
	}
	tx := m["data"].(map[string]interface{})["transaction"].(map[string]interface{})
	if tx["kind"].(string) == "ProgrammableTransaction" {
		inputs := tx["inputs"].([]interface{})
		for i := range inputs {
			input := inputs[i].(map[string]interface{})
			if input["type"].(string) == "pure" {
				var valueType *TypeTag
				if input["valueType"] != nil {
					tt := TypeTagFromStringMust(input["valueType"].(string))
					valueType = &tt
				}
				valueJSONDecoded := input["value"]
				valueJSON, _ := json.Marshal(valueJSONDecoded)
				pureValue := PureValueFromJSON(valueJSON, valueType)
				valueBytes, err := pureValue.RawBytes()
				if err != nil {
					panic(err)
				}
				input["valueType"] = nil
				input["value"] = valueBytes
			}
		}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

func containsStruct(tt *TypeTag) bool {
	if tt.Struct != nil {
		return true
	}
	if tt.Vector != nil {
		return containsStruct(tt.Vector)
	}
	return false
}

func TestDecodeV1TransactionFromBCSToJSON(t *testing.T) {
	b, _ := os.ReadFile(transactionBundleFile)
	var rawTxs []json.RawMessage
	err := json.Unmarshal(b, &rawTxs)
	if err != nil {
		t.Fatal(err)
	}

	serde.Trace = false

	ja := jsonassert.New(t)
	for i := range rawTxs {
		type rawTxJSON struct {
			Digest         string          `json:"digest"`
			Transaction    json.RawMessage `json:"transaction"`
			RawTransaction Base64Data      `json:"rawTransaction"`
		}
		var rawTx rawTxJSON
		err = json.Unmarshal(rawTxs[i], &rawTx)
		if err != nil {
			t.Fatal(err)
		}
		decodedData, err := DecodeSenderSignedData(rawTx.RawTransaction, VariationSUI)
		if err != nil {
			t.Fatal(err)
		}
		decodedTx := decodedData.Transactions[0]
		skip := false
		if len(decodedTx.TxSignatures) > 0 {
			assert.False(t, IsMultiSigBytes(decodedTx.TxSignatures[0]))
			assert.False(t, IsZkLoginSigBytes(decodedTx.TxSignatures[0]))
		}
		if decodedTx.Data.V1.Kind.ProgrammableTransaction != nil {
			commands := decodedTx.Data.V1.Kind.ProgrammableTransaction.Commands
			for i := range commands {
				if commands[i].Publish != nil || commands[i].Upgrade != nil {
					// As there is no aux information available, we cannot effectively
					// translate this kind of transaction from BCS to JSON.
					// This is because we need disassembled form of the package, which is not available in BCS.
					skip = true
				}
				if commands[i].MoveCall != nil {
					for _, tt := range commands[i].MoveCall.TypeArgs {
						// Struct cannot be represented in JSON without aux information.
						if containsStruct(&tt) {
							skip = true
						}
					}
				}
			}
		}
		if decodedTx.Data.V1.Kind.EndOfEpochTransaction != nil {
			skip = true
		}
		if skip {
			continue
		}

		actualJSON, err := json.MarshalIndent(decodedTx, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		expectedJSON := unresolvePureValueTypes(string(rawTx.Transaction))
		ja.Assertf(string(actualJSON), expectedJSON)
	}
}

func TestDecodeV1TransactionFromJSONToBCS(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()
	unresolvedValues := []bool{false}
	b, _ := os.ReadFile(transactionBundleFile)
	var rawTxs []json.RawMessage
	err := json.Unmarshal(b, &rawTxs)
	if err != nil {
		t.Fatal(err)
	}

	serde.Trace = false
	ja := jsonassert.New(t)
	for _, unresolved := range unresolvedValues {
		for i := range rawTxs {
			type rawTxJSON struct {
				Digest         string          `json:"digest"`
				Transaction    json.RawMessage `json:"transaction"`
				RawTransaction Base64Data      `json:"rawTransaction"`
			}
			var rawTx rawTxJSON
			err = json.Unmarshal(rawTxs[i], &rawTx)
			if err != nil {
				t.Fatal(err)
			}

			if len(rawTx.Transaction) == 0 || string(rawTx.Transaction) == "null" {
				continue
			}
			var inputTx *SenderSignedTransaction
			var inputJSON string
			if unresolved {
				inputJSON = unresolvePureValueTypes(string(rawTx.Transaction))
			} else {
				inputJSON = string(rawTx.Transaction)
			}
			err = json.Unmarshal([]byte(inputJSON), &inputTx)
			if err != nil {
				t.Fatal(err)
			}
			if inputTx.Data.V1.Kind.EndOfEpochTransaction != nil {
				continue
			}

			// Apply auxiliary information derived from BCS.
			// The json format, as specified by SUI, only provides disassembled form of the package.
			// As a go program, we do not have the ability to translate that string into binary bytecode.
			// However, the bytecode in binary form is given if we decode rawTransaction BCS.
			if err = DeriveAuxInformationFromBCSV1(inputTx.Data.V1,
				rawTx.RawTransaction.Data(), VariationSUI); err != nil {
				t.Fatal(err)
			}
			if inputTx.Data.V1.Expiration == nil {
				t.Fatal("expiration is nil")
			}

			j, err := json.Marshal(inputTx)
			if err != nil {
				t.Fatal(err)
			}
			ja.Assertf(string(j), inputJSON)

			inputTx.Intent = &IntentMessage{0, 0, 0}
			encodedBCS, err := EncodeSenderSignedData(&SenderSignedData{
				Transactions: []SenderSignedTransaction{*inputTx},
			}, VariationSUI)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, []byte(rawTx.RawTransaction), encodedBCS)
		}
	}
}

func TestUnmarshalEndOfEpochTransaction(t *testing.T) {
	bs := []string{
		`
{
		"digest": "ENrSxeQZU799AgQy6Zu7gBkQYbJxg1Dmddu2AtQn9DGa",
		"transaction": {
				"data": {
						"messageVersion": "v1",
						"transaction": {
								"kind": "EndOfEpochTransaction",
								"transactions": [
										{
												"AuthenticatorStateExpire": {
														"min_epoch": "512"
												}
										},
										{
												"BridgeStateCreate": "4btiuiMPvEENsttpZC7CZ53DruC3MAgfznDbASZ7DR6S"
										},
										{
												"ChangeEpoch": {
														"epoch": "513",
														"storage_charge": "36983455836400",
														"computation_charge": "8317341716428",
														"storage_rebate": "36346412715624",
														"epoch_start_timestamp_ms": "1725644149511"
												}
										}
								]
						},
						"sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
						"gasData": {
								"payment": [
										{
												"objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
												"version": 0,
												"digest": "11111111111111111111111111111111"
										}
								],
								"owner": "0x0000000000000000000000000000000000000000000000000000000000000000",
								"price": "1",
								"budget": "0"
						}
				},
				"txSignatures": [
						"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
				]
		},
		"timestampMs": "1725644149511",
		"checkpoint": "55455583"
}`,
		`
{
		"digest": "GPF674j88LcajcZBscd3SMwpQGJsYKeVtEy3HRMo5wSR",
		"transaction": {
				"data": {
						"messageVersion": "v1",
						"transaction": {
								"kind": "EndOfEpochTransaction",
								"transactions": [
										{
												"AuthenticatorStateExpire": {
														"min_epoch": "527"
												}
										},
										{
												"BridgeCommitteeUpdate": 332139966
										},
										{
												"ChangeEpoch": {
														"epoch": "528",
														"storage_charge": "41521478004400",
														"computation_charge": "9335078020592",
														"storage_rebate": "40343393758932",
														"epoch_start_timestamp_ms": "1726940162130"
												}
										}
								]
						},
						"sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
						"gasData": {
								"payment": [
										{
												"objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
												"version": 0,
												"digest": "11111111111111111111111111111111"
										}
								],
								"owner": "0x0000000000000000000000000000000000000000000000000000000000000000",
								"price": "1",
								"budget": "0"
						}
				},
				"txSignatures": [
						"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
				]
		},
		"timestampMs": "1726940162130",
		"checkpoint": "60782842"
}`,
	}

	for _, b := range bs {
		var txn TransactionResponseV1
		err := json.Unmarshal([]byte(b), &txn)
		assert.NoError(t, err)
		assert.NotNil(t, txn.Transaction.Data.V1.Kind.EndOfEpochTransaction)

		_, err = json.MarshalIndent(txn, "", "  ")
		assert.NoError(t, err)
		//fmt.Printf("%s\n", string(x))
	}
}

func TestUnmarshalEndOfEpochTransaction2(t *testing.T) {
	b := []byte(`
{
    "digest": "9FAqTdgdr78qBKRV9evasQzjJQepbQ5Cbk3Cie5tN9Fx",
    "transaction": {
        "data": {
            "messageVersion": "v1",
            "transaction": {
                "kind": "EndOfEpochTransaction",
                "transactions": [
                    {
                        "ChangeEpochV2": {
                            "epoch": "1",
                            "storage_charge": "673003833600",
                            "computation_charge": "19805700000",
                            "computation_charge_burned": "18774000000",
                            "storage_rebate": "622517124800",
                            "epoch_start_timestamp_ms": "1746517145196"
                        }
                    }
                ]
            },
            "sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "gasData": {
                "payment": [
                    {
                        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
                        "version": 0,
                        "digest": "11111111111111111111111111111111"
                    }
                ],
                "owner": "0x0000000000000000000000000000000000000000000000000000000000000000",
                "price": "1",
                "budget": "0"
            }
        },
        "txSignatures": [
            "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=="
        ]
    },
    "rawTransaction": "AQAAAAAEAQEBAAAAAAAAAAYAAAAAAAAAAIknspwAAACg/4KcBAAAAICBBF8EAAAAwH7p8JAAAAAAAAAAAAAAAGy2iKSWAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAWEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    "balanceChanges": [],
    "timestampMs": "1746517145196",
    "checkpoint": "366216"
}
`)

	var txn TransactionResponseV1
	err := json.Unmarshal(b, &txn)
	assert.NoError(t, err)
	assert.NotNil(t, txn.Transaction.Data.V1.Kind.EndOfEpochTransaction)
	assert.Equal(t, 1, len(txn.Transaction.Data.V1.Kind.EndOfEpochTransaction.Transactions))
	assert.NotNil(t, txn.Transaction.Data.V1.Kind.EndOfEpochTransaction.Transactions[0].ChangeEpochV2)
}

// TestDecodeAccumulatorSettlementPTB decodes an accumulator-settlement system PTB
// (nested accumulator move calls) that previously could not be decoded.
func TestDecodeAccumulatorSettlementPTB(t *testing.T) {
	rawTxStr := "AQAAAAAKBAEBAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACszPrT8mAAAAAAEACNsDAAAAAAAAAAgYAAAAAAAAAAAIAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACFmFjY3VtdWxhdG9yX3NldHRsZW1lbnQTc2V0dGxlbWVudF9wcm9sb2d1ZQAGAQAAAQEAAQIAAQMAAQMAAQMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACFGFjY3VtdWxhdG9yX21ldGFkYXRhIXJlY29yZF9hY2N1bXVsYXRvcl9vYmplY3RfY2hhbmdlcwADAQAAAQMAAQMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAABYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	rawTx, err := base64.StdEncoding.DecodeString(rawTxStr)
	assert.NoError(t, err)
	_, err = DecodeSenderSignedData(rawTx, VariationSUI)
	assert.NoError(t, err)
}
