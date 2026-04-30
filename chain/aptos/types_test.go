package aptos

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

//go:embed testdata/tx.json
var testJson []byte

func Test_ResourceChange(t *testing.T) {
	var tt = Transaction{}

	err := json.Unmarshal(testJson, &tt)
	assert.NoError(t, err)

	filter := ResourceChangeArgs{
		Addresses:                     []string{"*"},
		ResourceChangesMoveTypePrefix: "0xc6bc659f1649553c1a3fa05d9727433dc03843baac29473c817d06d39e7621ba::lending::Vault",
	}.ChangeFilter()

	filtered := utils.FilterArr(tt.Changes, filter)
	assert.Len(t, filtered, 2)
}

func Test_PruneTransaction(t *testing.T) {
	tx := Transaction{
		Changes: []*WriteSetChange{{
			WriteSetChange: &api.WriteSetChange{
				Type: api.WriteSetChangeVariantWriteResource,
				Inner: &api.WriteSetChangeWriteResource{
					Data: &api.MoveResource{
						Type: "0x1::aa::bb",
					},
				},
			},
		}, {
			WriteSetChange: &api.WriteSetChange{
				Type: api.WriteSetChangeVariantWriteResource,
				Inner: &api.WriteSetChangeWriteResource{
					Data: &api.MoveResource{
						Type: "0x1::aa::cc",
					},
				},
			},
		}},
	}
	c := TransactionFetchConfig{
		NeedAllEvents: false,
		ChangeResourceTypes: move.TypeSet{
			move.MustBuildType("0x1::aa::bb"),
		},
	}

	ntx := c.PruneTransaction(tx, nil)
	assert.Equal(t, 1, len(ntx.Changes))
	assert.Equal(t, utils.WrapPointer("0x1::aa::bb"), GetChangeResourceType(ntx.Changes[0].WriteSetChange))

	// tx not changed
	assert.Equal(t, 2, len(tx.Changes))
}

func Test_ResourceChangeWithGeneric(t *testing.T) {
	var tt = Transaction{}

	err := json.Unmarshal(testJson, &tt)
	assert.NoError(t, err)

	filter := ResourceChangeArgs{
		Addresses:                     []string{"*"},
		ResourceChangesMoveTypePrefix: "0x1::coin::CoinStore",
	}.ChangeFilter()
	filtered := utils.FilterArr(tt.Changes, filter)
	assert.Len(t, filtered, 2)

	// when specified generic type, only one change should be returned
	filter = ResourceChangeArgs{
		Addresses:                     []string{"*"},
		ResourceChangesMoveTypePrefix: "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>",
	}.ChangeFilter()
	filtered = utils.FilterArr(tt.Changes, filter)
	assert.Len(t, filtered, 1)
}

