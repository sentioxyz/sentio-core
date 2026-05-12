package evm

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"

	"sentioxyz/sentio-core/common/utils"
)

func TestEthereumHeader(t *testing.T) {
	raw := `{
		"difficulty": "0x2",
		"extraData": "0xd98301090a846765746889676f312e31352e3133856c696e75780000000000009b222264274d86e96d90ed62fca6e9e2924c0113d7dc251477cb92ee26a881c624a87b2c4a98954a742f3ba6daa28b4cef26486ca361106e71d2041443f8fadd01",
		"gasLimit": "0xe4e1c0",
		"gasUsed": "0x44ea5",
		"hash": "0xdf46600ec6aabf919979c7cc2cda1270d7fe4af1da6c63d084ce4e6a20a6fd0b",
		"logsBloom": "0x00200400000400000000009008000000000000000000000404000000000020000001000400000200000000400000000000000040000000000000010000000040000000000000000020000008000000200002000000000000080004000000000000000000000400000040101000000000410200020000000000000010000000000000000000000000000002000000000000001400000000000000005000000000040000000000000000000000000001000000000800000000000000000000000000021002000000000400000400000000008000000000020000080000000000000100000000000000000000080008010400000000000000000000000000011000",
		"miner": "0x0000000000000000000000000000000000000000",
		"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"nonce": "0x0000000000000000",
		"number": "0xf928e3",
		"parentHash": "0x8e6cfc4933d10bc0cf0b8eeb88affb3830f6e1c5decf76bee98e0dfc5ea700f5",
		"receiptsRoot": "0x077cdbac23211bb3e0f881813f6a2035eafcb87bf99dbb23b452a3ecf94034b3",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"stateRoot": "0xaa183fc81dd8fed0d604131de9b0b93a0f4be639d34a26e3f535dc83a1cc7c87",
		"timestamp": "0x62e2f525",
		"transactionsRoot": "0x95bb19987a63e511bb338c1a4438cbc2481cbd6bae7ea665487b2acb898b9427"
	}`
	var h1 *types.Header
	var h2 *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &h1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &h2); err != nil {
		t.Fatal(err)
	}
	if h1.Hash() != h2.Hash {
		t.Fatal("hash mismatch", "expected:", h1.Hash(), "got:", h2.Hash)
	}
	b1, _ := json.Marshal(h1)
	b2, _ := json.Marshal(h2)
	if string(b1) != string(b2) {
		t.Fatal("json mismatch", "\nexpected:\n", string(b1), "\ngot:\n", string(b2))
	}

	buf := new(bytes.Buffer)
	enc := msgpack.NewEncoder(buf)
	enc.SetCustomStructTag("json")
	enc.Encode(h2)
	b := buf.Bytes()

	dec := msgpack.NewDecoder(bytes.NewReader(b))
	dec.SetCustomStructTag("json")
	var h3 *types.Header
	if err := dec.Decode(&h3); err != nil {
		t.Fatal(err)
	}
	if h1.Hash() != h3.Hash() {
		t.Fatal("hash mismatch in backward compatibility test (msgpack)",
			"expected:", h1.Hash(), "got:", h3.Hash())
	}
}

func TestEIP4895Header(t *testing.T) {
	raw := `{
		"baseFeePerGas": "0x29543b5a0",
		"difficulty": "0x0",
		"extraData": "0x",
		"gasLimit": "0x1c9c380",
		"gasUsed": "0x12a6cb3",
		"hash": "0xc75be0f971073e0d6d4906161f62de621ce6090ab992e331386a8971421fa829",
		"logsBloom": "0x470dae6645a2444c8430200004aa3180400e50012a5838182181c830e853101250ba22000a0460802020008a010602d4480500a462122c601c925f8cc62d822a804110b99ff385d8f920100a503182061e05aae0c246a01a80f2888ac3e620296804318002314620504015a580700d111c02081443786410bc3930d0a30800d4170220764842b92aa81a3c6192b4183f00040c252a909216223501621c7ea34402188a4353201f800904bc02b010010624460241628483000115491c882a82020530522221a00a3b450468810703c43011cc10542000060040037b120d42618c3a1441a930f080418a13219821c00048c0a10250a93224c28042001a3a004743",
		"miner": "0x000095e79eac4d76aab57cb2c1f091d553b36ca0",
		"mixHash": "0x28e9b3702ff3862da1455a8bec42293b6639db69e2709dfcd8d083ff113a075c",
		"nonce": "0x0000000000000000",
		"number": "0x8447aa",
		"parentHash": "0xb74276a515c6aeff55688e5586221858a7a8c46e75535c44107ba5b2dc92c528",
		"receiptsRoot": "0x9106b99382880a684d712bdd993d6b5a30b4b3316482c44a467bd4a19902ce9a",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"stateRoot": "0x9cdbc17bbd6882afbcb3417fb24ee3125ac18d3b8c7aa64875e643d943e60c51",
		"timestamp": "0x64140b28",
		"transactionsRoot": "0xbbd327e2d46a03ba66637ba241b250c972c786c2ec593dce6fa8a4f18addaa18",
		"uncles": [],
		"withdrawalsRoot": "0xe968421d2d4652b9400cf26b05a0ea0244ad925eea02bf43e264adbb799b9c8f"
	}`

	var h1 *types.Header
	var h2 *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &h1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &h2); err != nil {
		t.Fatal(err)
	}
	if h1.Hash() != h2.Hash {
		t.Fatal("hash mismatch", "expected:", h1.Hash(), "got:", h2.Hash)
	}
	if h2.WithdrawalsHash.Hex() != "0xe968421d2d4652b9400cf26b05a0ea0244ad925eea02bf43e264adbb799b9c8f" {
		t.Fatal("withdrawals hash mismatch")
	}
	b1, _ := json.Marshal(h1)
	b2, _ := json.Marshal(h2)
	if string(b1) != string(b2) {
		t.Fatal("json mismatch", "\nexpected:\n", string(b1), "\ngot:\n", string(b2))
	}
}

