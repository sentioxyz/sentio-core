package ethereum

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/common"
	"sentioxyz/sentio-core/processor/protos"
	"testing"
)

func Test_listValue(t *testing.T) {
	var st protos.Data_EthLog
	err := json.Unmarshal([]byte(`
{
	"transaction_receipt": {
		"logs": [
      {
        "topics": [
          "0x00"
        ]
      }
		]
	}
}
`), &st)
	assert.NoError(t, err)

	// nil value will not panic
	r := MustBuildTransactionReceipt(st.TransactionReceipt)
	assert.Nil(t, r.TransactionHash)
	assert.Nil(t, r.TransactionIndex)
	assert.Nil(t, r.BlockHash)
	assert.Nil(t, r.BlockNumber)
	assert.Nil(t, r.CumulativeGasUsed)
	assert.Nil(t, r.GasUsed)
	assert.Nil(t, r.ContractAddress)
	assert.Nil(t, r.Status)
	assert.Nil(t, r.Root)
	assert.Nil(t, r.LogsBloom)
}

func Test_missLogs(t *testing.T) {
	var st protos.Data_EthLog
	err := json.Unmarshal([]byte(`{"transaction_receipt": {}}`), &st)
	assert.NoError(t, err)

	// miss logs will panic
	assert.Panics(t, func() {
		_ = MustBuildTransactionReceipt(st.TransactionReceipt)
	})
}

func Test_missTopics(t *testing.T) {
	var st protos.Data_EthLog
	err := json.Unmarshal([]byte(`
{
	"transaction_receipt": {
		"logs": [
      {
      }
		]
	}
}
`), &st)
	assert.NoError(t, err)

	re := MustBuildTransactionReceipt(st.TransactionReceipt)
	assert.Equal(t, 0, len(re.Logs.Data[0].Topics.Data))
}

