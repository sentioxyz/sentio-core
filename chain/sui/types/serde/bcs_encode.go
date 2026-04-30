package serde

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"sentioxyz/sentio-core/common/log"

	"github.com/fardream/go-bcs/bcs"
	"github.com/pkg/errors"
)

// Encoder takes an [io.Writer] and encodes value into it.
type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new [Encoder] from an [io.Writer]
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		w: w,
	}
}

func Encode(w io.Writer, v any) error {
	return NewEncoder(w).encode(reflect.ValueOf(v))
}

// Encode a value v into the encoder.
//
//   - If the value is [Marshaler], the corresponding
//     MarshalBCS implementation will be called.
//   - If the value is [Enum], it will be special handled for [Enum].
func (e *Encoder) Encode(v any) error {
	return e.encode(reflect.ValueOf(v))
}

// encode a value
func (e *Encoder) encode(v reflect.Value) error {
	if Trace {
		defer func() {
			log.Debug("<< encode", "type", v.Type().String())
		}()
		log.Debug(">> encode", "type", v.Type().String())
	}
	// if v not CanInterface,
	// this value is an unexported value, skip it.
	if !v.CanInterface() {
		return nil
	}

	// test for the two interfaces we defined.
	// 1. Marshaler
	// 2. Enum.
	if v.CanAddr() {
		i := v.Addr().Interface()
		if m, ismarshaler := i.(bcs.Marshaler); ismarshaler {
			bytes, err := m.MarshalBCS()
			if err != nil {
				return err
			}

			_, err = e.w.Write(bytes)

			return err
		}
		if _, isenum := i.(bcs.Enum); isenum {
			return e.encodeEnum(reflect.Indirect(v))
		}
	}
	i := v.Interface()
	if m, ismarshaler := i.(bcs.Marshaler); ismarshaler {
		bytes, err := m.MarshalBCS()
		if err != nil {
			return err
		}

		_, err = e.w.Write(bytes)

		return err
	}
	if _, isenum := i.(bcs.Enum); isenum {
		return e.encodeEnum(reflect.Indirect(v))
	}

	kind := v.Kind()

	switch kind { //nolint:exhaustive
	case reflect.Bool, // boolean
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, // all the ints
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64: // all the uints
		// use little endian to encode those.
		return binary.Write(e.w, binary.LittleEndian, v.Interface())

	case reflect.Pointer: // pointer
		// if v is nil pointer, use the zero value for v.
		// we don't check for optional flag here.
		// that should be checked when the container struct is encoded
		// if this pointer is contained in a struct.
		return e.encode(reflect.Indirect(v))

	case reflect.Interface:
		return e.encode(v.Elem())

	case reflect.Slice: // slices
		// check if the element is uint8 or byteslice
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.encodeByteSlice(v.Bytes())
		}
		return e.encodeSlice(v)

	case reflect.Array: // encode array
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.encodeByteArray(v.Bytes())
		}
		return e.encodeArray(v)

	case reflect.String:
		str := []byte(v.String())
		return e.encodeByteSlice(str)

	case reflect.Struct:
		return e.encodeStruct(v)

	case reflect.Chan, reflect.Func, reflect.Uintptr, reflect.UnsafePointer: // channel, func, pointers
		return nil

	default:
		return fmt.Errorf(
			"unsupported kind: %s",
			kind.String(),
		)
	}
}

// encodeEnum encodes an [Enum]
func (e *Encoder) encodeEnum(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		// ignore fields that are not exported
		if !field.CanInterface() {
			continue
		}

		fieldType := t.Field(i)
		// check the tag
		tag, err := parseTagValue(fieldType.Tag.Get(tagName))
		if err != nil {
			return err
		}
		if tag&tagValueIgnore > 0 {
			continue
		}
		fieldKind := field.Kind()
		if fieldKind != reflect.Pointer && fieldKind != reflect.Interface {
			return fmt.Errorf(
				"enum only supports fields that are either pointers or interfaces, unless they are ignored",
			)
		}
		if !field.IsNil() {
			if _, err := e.w.Write(bcs.ULEB128Encode(i)); err != nil {
				return err
			}
			if fieldKind == reflect.Pointer {
				return e.encode(reflect.Indirect(field))
			} else {
				return e.encode(v)
			}
		}
	}

	return fmt.Errorf("no field is set in the enum")
}

