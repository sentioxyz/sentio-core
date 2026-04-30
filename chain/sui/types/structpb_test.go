package types

import (
	"fmt"
	"os"
	"testing"

	"github.com/goccy/go-json"
	"github.com/kinbiko/jsonassert"

	"sentioxyz/sentio-core/common/utils"
)

func replaceNumberWithStringRecursive(m map[string]interface{}) {
	for k, v := range m {
		switch v := v.(type) {
		case map[string]interface{}:
			replaceNumberWithStringRecursive(v)
		case []interface{}:
			for i, elem := range v {
				switch v2 := elem.(type) {
				case map[string]interface{}:
					replaceNumberWithStringRecursive(v2)
				case float64:
					v[i] = fmt.Sprintf("%d", uint64(v2))
				}
			}
		case float64:
			m[k] = fmt.Sprintf("%d", uint64(v))
		}
	}
}

func changeAllNumberToStringJSON(s string) string {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(s), &m)
	if err != nil {
		panic(err)
	}
	replaceNumberWithStringRecursive(m)
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestTransactionResponseV1Structpb(t *testing.T) {
	b, _ := os.ReadFile(testDataFile)
	var rawTxs []json.RawMessage
	err := json.Unmarshal(b, &rawTxs)
	if err != nil {
		t.Fatal(err)
	}

	ja := jsonassert.New(t)
	for _, rawTx := range rawTxs {
		var tx *TransactionResponseV1
		err := json.Unmarshal(rawTx, &tx)
		if err != nil {
			t.Fatal(err)
		}

		s := utils.MarshalStructpb(tx)
		j, err := s.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		ja.Assertf(changeAllNumberToStringJSON(string(j)),
			changeAllNumberToStringJSON(string(rawTx)))
	}
}
