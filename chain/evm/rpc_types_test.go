package evm

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	json2 "github.com/goccy/go-json"
	"github.com/vmihailenco/msgpack/v5"
)

var rawLogJSON = `{
	"address": "0x00000000006c3852cbef3e08e8df289169ede581",
	"topics": [
		"0x9d9af8e38d66c62e2c12f0225249fd9d721c54b83f48d9352c97c6cacdcb6f31",
		"0x00000000000000000000000055bef545ae02b2403e760eb26fdfc34b32e1e72c",
		"0x000000000000000000000000004c00500000ad104d7dbd00e3ae0a5c00560c00"
	],
	"data": "0xcff8a50083410f7bc3a4f1307241b1278e4f15ccfd2ceb975627677bd61c98410000000000000000000000004dc583548a3407a5c5f6468ccc964523cb14e9d1000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000001be25a7a70a4fd35599bf70606e51eba74c23af000000000000000000000000000000000000000000000000000000000000040400000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a841ab489000000000000000000000000000055bef545ae02b2403e760eb26fdfc34b32e1e72c000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000048c2739500000000000000000000000000000000a26b00c1f0df003000390027140000faa71900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000009184e72a000000000000000000000000000034d726a6cc477bec0d4551e229ab6e1d4961ab9a",
	"blockNumber": "0xf42400",
	"transactionHash": "0x9b46fcf6ece683021265148fafebb3451f67561fc6f517e729f4996c6ac0ba35",
	"transactionIndex": "0xa5",
	"blockHash": "0x3dc4ef568ae2635db1419c5fec55c4a9322c05302ae527cd40bff380c1d465dd",
	"logIndex": "0x83",
	"removed": false
}`

var logCount = 10000

func initLogs(size int) []types.Log {
	var log types.Log
	if err := log.UnmarshalJSON([]byte(rawLogJSON)); err != nil {
		panic(err)
	}
	var logs []types.Log
	for i := 0; i < size; i++ {
		logs = append(logs, log)
	}
	return logs
}

func initLogsJSON(size int) []byte {
	logs := initLogs(size)
	j, err := json.Marshal(logs)
	if err != nil {
		panic(err)
	}
	return j
}

func initLogsMsgpack(size int) []byte {
	logs := initLogs(size)
	j, err := msgpack.Marshal(logs)
	if err != nil {
		panic(err)
	}
	return j
}