func TestMoonbeamHeader(t *testing.T) {
	raw := `{
		"author": "0xf51fd3dd05519d870dfb954d8a3a9835d3ad0959",
		"baseFeePerGas": "0x174876e800",
		"difficulty": "0x0",
		"extraData": "0x",
		"gasLimit": "0xe4e1c0",
		"gasUsed": "0x2f169",
		"hash": "0xfb7344716d6c2763fb6d7cc2222471eb515251792b72ea55fffbb5712591f907",
		"logsBloom": "0x0000000000000000000000008000000000000000000000000000400000000100000000000000000000010000000000000000000004000000000000000000000000000000000000000000000840000000000000000000000400000000c000000000000000020000000000000000200800000000000008000100000010020000000000000000000000000000000000004000000001000000080000000000000000000200000000000001000002000000000000000000000400000000000010000000000002004000000000000000020000004000000000001000000000000020001000000000000010000000000000000000001000000000400004000000000000",
		"miner": "0xf51fd3dd05519d870dfb954d8a3a9835d3ad0959",
		"nonce": "0x0000000000000000",
		"number": "0x2b6881",
		"parentHash": "0x2acd369f83f3b8416a90e90af16ec1f39f5062405e56ef342dfd251a7b93f268",
		"receiptsRoot": "0x2930edbb75ee9d738825b0591e0e561c6e2a2e3506beb9a852413722d0fdd1cd",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"size": "0x34b",
		"stateRoot": "0x46673147e9549a072f8d5c6799d8b9b208af3e16cb9cbd6975de9cdf25c495c2",
		"timestamp": "0x63d864e6",
		"totalDifficulty": "0x0",
		"transactionsRoot": "0x74ece83e343f3111f4668eeb0c7c2e96f6453a2b7c25f11cbfae1320cc090bfc"
	}`
	var hMap map[string]interface{}
	var h *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &hMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatal(err)
	}
	if h.Hash.String() != "0xfb7344716d6c2763fb6d7cc2222471eb515251792b72ea55fffbb5712591f907" {
		t.Fatal("hash mismatch", "got:", h.Hash)
	}
	b, _ := json.Marshal(h)
	var hMap2 map[string]interface{}
	if err := json.Unmarshal(b, &hMap2); err != nil {
		t.Fatal(err)
	}
	for key := range hMap {
		if hMap[key] != hMap2[key] {
			t.Fatal("deserialized json field mismatch", key, "expected:", hMap[key], "got:", hMap2[key])
		}
	}
}

