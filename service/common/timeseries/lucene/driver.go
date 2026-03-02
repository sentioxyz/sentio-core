package lucene

import (
	"github.com/blevesearch/bleve/search/query"
	"github.com/sentioxyz/qs"
)

type Driver interface {
	Render(q query.Query) (string, error)
}

func Parse(q string) (query.Query, error) {
	parser := qs.Parser{
		DefaultOp: qs.AND,
	}
	return parser.Parse(q)
}
