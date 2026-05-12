package types

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_inputsMarshal(t *testing.T) {
	raw := `
[
    {
        "type": "fundsWithdrawal",
        "reservation": {
            "maxAmountU64": "12345"
        },
        "typeArg": {
            "balance": "0x2::sui::SUI"
        },
        "withdrawFrom": "sender"
    }
]
`
	var inputs []CallArg
	assert.NoError(t, json.Unmarshal([]byte(raw), &inputs))
	assert.Equal(t, []CallArg{{
		FundsWithdrawal: &FundsWithdrawal{
			Amount:   utils.WrapPointer[uint64](12345),
			CoinType: utils.WrapPointer("0x2::sui::SUI"),
			Source:   utils.WrapPointer("sender"),
		},
	}}, inputs)

	b, err := json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal","typeArg":{"balance":"0x2::sui::SUI"},"withdrawFrom":"sender"}]`, string(b))

	inputs[0].FundsWithdrawal.Source = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal","typeArg":{"balance":"0x2::sui::SUI"}}]`, string(b))

	inputs[0].FundsWithdrawal.CoinType = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"reservation":{"maxAmountU64":"12345"},"type":"fundsWithdrawal"}]`, string(b))

	inputs[0].FundsWithdrawal.Amount = nil
	b, err = json.Marshal(inputs)
	assert.NoError(t, err)
	assert.Equal(t, `[{"type":"fundsWithdrawal"}]`, string(b))
}
