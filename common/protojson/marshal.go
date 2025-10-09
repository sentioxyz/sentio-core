package protojson

import (
	"bytes"
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var Unmarshaler = protojson.UnmarshalOptions{
	DiscardUnknown: true,
	AllowPartial:   true,
}

func Unmarshal(data []byte, pb proto.Message) error {
	return Unmarshaler.Unmarshal(data, pb)
}

var marshaller = protojson.MarshalOptions{}

func Marshal(pb proto.Message) ([]byte, error) {
	return marshaller.Marshal(pb)
}

func MustJSONMarshal(pb proto.Message) []byte {
	bs, _ := Marshal(pb)
	return bs
}

// IndentFormat is a wrapper of json.Indent
// usage: https://github.com/golang/protobuf/issues/1121
func IndentFormat(data []byte) ([]byte, error) {
	return IndentFormatWithOptions(data, "", "\t")
}

func IndentFormatWithOptions(data []byte, prefix, indent string) ([]byte, error) {
	var out bytes.Buffer
	err := json.Indent(&out, data, prefix, indent)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func MarshalInStableWithIndent(pb proto.Message) ([]byte, error) {
	data, err := Marshal(pb)
	if err != nil {
		return nil, err
	}
	return IndentFormat(data)
}