func initLogsGOB(size int) []byte {
	logs := initLogs(size)
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(logs)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func BenchmarkLogEncodeJSON(b *testing.B) {
	logs := initLogs(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(logs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogDecodeJSON(b *testing.B) {
	j := initLogsJSON(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decodedLogs []types.Log
		err := json.Unmarshal(j, &decodedLogs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogEncodeJSONGoccy(b *testing.B) {
	logs := initLogs(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json2.Marshal(logs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogDecodeJSONGoccy(b *testing.B) {
	j := initLogsJSON(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decodedLogs []types.Log
		err := json2.Unmarshal(j, &decodedLogs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogEncodeGOB(b *testing.B) {
	logs := initLogs(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := new(bytes.Buffer)
		err := gob.NewEncoder(buf).Encode(logs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogDecodeGOB(b *testing.B) {
	g := initLogsGOB(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decodedLogs []types.Log
		err := gob.NewDecoder(bytes.NewReader(g)).Decode(&decodedLogs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogEncodeMsgpack(b *testing.B) {
	logs := initLogs(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := msgpack.Marshal(logs)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkLogDecodeMsgpack(b *testing.B) {
	j := initLogsMsgpack(logCount)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decodedLogs []types.Log
		err := msgpack.Unmarshal(j, &decodedLogs)
		if err != nil {
			panic(err)
		}
	}
}

func TestTraceFilterArgsMarshalUnmarshalShouldEqual(t *testing.T) {
	fromBlock := hexutil.Uint64(1)
	toBlock := hexutil.Uint64(2)
	address1 := common.HexToAddress("0x1")
	address2 := common.HexToAddress("0x2")
	traceArgs := &TraceFilterArgs{
		FromBlock:   &fromBlock,
		ToBlock:     &toBlock,
		FromAddress: []common.Address{address1, address2},
		ToAddress:   []string{address1.Hex()},
	}
	j, err := json.Marshal(traceArgs)
	assert.NoError(t, err)
	var decodedArgs TraceFilterArgs
	err = json.Unmarshal(j, &decodedArgs)
	assert.NoError(t, err)
	assert.Equal(t, traceArgs, &decodedArgs)
}

func TestEthGetLogsArgsMarshalUnmarshalShouldEqual(t *testing.T) {
	fromBlock := hexutil.Uint64(1)
	toBlock := hexutil.Uint64(2)
	address1 := common.HexToAddress("0x1")
	address2 := common.HexToAddress("0x2")
	hash1 := common.HexToHash("0x1")
	hash2 := common.HexToHash("0x2")
	traceArgs := &EthGetLogsArgs{
		FromBlock: &fromBlock,
		ToBlock:   &toBlock,
		Addresses: []common.Address{address1, address2},
		Topics:    [][]common.Hash{{hash1, hash2}},
	}
	j, err := json.Marshal(traceArgs)
	assert.NoError(t, err)
	var decodedArgs EthGetLogsArgs
	err = json.Unmarshal(j, &decodedArgs)
	assert.NoError(t, err)
	assert.Equal(t, traceArgs, &decodedArgs)

}

func TestUnmarshalGethTraceBlockResult(t *testing.T) {
	{
		raw := `
{
  "txHash": "0x2f039bd03fd398c31a23e71b48c16349bd36bffdec0e8340db1386e8f19221dc",
  "result": {
    "from": "0xc8afc8ba4f0611c56a9afe4d9ec71e1693b01b81",
    "gas": "0x7fd1",
    "gasUsed": "0x60fe",
    "to": "0x59916da825d2d2ec1bf878d71c88826f6633ecca",
    "input": "0x49290c1c000000000000000000000000c8afc8ba4f0611c56a9afe4d9ec71e1693b01b81cb5c0de249c8eafeaf8470c27bc89f309850dcda21f9b0303dd5ea8db2ef0eee",
    "value": "0x3b1dfde91000",
    "type": "CALL"
  }
}`
		var r GethTraceBlockResult
		assert.NoError(t, json.Unmarshal([]byte(raw), &r))
		assert.NotNil(t, r.TxHash)
		assert.Equal(t, "0x2f039bd03fd398c31a23e71b48c16349bd36bffdec0e8340db1386e8f19221dc", r.TxHash.String())
	}
	{
		raw := `
{
  "result": {
    "from": "0xc8afc8ba4f0611c56a9afe4d9ec71e1693b01b81",
    "gas": "0x7fd1",
    "gasUsed": "0x60fe",
    "to": "0x59916da825d2d2ec1bf878d71c88826f6633ecca",
    "input": "0x49290c1c000000000000000000000000c8afc8ba4f0611c56a9afe4d9ec71e1693b01b81cb5c0de249c8eafeaf8470c27bc89f309850dcda21f9b0303dd5ea8db2ef0eee",
    "value": "0x3b1dfde91000",
    "type": "CALL"
  }
}`
		var r GethTraceBlockResult
		assert.NoError(t, json.Unmarshal([]byte(raw), &r))
		assert.Nil(t, r.TxHash)
		assert.Equal(t, common.HexToAddress("0xc8afc8ba4f0611c56a9afe4d9ec71e1693b01b81"), *r.Result.From)
	}
	{
		raw := `
{
  "from": "0xc8afc8ba4f0611c56a9afe4d9ec71e1693b01b81",
  "gas": "0x7fd1",
  "gasUsed": "0x60fe",
  "to": "0x59916da825d2d2ec1bf878d71c88826f6633ecca",
  "input": "0x49290c1c000000000000000000000000c8afc8ba4f0611c56a9afe4d9ec71e1693b01b81cb5c0de249c8eafeaf8470c27bc89f309850dcda21f9b0303dd5ea8db2ef0eee",
  "value": "0x3b1dfde91000",
  "type": "CALL"
}`
		var r GethTraceBlockResult
		assert.NoError(t, json.Unmarshal([]byte(raw), &r))
		assert.Nil(t, r.TxHash)
		assert.Equal(t, common.HexToAddress("0xc8afc8ba4f0611c56a9afe4d9ec71e1693b01b81"), *r.Result.From)
	}
}

func TestUnmarshalRequestArgs(t *testing.T) {
	json1 := `
		{
      "address": [
        "0xc852ac7aae4b0f0a0deb9e8a391eba2047d80026"
       ],
			"fromBlock": "0xf42400",
			"toBlock": "0xf42500"
		}`
	var getLogArgs *EthGetLogsArgs
	if err := json.Unmarshal([]byte(json1), &getLogArgs); err != nil {
		t.Fatal(err)
	}
	if len(getLogArgs.Addresses) != 1 {
		t.Fatal("expected 1 address")
	}
	if *getLogArgs.FromBlock != 16000000 {
		t.Fatalf("expected fromBlock to be 16000000, actual %d", *getLogArgs.FromBlock)
	}
	if *getLogArgs.ToBlock != 16000256 {
		t.Fatalf("expected toBlock to be 16000256, actual %d", *getLogArgs.ToBlock)
	}

	getLogArgs = nil
	json1 = `
		{
      "address": [
        "0xc852ac7aae4b0f0a0deb9e8a391eba2047d80026"
       ],
			"topics": [
				"0x9c700540bafaacdc1c1b9e3fa73f38d872b7acadbf89111a96d6b857c5574fb9"
			],
			"fromBlock": "0xf42400",
			"toBlock": "0xf42500"
		}`
	if err := json.Unmarshal([]byte(json1), &getLogArgs); err != nil {
		t.Fatal(err)
	}
	if len(getLogArgs.Addresses) != 1 {
		t.Fatal("expected 1 address")
	}
	if *getLogArgs.FromBlock != 16000000 {
		t.Fatalf("expected fromBlock to be 16000000, actual %d", *getLogArgs.FromBlock)
	}
	if *getLogArgs.ToBlock != 16000256 {
		t.Fatalf("expected toBlock to be 16000256, actual %d", *getLogArgs.ToBlock)
	}
	if len(getLogArgs.Topics) != 1 {
		t.Fatal("expected 1 topic")
	}
	if len(getLogArgs.Topics[0]) != 1 {
		t.Fatal("expected 1 topic")
	}
}

func TestRPCGetBlockResponse_UnmarshalJSON1(t *testing.T) {
	raw := `{
        "parentHash": "0xed6d282357ed5baaac56b8256f15b8f1cb93c0249bdc47416f59a4835b841718",
        "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
        "miner": "0x0000000000000000000000000000000000000000",
        "stateRoot": "0x618da2828be9d11aa6e7bdbed99f3b1c656716234364f8000f5089460dc0889e",
        "transactionsRoot": "0x8a59f99f6129ac1558956e33dc7eab72c980bf39b18f5e9143238a9783e3dbc5",
        "receiptsRoot": "0xd6c76504253021d37310e9bfb881824c697cad16a23d41ebfaf90e246df3905a",
        "logsBloom": "0x01040002800000000200010000800002040000000020080000000000000000000000040000400000011024000010005001000800004020000002000000200000002000000000000800000008004000008000800000480000100000008000020400000000020000082448010402000800800040000000040200000010000830000000610000000010000000800080008000200001000000000000000020000000800000000000200000010000020000208000000000008000000000000000000400000003000000802000020000000200000000400000800000000002000028000080000000000000080000000000000000000000080000401000000010000002",
        "difficulty": "0x0",
        "number": "0x97c056",
        "gasLimit": "0x12a05f200",
        "gasUsed": "0x14a923",
        "timestamp": "0x67bd2531",
        "timestampNano": "0x18275136136983a7",
        "extraData": "0x2996d9a70000000024b76b1b",
        "mixHash": "0x708fc380f649357afb1d0b4479eabc4f4e0f1d417f47a0fc6a7280549e21a573",
        "nonce": "0x0000000000000000",
        "baseFeePerGas": "0xba43b7400",
        "hash": "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276",
        "epoch": "0x323b",
        "totalDifficulty": "0x0",
        "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "blobGasUsed": "0x0",
        "excessBlobGas": "0x0",
        "transactions": [
            "0x10cdba24e6a8abd2decb678474f4472a6e594ffabbd2c06370789fe03e61dc10",
            "0xda94dd6960b13498c7afc2ece8bcdcdf0afac1267cd08b57cd40a05be4a3a4a3",
            "0x9692af282f298af1e0e9317417ec914fea194a487872c47e89eb37a73ae4e913"
        ],
        "size": "0x64f",
        "uncles": []
    }`
	var r RPCGetBlockResponse
	assert.NoError(t, json.Unmarshal([]byte(raw), &r))
	assert.Equal(t, "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276", r.Hash.String())
	assert.Equal(t, uint64(9945174), r.Number.Uint64())
	assert.Equal(t, "0x10cdba24e6a8abd2decb678474f4472a6e594ffabbd2c06370789fe03e61dc10", r.TxHashes[0].String())
	assert.Equal(t, "0xda94dd6960b13498c7afc2ece8bcdcdf0afac1267cd08b57cd40a05be4a3a4a3", r.TxHashes[1].String())
	assert.Equal(t, "0x9692af282f298af1e0e9317417ec914fea194a487872c47e89eb37a73ae4e913", r.TxHashes[2].String())

}

func TestRPCGetBlockResponse_UnmarshalJSON2(t *testing.T) {
	raw := `{
        "parentHash": "0xed6d282357ed5baaac56b8256f15b8f1cb93c0249bdc47416f59a4835b841718",
        "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
        "miner": "0x0000000000000000000000000000000000000000",
        "stateRoot": "0x618da2828be9d11aa6e7bdbed99f3b1c656716234364f8000f5089460dc0889e",
        "transactionsRoot": "0x8a59f99f6129ac1558956e33dc7eab72c980bf39b18f5e9143238a9783e3dbc5",
        "receiptsRoot": "0xd6c76504253021d37310e9bfb881824c697cad16a23d41ebfaf90e246df3905a",
        "logsBloom": "0x01040002800000000200010000800002040000000020080000000000000000000000040000400000011024000010005001000800004020000002000000200000002000000000000800000008004000008000800000480000100000008000020400000000020000082448010402000800800040000000040200000010000830000000610000000010000000800080008000200001000000000000000020000000800000000000200000010000020000208000000000008000000000000000000400000003000000802000020000000200000000400000800000000002000028000080000000000000080000000000000000000000080000401000000010000002",
        "difficulty": "0x0",
        "number": "0x97c056",
        "gasLimit": "0x12a05f200",
        "gasUsed": "0x14a923",
        "timestamp": "0x67bd2531",
        "timestampNano": "0x18275136136983a7",
        "extraData": "0x2996d9a70000000024b76b1b",
        "mixHash": "0x708fc380f649357afb1d0b4479eabc4f4e0f1d417f47a0fc6a7280549e21a573",
        "nonce": "0x0000000000000000",
        "baseFeePerGas": "0xba43b7400",
        "hash": "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276",
        "epoch": "0x323b",
        "totalDifficulty": "0x0",
        "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
        "blobGasUsed": "0x0",
        "excessBlobGas": "0x0",
        "transactions": [
            {
                "blockHash": "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276",
                "blockNumber": "0x97c056",
                "from": "0x872251f2c0cc5699c9e0c226371c4d747fda247f",
                "gas": "0x4c4b40",
                "gasPrice": "0xf5de81400",
                "hash": "0x10cdba24e6a8abd2decb678474f4472a6e594ffabbd2c06370789fe03e61dc10",
                "input": "0xa0f15331dd6d9c6af47561f04ad759cc06c34e342ef5a813553f4552c2c705bab8f1eef600000000000000000000000000000000000000000000000000d8e6240c5ea40dfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffbbe54000000000000000000000000000000000000000000000000000000000004418600000000000000000000000000000000000000000000000000000000000f4240",
                "nonce": "0x11cdb",
                "to": "0xf7e7285ebe537fdf1c1c4432aa1863721eac9a09",
                "transactionIndex": "0x0",
                "value": "0x0",
                "type": "0x0",
                "v": "0x147",
                "r": "0x677221f2986231a16610d72fc6abd3b922f79f00fbce746193d26e89ee5705e1",
                "s": "0x5e2434f07f26814c1dc9c1b061ebf9a7350a33fc17c3a7e747303876fa5354b",
                "maxFeePerBlobGas": null,
                "blobVersionedHashes": null
            },
            {
                "blockHash": "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276",
                "blockNumber": "0x97c056",
                "from": "0x79affe26c6e008a67ba8f6f2235395af049e1f82",
                "gas": "0x52f2e",
                "gasPrice": "0xd152f5b80",
                "hash": "0xda94dd6960b13498c7afc2ece8bcdcdf0afac1267cd08b57cd40a05be4a3a4a3",
                "input": "0x73fc4457000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000001cc01af006079affe26c6e008a67ba8f6f2235395af049e1f82000000000000000000000000000000000000000029219dd400f2bf60e5a23d13be72b486d4038894e000ebe000f0f800f5b800f78baee74606e85cc95851b3d968540ef2fe6676bfd2cd0f9e1b820d7e446f5d5e5626721d53eb37fd6f79dd5a1497eb0f2db093782a62d608538cc2853f7e2d161c000139041f1b366fe33f9a5a79de5120f2aee2577ebcc00101e067bd2655e00b99f709f800b80dd9f0b0cc368b3400c008e316e634bf8c00039e2fb66102314ce7b64ce5ce3e5183bc94ad380201090300f6128acb08f80100000000000000000000000000000000000000000000000000000001000276a400000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000014039e2fb66102314ce7b64ce5ce3e5183bc94ad380000000000000000000000000101230200480301270300f604012900809f46dd8f2a4016c26c1cf1f4ef90e5e1928d756b00070a0000000000000000000000000000000000000000000000000000000000000301ce050020040000011d0123000000004001a901ba01ba07002001ef01f500000000000000000000000000000000000000000000",
                "nonce": "0x2c",
                "to": "0xba7bac71a8ee550d89b827fe6d67bc3dca07b104",
                "transactionIndex": "0x1",
                "value": "0xde2d3c7b26b4ac000",
                "type": "0x0",
                "v": "0x147",
                "r": "0x4286b8dddb983e64643c3dceafe88720ecf07e1e9f99840cfc77823354e2ec8f",
                "s": "0x108c7efed0b78930d30f8dc3addaf3c262f06b1ae972a1e15720c37121e0b992",
                "maxFeePerBlobGas": null,
                "blobVersionedHashes": null
            },
            {
                "blockHash": "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276",
                "blockNumber": "0x97c056",
                "from": "0x451cfbe715cce467b201c68d411b75fe7032e308",
                "gas": "0xfc9f",
                "gasPrice": "0xd152f5b80",
                "hash": "0x9692af282f298af1e0e9317417ec914fea194a487872c47e89eb37a73ae4e913",
                "input": "0x2e1a7d4d00000000000000000000000000000000000000000000000cd7a5ded99a0a0000",
                "nonce": "0xa",
                "to": "0x039e2fb66102314ce7b64ce5ce3e5183bc94ad38",
                "transactionIndex": "0x2",
                "value": "0x0",
                "type": "0x0",
                "v": "0x148",
                "r": "0xab4ac5da75086d5a97f10e0e39a2619c9bc8f2b27da7b5a50a81323663ff7b9a",
                "s": "0x69d0343c85faa46506b475a6b4b6c15d4bf3cddddf238e3b561c5e530be5a0a",
                "maxFeePerBlobGas": null,
                "blobVersionedHashes": null
            }
        ],
        "size": "0x64f",
        "uncles": []
    }
`
	var r RPCGetBlockResponse
	assert.NoError(t, json.Unmarshal([]byte(raw), &r))
	assert.Equal(t, "0xcca89e6995c4d74fdad8ef27e8a4cc7b151c64372e24da875a6ebb175b634276", r.Hash.String())
	assert.Equal(t, uint64(9945174), r.Number.Uint64())
	assert.Equal(t, "0x10cdba24e6a8abd2decb678474f4472a6e594ffabbd2c06370789fe03e61dc10", r.Transactions[0].Hash.String())
	assert.Equal(t, "0xda94dd6960b13498c7afc2ece8bcdcdf0afac1267cd08b57cd40a05be4a3a4a3", r.Transactions[1].Hash.String())
	assert.Equal(t, "0x9692af282f298af1e0e9317417ec914fea194a487872c47e89eb37a73ae4e913", r.Transactions[2].Hash.String())
}

func TestUnmarshalTxNonce(t *testing.T) {
	t.Run("case1", func(t *testing.T) {
		raw := `{"nonce": "0x0000000000000000","s":"0x0ed24aac7c4dd48d3e4f13e5eafcaa7d662aa6702e28d6f2eac83fc27bcfd486"}`
		var x RPCTransaction
		assert.NoError(t, json.Unmarshal([]byte(raw), &x))
		assert.Equal(t, uint64(0), uint64(x.Nonce))
		b, err := json.Marshal(x.Nonce)
		assert.NoError(t, err)
		assert.Equal(t, `"0x0"`, string(b))
		b, err = json.Marshal(x.S)
		assert.NoError(t, err)
		assert.Equal(t, `"0xed24aac7c4dd48d3e4f13e5eafcaa7d662aa6702e28d6f2eac83fc27bcfd486"`, string(b))
	})
	t.Run("case2", func(t *testing.T) {
		raw := `{"nonce": "0x4a8d85","s":"0xed24aac7c4dd48d3e4f13e5eafcaa7d662aa6702e28d6f2eac83fc27bcfd486"}`
		var x RPCTransaction
		assert.NoError(t, json.Unmarshal([]byte(raw), &x))
		assert.Equal(t, uint64(4885893), uint64(x.Nonce))
		b, err := json.Marshal(x.Nonce)
		assert.NoError(t, err)
		assert.Equal(t, `"0x4a8d85"`, string(b))
		b, err = json.Marshal(x.S)
		assert.NoError(t, err)
		assert.Equal(t, `"0xed24aac7c4dd48d3e4f13e5eafcaa7d662aa6702e28d6f2eac83fc27bcfd486"`, string(b))
	})
	t.Run("case3", func(t *testing.T) {
		raw := `{"nonce": "0x0","s":"0x0"}`
		var x RPCTransaction
		assert.NoError(t, json.Unmarshal([]byte(raw), &x))
		assert.Equal(t, uint64(0), uint64(x.Nonce))
		b, err := json.Marshal(x.Nonce)
		assert.NoError(t, err)
		assert.Equal(t, `"0x0"`, string(b))
		b, err = json.Marshal(x.S)
		assert.NoError(t, err)
		assert.Equal(t, `"0x0"`, string(b))
	})
	t.Run("case4", func(t *testing.T) {
		raw := `{"nonce": "0x123","s":"0x123"}`
		var x RPCTransaction
		assert.NoError(t, json.Unmarshal([]byte(raw), &x))
		assert.Equal(t, uint64(291), uint64(x.Nonce))
		b, err := json.Marshal(x.Nonce)
		assert.NoError(t, err)
		assert.Equal(t, `"0x123"`, string(b))
		b, err = json.Marshal(x.S)
		assert.NoError(t, err)
		assert.Equal(t, `"0x123"`, string(b))
	})
}
