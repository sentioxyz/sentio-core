package anyutil

import "reflect"

var (
	floatType  = reflect.TypeOf(float64(0))
	stringType = reflect.TypeOf("")
)

// deref unwraps pointers and interfaces until it reaches a concrete value.
// If the input (or any level) is nil, returns (nil, true).
func deref(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, true
		}
		rv = rv.Elem()
	}
	return rv.Interface(), false
}
