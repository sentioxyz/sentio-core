package persistent

import (
	"context"
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"sentioxyz/sentio-core/driver/entity/schema"
	"strings"
	"time"
)

type Store interface {
	InitEntitySchema(ctx context.Context) error

	GetEntityType(entity string) *schema.Entity
	GetEntityOrInterfaceType(name string) schema.EntityOrInterface

	GetEntity(ctx context.Context, entityType *schema.Entity, chain string, id string) (*EntityBox, error)
	ListEntities(
		ctx context.Context,
		entityType *schema.Entity,
		chain string,
		filters []EntityFilter,
		limit int,
	) ([]*EntityBox, error)
	GetAllID(ctx context.Context, entityType *schema.Entity, chain string) ([]string, error)
	GetMaxID(ctx context.Context, entityType *schema.Entity, chain string) (int64, error)
	CountEntity(ctx context.Context, entityType *schema.Entity, chain string) (uint64, error)
	SetEntities(ctx context.Context, entityType *schema.Entity, boxes []EntityBox) (int, error)
	GrowthAggregation(ctx context.Context, chain string, curBlockTime time.Time) error
	Reorg(ctx context.Context, blockNumber int64, chain string) error
}

type EntityFilterOp int

const (
	EntityFilterOpEq EntityFilterOp = iota
	EntityFilterOpNe
	EntityFilterOpGt
	EntityFilterOpGe
	EntityFilterOpLt
	EntityFilterOpLe
	EntityFilterOpIn
	EntityFilterOpNotIn
	EntityFilterOpLike
	EntityFilterOpNotLike
	EntityFilterOpHasAll
	EntityFilterOpHasAny

	// some sample
	// FieldType EntityFieldValue FilterOp FilterValue Result
	// ======================================================
	// Int       123              =        null        false
	// Int       null             =        null        true
	// Int       null             =        123         false
	// ------------------------------------------------------
	// Int       123              !=       null        true
	// Int       null             !=       null        false
	// Int       null             !=       123         false (BE ATTENTION HERE!)
	// ------------------------------------------------------
	// Int       123              >        null        false
	// Int       null             >        123         false
	// Int       null             >        null        false
	// ------------------------------------------------------
	// Int       123              <        null        false
	// Int       null             <        123         false
	// Int       null             <        null        false
	// ------------------------------------------------------
	// Int       123              >=       null        false
	// Int       null             >=       123         false
	// Int       null             >=       null        false (BE ATTENTION HERE!)
	// ------------------------------------------------------
	// Int       123              <=       null        false
	// Int       null             <=       123         false
	// Int       null             <=       null        false (BE ATTENTION HERE!)
	// ------------------------------------------------------
	// Int       123              IN       [123,null]  true
	// Int       123              IN       [456,null]  false
	// Int       null             IN       [123,null]  true
	// Int       null             IN       [123]       false
	// ------------------------------------------------------
	// Int       123              !IN      [123,null]  false
	// Int       123              !IN      [456,null]  true
	// Int       null             !IN      [123,null]  false
	// Int       null             !IN      [123]       true
	// ------------------------------------------------------
	// String    abc              LIKE     %           true
	// String    abc              LIKE     null        false (BE ATTENTION HERE!)
	// String    null             LIKE     null        false (BE ATTENTION HERE!)
	// String    null             LIKE     %           false (BE ATTENTION HERE!)
	// ------------------------------------------------------
	// String    abc              !LIKE    %           false
	// String    abc              !LIKE    null        false (BE ATTENTION HERE!)
	// String    null             !LIKE    null        false (BE ATTENTION HERE!)
	// String    null             !LIKE    %           false (BE ATTENTION HERE!)
	// ------------------------------------------------------
	// [String!] [abc,def]        HAS_ALL  []          true
	// [String!] [abc,def]        HAS_ALL  [abc]       true
	// [String!] [abc,def]        HAS_ALL  [abc,def]   true
	// [String!] [abc,def]        HAS_ALL  [abc,xyz]   false
	// [String!] [abc,def]        HAS_ALL  [xyz]       false
	// [String!] [abc,def]        HAS_ALL  null        true  (BE ATTENTION HERE!)
	// [String!] null             HAS_ALL  [abc]       false (BE ATTENTION HERE!)
	// [String!] null             HAS_ALL  []          true  (BE ATTENTION HERE!)
	// [String!] null             HAS_ALL  null        true  (BE ATTENTION HERE!)
	// HAS_ALL means the size of intersection is equal to the size of target set
	// ------------------------------------------------------
	// [String!] [abc,def]        HAS_ANY  []          false
	// [String!] [abc,def]        HAS_ANY  [abc]       true
	// [String!] [abc,def]        HAS_ANY  [abc,def]   true
	// [String!] [abc,def]        HAS_ANY  [abc,xyz]   true
	// [String!] [abc,def]        HAS_ANY  [xyz]       false
	// [String!] [abc,def]        HAS_ANY  null        false (BE ATTENTION HERE!)
	// [String!] null             HAS_ANY  [abc]       false (BE ATTENTION HERE!)
	// [String!] null             HAS_ALL  []          false (BE ATTENTION HERE!)
	// [String!] null             HAS_ANY  null        false (BE ATTENTION HERE!)
	// HAS_ANY means the size of intersection is greater than 0
)

