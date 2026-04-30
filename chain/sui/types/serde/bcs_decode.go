package serde

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"runtime/debug"
	"sentioxyz/sentio-core/common/log"

	"github.com/fardream/go-bcs/bcs"
	"github.com/pkg/errors"
)

// Unmarshal unmarshal the bcs serialized data into v.
//
// Refer to notes in [Marshal] for details how data serialized/deserialized.
//
// During the unmarshalling process
//  1. if [Unmarshaler], use "UnmarshalBCS" method.
//  2. if not [Unmarshaler] but [Enum], use the specialization for [Enum].
//  3. otherwise standard process.
func Unmarshal(data []byte, v any) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}

// Decoder takes an [io.Reader] and decodes value from it.
type Decoder struct {
	r io.Reader
}

// NewDecoder creates a new [Decoder] from an [io.Reader]
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r: r,
	}
}

func Decode(r io.Reader, v any) error {
	return NewDecoder(r).Decode(v)
}

// Decode a value from the decoder.
//
//   - If the value is [Unmarshaler], the corresponding UnmarshalBCS will be called.
//   - If the value is [Enum], it will be special handled for [Enum]
func (d *Decoder) Decode(v any) error {
	reflectValue := reflect.ValueOf(v)
	if reflectValue.Kind() != reflect.Pointer || reflectValue.IsNil() {
		return fmt.Errorf("not a pointer or nil pointer")
	}

	return d.decode(reflectValue)
}

func (d *Decoder) decode(v reflect.Value) error {
	if Trace {
		defer func() {
			log.Debugf("<< decode type %s pos %d", v.Type().String(), getReaderPosForTracing(d.r))
		}()
		log.Debugf(">> decode type %s pos %d", v.Type().String(), getReaderPosForTracing(d.r))
	}
	// if v cannot interface, ignore
	if !v.CanInterface() {
		return nil
	}

	if v.CanAddr() {
		if i, isUnmarshaler := v.Addr().Interface().(bcs.Unmarshaler); isUnmarshaler {
			var err error
			_, err = i.UnmarshalBCS(d.r)
			return err
		}
	}
	if i, isUnmarshaler := v.Interface().(bcs.Unmarshaler); isUnmarshaler {
		var err error
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
			_, err = v.Interface().(bcs.Unmarshaler).UnmarshalBCS(d.r)
		} else {
			_, err = i.UnmarshalBCS(d.r)
		}
		return err
	}

	if _, isEnum := v.Interface().(bcs.Enum); isEnum {
		switch v.Kind() { //nolint:exhaustive
		case reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			return d.decodeEnum(v.Elem())
		default:
			return d.decodeEnum(v)
		}
	}

	switch v.Kind() { //nolint:exhaustive
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return d.decodeVanilla(v.Elem())

	case reflect.Interface:
		if v.IsNil() {
			debug.PrintStack()
			return fmt.Errorf("cannot decode into nil interface")
		}
		return d.decode(v.Elem())

	case reflect.Chan, reflect.Func, reflect.Uintptr, reflect.UnsafePointer:
		// silently ignore
		return nil
	default:
		return d.decodeVanilla(v)
	}
}

func (d *Decoder) decodeVanilla(v reflect.Value) error {
	kind := v.Kind()

	if !v.CanSet() {
		return fmt.Errorf("cannot change value of kind %s", kind.String())
	}

	switch v.Kind() { //nolint:exhaustive
	case reflect.Bool:
		t, err := d.readByte()
		if err != nil {
			return nil
		}

		if t == 0 {
			v.SetBool(false)
		} else {
			v.SetBool(true)
		}

		return nil

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, // ints
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64: // uints
		return binary.Read(d.r, binary.LittleEndian, v.Addr().Interface())

	case reflect.Struct:
		return d.decodeStruct(v)

	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return d.decodeByteSlice(v)
		}

		return d.decodeSlice(v)

	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return d.decodeByteArray(v)
		}
		return d.decodeArray(v)

	case reflect.String:
		return d.decodeString(v)

	default:
		return fmt.Errorf("unsupported vanilla decoding type: %s", kind.String())
	}
}

