package ckhmanager

import (
	"context"
	"unicode/utf8"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	tracerName         = "sentioxyz/sentio-core/common/clickhousemanager"
	maxSQLLength       = 512
	dbSystemClickHouse = "clickhouse"
)

var (
	dbSystemKey    = attribute.Key("db.system")
	dbQueryTextKey = attribute.Key("db.query.text")
)

// tracingConn is a decorator around Conn that wraps every DB operation with an OTel span.
type tracingConn struct {
	inner  Conn
	tracer trace.Tracer
}

func wrapWithTracing(c Conn) Conn {
	tp := otel.GetTracerProvider()
	// If the global provider is a noop (monitoring not enabled), skip wrapping.
	if _, isNoop := tp.(noopTracerProvider); isNoop {
		return c
	}
	return &tracingConn{
		inner:  c,
		tracer: tp.Tracer(tracerName),
	}
}

// noopTracerProvider is used for detection only; the actual noop type lives in the otel package.
type noopTracerProvider interface {
	trace.TracerProvider
	noopMarker()
}

func truncateSQL(sql string) string {
	if len(sql) <= maxSQLLength {
		return sql
	}
	// Truncate to maxSQLLength bytes and ensure valid UTF-8
	b := []byte(sql[:maxSQLLength])
	for !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b) + "..."
}

func (c *tracingConn) startSpan(ctx context.Context, op string, sql string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		dbSystemKey.String(dbSystemClickHouse),
		dbQueryTextKey.String(truncateSQL(sql)),
		attribute.String("db.clickhouse.host", c.inner.GetHost()),
	}
	return c.tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

func recordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// --- Conn interface implementation ---

func (c *tracingConn) GetClickhouseConn() clickhouse.Conn {
	return c.inner.GetClickhouseConn()
}

func (c *tracingConn) GetDatabase() string {
	return c.inner.GetDatabase()
}

func (c *tracingConn) GetCluster() string {
	return c.inner.GetCluster()
}

func (c *tracingConn) GetHost() string {
	return c.inner.GetHost()
}

func (c *tracingConn) GetPassword() string {
	return c.inner.GetPassword()
}

func (c *tracingConn) GetUsername() string {
	return c.inner.GetUsername()
}

func (c *tracingConn) GetSettings() clickhouse.Settings {
	return c.inner.GetSettings()
}

func (c *tracingConn) Close() {
	c.inner.Close()
}

func (c *tracingConn) Exec(ctx context.Context, sql string, args ...any) error {
	ctx, span := c.startSpan(ctx, "clickhouse.Exec", sql)
	defer span.End()

	err := c.inner.Exec(ctx, sql, args...)
	recordError(span, err)
	return err
}

func (c *tracingConn) Query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	ctx, span := c.startSpan(ctx, "clickhouse.Query", sql)
	defer span.End()

	rows, err := c.inner.Query(ctx, sql, args...)
	recordError(span, err)
	return rows, err
}

func (c *tracingConn) QueryRow(ctx context.Context, sql string, args ...any) driver.Row {
	ctx, span := c.startSpan(ctx, "clickhouse.QueryRow", sql)
	defer span.End()

	return c.inner.QueryRow(ctx, sql, args...)
}

func (c *tracingConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	ctx, span := c.startSpan(ctx, "clickhouse.PrepareBatch", query)
	defer span.End()

	batch, err := c.inner.PrepareBatch(ctx, query, opts...)
	recordError(span, err)
	return batch, err
}
