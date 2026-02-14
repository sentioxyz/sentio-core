package jsonrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/vmihailenco/msgpack/v5"
	"testing"
)

type testErr struct {
	code int
	msg  string
	data any
}

func (e testErr) Error() string {
	return e.msg
}

func (e testErr) ErrorCode() int {
	return e.code
}

func (e testErr) ErrorData() any {
	return e.data
}

func Test_marshalResult(t *testing.T) {
	req := JsonrpcMessage{
		Version: "2.0",
		ID:      json.RawMessage("1"),
	}
	var buf bytes.Buffer
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONResponse(&req, "good")))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"result":"good"}`+"\n", buf.String())
	}
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONErrorResponse(&req, nil, fmt.Errorf("bad"))))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"bad"}}`+"\n", buf.String())
	}
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONErrorResponse(&req, nil, &testErr{
			code: 123,
			msg:  "msg",
			data: map[string]any{"abc": 123, "foo": "bar"},
		})))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":123,"message":"msg","data":{"abc":123,"foo":"bar"}}}`+"\n", buf.String())
	}
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONErrorResponse(&req, "result", &testErr{
			code: 123,
			msg:  "msg",
			data: map[string]any{"abc": 123, "foo": "bar"},
		})))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":123,"message":"msg","data":{"abc":123,"foo":"bar"}},"result":"result"}`+"\n", buf.String())
	}
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONErrorResponse(&req, json.RawMessage(nil), fmt.Errorf("bad"))))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"bad"}}`+"\n", buf.String())
	}
	{
		buf.Reset()
		encoder := json.NewEncoder(&buf)
		assert.NoError(t, encoder.Encode(JSONErrorResponse(&req, (*string)(nil), fmt.Errorf("bad"))))
		assert.Equal(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"bad"}}`+"\n", buf.String())
	}
}

func Test_msgEncode(t *testing.T) {
	var b bytes.Buffer
	xx, _ := msgpack.Marshal(map[string]any{"aaaa": "aaaaa"})

	var x = map[string]any{
		"aaa": msgpack.RawMessage(xx),
		//"bbb": "ccc",
		//"ddd": map[string]any{
		//	"eee": "fff",
		//},
		//"ggg": []any{
		//	456,
		//	"hhh",
		//	map[string]any{
		//		"iii": 789,
		//	},
		//},
	}

	err := msgpack.NewEncoder(&b).Encode(x)
	assert.NoError(t, err)
	const column = 16
	for i, c := range b.Bytes() {
		if i%column == 0 {
			fmt.Printf("\n%04d-%04d:", i, i+column)
		}
		fmt.Printf(" 0x%02x", c)
	}

	//                     {     str3  a     a     a     {     str4  a     a     a     a     str5  a     a     a     a     a
	assert.Equal(t, []byte{0x81, 0xa3, 0x61, 0x61, 0x61, 0x81, 0xa4, 0x61, 0x61, 0x61, 0x61, 0xa5, 0x61, 0x61, 0x61, 0x61, 0x61}, b.Bytes())
}
