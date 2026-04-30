package objectx

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_CollectTagValue(t *testing.T) {

	type ObjectB struct {
		P1 string `xxx:"pp1"`
		P2 string `yyy:"pp2"`
		P3 string `xxx:"pp3"`
	}

	type ObjectA struct {
		Name  string `xxx:"name"`
		Email string `xxx:"email"`
		ObjectB
	}

	assert.PanicsWithError(t, "need a struct, but is a int", func() {
		_ = CollectTagValue(1, "xxx")
	})
	assert.Equal(t, []string{"name", "email", "pp1", "", "pp3"}, CollectTagValue(ObjectA{}, "xxx"))
	assert.Equal(t, []string{"", "", "", "pp2", ""}, CollectTagValue(&ObjectA{}, "yyy"))
	assert.Equal(t, []string{"pp2"}, CollectTagValue(&ObjectA{}, "yyy", HasTag("yyy")))
	assert.Equal(t, []string{"pp1", "pp3"}, CollectTagValue(ObjectB{}, "xxx", HasTag("xxx")))
	assert.Equal(t, []string{"pp2"}, CollectTagValue(&ObjectB{}, "yyy", HasTag("yyy")))
}

func Test_CollectFieldValues(t *testing.T) {
	type ObjectB struct {
		P1 int   `xxx:"pp1"`
		P2 int32 `yyy:"pp2"`
		P3 int64 `xxx:"pp3"`
	}

	type ObjectA struct {
		Name  string `xxx:"name"`
		Email string `xxx:"email"`
		ObjectB
	}

	a := ObjectA{
		Name:  "a",
		Email: "a@a.com",
		ObjectB: ObjectB{
			P1: 111,
			P2: 222,
			P3: 3,
		},
	}

	assert.Equal(t, []any{"a", "a@a.com", int(111), int64(3)}, CollectFieldValues(a, HasTag("xxx")))
	assert.Equal(t, []any{int32(222)}, CollectFieldValues(&a, HasTag("yyy")))
	assert.Equal(t, []any{int32(222)}, CollectFieldValues(&a, TagEqualTo("yyy", "pp2")))
	assert.Equal(t, []any(nil), CollectFieldValues(&a, TagEqualTo("yyy", "pp3")))
}

func Test_CollectFieldPointers(t *testing.T) {
	type ObjectB struct {
		P1 int   `xxx:"pp1"`
		P2 int32 `yyy:"pp2"`
		P3 int64 `xxx:"pp3"`
	}

	type ObjectA struct {
		Name  string `xxx:"name"`
		Email string `xxx:"email"`
		ObjectB
	}

	a := ObjectA{
		Name:  "a",
		Email: "a@a.com",
		ObjectB: ObjectB{
			P1: 111,
			P2: 222,
			P3: 3,
		},
	}

	assert.PanicsWithError(t, "need a pointer, but is a struct", func() {
		_ = CollectFieldPointers(a)
	})
	assert.Equal(t, []any{&a.Name, &a.Email, &a.P1, &a.P3}, CollectFieldPointers(&a, HasTag("xxx")))
	assert.Equal(t, []any{&a.P2}, CollectFieldPointers(&a, HasTag("yyy")))

	// modify using the pointer
	{
		ps := CollectFieldPointers(&a, HasTag("yyy"))
		p := ps[0].(*int32)
		*p = int32(2222)
		assert.Equal(t, int32(2222), a.P2)
	}
	{
		ps := CollectFieldPointers(&a, HasTag("xxx"))
		p0 := ps[0].(*string) // Mame
		p1 := ps[1].(*string) // Email
		p2 := ps[2].(*int)    // P1
		p3 := ps[3].(*int64)  // P3
		*p0 = "foo"
		*p1 = "bar"
		*p2 = 1111
		*p3 = 3333
		assert.Equal(t, ObjectA{
			Name:  "foo",
			Email: "bar",
			ObjectB: ObjectB{
				P1: 1111,
				P2: 2222,
				P3: 3333,
			},
		}, a)
	}

}
