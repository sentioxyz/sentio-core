package format

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_format(t *testing.T) {
	assert.Panics(t, func() {
		_ = Format("%aaa", nil)
	})

	assert.Panics(t, func() {
		_ = Format("%aaa#s", map[string]any{"bbb": 123})
	})

	testcases := [][]any{
		{"", "", map[string]any(nil)},
		{"", "", map[string]any{"d1": 123}},
		{"123%", "%d1#d%%", map[string]any{"d1": 123}},
		{"abc-123%", "%s1#s-%d1#d%%", map[string]any{"d1": 123, "s1": "abc"}},
		{"abc-123%=abc", "%s1#s-%d1#d%%=%s1#s", map[string]any{"d1": 123, "s1": "abc"}},
	}
	for i, testcase := range testcases {
		exp := testcase[0].(string)
		format := testcase[1].(string)
		args := testcase[2].(map[string]any)
		assert.Equal(t, exp, Format(format, args), fmt.Sprintf("testcase #%d: %#v", i, testcase))
	}
}

func Test_FormatV2(t *testing.T) {
	cs := struct {
		P1 string
		P2 uint
		P3 int64
		Q1 string
	}{
		P1: "abc",
		P2: 123,
		P3: -456,
		Q1: "haha",
	}
	cm := map[string]string{
		"P4": "",
		"P5": "x",
		"P6": "y",
		"Q2": "xxx",
	}
	assert.Equal(t, "abc123-456#@xy-$P7", FormatV2("$P1$P2$P3#$P4@$P5$P6-$P7", cs, cm))
}
