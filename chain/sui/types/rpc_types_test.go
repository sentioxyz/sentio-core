package types

import (
	"os"
	"testing"

	"github.com/pkg/errors"

	"github.com/goccy/go-json"
	"github.com/kinbiko/jsonassert"
)

func TestTransactionResponseV1JSON(t *testing.T) {
	b, _ := os.ReadFile(testDataFile)
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

func TestEventJSON(t *testing.T) {
	raw := `
{
	"id": {
			"txDigest": "4WmGHKKACJhh93CvCgL8QHKnZLjVcwim95T9nBjX19tn",
			"eventSeq": "20"
	},
	"packageId": "0x0000000000000000000000000000000000000000000000000000000000000003",
	"transactionModule": "sui_system",
	"sender": "0x0000000000000000000000000000000000000000000000000000000000000000",
	"type": "0x3::validator_set::ValidatorEpochInfoEventV2",
	"parsedJson": {
			"commission_rate": "200",
			"epoch": "9",
			"pool_staking_reward": "0",
			"pool_token_exchange_rate": {
					"pool_token_amount": "25000000000011551",
					"sui_amount": "25000000000577800"
			},
			"reference_gas_survey_quote": "1000",
			"stake": "25000000000577800",
			"storage_fund_staking_reward": "0",
			"tallying_rule_global_score": "1",
			"tallying_rule_reporters": [],
			"validator_address": "0xbba318294a51ddeafa50c335c8e77202170e1f272599a2edc40592100863f638",
			"voting_power": "54"
	},
	"bcs": ""
}
`

	var ev Event
	err := json.Unmarshal([]byte(raw), &ev)
	if err != nil {
		t.Fatal(err)
	}
}