func Test_marshalTxn(t *testing.T) {
	raw := `
{
    "type": "user_transaction",
    "version": "19809710001",
    "hash": "288D87D6E68BE5F59A985A6E527E9834892EF872F72D864D80823B74EC214A0C",
    "gas_used": "279407",
    "success": true,
    "sender": "0x00000000d36864869831eea734a64932ce182b7a99609e1c9fae685deed878af",
    "sequence_number": "0",
    "max_gas_amount": "563170",
    "gas_unit_price": "0",
    "expiration_timestamp_secs": "0",
    "payload": {
        "arguments": [],
        "function": "_::_::_",
        "type": "",
        "type_arguments": []
    },
    "timestamp": "1746707553000000"
}
`
	var inner api.CommittedTransaction
	assert.NoError(t, json.Unmarshal([]byte(raw), &inner))
	tx := Transaction{
		CommittedTransaction: &inner,
		Events: []*Event{{
			Event: &api.Event{
				Type:           "aaa",
				SequenceNumber: 1,
			},
			Index: 1,
		}},
	}
	r, err := json.Marshal(tx)
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"user_transaction","version":"19809710001","hash":"288D87D6E68BE5F59A985A6E527E9834892EF872F72D864D80823B74EC214A0C","accumulator_root_hash":"","state_change_hash":"","event_root_hash":"","gas_used":"279407","success":true,"vm_status":"","changes":null,"events":[{"type":"aaa","guid":null,"sequence_number":"1","data":null,"event_index":1}],"sender":"0x00000000d36864869831eea734a64932ce182b7a99609e1c9fae685deed878af","sequence_number":"0","max_gas_amount":"563170","gas_unit_price":"0","expiration_timestamp_secs":"0","payload":{"arguments":[],"function":"_::_::_","type":"","type_arguments":[]},"signature":null,"timestamp":"1746707553000000","state_checkpoint_hash":null}`, string(r))
	//fmt.Println(string(r))
}

func Test_marshalTxn2(t *testing.T) {
	raw := `
{
  "version": "3000000000",
  "hash": "0x6d0f8c37851c83d709304122cf52be17adc428ffd24d3b3ec1bb5c25b904d33f",
  "state_change_hash": "0xa8e1a7b031767d504b327f3324b3cc0846a5dae94c76bc3bd3c63a4004df87e9",
  "event_root_hash": "0x84614e61e9afd9ba8c3edc2bdbfbafc67a2d7c75cee01ce49d0081aa44281d31",
  "state_checkpoint_hash": null,
  "gas_used": "997",
  "success": true,
  "vm_status": "Executed successfully",
  "accumulator_root_hash": "0x50a7a42231bec198184e44f4d158f4862740eb8633352394519f037be11f5521",
  "changes": [
    {
      "address": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
      "state_key_hash": "0x264bf8d0cf198013bbf2a51f451b3c7bed130cf97585e84c60f5dc92c809c7e4",
      "data": {
        "type": "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>",
        "data": {
          "coin": {
            "value": "13823716300"
          },
          "deposit_events": {
            "counter": "4",
            "guid": {
              "id": {
                "addr": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
                "creation_num": "2"
              }
            }
          },
          "frozen": false,
          "withdraw_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
                "creation_num": "3"
              }
            }
          }
        }
      },
      "type": "write_resource"
    },
    {
      "address": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
      "state_key_hash": "0x283062fae3a12c34c7efa4f68c410b29e8cf67257c7c0b42728b2843f924934a",
      "data": {
        "type": "0x1::account::Account",
        "data": {
          "authentication_key": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
          "coin_register_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
                "creation_num": "0"
              }
            }
          },
          "guid_creation_num": "6",
          "key_rotation_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
                "creation_num": "1"
              }
            }
          },
          "rotation_capability_offer": {
            "for": {
              "vec": []
            }
          },
          "sequence_number": "1583",
          "signer_capability_offer": {
            "for": {
              "vec": []
            }
          }
        }
      },
      "type": "write_resource"
    },
    {
      "address": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
      "state_key_hash": "0x1fe098bfd864aab27cae8c32643dca5ac43f9b40f419547a300ab2dc540ba029",
      "data": {
        "type": "0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>",
        "data": {
          "coin": {
            "value": "0"
          },
          "deposit_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
                "creation_num": "2"
              }
            }
          },
          "frozen": false,
          "withdraw_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
                "creation_num": "3"
              }
            }
          }
        }
      },
      "type": "write_resource"
    },
    {
      "address": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
      "state_key_hash": "0xc1693eb5dfbbe0589d183032554607492772085427ad645536ecc83270ecb5ab",
      "data": {
        "type": "0x1::account::Account",
        "data": {
          "authentication_key": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
          "coin_register_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
                "creation_num": "0"
              }
            }
          },
          "guid_creation_num": "4",
          "key_rotation_events": {
            "counter": "0",
            "guid": {
              "id": {
                "addr": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
                "creation_num": "1"
              }
            }
          },
          "rotation_capability_offer": {
            "for": {
              "vec": []
            }
          },
          "sequence_number": "0",
          "signer_capability_offer": {
            "for": {
              "vec": []
            }
          }
        }
      },
      "type": "write_resource"
    },
    {
      "state_key_hash": "0x6e4b28d40f98a106a65163530924c0dcb40c1349d3aa915d108b4d6cfc1ddb19",
      "handle": "0x1b854694ae746cdbd8d44186ca4929b2b337df21d1c74633be19b2710552fdca",
      "key": "0x0619dc29a0aac8fa146714058e8dd6d2d0f3bdf5f6331907bf91f3acd81e6935",
      "value": "0x4e98636a6a8899010000000000000000",
      "data": null,
      "type": "write_table_item"
    }
  ],
  "sender": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
  "sequence_number": "1582",
  "max_gas_amount": "200000",
  "gas_unit_price": "100",
  "expiration_timestamp_secs": "1751602230",
  "payload": {
    "function": "0x1::aptos_account::create_account",
    "type_arguments": [],
    "arguments": [
      "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420"
    ],
    "type": "entry_function_payload"
  },
  "signature": {
    "sender": {
      "public_key": "0xe623d4401a30dc26bc47448ac49e4ed736cd77f44a242484e944da7eeccac55a",
      "signature": "0x4f23779881272c9f70acdce52774763a98a76808c8cd1600de508c0f6f755b7bb52c961341199e18448e290d88486adb3d7508485956469f0560b7a74fce9305",
      "type": "ed25519_signature"
    },
    "secondary_signer_addresses": [],
    "secondary_signers": [],
    "fee_payer_address": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
    "fee_payer_signer": {
      "public_key": "0xe623d4401a30dc26bc47448ac49e4ed736cd77f44a242484e944da7eeccac55a",
      "signature": "0x4f23779881272c9f70acdce52774763a98a76808c8cd1600de508c0f6f755b7bb52c961341199e18448e290d88486adb3d7508485956469f0560b7a74fce9305",
      "type": "ed25519_signature"
    },
    "type": "fee_payer_signature"
  },
  "replay_protection_nonce": null,
  "events": [
    {
      "guid": {
        "creation_number": "0",
        "account_address": "0x0"
      },
      "sequence_number": "0",
      "type": "0x1::account::CoinRegister",
      "data": {
        "account": "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420",
        "type_info": {
          "account_address": "0x1",
          "module_name": "0x6170746f735f636f696e",
          "struct_name": "0x4170746f73436f696e"
        }
      }
    },
    {
      "guid": {
        "creation_number": "0",
        "account_address": "0x0"
      },
      "sequence_number": "0",
      "type": "0x1::transaction_fee::FeeStatement",
      "data": {
        "execution_gas_units": "5",
        "io_gas_units": "4",
        "storage_fee_octas": "98800",
        "storage_fee_refund_octas": "0",
        "total_charge_gas_units": "997"
      }
    }
  ],
  "timestamp": "1751602210266830",
  "type": "user_transaction"
}
`
	var txn Transaction
	assert.NoError(t, json.Unmarshal([]byte(raw), &txn))
	txn.Events = nil
	txn.Changes = nil
	b, err := json.MarshalIndent(txn, "", "  ")
	assert.NoError(t, err)
	after := `{
  "type": "user_transaction",
  "version": "3000000000",
  "hash": "0x6d0f8c37851c83d709304122cf52be17adc428ffd24d3b3ec1bb5c25b904d33f",
  "accumulator_root_hash": "0x50a7a42231bec198184e44f4d158f4862740eb8633352394519f037be11f5521",
  "state_change_hash": "0xa8e1a7b031767d504b327f3324b3cc0846a5dae94c76bc3bd3c63a4004df87e9",
  "event_root_hash": "0x84614e61e9afd9ba8c3edc2bdbfbafc67a2d7c75cee01ce49d0081aa44281d31",
  "gas_used": "997",
  "success": true,
  "vm_status": "Executed successfully",
  "changes": null,
  "events": null,
  "sender": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
  "sequence_number": "1582",
  "max_gas_amount": "200000",
  "gas_unit_price": "100",
  "expiration_timestamp_secs": "1751602230",
  "payload": {
    "type": "entry_function_payload",
    "function": "0x1::aptos_account::create_account",
    "type_arguments": [],
    "arguments": [
      "0x65c57d232b8fb290bd277c4bd186953023082b4de72c735bf31c3c12dabf3420"
    ]
  },
  "signature": {
    "type": "fee_payer_signature",
    "fee_payer_address": "0x28f25e84444c01667ca83b62e6816465446d08542c9aac7a0486043acc4e3291",
    "fee_payer_signer": {
      "type": "ed25519_signature",
      "public_key": "0xe623d4401a30dc26bc47448ac49e4ed736cd77f44a242484e944da7eeccac55a",
      "signature": "0x4f23779881272c9f70acdce52774763a98a76808c8cd1600de508c0f6f755b7bb52c961341199e18448e290d88486adb3d7508485956469f0560b7a74fce9305"
    },
    "secondary_signer_addresses": [],
    "secondary_signers": [],
    "sender": {
      "type": "ed25519_signature",
      "public_key": "0xe623d4401a30dc26bc47448ac49e4ed736cd77f44a242484e944da7eeccac55a",
      "signature": "0x4f23779881272c9f70acdce52774763a98a76808c8cd1600de508c0f6f755b7bb52c961341199e18448e290d88486adb3d7508485956469f0560b7a74fce9305"
    }
  },
  "timestamp": "1751602210266830",
  "state_checkpoint_hash": null
}`
	assert.Equal(t, after, string(b))
}

func Test_marshalEventExtend(t *testing.T) {
	ev := Event{
		Event: &api.Event{
			Type:           "aaa",
			SequenceNumber: 1,
			RawData:        json.RawMessage("null"),
		},
		Index: 1,
	}

	r, err := json.Marshal(ev)
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"aaa","guid":null,"sequence_number":"1","data":null,"event_index":1}`, string(r))

	r, err = json.Marshal(&ev)
	assert.NoError(t, err)
	assert.Equal(t, `{"type":"aaa","guid":null,"sequence_number":"1","data":null,"event_index":1}`, string(r))

	var ev2 Event
	assert.NoError(t, json.Unmarshal(r, &ev2))
	assert.Equal(t, ev, ev2)
}