func (p EntityFilterOp) String() string {
	switch p {
	case EntityFilterOpEq:
		return "="
	case EntityFilterOpNe:
		return "!="
	case EntityFilterOpGt:
		return ">"
	case EntityFilterOpGe:
		return ">="
	case EntityFilterOpLt:
		return "<"
	case EntityFilterOpLe:
		return "<="
	case EntityFilterOpIn:
		return "in"
	case EntityFilterOpNotIn:
		return "!in"
	case EntityFilterOpLike:
		return "like"
	case EntityFilterOpNotLike:
		return "!like"
	case EntityFilterOpHasAll:
		return "hasAll"
	case EntityFilterOpHasAny:
		return "hasAny"
	default:
		return fmt.Sprintf("<UnknownOp %d>", p)
	}
}

type EntityFilter struct {
	Field *types.FieldDefinition
	Op    EntityFilterOp
	Value []any
	idSet map[string]bool
}

func (f *EntityFilter) Init() error {
	if f.Field.Name == schema.EntityPrimaryFieldName && (f.Op == EntityFilterOpIn || f.Op == EntityFilterOpNotIn) {
		// condition id IN [...] or id NOT IN [...], will fill idSet
		f.idSet = make(map[string]bool)
		for i, val := range f.Value {
			if s, is := val.(string); !is {
				return fmt.Errorf("#%d value (%v) is not a string", i, val)
			} else {
				f.idSet[s] = true
			}
		}
	}
	return nil
}

func (f EntityFilter) String() string {
	const maxPreviewNumber = 5
	var values []string
	convert := func(dst []string, src []any) {
		for i, val := range src {
			if v, is := val.(fmt.Stringer); is {
				dst[i] = v.String()
			} else {
				dst[i] = fmt.Sprintf("%v", val)
			}
		}
	}

	if len(f.Value) > 2*maxPreviewNumber {
		values = make([]string, 2*maxPreviewNumber+1)
		convert(values, f.Value[:maxPreviewNumber])
		convert(values[maxPreviewNumber+1:], f.Value[len(f.Value)-maxPreviewNumber:])
		values[maxPreviewNumber] = "..."
	} else {
		values = make([]string, len(f.Value))
		convert(values, f.Value)
	}

	return fmt.Sprintf("%s:%s %s [%s]:%d",
		f.Field.Name,
		f.Field.Type.String(),
		f.Op.String(),
		strings.Join(values, ","),
		len(f.Value))
}

func EntityFiltersString(filters []EntityFilter) string {
	str := make([]string, len(filters))
	for i, f := range filters {
		str[i] = f.String()
	}
	return strings.Join(str, ",")
}
