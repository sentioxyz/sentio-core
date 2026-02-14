package jsonrpc

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

func Test_CallMethod(t *testing.T) {
	type Book struct {
		Name   string `json:"name"`
		Author string `json:"author"`
	}
	f0 := func(ctx context.Context, arg0 int) (int, error) {
		return arg0 + 1, nil
	}
	f1 := func(ctx context.Context, book Book) (Book, error) {
		return Book{
			Name:   book.Name + "_xxx",
			Author: book.Author,
		}, nil
	}
	f2 := func(ctx context.Context, book Book, num int64) (Book, error) {
		return Book{
			Name:   book.Name + "_" + strconv.FormatInt(num, 10),
			Author: book.Author,
		}, nil
	}

	r, err := CallMethod(f0, context.Background(), []byte("123"))
	assert.NoError(t, err)
	assert.Equal(t, 124, r)

	r, err = CallMethod(f0, context.Background(), []byte("[123]"))
	assert.NoError(t, err)
	assert.Equal(t, 124, r)

	r, err = CallMethod(f1, context.Background(), []byte(`{"name":"foo","author":"bar"}`))
	assert.NoError(t, err)
	assert.Equal(t, Book{
		Name:   "foo_xxx",
		Author: "bar",
	}, r)

	r, err = CallMethod(f2, context.Background(), []byte(`[{"name":"foo","author":"bar"},123]`))
	assert.NoError(t, err)
	assert.Equal(t, Book{
		Name:   "foo_123",
		Author: "bar",
	}, r)

	r, err = CallMethod(f2, context.Background(), []byte(`[{"name":"foo","author":"bar"}]`))
	assert.NoError(t, err)
	assert.Equal(t, Book{
		Name:   "foo_0",
		Author: "bar",
	}, r)

	r, err = CallMethod(f2, context.Background(), []byte(`[{"name":"foo","author":"bar"},456,789]`))
	assert.Equal(t, err.Error(), "too many arguments, want at most 2")
	assert.Nil(t, r)
}

type testObj struct {
	p int
}

func (o *testObj) foo1(ctx context.Context, arg0 int) (int, error) {
	return arg0 + o.p, nil
}

func (o *testObj) foo2(ctx context.Context, arg0 string, arg1 int) (string, error) {
	return fmt.Sprintf("%s_%d", arg0, arg1+o.p), nil
}

func Test_CallMethod2(t *testing.T) {
	obj := testObj{234}

	r, err := CallMethod(obj.foo1, context.Background(), []byte("123"))
	assert.NoError(t, err)
	assert.Equal(t, 357, r)

	r, err = CallMethod(obj.foo1, context.Background(), []byte("[123]"))
	assert.NoError(t, err)
	assert.Equal(t, 357, r)

	r, err = CallMethod(obj.foo2, context.Background(), []byte(`["abc",123]`))
	assert.NoError(t, err)
	assert.Equal(t, "abc_357", r)
}
