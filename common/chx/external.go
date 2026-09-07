package chx

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/ext"
)

// ExternalTableCtx attaches external tables to the query executed with the returned context.
// External tables are sent together with the query over the same connection, so unlike temporary tables they
// are not bound to whichever pooled connection created them.
func ExternalTableCtx(ctx context.Context, tables ...*ext.Table) context.Context {
	if len(tables) == 0 {
		return ctx
	}
	return clickhouse.Context(ctx, clickhouse.WithExternalTable(tables...))
}
