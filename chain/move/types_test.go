package move

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"testing"
)

func Test_BuildType(t *testing.T) {
	var p Type
	var err error

	p, err = BuildType("a")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		Simple: utils.WrapPointer("a"),
	}, p)
	p, err = BuildType("a::b")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "",
		},
	}, p)

	p, err = BuildType("a::b::c")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "c",
		},
	}, p)

	p, err = BuildType("a::b::c::d")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "c::d",
		},
	}, p)

	p, err = BuildType("a::b::c<a>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
		},
	}, p)

	p, err = BuildType("a::b::c<a,b>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
			{Simple: utils.WrapPointer("b")},
		},
	}, p)

	p, err = BuildType("a::b::c<a, 0x1::foo::bar<b>, any>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "a",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
			{
				FQN: &FullyName{
					Address: "0x1",
					Module:  "foo",
					Name:    "bar",
				},
				Args: TypeArgs{
					{Simple: utils.WrapPointer("b")},
				},
			},
			{},
		},
	}, p)

	p, err = BuildType("a::b::c<a, 0x1::foo::bar<b>>, any>")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing '<'"))

	p, err = BuildType("a::b::c<a, 0x1::foo::bar<<b>, any>")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "missing '>'"))

	p, err = BuildType("a::b::c<a, 0x1::foo::bar<b>, any")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "should be wrapped in <>"))
}

func Test_BuildType_useShortAddr(t *testing.T) {
	var p Type
	var err error

	p, err = BuildType("0x000000")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		Simple: utils.WrapPointer("0x000000"),
	}, p)
	p, err = BuildType("0x0001::b")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		Simple: utils.WrapPointer("0x0001::b"),
	}, p)

	p, err = BuildType("0x00000::b::c")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x0",
			Module:  "b",
			Name:    "c",
		},
	}, p)
	p, err = BuildType("0x00001::b::c")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x1",
			Module:  "b",
			Name:    "c",
		},
	}, p)

	p, err = BuildType("0x001::b::c::d")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x1",
			Module:  "b",
			Name:    "c::d",
		},
	}, p)

	p, err = BuildType("0x0001::b::c<a>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x1",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
		},
	}, p)

	p, err = BuildType("0x0001::b::c<a,b>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x1",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
			{Simple: utils.WrapPointer("b")},
		},
	}, p)

	p, err = BuildType("0x0001::b::c<a, 0x0002::foo::bar<b>, any>")
	assert.NoError(t, err)
	assert.NotNil(t, Type{
		FQN: &FullyName{
			Address: "0x1",
			Module:  "b",
			Name:    "c",
		},
		Args: TypeArgs{
			{Simple: utils.WrapPointer("a")},
			{
				FQN: &FullyName{
					Address: "0x2",
					Module:  "foo",
					Name:    "bar",
				},
				Args: TypeArgs{
					{Simple: utils.WrapPointer("b")},
				},
			},
			{},
		},
	}, p)
}

func Test_Include(t *testing.T) {
	p, _ := BuildType("a::b::c<a,b>")
	assert.False(t, p.Include(Type{}))
	assert.False(t, p.Include(MustBuildType("a")))
	assert.False(t, p.Include(MustBuildType("a::b::c")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,c>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,b,c>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,b>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,b<x>>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,b<x,y>>")))

	p, _ = BuildType("a::b::c")
	assert.False(t, p.Include(Type{}))
	assert.False(t, p.Include(MustBuildType("a")))
	assert.True(t, p.Include(MustBuildType("a::b::c")))
	assert.False(t, p.Include(MustBuildType("aa::b::c")))
	assert.False(t, p.Include(MustBuildType("a::bb::c")))
	assert.False(t, p.Include(MustBuildType("a::b::cc")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,c>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,b,c>")))
	assert.True(t, p.Include(MustBuildType("a::b::c<a,b>")))

	p, _ = BuildType("a::::c<any,b>")
	assert.False(t, p.Include(Type{}))
	assert.False(t, p.Include(MustBuildType("a")))
	assert.False(t, p.Include(MustBuildType("a::b::c")))
	assert.False(t, p.Include(MustBuildType("aa::b::c")))
	assert.False(t, p.Include(MustBuildType("a::bb::c")))
	assert.False(t, p.Include(MustBuildType("a::b::cc")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,c>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,b,c>")))
	assert.True(t, p.Include(MustBuildType("a::b1::c<a,b>")))
	assert.True(t, p.Include(MustBuildType("a::b2::c<b,b>")))

	p, _ = BuildType("a::*::c<a<*>,b>")
	assert.False(t, p.Include(Type{}))
	assert.False(t, p.Include(MustBuildType("a")))
	assert.False(t, p.Include(MustBuildType("a::b::c")))
	assert.False(t, p.Include(MustBuildType("aa::b::c")))
	assert.False(t, p.Include(MustBuildType("a::bb::c")))
	assert.False(t, p.Include(MustBuildType("a::b::cc")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,c>")))
	assert.False(t, p.Include(MustBuildType("a::b::c<a,b,c>")))
	assert.True(t, p.Include(MustBuildType("a::b1::c<a<a1>,b>")))
	assert.True(t, p.Include(MustBuildType("a::b1::c<a<a2>,b>")))
	assert.True(t, p.Include(MustBuildType("a::b2::c<a<a2>,b<b2>>")))
	assert.False(t, p.Include(MustBuildType("a::b1::c<a<a2>,c>")))
	assert.False(t, p.Include(MustBuildType("a::b2::c<a<a1,a2>,b>")))
}

func Test_jsonMarshal(t *testing.T) {
	p, _ := BuildType(" a :: ::c < a < any>, b>   ")
	s, err := json.Marshal(p)
	assert.NoError(t, err)
	assert.Equal(t, `"a::*::c\u003ca\u003c*\u003e,b\u003e"`, string(s))

	var a Type
	assert.NoError(t, json.Unmarshal(s, &a))
	assert.True(t, p.Equal(&a))
	assert.Equal(t, p, a)
}
