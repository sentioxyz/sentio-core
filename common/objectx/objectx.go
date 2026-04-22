package objectx

import (
	"fmt"
	"reflect"
	"sentioxyz/sentio-core/common/set"
)

type Filter func([]reflect.StructField) bool

func HasTag(tagName string) Filter {
	return func(up []reflect.StructField) bool {
		_, has := up[len(up)-1].Tag.Lookup(tagName)
		return has
	}
}

func NoTag(tagName, tagValue string) Filter {
	return func(up []reflect.StructField) bool {
		for _, fd := range up {
			if tv, has := fd.Tag.Lookup(tagName); has && tv == tagValue {
				return false
			}
		}
		return true
	}
}

func AnyHasTagEqualTo(tagName, tagValue string) Filter {
	return func(up []reflect.StructField) bool {
		for _, fd := range up {
			if tv, has := fd.Tag.Lookup(tagName); has && tv == tagValue {
				return true
			}
		}
		return false
	}
}

func TagEqualTo(tagName, tagValue string) Filter {
	return func(up []reflect.StructField) bool {
		tv, has := up[len(up)-1].Tag.Lookup(tagName)
		return has && tv == tagValue
	}
}

func TagValueIn(tagName string, tagValues ...string) Filter {
	s := set.New(tagValues...)
	return func(up []reflect.StructField) bool {
		tv, has := up[len(up)-1].Tag.Lookup(tagName)
		return has && s.Contains(tv)
	}
}

func TagNotEqualTo(tagName, tagValue string) Filter {
	return func(up []reflect.StructField) bool {
		tv, has := up[len(up)-1].Tag.Lookup(tagName)
		return has && tv != tagValue
	}
}

func (f Filter) And(ano Filter) Filter {
	return func(fd []reflect.StructField) bool {
		return f(fd) && ano(fd)
	}
}

func (f Filter) Or(or Filter) Filter {
	return func(fd []reflect.StructField) bool {
		return f(fd) || or(fd)
	}
}

type Watcher func([]reflect.StructField, reflect.Value)

func check(path []reflect.StructField, filters ...Filter) bool {
	for _, filter := range filters {
		if !filter(path) {
			return false
		}
	}
	return true
}

func walk(val reflect.Value, up []reflect.StructField, watcher Watcher, filters ...Filter) {
	for val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	typ := val.Type()
	if typ.Kind() != reflect.Struct {
		panic(fmt.Errorf("need a struct, but is a %s", typ.Kind()))
	}
	for i, n := 0, typ.NumField(); i < n; i++ {
		field, value := typ.Field(i), val.Field(i)
		path := append(up, field)
		if typ.Field(i).Anonymous {
			walk(value, path, watcher, filters...)
		} else if check(path, filters...) {
			watcher(path, value)
		}
	}
}

func Walk(obj any, watcher Watcher, filters ...Filter) {
	walk(reflect.ValueOf(obj), nil, watcher, filters...)
}

func CollectTagValue(obj any, tagName string, filters ...Filter) (tagValues []string) {
	Walk(obj, func(fields []reflect.StructField, _ reflect.Value) {
		tagValues = append(tagValues, fields[len(fields)-1].Tag.Get(tagName))
	}, filters...)
	return tagValues
}

func CollectFieldValues(obj any, filters ...Filter) (values []any) {
	Walk(obj, func(_ []reflect.StructField, value reflect.Value) {
		values = append(values, value.Interface())
	}, filters...)
	return values
}

func CollectFieldPointers(obj any, filters ...Filter) (pointers []any) {
	if typ := reflect.TypeOf(obj); typ.Kind() != reflect.Ptr {
		panic(fmt.Errorf("need a pointer, but is a %s", typ.Kind()))
	}
	Walk(obj, func(_ []reflect.StructField, value reflect.Value) {
		pointers = append(pointers, value.Addr().Interface())
	}, filters...)
	return pointers
}