func (d *Decoder) decodeString(v reflect.Value) error {
	size, _, err := bcs.ULEB128Decode[int](d.r)
	if err != nil {
		return err
	}

	tmp := make([]byte, size)

	read, err := d.r.Read(tmp)
	if err != nil {
		return err
	}

	if size != read {
		return fmt.Errorf("wrong number of bytes read for []byte, want: %d, got %d", size, read)
	}

	v.Set(reflect.ValueOf(string(tmp)))

	if Trace {
		log.Debugf("decode string, size %d value %s", size, v.String())
	}

	return nil
}

func (d *Decoder) readByte() (byte, error) {
	b := make([]byte, 1)
	n, err := d.r.Read(b)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("EOF")
	}

	return b[0], nil
}

func (d *Decoder) decodeStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if Trace {
			log.Debugf("decode field, name %s type %s", v.Type().Field(i).Name, v.Type().Field(i).Type)
		}
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
			present, err := d.readByte()
			if err != nil {
				return err
			}
			if present == 0 {
				field.Set(reflect.Zero(field.Type()))
			} else {
				field.Set(reflect.New(field.Type().Elem()))
				err := d.decode(field.Elem())
				if err != nil {
					return err
				}
			}
			continue
		}

		if err := d.decode(field); err != nil {
			return errors.Wrap(err, v.Type().Field(i).Name)
		}
	}

	return nil
}

func (d *Decoder) decodeEnum(v reflect.Value) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("only support struct for Enum, got %s", v.Kind().String())
	}
	enumID, _, err := bcs.ULEB128Decode[int](d.r)
	if err != nil {
		return err
	}

	if enumID >= v.NumField() {
		return fmt.Errorf("invalid enum id %d, max %d, type is %s", enumID, v.NumField(), v.Type())
	}
	field := v.Field(enumID)
	if Trace {
		log.Debugf("decode variant, name %s", v.Type().Field(enumID).Name)
	}

	return d.decode(field)
}

func ReadByteSlice(r io.Reader) ([]byte, error) {
	size, _, err := bcs.ULEB128Decode[int](r)
	if err != nil {
		return nil, err
	}

	if Trace {
		log.Debugf("read byte slice, size %d pos %d", size, getReaderPosForTracing(r))
	}

	tmp := make([]byte, size)

	read, err := r.Read(tmp)
	if err != nil {
		return nil, err
	}

	if size != read {
		return nil, fmt.Errorf("wrong number of bytes read for []byte, want: %d, got %d", size, read)
	}

	return tmp, err
}

func (d *Decoder) decodeByteSlice(v reflect.Value) error {
	tmp, err := ReadByteSlice(d.r)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(tmp))
	return nil
}

func (d *Decoder) decodeByteArray(v reflect.Value) error {
	size := v.Len()
	if Trace {
		log.Debugf("decode byte array, size %d pos %d", size, getReaderPosForTracing(d.r))
	}

	read, err := d.r.Read(v.Bytes())
	if err != nil {
		return err
	}

	if size != read {
		return fmt.Errorf("wrong number of bytes read for []byte, want: %d, got %d", size, read)
	}
	return nil
}

func (d *Decoder) decodeArray(v reflect.Value) error {
	size := v.Len()
	t := v.Type()

	if Trace {
		log.Debugf("decode array, size %d pos %d", size, getReaderPosForTracing(d.r))
	}

	for i := 0; i < size; i++ {
		v.Index(i).Set(reflect.New(t.Elem()))
		if err := d.decode(v.Index(i)); err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) decodeSlice(v reflect.Value) error {
	size, _, err := bcs.ULEB128Decode[int](d.r)
	if err != nil {
		return err
	}

	if Trace {
		log.Debugf("decode slice [%d]%s", size, v.Type().Elem().Name())
	}

	t := v.Type()
	tmp := reflect.MakeSlice(t, 0, size)
	for i := 0; i < size; i++ {
		if Trace {
			log.Debugf("decode slice elem %s[%d]", v.Type().Elem().Name(), i)
		}
		ind := reflect.New(t.Elem())
		if err := d.decode(ind); err != nil {
			return err
		}
		tmp = reflect.Append(tmp, ind.Elem())
	}

	v.Set(tmp)

	return nil
}
