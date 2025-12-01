package anyutil

import (
	"reflect"
	"strconv"
	"strings"
)

const (
	Delimiter       = "."
	ArrayIndexLeft  = '['
	ArrayIndexRight = ']'
	ArrayIndexAny   = "*"
)

func parsePropertyKey(key string) (property string, index int, allIndex bool, is bool) {
	property = key
	if key == "" {
		return
	}
	if key[len(key)-1] != ArrayIndexRight {
		return
	}
	var p int
	for p = 0; p < len(key) && key[p] != ArrayIndexLeft; p++ {
	}
	if p >= len(key) {
		return
	}
	property = key[:p]
	indexStr := key[p+1 : len(key)-1]
	if indexStr == ArrayIndexAny {
		allIndex, is = true, true
		return
	}
	id, err := strconv.ParseInt(indexStr, 10, 32)
	if err != nil {
		return
	}
	index, is = int(id), true
	return
}

func GetPropertyByPath(root any, path string) (any, bool) {
	if path == "" {
		return root, true
	}

	var obj map[string]any
	switch r := root.(type) {
	case map[string]any:
		obj = r
	case map[any]any:
		obj = ToStringAnyMap(r)
	default:
		return nil, false
	}

	top, down, _ := strings.Cut(path, Delimiter)
	propertyName, index, allIndex, isArr := parsePropertyKey(top)

	property, has := obj[propertyName]
	if !has {
		return nil, false
	}
	if !isArr {
		return GetPropertyByPath(property, down)
	}
	value := reflect.ValueOf(property)
	switch value.Type().Kind() {
	case reflect.Array, reflect.Slice:
		if !allIndex {
			if index >= value.Len() || index < -value.Len() {
				return nil, false // out of range
			}
			if index < 0 {
				index += value.Len()
			}
			return GetPropertyByPath(value.Index(index).Interface(), down)
		}
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i], _ = GetPropertyByPath(value.Index(i).Interface(), down)
		}
		return result, true
	default:
		return nil, false
	}
}

func GetPropertyByPathWithDefault(root any, path string, defaultValue any) any {
	v, has := GetPropertyByPath(root, path)
	if !has {
		return defaultValue
	}
	return v
}