func Test_marshalChange(t *testing.T) {
	var c WriteSetChange
	raw := `{
  "type": "write_resource",
  "address": "0x061b1c40cd7e68b9e8ed4bf26a276d44dca0ed23acf62500563a0a515a3375a8",
  "state_key_hash": "0xbbf44e4d60a59f4dab3512900f4836327f6ba4500a337500a6b1884ab2c2b28d",
  "data": {
    "type": "0x1::fungible_asset::FungibleStore",
    "data": {
      "balance": "57934535",
      "frozen": false,
      "metadata": {
        "inner": "0x2b3be0a97a73c87ff62cbdd36837a9fb5bbd1d7f06a73b7ed62ec15c5326c1b8"
      }
    }
  }
}`
	assert.NoError(t, json.Unmarshal([]byte(raw), &c))

	b, err := json.MarshalIndent(c, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, raw, string(b))

	b, err = json.MarshalIndent(&c, "", "  ")
	assert.NoError(t, err)
	assert.Equal(t, raw, string(b))

}

func Test_marshalMinTxWithChanges(t *testing.T) {
	var tx = MinimalistTransactionWithChanges{
		MinimalistTransaction: MinimalistTransaction{
			Version:     1,
			Hash:        "xxx",
			TimestampMS: 36000,
		},
	}

	b, err := json.Marshal(tx)
	assert.NoError(t, err)
	assert.Equal(t, `{"version":1,"hash":"xxx","timestamp":36000,"changes":null}`, string(b))

	var tx2 MinimalistTransactionWithChanges
	assert.NoError(t, json.Unmarshal(b, &tx2))
	assert.Equal(t, tx, tx2)
}

