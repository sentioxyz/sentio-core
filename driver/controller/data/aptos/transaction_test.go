package aptos

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/aptos"
)

func Test_txJsonMarshal(t *testing.T) {
	raw := aptos.MinimalistTransaction{
		Version:     123,
		Hash:        "abc",
		TimestampMS: 123,
	}
	b0, err := json.Marshal(raw)
	//t.Logf("%s", string(b0))
	assert.NoError(t, err)

	var tx MinimalistTransaction
	assert.NoError(t, json.Unmarshal(b0, &tx))
	assert.Equal(t, raw.Version, tx.Version)
	assert.Equal(t, raw.Hash, tx.Hash)
	assert.Equal(t, raw.TimestampMS, tx.TimestampMS)

	b1, err := json.Marshal(tx)
	//t.Logf("%s", string(b1))
	assert.NoError(t, err)
	assert.Equal(t, b0, b1)
}

func Test_diffBetweenNullAndEmptyArr(t *testing.T) {
	var a []int
	assert.True(t, a == nil)
	assert.True(t, len(a) == 0)
	a = []int{}
	assert.False(t, a == nil)
	assert.True(t, len(a) == 0)
	a = []int{1}
	assert.False(t, a == nil)
	assert.False(t, len(a) == 0)
}
