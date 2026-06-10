package types

import (
	"os"
	"testing"

	"github.com/pkg/errors"

	"github.com/goccy/go-json"
	"github.com/kinbiko/jsonassert"
)

func TestTransactionResponseV1JSON(t *testing.T) {
	b, _ := os.ReadFile(transactionBundleFile)
	var rawTxs []json.RawMessage
	err := json.Unmarshal(b, &rawTxs)
	if err != nil {
		t.Fatal(err)
	}

	ja := jsonassert.New(t)
	for _, rawTx := range rawTxs {
		var tx TransactionResponseV1
		err := json.Unmarshal(rawTx, &tx)
		if err != nil {
			t.Fatal(errors.Wrap(err, string(rawTx)))
		}

		b, err := json.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		ja.Assertf(string(b), string(rawTx))
	}
}
