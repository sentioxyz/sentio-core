package evm

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
)

type traceTests struct {
	Geth   *GethTrace     `json:"geth"`
	Parity []*ParityTrace `json:"parity"`
}

//go:embed testdata/trace-types.json
var testData []byte

func TestGethToParity(t *testing.T) {
	var tests []traceTests
	err := json.Unmarshal(testData, &tests)
	if err != nil {
		t.Fatal(err)
	}
	for i := range tests {
		for j := range tests[i].Parity {
			v := tests[i].Parity[j]

			// these are not supported by geth
			v.TransactionHash = nil
			v.TransactionPosition = 0
			v.BlockNumber = 0
			v.BlockHash = common.Hash{}
		}
	}
	for _, test := range tests {
		parity := GethToParityTrace(test.Geth, nil)
		if len(parity) != len(test.Parity) {
			t.Fatal("parity traces length mismatch, expected", len(test.Parity), "got", len(parity))
		}
		for i := range parity {
			j1, _ := json.MarshalIndent(parity[i], "", "  ")
			j2, _ := json.MarshalIndent(test.Parity[i], "", "  ")
			if string(j1) != string(j2) {
				t.Fatal("parity traces mismatch, expected\n", string(j2), "\ngot\n", string(j1))
			}
		}
	}
}

func removePrecompiledCalls(gr *GethTrace) {
	gr.Calls = gethFilterPrecompileCalls(gr.Calls)
	for i := range gr.Calls {
		removePrecompiledCalls(&gr.Calls[i])
	}
}

func TestParityToGeth(t *testing.T) {
	var tests []traceTests
	err := json.Unmarshal(testData, &tests)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		gr, err := ParityToGethTrace(test.Parity)
		if err != nil {
			t.Fatal(err)
		}
		if test.Geth.Type == "CREATE" {
			// we do not store nor support Code field in ParityTraceResult
			test.Geth.Output = nil
		}
		// precompiled calls are not included in ParityTrace
		removePrecompiledCalls(test.Geth)

		j1, _ := json.MarshalIndent(gr, "", "  ")
		j2, _ := json.MarshalIndent(test.Geth, "", "  ")
		if string(j1) != string(j2) {
			t.Fatal("geth traces mismatch, expected\n", string(j2), "\ngot\n", string(j1))
		}
	}
}

