package types

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

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
			"epoch": "9",
			"voting_power": "54"
	},
	"bcs": ""
}
`
	var ev Event
	assert.NoError(t, json.Unmarshal([]byte(raw), &ev))

	assert.Equal(t, "4WmGHKKACJhh93CvCgL8QHKnZLjVcwim95T9nBjX19tn", ev.ID.TxDigest.String())
	assert.Equal(t, uint64(20), ev.ID.EventSeq.Uint64())
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000003", ev.PackageID.String())
	assert.Equal(t, "sui_system", ev.TransactionModule)
	assert.Equal(t, "0x3::validator_set::ValidatorEpochInfoEventV2", ev.Type.String())
	// parsedJson is kept as raw json
	assert.JSONEq(t, `{"epoch":"9","voting_power":"54"}`, string(ev.Fields))
}
