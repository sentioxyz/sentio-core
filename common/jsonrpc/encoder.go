package jsonrpc

import (
	"bytes"
	"encoding/json"
	"github.com/bytedance/sonic/encoder"
	"github.com/vmihailenco/msgpack/v5"
	"io"
)

type Encoder interface {
	Marshal(any) (any, int, error)
	Encode(io.Writer, any) error
	ContentType() string
}

type MsgpackEncoder struct {
}

func (enc MsgpackEncoder) Encode(w io.Writer, v any) error {
	e := msgpack.NewEncoder(w)
	e.SetCustomStructTag("json")
	return e.Encode(v)
}

func (enc MsgpackEncoder) Marshal(v any) (any, int, error) {
	var buf bytes.Buffer
	err := enc.Encode(&buf, v)
	return msgpack.RawMessage(buf.Bytes()), buf.Len(), err
}

func (enc MsgpackEncoder) ContentType() string {
	return "application/msgpack"
}

type JsonEncoder struct {
}

func (enc JsonEncoder) Encode(w io.Writer, v any) error {
	return encoder.NewStreamEncoder(w).Encode(v)
}

func (enc JsonEncoder) Marshal(v any) (any, int, error) {
	var buf bytes.Buffer
	err := enc.Encode(&buf, v)
	return json.RawMessage(buf.Bytes()), buf.Len(), err
}

func (enc JsonEncoder) ContentType() string {
	return "application/json"
}