func TestPolygonHeader(t *testing.T) {
	raw := `{
		"difficulty": "0x15",
		"extraData": "0xd583010a0383626f7286676f312e3135856c696e757800000000000000000000ee8a34233ae4522e1ded0e4f62657c22abbad3f691dbf20c8474bbd7ed6f5328630d694b7c3a2d2b25fdd369817ddd6e7222eabc1b6662b09b376e2228d84fcf00",
		"gasLimit": "0x13929de",
		"gasUsed": "0x1391567",
		"hash": "0xe9bafedababf02545c6d8f0b8132146b2579307133dbc612f1ff5ce19fe3b7ce",
		"logsBloom": "0xd768f95633eeaa2e52692ddec5f556ac701a6463e57f0e6a8b80c81c174bf378e78928c5e4b2b2b3c5a3c819f87a37c5bb23dd09e4d8fb9619f316c97fae2355b0b5d2cb2f8c4d6f287b060cc5cb7ba89f108530a5eed1d1a02bf1f4e932eef5421c2ffc4fc2c5969f6d63fe259d286181448099c8bc97878d6d66528a1a2f99aa930bbf668d246819faa55c5c27db6d445ff2b3ba2fcd4acecde66d74b51a3db2d00482f708fe766369ec9df929f3317747f82dbf97f6ba7d9ae19e20d03d50f3e9a2c28ce31187ecc3d372c43fca8f6a9496e37f69ea345f3b81afc41ae0233356cdc0101390cb15ed33351762f877cd7c62c9d6a550d176f2d92897b4be0f",
		"miner": "0x1fbc8746975598d58b0757eb2a273324dd28f6a0",
		"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"nonce": "0x0000000000000000",
		"number": "0xf841b4",
		"parentHash": "0x9afdedff708041a24afc8af9bec34a2dbb4e53149900638502961885565b36b4",
		"receiptsRoot": "0xa2e56995dd7747662ccf076c592b135b546d1f050229b7f5b7ccd48fa393ea41",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"size": "0x237f2",
		"stateRoot": "0x3b981ed22face9a5343ec219b5e4c4f0bdfa9942aaef2ba52434a12cb6d5f2bb",
		"timestamp": "0x60da7c56",
		"totalDifficulty": "0xa3a3a75",
		"transactionsRoot": "0x566fd1193001c688426da9e05917ccfe3e4ddc9718fa8bd897342ae996682aa2"
	}`
	var hMap map[string]interface{}
	var h *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &hMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatal(err)
	}
	if h.Hash.String() != "0xe9bafedababf02545c6d8f0b8132146b2579307133dbc612f1ff5ce19fe3b7ce" {
		t.Fatal("hash mismatch", "got:", h.Hash)
	}
	b, _ := json.Marshal(h)
	var hMap2 map[string]interface{}
	if err := json.Unmarshal(b, &hMap2); err != nil {
		t.Fatal(err)
	}
	for key := range hMap {
		if hMap[key] != hMap2[key] {
			t.Fatal("deserialized json field mismatch", key, "expected:", hMap[key], "got:", hMap2[key])
		}
	}
}

func TestArbitrumHeader(t *testing.T) {
	raw := `{
    "baseFeePerGas": "0x5f5e100",
    "difficulty": "0x1",
    "extraData": "0x03e15cc0fcd490292c1a397e7dabe1be29b542724014f8f7fba09684e742b8e9",
    "gasLimit": "0x4000000000000",
    "gasUsed": "0x84ecf",
    "hash": "0x63dea9d1cb52bf934361a70f3abb6cdc29a069c029ee1e85b9b4675eb031beb1",
    "l1BlockNumber": "0xfd03cd",
    "logsBloom": "0x00000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010080000000000000008000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000090000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008002000000000004000000008000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000",
    "miner": "0xa4b000000000000000000073657175656e636572",
    "mixHash": "0x000000000000a8130000000000fd03cd000000000000000a0000000000000000",
    "nonce": "0x0000000000071524",
    "number": "0x38419c8",
    "parentHash": "0xdcbace1f08a30fae90c7d55ebc7ab5c3632fa565fdd4225f4c979773ba4ec0b4",
    "receiptsRoot": "0xe8aa82c2af165dd98906f42ce3a9a70d3698a59d5d6f47ecd4aaf9636c5f9994",
    "sendCount": "0xa813",
    "sendRoot": "0x03e15cc0fcd490292c1a397e7dabe1be29b542724014f8f7fba09684e742b8e9",
    "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
    "size": "0x36a",
    "stateRoot": "0x79935470ac28e851cf10772881353c904c5413a9c8a9074776edbd4afed2fb37",
    "timestamp": "0x63e3208d",
    "totalDifficulty": "0x2313c80",
    "transactionsRoot": "0x8c64eae012c6370072bce73fa3c2e15a01e36354b47dacb4217a32e4ef8b73ce"
  }`
	var hMap map[string]interface{}
	var h *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &hMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatal(err)
	}
	if h.Hash.String() != "0x63dea9d1cb52bf934361a70f3abb6cdc29a069c029ee1e85b9b4675eb031beb1" {
		t.Fatal("hash mismatch", "got:", h.Hash)
	}
	if h.L1BlockNumber.String() != "0xfd03cd" {
		t.Fatal("l1 block number mismatch", "got:", h.L1BlockNumber)
	}
	b, _ := json.Marshal(h)
	var hMap2 map[string]interface{}
	if err := json.Unmarshal(b, &hMap2); err != nil {
		t.Fatal(err)
	}
	for key := range hMap {
		if hMap[key] != hMap2[key] {
			t.Fatal("deserialized json field mismatch", key, "expected:", hMap[key], "got:", hMap2[key])
		}
	}
}