func Test_ChangeFilterMarshal(t *testing.T) {
	cf := ChangeFilter{
		Address: set.New[aptos.AccountAddress](
			aptos.AccountAddress(common.HexToHash("0x1")),
			aptos.AccountAddress(common.HexToHash("0x1234")),
			aptos.AccountAddress(common.HexToHash("0x12345667890")),
		),
	}
	b, err := json.Marshal(cf)
	assert.NoError(t, err)
	fmt.Printf("%s\n", string(b))
	fmt.Printf("%s\n", cf.String())
}

func Test_EventFilterMarshal(t *testing.T) {
	t.Run("no address", func(t *testing.T) {
		ef := EventFilter{
			Type: move.MustBuildType("0x1::coin::COIN"),
		}
		b, err := json.Marshal(ef)
		assert.NoError(t, err)
		log.Infof("%s", string(b))
		var ef2 EventFilter
		assert.NoError(t, json.Unmarshal(b, &ef2))
		assert.Equal(t, ef, ef2)
	})
	t.Run("with address", func(t *testing.T) {
		addr := (aptos.AccountAddress)(common.HexToHash("0x1234"))
		ef := EventFilter{
			Type:              move.MustBuildType("0x1::coin::COIN"),
			GuiAccountAddress: &addr,
		}
		b, err := json.Marshal(ef)
		assert.NoError(t, err)
		log.Infof("%s", string(b))
		var ef2 EventFilter
		assert.NoError(t, json.Unmarshal(b, &ef2))
		assert.Equal(t, ef, ef2)
	})
}
