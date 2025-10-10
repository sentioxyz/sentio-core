package db

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"sentioxyz/sentio-core/common/protojson"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm/schema"
)

type ProtoJSONSerializer struct {
}

func (ProtoJSONSerializer) Scan(
	ctx context.Context,
	field *schema.Field,
	dst reflect.Value,
	dbValue interface{},
) (err error) {
	fieldValue := reflect.New(field.FieldType)

	if dbValue != nil {
		var bytes []byte
		switch v := dbValue.(type) {
		case []byte:
			bytes = v
		case string:
			bytes = []byte(v)
		default:
			return fmt.Errorf("failed to unmarshal JSONB value: %#v", dbValue)
		}

		message, ok := fieldValue.Interface().(proto.Message)
		if ok {
			err = protojson.Unmarshal(bytes, message)
		} else {
			err = json.Unmarshal(bytes, fieldValue.Interface())
		}
	}

	field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())
	return
}

func (ProtoJSONSerializer) Value(
	ctx context.Context,
	field *schema.Field,
	dst reflect.Value,
	fieldValue interface{},
) (interface{}, error) {
	if message, ok := fieldValue.(proto.Message); ok {
		return protojson.Marshal(message)
	} else {
		return json.Marshal(fieldValue)
	}
}
