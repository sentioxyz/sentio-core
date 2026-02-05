package schema

type Field interface {
	String() string
	GetName() string
	GetDisplayName() string
	GetType() string
	IsBuiltIn() bool
	IsJSON() bool
	IsToken() bool
	IsString() bool
	IsNumeric() bool
	IsBool() bool
	IsTime() bool
	IsArray() bool
	TypeMatch(value any) (any, bool)

	// deprecated
	GetExtend() map[string]Field
	Equal(Field) (bool, string)
}

type Meta interface {
	Hash() string
	GetFieldTypeValue(name string) any
	IsNumeric(name string) bool
	IsBool(name string) bool
	IsString(name string) bool
	IsTime(name string) bool
	IsJSON(name string) bool
	IsToken(name string) bool
	GetReservedFields() []Field
	GetFields() map[string]Field
	GetInvertedIndex() map[string]map[string]map[string]bool
}