func WriteByteSlice(b []byte) ([]byte, error) {
	l := len(b)
	buf := bytes.NewBuffer(nil)
	if _, err := buf.Write(bcs.ULEB128Encode(l)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(b); err != nil {
		return nil, err
	}
	if Trace {
		log.Debug("write byte slice of size", "len", buf.Len())
	}

	return buf.Bytes(), nil
}

// encodeByteSlice is specialized since bytes those can be simply put into the output.
func (e *Encoder) encodeByteSlice(b []byte) error {
	l := len(b)
	if _, err := e.w.Write(bcs.ULEB128Encode(l)); err != nil {
		return err
	}

	if _, err := e.w.Write(b); err != nil {
		return err
	}

	return nil
}

func (e *Encoder) encodeByteArray(v []byte) error {
	_, err := e.w.Write(v)
	return err
}

func (e *Encoder) encodeArray(v reflect.Value) error {
	length := v.Len()
	for i := 0; i < length; i++ {
		if err := e.encode(v.Index(i)); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeSlice(v reflect.Value) error {
	length := v.Len()
	if _, err := e.w.Write(bcs.ULEB128Encode(length)); err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		if err := e.encode(v.Index(i)); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) encodeStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if Trace {
			log.Debug("encode field", "name", t.Field(i).Name, "type", t.Field(i).Type)
		}
		// if a field is not exported, ignore
		if !field.CanInterface() {
			continue
		}
		tag, err := parseTagValue(t.Field(i).Tag.Get(tagName))
		if err != nil {
			return err
		}
		// ignored
		if tag&tagValueIgnore != 0 {
			continue
		}

		// optional
		if tag&tagValueOptional != 0 {
			if field.Kind() != reflect.Pointer && field.Kind() != reflect.Interface {
				return fmt.Errorf("optional field can only be pointer or interface")
			}
			if field.IsNil() {
				_, err := e.w.Write([]byte{0})
				if err != nil {
					return err
				}
			} else {
				if _, err := e.w.Write([]byte{1}); err != nil {
					return err
				}
				if err := e.encode(field.Elem()); err != nil {
					return err
				}
			}
			continue
		}
		// finally
		if err := e.encode(field); err != nil {
			return errors.Wrap(err, t.Field(i).Name)
		}
	}

	return nil
}

// Marshal a value into bcs bytes.
//
// Many constructs supported by bcs don't exist in golang or move-lang.
//
//   - [Enum] is used to simulate the effects of rust enum.
//   - Use tag `optional` to indicate an optional value in rust.
//     the field must be pointer or interface.
//   - Use tag `-` to ignore fields.
//   - Unexported fields are ignored.
//
// Note that bcs doesn't have schema, and field names are irrelevant. The fields
// of struct are serialized in the order that they are defined.
//
// Pointers are serialized as the type they point to. Nil pointers will be serialized
// as zero value of the type they point to unless it's marked as `optional`.
//
// Arrays are serialized as fixed length vector (or serialize the each object individually without prefixing
// the length of the array).
//
// Vanilla maps are not supported, however, the code will error if map is encountered to call out they are
// not supported and either ignore or provide a customized marshal function.
//
// Channels, functions are silently ignored.
//
// During marshalling process, how v is marshalled depends on if v implemented [Marshaler] or [Enum]
//  1. if [Marshaler], use "MarshalBCS" method.
//  2. if not [Marshaler] but [Enum], use specialization for [Enum].
//  3. otherwise standard process.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	e := NewEncoder(&b)

	if err := e.Encode(v); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// MustMarshal [Marshal] v, and panics if error.
func MustMarshal(v any) []byte {
	result, err := Marshal(v)
	if err != nil {
		panic(err)
	}

	return result
}
