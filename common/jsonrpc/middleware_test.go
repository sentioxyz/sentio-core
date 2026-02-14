package jsonrpc

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

type testService struct {
	p int
}

func (s *testService) Foo(ctx context.Context, arg0 int) (int, error) {
	return arg0 + s.p, nil
}

func (s *testService) Foo2(ctx context.Context, arg0 string, arg1 int) (string, error) {
	return fmt.Sprintf("%s_%d", arg0, arg1+s.p), nil
}

func (s *testService) Foo3(ctx context.Context, arg0 int) (int, error) {
	return s.p / arg0, nil
}

func Test_MakeServiceAsMiddleware(t *testing.T) {
	s := testService{p: 234}
	mc := MiddlewareChain{MakeServiceAsMiddleware("xx", &s)}

	r, err := mc.CallMethod(context.Background(), "xx_foo", []byte(`123`))
	assert.Nil(t, err)
	assert.Equal(t, 357, r)

	r, err = mc.CallMethod(context.Background(), "xx_foo", []byte("[123]"))
	assert.NoError(t, err)
	assert.Equal(t, 357, r)

	r, err = mc.CallMethod(context.Background(), "xx_foo2", []byte(`["abc",123]`))
	assert.NoError(t, err)
	assert.Equal(t, "abc_357", r)

	_, err = mc.CallMethod(context.Background(), "xx_foo3", []byte(`0`))
	assert.ErrorContains(t, err, "method handler crashed: runtime error: integer divide by zero")

	_, err = mc.CallMethod(context.Background(), "xx_fooX", []byte(`0`))
	assert.ErrorContains(t, err, "reaches final handler, method xx_fooX is not handled")
}
