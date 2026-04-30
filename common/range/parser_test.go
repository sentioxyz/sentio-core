package rg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RangeParserMarshal(t *testing.T) {
	p := RangeParser{}
	var buf bytes.Buffer

	// empty range → "EMPTY"
	assert.NoError(t, p.Marshal(EmptyRange, &buf))
	assert.Equal(t, "EMPTY", buf.String())

	// infinite range → "5,INF"
	buf.Reset()
	assert.NoError(t, p.Marshal(Range{Start: 5}, &buf))
	assert.Equal(t, "5,INF", buf.String())

	// normal range → "5,10"
	buf.Reset()
	assert.NoError(t, p.Marshal(NewRange(5, 10), &buf))
	assert.Equal(t, "5,10", buf.String())

	// zero start → "0,0"
	buf.Reset()
	assert.NoError(t, p.Marshal(NewRange(0, 0), &buf))
	assert.Equal(t, "0,0", buf.String())
}

func Test_RangeParserUnmarshal(t *testing.T) {
	p := RangeParser{}

	// "EMPTY" → EmptyRange
	r, err := p.Unmarshal(strings.NewReader("EMPTY"))
	assert.NoError(t, err)
	assert.True(t, r.IsEmpty())

	// infinite
	r, err = p.Unmarshal(strings.NewReader("5,INF"))
	assert.NoError(t, err)
	assert.Equal(t, uint64(5), r.Start)
	assert.Nil(t, r.End)

	// normal range
	r, err = p.Unmarshal(strings.NewReader("5,10"))
	assert.NoError(t, err)
	assert.Equal(t, NewRange(5, 10), r)

	// surrounding whitespace is accepted
	r, err = p.Unmarshal(strings.NewReader("  5  ,  10  "))
	assert.NoError(t, err)
	assert.Equal(t, NewRange(5, 10), r)
}

func Test_RangeParserUnmarshalErrors(t *testing.T) {
	p := RangeParser{}

	// no comma → ErrInvalidFormat
	_, err := p.Unmarshal(strings.NewReader("123"))
	assert.ErrorIs(t, err, ErrInvalidFormat)

	// empty string → ErrInvalidFormat
	_, err = p.Unmarshal(strings.NewReader(""))
	assert.ErrorIs(t, err, ErrInvalidFormat)

	// non-numeric start
	_, err = p.Unmarshal(strings.NewReader("abc,10"))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidFormat)

	// non-numeric end (not "INF")
	_, err = p.Unmarshal(strings.NewReader("5,abc"))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidFormat)
}

func Test_RangeParserRoundtrip(t *testing.T) {
	p := RangeParser{}
	cases := []Range{
		EmptyRange,
		Range{Start: 0},
		NewRange(0, 0),
		NewRange(100, 200),
		Range{Start: 999},
	}
	for _, orig := range cases {
		var buf bytes.Buffer
		assert.NoError(t, p.Marshal(orig, &buf))
		got, err := p.Unmarshal(&buf)
		assert.NoError(t, err)
		assert.Truef(t, orig.Equal(got), "roundtrip mismatch: orig=%s got=%s", orig, got)
	}
}

func Test_SetParserMarshal(t *testing.T) {
	p := SetParser{}
	var buf bytes.Buffer

	// empty set → empty output
	assert.NoError(t, p.Marshal(EmptyRangeSet, &buf))
	assert.Equal(t, "", buf.String())

	// single-segment set
	buf.Reset()
	assert.NoError(t, p.Marshal(RangeSet{Range: NewRange(1, 5)}, &buf))
	assert.Equal(t, "1,5", buf.String())

	// multi-segment set: [1,1][4,6][10,10]
	buf.Reset()
	set := RangeSet{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}}
	assert.NoError(t, p.Marshal(set, &buf))
	assert.Equal(t, "1,1|4,6|10,10", buf.String())

	// infinite-end set
	buf.Reset()
	assert.NoError(t, p.Marshal(RangeSet{Range: Range{Start: 5}}, &buf))
	assert.Equal(t, "5,INF", buf.String())
}

func Test_SetParserUnmarshal(t *testing.T) {
	p := SetParser{}

	// single range
	rs, err := p.Unmarshal(strings.NewReader("1,5"))
	assert.NoError(t, err)
	assert.Equal(t, RangeSet{Range: NewRange(1, 5)}, rs)

	// multi-segment
	rs, err = p.Unmarshal(strings.NewReader("1,1|4,6|10,10"))
	assert.NoError(t, err)
	assert.Equal(t, NewRange(1, 10), rs.Range)
	assert.Equal(t, [][2]uint64{{2, 3}, {7, 9}}, rs.Holes)

	// infinite end
	rs, err = p.Unmarshal(strings.NewReader("0,INF"))
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), rs.Start)
	assert.Nil(t, rs.End)

	// malformed segment → error
	_, err = p.Unmarshal(strings.NewReader("1,5|bad"))
	assert.Error(t, err)
}

func Test_SetParserRoundtrip(t *testing.T) {
	p := SetParser{}
	cases := []RangeSet{
		{Range: NewRange(1, 5)},
		{Range: NewRange(1, 10), Holes: [][2]uint64{{2, 3}, {7, 9}}},
		{Range: Range{Start: 100}},
		{Range: Range{Start: 1}, Holes: [][2]uint64{{3, 4}}},
	}
	for _, orig := range cases {
		var buf bytes.Buffer
		assert.NoError(t, p.Marshal(orig, &buf))
		got, err := p.Unmarshal(&buf)
		assert.NoError(t, err)
		assert.Truef(t, orig.Equal(got), "roundtrip mismatch: orig=%s got=%s", orig, got)
	}
}
