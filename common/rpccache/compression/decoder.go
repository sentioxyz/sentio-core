package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
)

func Decode[T any](encoded string) (*T, error) {
	var decoded T
	buf := bytes.NewBufferString(encoded)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()
	decoder := gob.NewDecoder(gr)
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return &decoded, nil
}
