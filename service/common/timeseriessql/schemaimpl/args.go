package schemaimpl

import protosanalytic "sentioxyz/sentio-core/service/analytic/protos"

type ColumnSchema interface {
	GetName() string
	GetType() string
	IsBuiltIn() bool
	ToProto() *protosanalytic.Table_Column
}

type TableSchema interface {
	GetName() string
	GetColumns() map[string]ColumnSchema
	GetType() protosanalytic.Table_TableType
	GetLabels() map[string]string
	ToProto() *protosanalytic.Table
}

type Args interface {
	GetTableSchemaByName(string) (TableSchema, bool)
	GetAllTableSchema() map[string]TableSchema
	AddTableSchema(string, TableSchema)
	Dump() string
}
