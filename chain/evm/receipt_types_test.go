package evm

import (
	"encoding/json"
	"testing"
)

func TestArbitrumReceipt(t *testing.T) {
	raw := `{
		"blockHash": "0x63dea9d1cb52bf934361a70f3abb6cdc29a069c029ee1e85b9b4675eb031beb1",
		"blockNumber": "0x38419c8",
		"contractAddress": null,
		"cumulativeGasUsed": "0x0",
		"gasUsed": "0x0",
		"gasUsedForL1": "0x0",
		"l1BlockNumber": "0xfd03cd",
		"logs": [],
		"logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		"status": "0x1",
		"transactionHash": "0x44cdb31420cca4c3916ef0d6b1c7bf6b971983036ba736599388c4bceb748b7d",
		"transactionIndex": "0x0",
		"type": "0x6a"
	}`
	var receipt ExtendedReceipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		t.Fatal(err)
	}

	b, _ := json.Marshal(&receipt)

	var hMap map[string]interface{}
	var hMap2 map[string]interface{}
	if err := json.Unmarshal(b, &hMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &hMap2); err != nil {
		t.Fatal(err)
	}
	for k, v := range hMap2 {
		if v2, ok := hMap[k]; !ok {
			t.Fatal("key not found", k)
		} else {
			switch v.(type) {
			case []interface{}: // logs
				if len(v.([]interface{})) != len(v2.([]interface{})) {
					t.Fatal("length mismatch", k, len(v.([]interface{})), len(v2.([]interface{})))
				}
				for i := range v.([]interface{}) {
					b1, _ := json.Marshal(v.([]interface{})[i])
					b2, _ := json.Marshal(v2.([]interface{})[i])
					if string(b1) != string(b2) {
						t.Fatal("value mismatch", k, string(b1), string(b2))
					}
				}
			default:
				if v != v2 {
					t.Fatal("value mismatch", k, v, v2)
				}
			}
		}
	}
}

func TestEthereumReceipt(t *testing.T) {
	raw := `{
		"blockHash": "0xb878d78d7b7b8759dc82581494c3235c842b1bb3b573e9ddd145c8e8544b448e",
		"blockNumber": "0x5a0294",
		"contractAddress": null,
		"cumulativeGasUsed": "0x60b5e0",
		"gasUsed": "0xce54",
		"logs": [
			{
				"address": "0xc54083e77f913a4f99e1232ae80c318ff03c9d17",
				"topics": [
					"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
					"0x000000000000000000000000e007071966b9e219787a07752fb3fa5ca207e7f9",
					"0x0000000000000000000000009a627066ba9d04fb8981150c03eb90f8793cfdbf"
				],
				"data": "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000",
				"blockNumber": "0x5a0294",
				"transactionHash": "0x662cc0851966b1554e43716d55dc1365ccdcbc95155e6dcddc8d22ad5b7dd51b",
				"transactionIndex": "0x77",
				"blockHash": "0xb878d78d7b7b8759dc82581494c3235c842b1bb3b573e9ddd145c8e8544b448e",
				"blockTimestamp": "0x55c8fcf3",
				"logIndex": "0x77",
				"removed": false
			}
		],
		"logsBloom": "0x00080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000002000018000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000100000000000000000000000000002000000000000000000000200000000000000000000000000000",
		"status": "0x1",
		"transactionHash": "0x662cc0851966b1554e43716d55dc1365ccdcbc95155e6dcddc8d22ad5b7dd51b",
		"transactionIndex": "0x77",
    "effectiveGasPrice": "0x37fa79b2b9",
		"type": "0x0"
	}`
	var receipt ExtendedReceipt
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		t.Fatal(err)
	}

	b, _ := json.Marshal(&receipt)
	var hMap map[string]interface{}
	var hMap2 map[string]interface{}
	if err := json.Unmarshal(b, &hMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &hMap2); err != nil {
		t.Fatal(err)
	}
	for k, v := range hMap2 {
		if v2, ok := hMap[k]; !ok {
			t.Fatal("key not found", k)
		} else {
			switch v.(type) {
			case []interface{}: // logs
				if len(v.([]interface{})) != len(v2.([]interface{})) {
					t.Fatal("length mismatch", k, len(v.([]interface{})), len(v2.([]interface{})))
				}
				for i := range v.([]interface{}) {
					b1, _ := json.Marshal(v.([]interface{})[i])
					b2, _ := json.Marshal(v2.([]interface{})[i])
					if string(b1) != string(b2) {
						t.Fatal("value mismatch", k, string(b1), string(b2))
					}
				}
			default:
				if v != v2 {
					t.Fatal("value mismatch", k, v, v2)
				}
			}
		}
	}
}