func TestEmptyGethTrace(t *testing.T) {
	// Seen by cronos debug_traceBlockByNumber(5544)
	raw := `{"from":"","gas":"","gasUsed":"","input":"","type":""}`
	var gt GethTrace
	err := json.Unmarshal([]byte(raw), &gt)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalGethTrace(t *testing.T) {
	cnt1 := `
{
	"result": {
		"type": "CALL",
		"from": "0x14c6479c88b2c445d0bdb67185da99e2cc9e0939",
		"to": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
		"value": "0x0",
		"gas": "0x3c63e",
		"gasUsed": "0x277df",
		"input": "0x70fef1da00000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc20000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d00000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8000000000000000000000000000000000000000000000000030acfa033e8d9620000000000000000000000005200a0e9b161bc59feecb165fe2592bef3e1847a0000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000005f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000065544ac70000000000000000000000000000000000000000000000000000000000000000",
		"calls": [
			{
				"type": "STATICCALL",
				"from": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
				"to": "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
				"gas": "0x3a4aa",
				"gasUsed": "0xa39",
				"input": "0x70a0823100000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e0939",
				"output": "0x000000000000000000000000000000000000000000000000002c06ce171eeca8"
			},
			{
				"type": "CALL",
				"from": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
				"to": "0x5200a0e9b161bc59feecb165fe2592bef3e1847a",
				"value": "0x0",
				"gas": "0x38e1b",
				"gasUsed": "0xbbde",
				"input": "0x199f726000000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce000000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8",
				"calls": [
					{
						"type": "CALL",
						"from": "0x5200a0e9b161bc59feecb165fe2592bef3e1847a",
						"to": "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
						"value": "0x0",
						"gas": "0x374ae",
						"gasUsed": "0xafeb",
						"input": "0x23b872dd00000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8",
						"output": "0x0000000000000000000000000000000000000000000000000000000000000001"
					}
				]
			}
		]
	}
}
`
	cnt2 := `
{
	"type": "CALL",
	"from": "0x14c6479c88b2c445d0bdb67185da99e2cc9e0939",
	"to": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
	"value": "0x0",
	"gas": "0x3c63e",
	"gasUsed": "0x277df",
	"input": "0x70fef1da00000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc20000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d00000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8000000000000000000000000000000000000000000000000030acfa033e8d9620000000000000000000000005200a0e9b161bc59feecb165fe2592bef3e1847a0000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000005f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000065544ac70000000000000000000000000000000000000000000000000000000000000000",
	"calls": [
		{
			"type": "STATICCALL",
			"from": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
			"to": "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
			"gas": "0x3a4aa",
			"gasUsed": "0xa39",
			"input": "0x70a0823100000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e0939",
			"output": "0x000000000000000000000000000000000000000000000000002c06ce171eeca8"
		},
		{
			"type": "CALL",
			"from": "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
			"to": "0x5200a0e9b161bc59feecb165fe2592bef3e1847a",
			"value": "0x0",
			"gas": "0x38e1b",
			"gasUsed": "0xbbde",
			"input": "0x199f726000000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce000000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8",
			"calls": [
				{
					"type": "CALL",
					"from": "0x5200a0e9b161bc59feecb165fe2592bef3e1847a",
					"to": "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
					"value": "0x0",
					"gas": "0x374ae",
					"gasUsed": "0xafeb",
					"input": "0x23b872dd00000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8",
					"output": "0x0000000000000000000000000000000000000000000000000000000000000001"
				}
			]
		}
	]
}
`
	hexToAddress := func(str string) *common.Address {
		addr := common.HexToAddress(str)
		return &addr
	}

	exp := GethTrace{
		Type:    "CALL",
		From:    hexToAddress("0x14c6479c88b2c445d0bdb67185da99e2cc9e0939"),
		To:      "0xdb5889e35e379ef0498aae126fc2cce1fbd23216",
		Value:   "0x0",
		Gas:     (*hexutil.Big)(hexutil.MustDecodeBig("0x3c63e")),
		GasUsed: (*hexutil.Big)(hexutil.MustDecodeBig("0x277df")),
		Input:   hexutil.MustDecode("0x70fef1da00000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0000000000000000000000000c02aaa39b223fe8d0a0e5c4f27ead9083c756cc20000000000000000000000007a250d5630b4cf539739df2c5dacb4c659f2488d00000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8000000000000000000000000000000000000000000000000030acfa033e8d9620000000000000000000000005200a0e9b161bc59feecb165fe2592bef3e1847a0000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000640000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000005f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000065544ac70000000000000000000000000000000000000000000000000000000000000000"),
		Output:  nil,
		Error:   "",
		Calls: []GethTrace{{
			Type:    "STATICCALL",
			From:    hexToAddress("0xdb5889e35e379ef0498aae126fc2cce1fbd23216"),
			To:      "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
			Value:   "",
			Gas:     (*hexutil.Big)(hexutil.MustDecodeBig("0x3a4aa")),
			GasUsed: (*hexutil.Big)(hexutil.MustDecodeBig("0xa39")),
			Input:   hexutil.MustDecode("0x70a0823100000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e0939"),
			Output:  hexutil.MustDecode("0x000000000000000000000000000000000000000000000000002c06ce171eeca8"),
			Error:   "",
		}, {
			Type:    "CALL",
			From:    hexToAddress("0xdb5889e35e379ef0498aae126fc2cce1fbd23216"),
			To:      "0x5200a0e9b161bc59feecb165fe2592bef3e1847a",
			Value:   "0x0",
			Gas:     (*hexutil.Big)(hexutil.MustDecodeBig("0x38e1b")),
			GasUsed: (*hexutil.Big)(hexutil.MustDecodeBig("0xbbde")),
			Input:   hexutil.MustDecode("0x199f726000000000000000000000000011ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce000000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8"),
			Output:  nil,
			Error:   "",
			Calls: []GethTrace{{
				Type:    "CALL",
				From:    hexToAddress("0x5200a0e9b161bc59feecb165fe2592bef3e1847a"),
				To:      "0x11ecca3e2e4ec2dff8f2175ad11cbd413ccd0ce0",
				Value:   "0x0",
				Gas:     (*hexutil.Big)(hexutil.MustDecodeBig("0x374ae")),
				GasUsed: (*hexutil.Big)(hexutil.MustDecodeBig("0xafeb")),
				Input:   hexutil.MustDecode("0x23b872dd00000000000000000000000014c6479c88b2c445d0bdb67185da99e2cc9e093900000000000000000000000086d9afb1835b7162bf91d3bdcd57fcc6c60bfd3a000000000000000000000000000000000000000000000000002c06ce171eeca8"),
				Output:  hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000001"),
				Error:   "",
			}},
		}},
	}

	check := func(title, cnt string) {
		var gr GethTraceBlockResult
		//start := time.Now()
		err := json.Unmarshal([]byte(cnt), &gr)
		//fmt.Printf("case %s used %v\n", title, time.Since(start))
		assert.NoError(t, err, title)
		assert.Equal(t, &exp, gr.Result, title)
	}
	//for i := 0; i < 10; i++ {
	check("#1", cnt1)
	check("#2", cnt2)
	//}
}

func TestUnmarshalGethTraceArr(t *testing.T) {
	cnt1 := `
[
	{
		"result": {
			"type": "aaa"
		}
	},
	{
		"result": {
			"type": "bbb"
		}
	}
]
`
	cnt2 := `
[
	{
		"type": "ccc"
	},
	{
		"type": "ddd"
	}
]
`
	{
		var gr []GethTraceBlockResult
		assert.NoError(t, json.Unmarshal([]byte(cnt1), &gr))
		assert.Equal(t, "aaa", gr[0].Result.Type)
		assert.Equal(t, "bbb", gr[1].Result.Type)
	}
	{
		var gr []GethTraceBlockResult
		assert.NoError(t, json.Unmarshal([]byte(cnt2), &gr))
		assert.Equal(t, "ccc", gr[0].Result.Type)
		assert.Equal(t, "ddd", gr[1].Result.Type)
	}
}
