package schema

import (
	"github.com/graph-gophers/graphql-go"
)

// reference:
// - https://thegraph.com/docs/en/developing/creating-a-subgraph/#the-graphql-schema
// - https://thegraph.com/docs/en/developing/creating-a-subgraph/#built-in-scalar-types
// - https://graphql.cn/learn/schema/#scalar-types

const schemaBase = `

directive @entity(immutable: Boolean! = false, sparse: Boolean! = true, timeseries: Boolean! = false) on OBJECT
directive @cache(sizeMB: Int!) on OBJECT
directive @regularPolling on OBJECT
directive @dailySnapshot on OBJECT
directive @hourlySnapshot on OBJECT
directive @transaction on OBJECT
directive @aggregation(intervals: [String!]!, source: String!) on OBJECT

directive @derivedFrom(field: String!) on FIELD_DEFINITION
directive @index(type: String!) on FIELD_DEFINITION
directive @dbType(type: String!) on FIELD_DEFINITION
directive @aggregate(fn: String!, arg: String!) on FIELD_DEFINITION

scalar Bytes
scalar String
scalar Boolean
scalar Int
scalar Int8
scalar Timestamp
scalar Float
scalar BigInt
scalar BigDecimal
scalar ID

schema {
	query: Query
}

type Query {
}
`

func ParseSchema(schemaCnt string) (*Schema, error) {
	s, err := graphql.ParseSchema(schemaCnt+schemaBase, nil)
	if err != nil {
		return nil, err
	}
	return &Schema{Schema: s.ASTSchema()}, nil
}

func ParseAndVerifySchema(schemaCnt string, opts ...VerifyOption) (*Schema, error) {
	sch, err := ParseSchema(schemaCnt)
	if err == nil {
		err = sch.Verify(opts...)
	}
	return sch, err
}
