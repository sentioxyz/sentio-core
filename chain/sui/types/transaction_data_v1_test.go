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

var testDataFile = "testdata/txs-v1.json"

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
	b, _ := os.ReadFile(testDataFile)
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
		decodedData, err := DecodeSenderSignedData(rawTx.RawTransaction)
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
	b, _ := os.ReadFile(testDataFile)
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
				rawTx.RawTransaction.Data()); err != nil {
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
			})
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, []byte(rawTx.RawTransaction), encodedBCS)
		}
	}
}

func TestDecodeConsensusCommitPrologueV3(t *testing.T) {
	b := []byte(`
{
  "digest": "6xzh4jA4QYnFSiGhY6RjEVLkZreQBqPkBbTBd4dWDUzJ",
  "transaction": {
    "data": {
      "messageVersion": "v1",
      "transaction": {
        "kind": "ConsensusCommitPrologueV3",
        "epoch": "485",
        "round": "1114892",
        "sub_dag_index": null,
        "commit_timestamp_ms": "1723303025583",
        "consensus_commit_digest": "8eERDdfsxcWQDuYmjHJ4itT87YUpxGEh62VRv8NiXWzf",
        "consensus_determined_version_assignments": {
          "CancelledTransactions": []
        }
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
  "effects": {
    "messageVersion": "v1",
    "status": {
      "status": "success"
    },
    "executedEpoch": "485",
    "gasUsed": {
      "computationCost": "0",
      "storageCost": "0",
      "storageRebate": "0",
      "nonRefundableStorageFee": "0"
    },
    "modifiedAtVersions": [
      {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
        "sequenceNumber": "58670927"
      }
    ],
    "sharedObjects": [
      {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
        "version": 58670927,
        "digest": "4o8d2EZQWMpJ8PQ6Jg4L4CUKbvWSaRnc35fXJEJG81uG"
      }
    ],
    "transactionDigest": "6xzh4jA4QYnFSiGhY6RjEVLkZreQBqPkBbTBd4dWDUzJ",
    "mutated": [
      {
        "owner": {
          "Shared": {
            "initial_shared_version": 1
          }
        },
        "reference": {
          "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
          "version": 58670928,
          "digest": "3Mv2ACpuEV3kjVut9n9ZwjT4eXNZbTdAjFrYTXx3fbSE"
        }
      }
    ],
    "gasObject": {
      "owner": {
        "AddressOwner": "0x0000000000000000000000000000000000000000000000000000000000000000"
      },
      "reference": {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "version": 0,
        "digest": "11111111111111111111111111111111"
      }
    },
    "dependencies": [
      "BRLFQqZU5VVD5xj83iEGSvTwW9WKwQmEJ5WDBeoK5jFC"
    ]
  },
  "timestampMs": "1723303025773",
  "checkpoint": "45815591"
}
`)
	var txn TransactionResponseV1
	err := json.Unmarshal(b, &txn)
	assert.NoError(t, err)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV2)
	assert.NotNil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV3)
}

func TestDecodeConsensusCommitPrologueV1(t *testing.T) {
	b := []byte(`{
  "digest": "33k7NKRbFps4LTHY35topzztH6CiNS2zHviwRo4Vi5nZ",
  "transaction": {
    "data": {
      "messageVersion": "v1",
      "transaction": {
        "kind": "ConsensusCommitPrologueV1",
        "epoch": "0",
        "round": "8",
        "sub_dag_index": null,
        "commit_timestamp_ms": "1746433578570",
        "consensus_commit_digest": "GzmmGvaNkYGgGD9833XQbXjYUkRNeEN2cnBr1k9KZQgY",
        "consensus_determined_version_assignments": {
          "CancelledTransactions": []
        }
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
  "rawTransaction": "AQAAAAACAAAAAAAAAAAIAAAAAAAAAABKlo2flgEAACDtrAYrUxNar1y2s7SUsrWYE5Y7adU/FcgcytHLOm6SEQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAABYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  "effects": {
    "messageVersion": "v1",
    "status": {
      "status": "success"
    },
    "executedEpoch": "0",
    "gasUsed": {
      "computationCost": "0",
      "computationCostBurned": "0",
      "storageCost": "0",
      "storageRebate": "0",
      "nonRefundableStorageFee": "0"
    },
    "modifiedAtVersions": [
      {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
        "sequenceNumber": "3"
      }
    ],
    "sharedObjects": [
      {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
        "version": 3,
        "digest": "2UWDPqEUpQmJWjjL57uT53DZRhzkjwV5pSDdTkm4C6Qq"
      }
    ],
    "transactionDigest": "33k7NKRbFps4LTHY35topzztH6CiNS2zHviwRo4Vi5nZ",
    "mutated": [
      {
        "owner": {
          "Shared": {
            "initial_shared_version": 1
          }
        },
        "reference": {
          "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
          "version": 4,
          "digest": "GL6yfisYnSvhiT5GunipDaELjPeLUAuaPjkQDceJDz2d"
        }
      }
    ],
    "gasObject": {
      "owner": {
        "AddressOwner": "0x0000000000000000000000000000000000000000000000000000000000000000"
      },
      "reference": {
        "objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "version": 0,
        "digest": "11111111111111111111111111111111"
      }
    },
    "dependencies": [
      "GMr8FPAb7iwPha9ZmUrNHAhA2wdMr3FhxAKvWMbYnXSw"
    ]
  },
  "events": [],
  "objectChanges": [
    {
      "type": "mutated",
      "sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
      "owner": {
        "Shared": {
          "initial_shared_version": 1
        }
      },
      "objectType": "0x2::clock::Clock",
      "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
      "version": "4",
      "previousVersion": "3",
      "digest": "GL6yfisYnSvhiT5GunipDaELjPeLUAuaPjkQDceJDz2d"
    }
  ],
  "balanceChanges": [],
  "timestampMs": "1746433578570",
  "checkpoint": "3"
}
`)
	var txn TransactionResponseV1
	err := json.Unmarshal(b, &txn)
	assert.NoError(t, err)
	assert.NotNil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV1)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV2)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV3)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV4)
	bb, err := json.MarshalIndent(txn, "", "  ")
	assert.NoError(t, err)
	t.Logf("%s", string(bb))
}

