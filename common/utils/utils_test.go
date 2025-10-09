package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func Test_Fetch(t *testing.T) {
	var a *string
	assert.Equal(t, "aa", Fetch(a, "aa"))
	var v = "bb"
	a = &v
	assert.Equal(t, "bb", Fetch(a, "aa"))
}

func Test_AddURLMosaic(t *testing.T) {
	testcases := [][]string{
		{"http://nodes.sea.sentio.xyz/ethereum", "http://*****.***.sentio.***/ethereum"},
		{"http://sentio-0.sentio.xyz:8080/ethereum", "http://sentio**.sentio.***:8080/ethereum"},
		{"https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fdeb8ee7b", "https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fxxxxxxxx"},
		{"https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fdeb8ee7b/ethereum", "https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fxxxxxxxx/ethereum"},
		{"https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fdeb8ee7b/ethereum/b0ebe560-22bd-437f-a30f-3e6fdeb8ee7b", "https://eth-mainnet.blastapi.io/b0ebe560-22bd-437f-a30f-3e6fxxxxxxxx/ethereum/b0ebe560-22bd-437f-a30f-3e6fxxxxxxxx"},
		{"https://rpc.startale.com/astar-zkevm", "https://rpc.startale.com/astar-zkevm"},
		{"https://rpc.startale.com/12345678901234567890?x=12345678901234567890", "https://rpc.startale.com/123456789012xxxxxxxx?x=123456789012xxxxxxxx"},
		{"https://user:passwd@rpc.startale.com", "https://xxxx:xxxxxx@rpc.startale.com"},
	}
	for i, testcase := range testcases {
		assert.Equal(t, testcase[1], AddURLMosaic(testcase[0]), fmt.Sprintf("testcase #%d %#v", i, testcase))
	}
}

func Test_AddOwnerNameMosaic(t *testing.T) {
	testcases := [][]string{
		{"", ""},
		{"a", "a"},
		{"ab", "ab"},
		{"abc", "a*c"},
		{"abcd", "a**d"},
		{"abcde", "a***e"},
		{"abcdef", "ab**ef"},
		{"abcdefg", "ab***fg"},
		{"abcdefgh", "ab****gh"},
		{"abcdefghi", "abc***ghi"},
		{"abcdefghij", "abc****hij"},
		{"01234567890123456789", "012**************789"},
	}
	for i, testcase := range testcases {
		assert.Equal(t, testcase[1], AddOwnerNameMosaic(testcase[0]), fmt.Sprintf("testcase #%d %#v", i, testcase))
	}
}

func Test_WrapPointerForArray(t *testing.T) {
	type object struct {
		A string
	}
	arr := []object{{A: "abc"}, {A: "def"}, {A: "xyz"}}
	parr := WrapPointerForArray(arr)
	parr[0].A = "123"
	parr[2].A = "456"
	assert.Equal(t, []object{{A: "123"}, {A: "def"}, {A: "456"}}, arr)
}

func Test_ZeroOrUInt64(t *testing.T) {
	assert.Equal(t, uint64(0), ZeroOrUInt64(nil))
	assert.Equal(t, uint64(1), ZeroOrUInt64(big.NewInt(1)))
}