func Test_normal(t *testing.T) {
	var st protos.Data_EthLog
	err := json.Unmarshal([]byte(`
{
  "log": {
    "address": "0x316f9708bB98af7dA9c68C1C3b5e79039cD336E3",
    "blockHash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
    "blockNumber": "0xe9f10c",
    "data": "0x",
    "logIndex": "0xaa",
    "removed": false,
    "topics": [
      "0x3da528dfe78562a1f409134989443b5f21ee92023a64b90dedeb2002415189b6",
      "0x000000000000000000000000c3d688b66703497daa19211eedff47f25384cdc3",
      "0x00000000000000000000000042f9505a376761b180e27a01ba0554244ed1de7d"
    ],
    "transactionHash": "0xdc9a1d363a70c5473fbfaf54bf85f53082366b9cd4ac608eb2fd1aed67886ec3",
    "transactionIndex": "0x66"
  },
  "timestamp": {
    "seconds": 1660369110
  },
  "transaction": {
    "blockHash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
    "blockNumber": "0xe9f10c",
    "chainId": "0x1",
    "from": "0x343715FA797B8e9fe48b9eFaB4b54f01CA860e78",
    "gas": "0x40ec44",
    "gasPrice": "0x300e98aa3",
    "hash": "0xdc9a1d363a70c5473fbfaf54bf85f53082366b9cd4ac608eb2fd1aed67886ec3",
    "input": "0xc7d20733000000000000000000000000316f9708bb98af7da9c68c1c3b5e79039cd336e3000000000000000000000000c3d688b66703497daa19211eedff47f25384cdc3000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000041c9f7fb900000000000000000000000000000000000000000000000000000000",
    "maxFeePerGas": "0x4176f4e99",
    "maxPriorityFeePerGas": "0x59682f00",
    "nonce": "0x9",
    "r": "0x58b2690bedf7559aaf5625f8951fbf249a2ada453f38f25530639ca4042a2f5b",
    "s": "0x681941444efc4d4063129e174bd0a81f59318789fa708785a699c7d3f590defe",
    "to": "0x1EC63B5883C3481134FD50D5DAebc83Ecd2E8779",
    "transactionIndex": "0x66",
    "type": "0x2",
    "v": "0x1",
    "value": "0x0"
  },
  "transaction_receipt": {
    "blockHash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
    "blockNumber": "0xe9f10c",
    "cumulativeGasUsed": "0xaa79da",
    "gasUsed": "0x3df228",
    "logs": [
      {
        "address": "0x316f9708bB98af7dA9c68C1C3b5e79039cD336E3",
        "blockHash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
        "blockNumber": "0xe9f10c",
        "data": "0x",
        "logIndex": "0xaa",
        "removed": false,
        "topics": [
          "0x3da528dfe78562a1f409134989443b5f21ee92023a64b90dedeb2002415189b6",
          "0x000000000000000000000000c3d688b66703497daa19211eedff47f25384cdc3",
          "0x00000000000000000000000042f9505a376761b180e27a01ba0554244ed1de7d"
        ],
        "transactionHash": "0xdc9a1d363a70c5473fbfaf54bf85f53082366b9cd4ac608eb2fd1aed67886ec3",
        "transactionIndex": "0x66"
      },
      {
        "address": "0xc3d688B66703497DAA19211EEdff47f25384cdc3",
        "blockHash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
        "blockNumber": "0xe9f10c",
        "data": "0x",
        "logIndex": "0xab",
        "removed": false,
        "topics": [
          "0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b",
          "0x00000000000000000000000042f9505a376761b180e27a01ba0554244ed1de7d"
        ],
        "transactionHash": "0xdc9a1d363a70c5473fbfaf54bf85f53082366b9cd4ac608eb2fd1aed67886ec3",
        "transactionIndex": "0x66"
      }
    ],
    "logsBloom": "0x00000000000000000000000000000000400080000000000000000000000000100000000000000000000000000004000000000000000000000000000000000040000000000000000000000000000402000000000000000000000000000020000000000000000000000000000400000000000000000000010000020000000000000000000000000000000000000000000000000040000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000800000004000040000000000000000000000000000000000010",
    "status": "0x1",
    "transactionHash": "0xdc9a1d363a70c5473fbfaf54bf85f53082366b9cd4ac608eb2fd1aed67886ec3",
    "transactionIndex": "0x66",
    "type": "0x2"
  },
  "block": {
    "baseFeePerGas": "0x2a7815ba3",
    "difficulty": "0x2b59d6a535b3e2",
    "extraData": "0x486976656f6e2063612d68656176792d6d756c",
    "gasLimit": "0x1c9c380",
    "gasUsed": "0xd746d8",
    "hash": "0x526b6df4d84756de3e7b0e5b4b495571015b0ec029dc66743461a314b0486f6d",
    "logsBloom": "0x90b05a221d80f1129b808070951112b3616596004c568e24c0d140020408d55005c52760040e90201101d8400a0d19550b2a890c0afdb95218082182106e384124120127a991082be812201879444a7247812a188a55546021c45c2294204c2a5e507502d306358a89ae70849c490969402120218a080558102393740289a43127240f06c62e5054cc69008a5043e540820043e191a0d888892244460c1930059f0c6103012ae6c61a4dc8d38b422041c00e4a4420d0250f23a4228c0319e2085b0d5c2e292c08130209020e008b5aa401189ec884aa003507391f9a021b6001c038e9080634cea41d8737c06968d8220548581f4658405001ed48440d56587c",
    "miner": "0x1aD91ee08f21bE3dE0BA2ba6918E714dA6B45836",
    "mixHash": "0x0f35ef7a9dd84450ac559e391efbe19743e43617fedae1d5397dbd5480c30a79",
    "nonce": "0x90ccd2684f89c7ce",
    "number": "0xe9f10c",
    "parentHash": "0x18db444f962bdd8e87831b5b69135803536408fe051d79ac115ca8b42a404222",
    "receiptsRoot": "0x406227828632f3dff7cd5a055ae778103f78425606156319310e4931d2bea3ad",
    "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
    "size": "0xc9ee",
    "stateRoot": "0xfe46a430d5c2b6383a5ea565197e9f39e20d74cd96c5005953f563f38e350897",
    "timestamp": "0x62f738d6",
    "totalDifficulty": "0xbe8f944db0ed95f5bee",
    "transactionsRoot": "0xb160b0cd9ee265ff3e0c5f7d2da752befaca47ee997cddd08d40695886d8c398"
  }
}
`), &st)
	assert.NoError(t, err)
	assert.NotPanics(t, func() {
		_ = MustBuildTransactionReceipt(st.TransactionReceipt)
	})
}