func TestDecodeConsensusCommitPrologueV4(t *testing.T) {
	b := []byte(`
{
    "digest": "5apeWdHZfqMwYQQyEzC4Gd6cmLHSkMhzwK1KrobwBXoR",
    "transaction": {
        "data": {
            "messageVersion": "v1",
            "transaction": {
                "kind": "ConsensusCommitPrologueV4",
                "epoch": "679",
                "round": "68",
                "sub_dag_index": null,
                "commit_timestamp_ms": "1742414481414",
                "consensus_commit_digest": "FuePmPm7Z5vn3JExMHZ8nv9PAUgiZAffMG8tBwKEzGRT",
                "consensus_determined_version_assignments": {
                    "CancelledTransactions": []
                },
                "additional_state_digest": "BeN3q7iX6AMhft2coXcfpwxy7fkQQNDjGmvCWJGnGXmz"
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
    "rawTransaction": "AQAAAAAJpwIAAAAAAABEAAAAAAAAAAAGCP+vlQEAACDdgBlQY/wy+2Vxb7wXrcpQ0ysacPYpLxugbksQJzxMHgAAIJ4nKL06ire/kuJHxDuLrGLdFxw9oDRVTlf7vDQXIDyxAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAABYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    "effects": {
        "messageVersion": "v1",
        "status": {
            "status": "success"
        },
        "executedEpoch": "679",
        "gasUsed": {
            "computationCost": "0",
            "storageCost": "0",
            "storageRebate": "0",
            "nonRefundableStorageFee": "0"
        },
        "modifiedAtVersions": [
            {
                "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
                "sequenceNumber": "361153399"
            }
        ],
        "sharedObjects": [
            {
                "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
                "version": 361153399,
                "digest": "B2k3D7eddLmxQZ2EVHKBuLssa1EkBwhcweGDVHxNSVtW"
            }
        ],
        "transactionDigest": "5apeWdHZfqMwYQQyEzC4Gd6cmLHSkMhzwK1KrobwBXoR",
        "mutated": [
            {
                "owner": {
                    "Shared": {
                        "initial_shared_version": 1
                    }
                },
                "reference": {
                    "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
                    "version": 361153400,
                    "digest": "2Txz6GzPeab3bc2h5LVeVfScwDAPKaDEXiao8py5Yto3"
                }
            }
        ],
        "gasObject": {
            "owner": {
                "AddressOwner": "0x0000000000000000000000000000000000000000000000000000000000000000"
            },
            "reference": {
                "objectId": "0x0000000000000000000000000000000000000000000000000000000000000000",
                "version": 0,
                "digest": "11111111111111111111111111111111"
            }
        },
        "dependencies": [
            "C2fjNT6wPmqVHeDNmQVBUBhXLC6HZ3nS28tvc8MfVMsY"
        ]
    },
    "events": [],
    "objectChanges": [
        {
            "type": "mutated",
            "sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
            "owner": {
                "Shared": {
                    "initial_shared_version": 1
                }
            },
            "objectType": "0x2::clock::Clock",
            "objectId": "0x0000000000000000000000000000000000000000000000000000000000000006",
            "version": "361153400",
            "previousVersion": "361153399",
            "digest": "2Txz6GzPeab3bc2h5LVeVfScwDAPKaDEXiao8py5Yto3"
        }
    ],
    "balanceChanges": [],
    "timestampMs": "1742414481414",
    "checkpoint": "175059431"
}
`)
	var txn TransactionResponseV1
	err := json.Unmarshal(b, &txn)
	assert.NoError(t, err)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV2)
	assert.Nil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV3)
	assert.NotNil(t, txn.Transaction.Data.V1.Kind.ConsensusCommitPrologueV4)
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

func Test_decode(t *testing.T) {
	t.Skip("sui-testnet tx 8E534zkMCtVampdzTu6wB97qzjviz9a1HYXa9PipC8Kg cannot DecodeSenderSignedData")

	rawTxStr := "AQAAAAAKBAEBAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACszPrT8mAAAAAAEACNsDAAAAAAAAAAgYAAAAAAAAAAAIAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACFmFjY3VtdWxhdG9yX3NldHRsZW1lbnQTc2V0dGxlbWVudF9wcm9sb2d1ZQAGAQAAAQEAAQIAAQMAAQMAAQMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACFGFjY3VtdWxhdG9yX21ldGFkYXRhIXJlY29yZF9hY2N1bXVsYXRvcl9vYmplY3RfY2hhbmdlcwADAQAAAQMAAQMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAABYQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	rawTx, err := base64.StdEncoding.DecodeString(rawTxStr)
	assert.NoError(t, err)
	_, err = DecodeSenderSignedData(rawTx)
	assert.NoError(t, err)
}
