package compress

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
)

type compressPayload struct {
	CompressMethod string `json:"compress_method,omitempty"`
	Data           []byte `json:"data,omitempty"`
}

const (
	compressMethodGZIP = "gzip"
)

func Load(raw []byte, d any) (err error) {
	if len(raw) == 0 {
		return nil
	}
	var payload compressPayload
	_ = json.Unmarshal(raw, &payload)
	var r io.Reader
	switch payload.CompressMethod {
	case compressMethodGZIP:
		r, err = gzip.NewReader(bytes.NewReader(payload.Data))
		if err != nil {
			return errors.Wrapf(err, "try to load as compressed payload failed")
		}
	default:
		r = bytes.NewReader(raw)
	}
	return json.NewDecoder(r).Decode(d)
}

func Dump(d any) ([]byte, error) {
	return dump(d, compressMethodGZIP)
}

func dump(d any, compressMethod string) ([]byte, error) {
	// prepare WriteCloser by compressMethod
	var buf bytes.Buffer
	var w io.WriteCloser
	switch compressMethod {
	case compressMethodGZIP:
		w = gzip.NewWriter(&buf)
	default:
		return nil, errors.Errorf("compress method %s not supported", compressMethod)
	}
	// json marshal and do compress
	err := json.NewEncoder(w).Encode(d)
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		return nil, errors.Wrapf(err, "dump with compress method %s failed", compressMethod)
	}
	// build payload and json marshal
	var raw []byte
	raw, err = json.Marshal(compressPayload{
		CompressMethod: compressMethod,
		Data:           buf.Bytes(),
	})
	if err != nil {
		err = errors.Wrapf(err, "dump with compress method %s failed", compressMethod)
	}
	return raw, err
}