func Test_marshal(t *testing.T) {
	ev := &Event{
		Address:             common.MustBuildAddressFromString("0xaabbccddaabbccddaabbccddaabbccdd01010101"),
		LogIndex:            common.MustBuildBigInt("1"),
		TransactionLogIndex: common.MustBuildBigInt("2"),
		LogType:             wasm.BuildString("str1"),
		Block: &Block{
			Hash:             wasm.MustBuildByteArrayFromHex("0xabcd0001"),
			ParentHash:       wasm.MustBuildByteArrayFromHex("0xabcd0002"),
			UnclesHash:       wasm.MustBuildByteArrayFromHex("0xabcd0003"),
			Author:           common.MustBuildAddressFromString("0xaabbccddaabbccddaabbccddaabbccdd01010102"),
			StateRoot:        wasm.MustBuildByteArrayFromHex("0xabcd0005"),
			TransactionsRoot: wasm.MustBuildByteArrayFromHex("0xabcd0006"),
			ReceiptsRoot:     nil,
			Number:           common.MustBuildBigInt("3"),
			GasUsed:          common.MustBuildBigInt("4"),
			GasLimit:         common.MustBuildBigInt("5"),
			Timestamp:        common.MustBuildBigInt("6"),
			Difficulty:       common.MustBuildBigInt("7"),
			TotalDifficulty:  common.MustBuildBigInt("8"),
			Size:             common.MustBuildBigInt("9"),
			BaseFeePerGas:    nil,
		},
		Transaction: nil,
		Parameters:  nil,
		Receipt: &TransactionReceipt{
			TransactionHash:   wasm.MustBuildByteArrayFromHex("0xabcd0007"),
			TransactionIndex:  common.MustBuildBigInt("0xa"),
			BlockHash:         wasm.MustBuildByteArrayFromHex("0xabcd0008"),
			BlockNumber:       common.MustBuildBigInt("0xb"),
			CumulativeGasUsed: common.MustBuildBigInt("0xc"),
			GasUsed:           common.MustBuildBigInt("0xd"),
			ContractAddress:   common.MustBuildAddressFromString("0xaabbccddaabbccddaabbccddaabbccdd01010103"),
			Logs: &wasm.ObjectArray[*Log]{
				Data: []*Log{
					{
						Address: common.MustBuildAddressFromString("0xaabbccddaabbccddaabbccddaabbccdd01010104"),
						Topics: &wasm.ObjectArray[*wasm.ByteArray]{
							Data: []*wasm.ByteArray{
								wasm.MustBuildByteArrayFromHex("0xabcd0009"),
								wasm.MustBuildByteArrayFromHex("0xabcd0010"),
							},
						},
						Data:                wasm.MustBuildByteArrayFromHex("0xabcd0011"),
						BlockHash:           wasm.MustBuildByteArrayFromHex("0xabcd0012"),
						BlockNumber:         common.MustBuildBigInt("0xe"),
						TransactionHash:     wasm.MustBuildByteArrayFromHex("0xabcd0013"),
						TransactionIndex:    common.MustBuildBigInt("0xf"),
						LogIndex:            common.MustBuildBigInt("16"),
						TransactionLogIndex: common.MustBuildBigInt("17"),
						LogType:             wasm.BuildString("str2"),
						Removed:             &common.Wrapped[wasm.Bool]{Inner: false},
					},
					{
						Address: common.MustBuildAddressFromString("0xaabbccddaabbccddaabbccddaabbccdd01010105"),
						Topics: &wasm.ObjectArray[*wasm.ByteArray]{
							Data: []*wasm.ByteArray{
								wasm.MustBuildByteArrayFromHex("0xabcd0014"),
								wasm.MustBuildByteArrayFromHex("0xabcd0015"),
							},
						},
						Data:                wasm.MustBuildByteArrayFromHex("0xabcd0016"),
						BlockHash:           wasm.MustBuildByteArrayFromHex("0xabcd0017"),
						BlockNumber:         common.MustBuildBigInt("18"),
						TransactionHash:     wasm.MustBuildByteArrayFromHex("0xabcd0018"),
						TransactionIndex:    common.MustBuildBigInt("19"),
						LogIndex:            common.MustBuildBigInt("20"),
						TransactionLogIndex: common.MustBuildBigInt("21"),
						LogType:             wasm.BuildString("str3"),
						Removed:             nil,
					},
				},
			},
			Status:    common.MustBuildBigInt("22"),
			Root:      wasm.MustBuildByteArrayFromHex("0xabcd0019"),
			LogsBloom: wasm.MustBuildByteArrayFromHex("0xabcd0020"),
		},
	}

	x, _ := json.MarshalIndent(ev, "", "  ")
	assert.Equal(t, `{
  "Address": "0xaabbccddaabbccddaabbccddaabbccdd01010101",
  "LogIndex": "0x1",
  "TransactionLogIndex": "0x2",
  "LogType": "str1",
  "Block": {
    "Hash": "0xabcd0001",
    "ParentHash": "0xabcd0002",
    "UnclesHash": "0xabcd0003",
    "Author": "0xaabbccddaabbccddaabbccddaabbccdd01010102",
    "StateRoot": "0xabcd0005",
    "TransactionsRoot": "0xabcd0006",
    "ReceiptsRoot": null,
    "Number": "0x3",
    "GasUsed": "0x4",
    "GasLimit": "0x5",
    "Timestamp": "0x6",
    "Difficulty": "0x7",
    "TotalDifficulty": "0x8",
    "Size": "0x9",
    "BaseFeePerGas": null
  },
  "Transaction": null,
  "Parameters": null,
  "Receipt": {
    "TransactionHash": "0xabcd0007",
    "TransactionIndex": "0xa",
    "BlockHash": "0xabcd0008",
    "BlockNumber": "0xb",
    "CumulativeGasUsed": "0xc",
    "GasUsed": "0xd",
    "ContractAddress": "0xaabbccddaabbccddaabbccddaabbccdd01010103",
    "Logs": [
      {
        "Address": "0xaabbccddaabbccddaabbccddaabbccdd01010104",
        "Topics": [
          "0xabcd0009",
          "0xabcd0010"
        ],
        "Data": "0xabcd0011",
        "BlockHash": "0xabcd0012",
        "BlockNumber": "0xe",
        "TransactionHash": "0xabcd0013",
        "TransactionIndex": "0xf",
        "LogIndex": "0x10",
        "TransactionLogIndex": "0x11",
        "LogType": "str2",
        "Removed": false
      },
      {
        "Address": "0xaabbccddaabbccddaabbccddaabbccdd01010105",
        "Topics": [
          "0xabcd0014",
          "0xabcd0015"
        ],
        "Data": "0xabcd0016",
        "BlockHash": "0xabcd0017",
        "BlockNumber": "0x12",
        "TransactionHash": "0xabcd0018",
        "TransactionIndex": "0x13",
        "LogIndex": "0x14",
        "TransactionLogIndex": "0x15",
        "LogType": "str3",
        "Removed": null
      }
    ],
    "Status": "0x16",
    "Root": "0xabcd0019",
    "LogsBloom": "0xabcd0020"
  }
}`, string(x))
	fmt.Printf("%s\n", string(x))
}
