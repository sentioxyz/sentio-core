package utils

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_dup(t *testing.T) {
	testcases := [][]any{
		{"", "", 0, ""},
		{"", "", 1, ""},
		{"", "", 2, ""},

		{"", "d", 0, ""},
		{"", "d", 1, ""},
		{"", "d", 2, "d"},
		{"", "d", 3, "dd"},

		{"s", "", 0, ""},
		{"s", "", 1, "s"},
		{"s", "", 2, "ss"},
		{"s", "", 3, "sss"},

		{"s", "d", 0, ""},
		{"s", "d", 1, "s"},
		{"s", "d", 2, "sds"},
		{"s", "d", 3, "sdsds"},
	}

	for i, testcase := range testcases {
		s := testcase[0].(string)
		d := testcase[1].(string)
		n := testcase[2].(int)
		exp := testcase[3].(string)
		assert.Equal(t, exp, Dup(s, d, n), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_StringSummary(t *testing.T) {
	testcases := [][]any{
		{"", 0, ""},
		{"", 1, ""},
		{"", 2, ""},
		{"a", 0, "...(total len 1)"},
		{"a", 1, "a"},
		{"a", 2, "a"},
		{"abc", 1, "a...(total len 3)"},
		{"abc", 2, "ab...(total len 3)"},
		{"abc", 3, "abc"},
		{"abc", 4, "abc"},
	}
	for i, testcase := range testcases {
		s := testcase[0].(string)
		l := testcase[1].(int)
		exp := testcase[2].(string)
		assert.Equal(t, exp, StringSummary(s, l), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_SplitByRunes(t *testing.T) {
	testcases := [][]any{
		// no delimiter
		{"", []rune{'_', '%'}, '\\', []string{""}},
		{"中", []rune{'_', '%'}, '\\', []string{"中"}},
		{"\\", []rune{'_', '%'}, '\\', []string{"\\"}},
		{"\\a", []rune{'_', '%'}, '\\', []string{"a"}},
		{"a\\", []rune{'_', '%'}, '\\', []string{"a\\"}},
		// has delimiter
		{"_", []rune{'_', '%'}, '\\', []string{"", "_", ""}},
		{"a_", []rune{'_', '%'}, '\\', []string{"a", "_", ""}},
		{"_b", []rune{'_', '%'}, '\\', []string{"", "_", "b"}},
		{"a_b", []rune{'_', '%'}, '\\', []string{"a", "_", "b"}},
		{"_%", []rune{'_', '%'}, '\\', []string{"", "_", "", "%", ""}},
		{"a_%", []rune{'_', '%'}, '\\', []string{"a", "_", "", "%", ""}},
		{"_a%", []rune{'_', '%'}, '\\', []string{"", "_", "a", "%", ""}},
		{"_%a", []rune{'_', '%'}, '\\', []string{"", "_", "", "%", "a"}},
		{"a_b%", []rune{'_', '%'}, '\\', []string{"a", "_", "b", "%", ""}},
		{"a_%c", []rune{'_', '%'}, '\\', []string{"a", "_", "", "%", "c"}},
		{"_b%c", []rune{'_', '%'}, '\\', []string{"", "_", "b", "%", "c"}},
		{"a_b%c", []rune{'_', '%'}, '\\', []string{"a", "_", "b", "%", "c"}},
		// has escaped delimiter
		{"\\_", []rune{'_', '%'}, '\\', []string{"_"}},
		{"a\\_", []rune{'_', '%'}, '\\', []string{"a_"}},
		{"\\_b", []rune{'_', '%'}, '\\', []string{"_b"}},
		{"a\\_b", []rune{'_', '%'}, '\\', []string{"a_b"}},
		{"\\_%", []rune{'_', '%'}, '\\', []string{"_", "%", ""}},
		{"_\\%", []rune{'_', '%'}, '\\', []string{"", "_", "%"}},
		{"a\\_%", []rune{'_', '%'}, '\\', []string{"a_", "%", ""}},
		{"a_\\%", []rune{'_', '%'}, '\\', []string{"a", "_", "%"}},
		{"\\_a%", []rune{'_', '%'}, '\\', []string{"_a", "%", ""}},
		{"_a\\%", []rune{'_', '%'}, '\\', []string{"", "_", "a%"}},
		{"\\_%a", []rune{'_', '%'}, '\\', []string{"_", "%", "a"}},
		{"_\\%a", []rune{'_', '%'}, '\\', []string{"", "_", "%a"}},
		{"a\\_b%", []rune{'_', '%'}, '\\', []string{"a_b", "%", ""}},
		{"a_b\\%", []rune{'_', '%'}, '\\', []string{"a", "_", "b%"}},
		{"a\\_%c", []rune{'_', '%'}, '\\', []string{"a_", "%", "c"}},
		{"a_\\%c", []rune{'_', '%'}, '\\', []string{"a", "_", "%c"}},
		{"\\_b%c", []rune{'_', '%'}, '\\', []string{"_b", "%", "c"}},
		{"_b\\%c", []rune{'_', '%'}, '\\', []string{"", "_", "b%c"}},
		{"a\\_b%c", []rune{'_', '%'}, '\\', []string{"a_b", "%", "c"}},
		{"a_b\\%c", []rune{'_', '%'}, '\\', []string{"a", "_", "b%c"}},
	}
	for i, testcase := range testcases {
		origin := testcase[0].(string)
		delim := testcase[1].([]rune)
		escape := testcase[2].(rune)
		exp := testcase[3].([]string)
		assert.Equal(t, exp, SplitByRunes(origin, delim, escape), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_Escape(t *testing.T) {
	testcases := [][]any{
		{"", "0123456789", ""},
		{"a", "0123456789", "a"},
		{"ab", "0123456789", "ab"},
		{"0ab", "0123456789", "\\0ab"},
		{"a0b", "0123456789", "a\\0b"},
		{"ab0", "0123456789", "ab\\0"},
		{"1ab", "0123456789", "\\1ab"},
		{"a1b", "0123456789", "a\\1b"},
		{"ab1", "0123456789", "ab\\1"},
		{"9ab", "0123456789", "\\9ab"},
		{"a9b", "0123456789", "a\\9b"},
		{"ab9", "0123456789", "ab\\9"},
	}
	for i, testcase := range testcases {
		origin := testcase[0].(string)
		ctrl := testcase[1].(string)
		exp := testcase[2].(string)
		assert.Equal(t, exp, Escape(origin, []rune(ctrl), '\\'), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_LikePatternToRegexp(t *testing.T) {
	testcases := [][]any{
		{"", "a", false},
		{"", "", true},

		{"a", "", false},
		{"a", "a", true},
		{"a", "ab", false},
		{"ab", "", false},
		{"ab", "a", false},
		{"ab", "ab", true},

		{"%ab", "a", false},
		{"%ab", "b", false},
		{"%ab", "ab", true},
		{"%ab", "xab", true},
		{"%ab", "xxab", true},
		{"%ab", "axb", false},
		{"%ab", "abx", false},
		{"%ab", ".ab", true},
		{"%ab", "ac", false},

		{"a%b", "a", false},
		{"a%b", "b", false},
		{"a%b", "ab", true},
		{"a%b", "xab", false},
		{"a%b", "axb", true},
		{"a%b", "axxb", true},
		{"a%b", "abx", false},
		{"a%b", ".ab", false},
		{"a%b", "ac", false},

		{"ab%", "a", false},
		{"ab%", "b", false},
		{"ab%", "ab", true},
		{"ab%", "xab", false},
		{"ab%", "axb", false},
		{"ab%", "abx", true},
		{"ab%", "abxx", true},
		{"ab%", ".ab", false},
		{"ab%", "ac", false},

		{"[ab]%", "a", false},
		{"[ab]%", "b", false},
		{"[ab]%", "ab", false},
		{"[ab]%", "[ab]", true},
		{"[ab]%", "x[ab]", false},
		{"[ab]%", "[axb]", false},
		{"[ab]%", "[ab]x", true},
		{"[ab]%", "[ab]xx", true},
		{"[ab]%", ".ab", false},
		{"[ab]%", "ac", false},

		{"[a\\\\b]%", "a", false},
		{"[a\\\\b]%", "b", false},
		{"[a\\\\b]%", "ab", false},
		{"[a\\\\b]%", "[a\\b]", true},
		{"[a\\\\b]%", "x[a\\b]", false},
		{"[a\\\\b]%", "[axb]", false},
		{"[a\\\\b]%", "[a\\b]x", true},
		{"[a\\\\b]%", "[a\\b]xx", true},
		{"[a\\\\b]%", ".a\\b", false},
		{"[a\\\\b]%", "a\\c", false},

		{"aaa%bbb_", "aaabbb", false},
		{"aaa%bbb_", "aaabbbb", true},
		{"aaa%bbb_", "aaabbbbb", true},
		{"aaa%bbb_", "aaabbbbbc", true},
		{"aaa%bbb_", "aaabbbbbcc", false},
	}
	for i, testcase := range testcases {
		pattern := testcase[0].(string)
		origin := testcase[1].(string)
		exp := testcase[2].(bool)
		assert.Equal(t, exp, LikePatternToRegexp(pattern).MatchString(origin), fmt.Sprintf("case #%d: %v", i, testcase))
	}
}

func Test_JoinWithQuote(t *testing.T) {
	assert.Equal(t, "", JoinWithQuote(nil, ",", "#"))
	assert.Equal(t, "#a#", JoinWithQuote([]string{"a"}, ",", "#"))
	assert.Equal(t, "#a#,##,#b#", JoinWithQuote([]string{"a", "", "b"}, ",", "#"))
}

func Test_ToString(t *testing.T) {
	assert.Equal(t, (*string)(nil), NullOrToString((*common.Hash)(nil)))
	assert.Equal(t, WrapPointer("0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"), NullOrToString(&common.Hash{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}))

	assert.Equal(t, (*common.Hash)(nil), NullOrFromString(nil, common.HexToHash))
	assert.Equal(t, &common.Hash{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}, NullOrFromString(WrapPointer("0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"), common.HexToHash))

	assert.Equal(t, "", EmptyStringIfNil(nil))
	assert.Equal(t, "abc", EmptyStringIfNil(WrapPointer("abc")))

	assert.Equal(t, []string{"1", "2", "3", "4"}, MapSliceNoError([]uint64{1, 2, 3, 4}, UIntFormatter(10)))
	assert.Equal(t, []string{"b", "c", "14", "ff"}, MapSliceNoError([]uint64{11, 12, 20, 255}, UIntFormatter(16)))
}
