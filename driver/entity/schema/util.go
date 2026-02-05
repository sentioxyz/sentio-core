package schema

import (
	"github.com/graph-gophers/graphql-go/types"
	"sentioxyz/sentio-core/common/utils"
)

var fixedFieldKind = []string{"SCALAR", "ENUM"}

func isFixedFieldType(typ types.Type) bool {
	return utils.IndexOf(fixedFieldKind, BreakType(typ).InnerType().Kind()) >= 0
}

func findEntityByName(entities []*Entity, entityName string) *Entity {
	for _, e := range entities {
		if e.Name == entityName {
			return e
		}
	}
	return nil
}
