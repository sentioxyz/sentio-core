package jsonutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

type Tracker func(path string, or, pa any)

func Patch(origin, patch []byte, tracker Tracker) (final []byte, err error) {
	var o, p map[string]any
	var decoder *json.Decoder
	decoder = json.NewDecoder(bytes.NewReader(origin))
	decoder.UseNumber()
	if err = decoder.Decode(&o); err != nil {
		return
	}
	decoder = json.NewDecoder(bytes.NewReader(patch))
	decoder.UseNumber()
	if err = decoder.Decode(&p); err != nil {
		return
	}
	if err = patchObject("", o, p, tracker); err != nil {
		return
	}
	return json.Marshal(o)
}

const wildcard = "*"

func patchObject(path string, origin, patch map[string]any, tracker Tracker) error {
	for key, pv := range patch {
		if key == wildcard {
			for key, ov := range origin {
				keyPath := path + "." + key
				if reflect.ValueOf(ov).Type().String() != reflect.ValueOf(pv).Type().String() {
					continue
				} else if reflect.ValueOf(ov).Kind() == reflect.Map {
					if err := patchObject(keyPath, ov.(map[string]any), pv.(map[string]any), tracker); err != nil {
						return err
					}
					continue
				}
				if tracker != nil {
					tracker(keyPath, ov, pv)
				}
				origin[key] = pv
			}
			continue
		}

		ov, has := origin[key]
		keyPath := path + "." + key

		if has {
			if reflect.ValueOf(ov).Type().String() != reflect.ValueOf(pv).Type().String() {
				return fmt.Errorf("cannot patch %s/%T by %T", keyPath, ov, pv)
			} else if reflect.ValueOf(ov).Kind() == reflect.Map {
				if err := patchObject(keyPath, ov.(map[string]any), pv.(map[string]any), tracker); err != nil {
					return err
				}
				continue
			}
		}

		if tracker != nil {
			tracker(keyPath, ov, pv)
		}
		origin[key] = pv
	}
	return nil
}
