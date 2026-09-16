package evm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_BlockExtendRequirement_MergeThenTrim(t *testing.T) {
	var r BlockExtendRequirement
	r.Merge(BlockExtendRequirement{SpecialTransactions: []string{"a", "b"}, SpecialTransactionReceipts: []string{"a"}})
	r.Merge(BlockExtendRequirement{SpecialTransactions: []string{"b", "c"}, AllTransactionReceipts: true})
	r.Merge(BlockExtendRequirement{SpecialTransactionReceiptLogs: []string{"a", "a"}})
	// Merge only appends
	assert.Equal(t, []string{"a", "b", "b", "c"}, r.SpecialTransactions)
	assert.Equal(t, []string{"a"}, r.SpecialTransactionReceipts)
	assert.True(t, r.AllTransactionReceipts)

	r.Trim()
	assert.ElementsMatch(t, []string{"a", "b", "c"}, r.SpecialTransactions)
	assert.Nil(t, r.SpecialTransactionReceipts, "All* makes the special list redundant")
	assert.Equal(t, []string{"a"}, r.SpecialTransactionReceiptLogs)
}
