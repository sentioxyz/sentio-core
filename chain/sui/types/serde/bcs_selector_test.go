package serde

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func u64p(v uint64) *uint64 { return &v }

// position-mode enum: no enumNum tags -> variant index == field position.
type posEnum struct {
	A *uint64
	B *uint64
}

func (posEnum) IsBcsEnum() {}

// tag-mode enum: x and y assign swapped indices to A/B; C exists only on x.
type tagEnum struct {
	A *uint64 `bcs:"enumNum[x]=0,enumNum[y]=1"`
	B *uint64 `bcs:"enumNum[x]=1,enumNum[y]=0"`
	C *uint64 `bcs:"enumNum[x]=2"`
}

func (tagEnum) IsBcsEnum() {}

type dupEnum struct {
	A *uint64 `bcs:"enumNum[x]=0"`
	B *uint64 `bcs:"enumNum[x]=0"`
}

func (dupEnum) IsBcsEnum() {}

// plain struct with a field that is optional only under selector x.
type optStruct struct {
	X *uint64 `bcs:"optional[x]"`
}

func enc(t *testing.T, selector string, v any) []byte {
	t.Helper()
	buf := bytes.NewBuffer(nil)
	require.NoError(t, NewEncoderForSelector(buf, selector).Encode(v))
	return buf.Bytes()
}

func dec(selector string, b []byte, out any) error {
	return NewDecoderForSelector(bytes.NewReader(b), selector).Decode(out)
}

func TestParseTag(t *testing.T) {
	t.Run("ignore global vs scoped", func(t *testing.T) {
		ft, err := parseTag("-")
		require.NoError(t, err)
		assert.True(t, ft.isIgnored("anything"))

		ft, err = parseTag("-[sui]")
		require.NoError(t, err)
		assert.True(t, ft.isIgnored("sui"))
		assert.False(t, ft.isIgnored("iota"))
		assert.False(t, ft.isIgnored(""))
	})

	t.Run("optional global vs scoped", func(t *testing.T) {
		ft, err := parseTag("optional")
		require.NoError(t, err)
		assert.True(t, ft.isOptional(""))
		assert.True(t, ft.isOptional("iota"))

		ft, err = parseTag("optional[iota]")
		require.NoError(t, err)
		assert.True(t, ft.isOptional("iota"))
		assert.False(t, ft.isOptional("sui"))
	})

	t.Run("enumNum scoped", func(t *testing.T) {
		ft, err := parseTag("enumNum[sui]=2,enumNum[iota]=1")
		require.NoError(t, err)
		n, ok := ft.variantNum("sui")
		assert.True(t, ok)
		assert.Equal(t, 2, n)
		n, ok = ft.variantNum("iota")
		assert.True(t, ok)
		assert.Equal(t, 1, n)
		_, ok = ft.variantNum("other")
		assert.False(t, ok)
		assert.True(t, ft.hasAnyEnumNum())
	})

	t.Run("enumNum global resolves for any selector", func(t *testing.T) {
		ft, err := parseTag("enumNum=5")
		require.NoError(t, err)
		n, ok := ft.variantNum("whatever")
		assert.True(t, ok)
		assert.Equal(t, 5, n)
	})

	t.Run("errors", func(t *testing.T) {
		for _, bad := range []string{"bogus", "enumNum", "enumNum[sui]", "optional=1", "-=x", "-[sui"} {
			_, err := parseTag(bad)
			assert.Error(t, err, "tag %q should error", bad)
		}
	})
}

func TestSelectorValuePrecedence(t *testing.T) {
	ft, err := parseTag("optional,optional[x]") // global true + scoped x true
	require.NoError(t, err)
	assert.True(t, ft.isOptional("x"))
	assert.True(t, ft.isOptional("y")) // falls back to global
}

func TestEnumPositionModeUnchanged(t *testing.T) {
	// no tags -> selector is irrelevant, variant index == field position
	bX := enc(t, "x", posEnum{B: u64p(7)})
	bDefault := enc(t, "", posEnum{B: u64p(7)})
	assert.Equal(t, bDefault, bX)
	assert.Equal(t, byte(1), bX[0]) // B is field index 1

	var out posEnum
	require.NoError(t, dec("y", bX, &out)) // selector ignored in position mode
	assert.Nil(t, out.A)
	require.NotNil(t, out.B)
	assert.Equal(t, uint64(7), *out.B)
}

func TestEnumTagModeCrossChain(t *testing.T) {
	// B under x is variant 1; under y is variant 0
	bx := enc(t, "x", tagEnum{B: u64p(9)})
	by := enc(t, "y", tagEnum{B: u64p(9)})
	assert.Equal(t, byte(1), bx[0])
	assert.Equal(t, byte(0), by[0])

	// decode x-bytes (variant 1) under x -> B; under y, variant 1 -> A
	var asX tagEnum
	require.NoError(t, dec("x", bx, &asX))
	require.NotNil(t, asX.B)
	assert.Equal(t, uint64(9), *asX.B)

	var asY tagEnum
	require.NoError(t, dec("y", bx, &asY))
	require.NotNil(t, asY.A) // same bytes, different field per selector
	assert.Nil(t, asY.B)
	assert.Equal(t, uint64(9), *asY.A)
}

func TestEnumTagPartialAbsentVariant(t *testing.T) {
	// C exists only under x
	bc := enc(t, "x", tagEnum{C: u64p(3)})
	assert.Equal(t, byte(2), bc[0])

	// encoding C under y must fail (no variant for y)
	buf := bytes.NewBuffer(nil)
	err := NewEncoderForSelector(buf, "y").Encode(tagEnum{C: u64p(3)})
	assert.Error(t, err)

	// decoding variant 2 under y must fail (not defined for y)
	var out tagEnum
	assert.Error(t, dec("y", bc, &out))
}

func TestEnumTaggedButSelectorResolvesNothing(t *testing.T) {
	// selector z has no enumNum on any field -> must error, not fall back to position
	var out tagEnum
	assert.Error(t, dec("z", []byte{0x00}, &out))

	buf := bytes.NewBuffer(nil)
	assert.Error(t, NewEncoderForSelector(buf, "z").Encode(tagEnum{A: u64p(1)}))
}

func TestEnumDuplicateNum(t *testing.T) {
	var out dupEnum
	assert.Error(t, dec("x", []byte{0x00}, &out))
}

func TestStructPerSelectorOptional(t *testing.T) {
	// under x: X is Option -> present flag; under y: X is a plain pointer (8 bytes)
	bxNone := enc(t, "x", optStruct{X: nil})
	assert.Equal(t, []byte{0x00}, bxNone)

	bxSome := enc(t, "x", optStruct{X: u64p(5)})
	assert.Len(t, bxSome, 1+8)
	assert.Equal(t, byte(1), bxSome[0])

	bySome := enc(t, "y", optStruct{X: u64p(5)})
	assert.Len(t, bySome, 8) // no present flag

	var ox optStruct
	require.NoError(t, dec("x", bxNone, &ox))
	assert.Nil(t, ox.X)
	require.NoError(t, dec("x", bxSome, &ox))
	require.NotNil(t, ox.X)
	assert.Equal(t, uint64(5), *ox.X)

	var oy optStruct
	require.NoError(t, dec("y", bySome, &oy))
	require.NotNil(t, oy.X)
	assert.Equal(t, uint64(5), *oy.X)
}
