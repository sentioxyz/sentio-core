package statemirror

import "encoding/json"

type JSONCodec[K comparable, V any] struct {
	FieldFunc func(K) (string, error)
	ParseFunc func(string) (K, error)
}

func (c JSONCodec[K, V]) Field(k K) (string, error) { return c.FieldFunc(k) }
func (c JSONCodec[K, V]) ParseField(field string) (K, error) {
	return c.ParseFunc(field)
}
func (c JSONCodec[K, V]) Encode(v V) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func (c JSONCodec[K, V]) Decode(s string) (V, error) {
	var v V
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}
