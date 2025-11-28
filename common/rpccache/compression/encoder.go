package compression

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
)

func Encode[T any](data *T) (string, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	encoder := gob.NewEncoder(gw)
	if err := encoder.Encode(data); err == nil {
		if err := gw.Flush(); err != nil {
			return "", err
		}
		if err := gw.Close(); err != nil {
			return "", err
		}
		return buf.String(), nil
	} else {
		return "", err
	}
}