func TestHeaderToStructpb(t *testing.T) {
	raw := `{
		"difficulty": "0x15",
		"extraData": "0xd583010a0383626f7286676f312e3135856c696e757800000000000000000000ee8a34233ae4522e1ded0e4f62657c22abbad3f691dbf20c8474bbd7ed6f5328630d694b7c3a2d2b25fdd369817ddd6e7222eabc1b6662b09b376e2228d84fcf00",
		"gasLimit": "0x13929de",
		"gasUsed": "0x1391567",
		"baseFeePerGas": "0x174876e800",
		"hash": "0xe9bafedababf02545c6d8f0b8132146b2579307133dbc612f1ff5ce19fe3b7ce",
		"logsBloom": "0xd768f95633eeaa2e52692ddec5f556ac701a6463e57f0e6a8b80c81c174bf378e78928c5e4b2b2b3c5a3c819f87a37c5bb23dd09e4d8fb9619f316c97fae2355b0b5d2cb2f8c4d6f287b060cc5cb7ba89f108530a5eed1d1a02bf1f4e932eef5421c2ffc4fc2c5969f6d63fe259d286181448099c8bc97878d6d66528a1a2f99aa930bbf668d246819faa55c5c27db6d445ff2b3ba2fcd4acecde66d74b51a3db2d00482f708fe766369ec9df929f3317747f82dbf97f6ba7d9ae19e20d03d50f3e9a2c28ce31187ecc3d372c43fca8f6a9496e37f69ea345f3b81afc41ae0233356cdc0101390cb15ed33351762f877cd7c62c9d6a550d176f2d92897b4be0f",
		"miner": "0x1fbc8746975598d58b0757eb2a273324dd28f6a0",
		"mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
		"nonce": "0x0000000000000000",
		"number": "0xf841b4",
		"parentHash": "0x9afdedff708041a24afc8af9bec34a2dbb4e53149900638502961885565b36b4",
		"receiptsRoot": "0xa2e56995dd7747662ccf076c592b135b546d1f050229b7f5b7ccd48fa393ea41",
		"sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"size": "0x237f2",
		"stateRoot": "0x3b981ed22face9a5343ec219b5e4c4f0bdfa9942aaef2ba52434a12cb6d5f2bb",
		"timestamp": "0x60da7c56",
		"totalDifficulty": "0xa3a3a75",
		"transactionsRoot": "0x566fd1193001c688426da9e05917ccfe3e4ddc9718fa8bd897342ae996682aa2"
	}`
	var h *ExtendedHeader
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		t.Fatal(err)
	}
	s := utils.ConvertToStructpb(h, reflect.TypeOf(ExtendedHeader{}))
	if len(s.Fields) != 19 {
		t.Fatal("wrong number of fields", len(s.Fields))
	}
	if s.Fields["timestamp"].GetStringValue() != "0x60da7c56" {
		t.Fatal("wrong timestamp", s.Fields["timestamp"].GetStringValue())
	}
}

func TestUnmarshalTronHeader(t *testing.T) {
	raw := `{
        "baseFeePerGas": "0x0",
        "difficulty": "0x0",
        "extraData": "0x",
        "gasLimit": "0x19af2d050",
        "gasUsed": "0x7bbef6",
        "hash": "0x0000000004e729192693e178ef8e2188f696b218be102a0a610fce22a24769be",
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "miner": "0x1761716b76c6a3d885299c366826046c09b08d26",
        "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "nonce": "0x0000000000000000",
        "number": "0x4e72919",
        "parentHash": "0x0000000004e72918c7f6b14d7f6ec9aff0ad5b42b4b2b15b02489c3e5dc6f110",
        "receiptsRoot": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "sha3Uncles": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "size": "0x19cbd",
        "stateRoot": "0x",
        "timestamp": "0x69f19f53",
        "totalDifficulty": "0x0",
        "transactions": [],
        "transactionsRoot": "0x01d68fe4afb16c2422b1f24de02438e169e0c8c81f9326196792d67cbef14215",
        "uncles": []
    }`
	var h ExtendedHeader
	assert.NoError(t, json.Unmarshal([]byte(raw), &h))
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", h.Root.String())
	assert.Equal(t, "0x0000000004e729192693e178ef8e2188f696b218be102a0a610fce22a24769be", h.Hash.String())
}
