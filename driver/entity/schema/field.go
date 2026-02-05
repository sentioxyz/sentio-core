package schema

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type ForeignKeyField struct {
	*types.FieldDefinition
}

func (f *ForeignKeyField) getReverseFieldName() (string, bool, error) {
	d := f.Directives.Get(DerivedFromDirectiveName)
	if d == nil {
		return "", false, nil
	}
	for _, arg := range d.Arguments {
		if arg.Name.Name == DerivedFromFieldArgName {
			argVal := arg.Value.Deserialize(nil)
			if name, is := argVal.(string); is {
				return name, true, nil
			}
			return "", true, fmt.Errorf("argument %q of @%s directive is %v, not a string",
				DerivedFromFieldArgName, DerivedFromDirectiveName, argVal)
		}
	}
	return "", true, fmt.Errorf("argument %q of @%s directive is missing",
		DerivedFromFieldArgName, DerivedFromDirectiveName)
}

func (f *ForeignKeyField) IsReverseField() bool {
	_, has, _ := f.getReverseFieldName()
	return has
}

func (f *ForeignKeyField) GetReverseFieldName() string {
	name, _, _ := f.getReverseFieldName()
	return name
}

func (f *ForeignKeyField) GetReverseField() *ForeignKeyField {
	name, has, _ := f.getReverseFieldName()
	if !has {
		return nil
	}
	return f.GetTarget().GetForeignKeyFieldByName(name)
}

// use the primary key pointed to by the foreign key instead of the entity type pointed to.
// If EntityA has a primary field ID with type `String!`, the fixed type of the foreign key field will be:
//
//	EntityA    => String
//	EntityA!   => String!
//	[EntityA]  => [String]
//	[EntityA!] => [String!]
func (f *ForeignKeyField) getFixedFieldType() (types.Type, error) {
	var foreignEntity EntityOrInterface
	typeChain := BreakType(f.Type)
	switch innerType := typeChain.InnerType().(type) {
	case *types.ObjectTypeDefinition:
		if innerType.Directives.Get(EntityDirectiveName) == nil {
			return nil, fmt.Errorf("invalid type kind %s, it is not a Entity", innerType.Kind())
		}
		foreignEntity = NewEntity(innerType)
	case *types.InterfaceTypeDefinition:
		foreignEntity = NewInterface(innerType)
	default:
		return nil, fmt.Errorf("invalid type kind %s", innerType.Kind())
	}
	if foreignEntity.GetPrimaryKeyField() == nil {
		return nil, fmt.Errorf("%s do not have primary key field", foreignEntity.GetName())
	}
	return typeChain.Restructure(BreakType(foreignEntity.GetPrimaryKeyField().Type).InnerType()), nil
}

func (f *ForeignKeyField) GetFixedFieldType() types.Type {
	typ, _ := f.getFixedFieldType()
	return typ
}

func (f *ForeignKeyField) getTarget() (EntityOrInterface, error) {
	switch innerType := BreakType(f.Type).InnerType().(type) {
	case *types.ObjectTypeDefinition:
		return NewEntity(innerType), nil
	case *types.InterfaceTypeDefinition:
		return NewInterface(innerType), nil
	default:
		return nil, fmt.Errorf("invalid type kind %s", innerType.Kind())
	}
}

func (f *ForeignKeyField) GetTarget() EntityOrInterface {
	item, _ := f.getTarget()
	return item
}

func (f *ForeignKeyField) GetTargetEntities() []*Entity {
	item, _ := f.getTarget()
	return item.ListEntities()
}

func (f *ForeignKeyField) NormalizeForeignKeyFieldValue(fpkList []string) any {
	fieldTypeChain := BreakType(f.Type)
	if fieldTypeChain.CountListLayer() > 0 {
		// return an array
		if fieldTypeChain.InnerTypeNullable() {
			// nullable, use string pointer array
			return utils.WrapPointerForArray(fpkList)
		} else {
			// non-nullable, use string array
			return fpkList
		}
	} else {
		// return one
		if fieldTypeChain.InnerTypeNullable() {
			// nullable, use string pointer
			if len(fpkList) == 0 {
				return nil
			} else {
				return utils.WrapPointer(fpkList[0])
			}
		} else {
			// non-nullable, use string
			if len(fpkList) == 0 {
				return ""
			} else {
				return fpkList[0]
			}
		}
	}
}

func GetFieldDBType(f *types.FieldDefinition) (string, bool, error) {
	d := f.Directives.Get(DBTypeDirectiveName)
	if d == nil {
		return "", false, nil
	}
	arg, has := d.Arguments.Get(DBTypeDirectiveTypeArgName)
	if !has || arg == nil {
		return "", false, errors.Errorf("missing argument %s", DBTypeDirectiveTypeArgName)
	}
	v := arg.Deserialize(nil)
	if v == nil {
		return "", false, errors.Errorf("argument %s value %q is invalid", DBTypeDirectiveTypeArgName, arg.String())
	}
	str, is := v.(string)
	if !is {
		return "", false, errors.Errorf("argument %s value %q is not a string", DBTypeDirectiveTypeArgName, arg.String())
	}
	// check whether the field type and dbType are compatible
	typeChain := BreakType(f.Type)
	switch innerType := typeChain.InnerType().(type) {
	case *types.ScalarTypeDefinition:
		switch innerType.Name {
		case "String":
			switch strings.ToLower(str) {
			case "json":
				if typeChain.CountListLayer() > 0 {
					return "", false, errors.Errorf("dbType of field type %s cannot be %q", f.Type.String(), str)
				}
			default:
				return "", false, errors.Errorf("dbType of field type %s cannot be %q", f.Type.String(), str)
			}
		default:
			return "", false, errors.Errorf("field type %s cannot use @%s directive", f.Type.String(), DBTypeDirectiveName)
		}
	case *types.EnumTypeDefinition:
		return "", false, errors.Errorf("enum field cannot use @%s directive", DBTypeDirectiveName)
	case *types.ObjectTypeDefinition:
		return "", false, errors.Errorf("foreign key field cannot use @%s directive", DBTypeDirectiveName)
	}
	return str, true, nil
}

func GetIndex(f *types.FieldDefinition) (string, bool, error) {
	d := f.Directives.Get(IndexDirectiveName)
	if d == nil {
		return "", false, nil
	}
	arg, has := d.Arguments.Get(IndexDirectiveTypeArgName)
	if !has || arg == nil {
		return "", true, nil
	}
	v := arg.Deserialize(nil)
	if v == nil {
		return "", true, nil
	}
	str, is := v.(string)
	if !is {
		return "", true, fmt.Errorf("argument %s value %q is not a string", IndexDirectiveTypeArgName, arg.String())
	}
	return str, true, nil
}
