package clickhouse

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
)

func queryAndScan(
	ctx context.Context,
	conn chx.Conn,
	scanner func(rows driver.Rows) error,
	sql string,
	sqlArgs ...any,
) error {
	rows, err := conn.Query(ctx, sql, sqlArgs...)
	if err != nil {
		return fmt.Errorf("execute sql failed: %w", err)
	}
	if err = scanner(rows); err != nil {
		_ = rows.Close()
		return fmt.Errorf("scaner result failed: %w", err)
	}
	if err = rows.Close(); err != nil {
		return fmt.Errorf("close rows failed: %w", err)
	}
	return nil
}

func queryCount(ctx context.Context, conn chx.Conn, sql string, args ...any) (count uint64, err error) {
	err = queryAndScan(ctx, conn, func(rows driver.Rows) error {
		if !rows.Next() {
			return nil
		}
		return rows.Scan(&count)
	}, sql, args...)
	return
}

func scanRow(rows driver.Rows, fields []timeseries.Field) (timeseries.Row, error) {
	placeholders := make([]any, len(fields))
	for i, field := range fields {
		switch field.Type {
		case timeseries.FieldTypeString:
			var v string
			placeholders[i] = &v
		case timeseries.FieldTypeBool:
			var v bool
			placeholders[i] = &v
		case timeseries.FieldTypeTime:
			var v time.Time
			placeholders[i] = &v
		case timeseries.FieldTypeInt:
			var v int64
			placeholders[i] = &v
		case timeseries.FieldTypeBigInt:
			var v *big.Int
			placeholders[i] = &v
		case timeseries.FieldTypeFloat:
			var v float64
			placeholders[i] = &v
		case timeseries.FieldTypeBigFloat:
			var v decimal.Decimal
			placeholders[i] = &v
		}
	}
	err := rows.Scan(placeholders...)
	if err != nil {
		return nil, err
	}
	row := make(timeseries.Row, len(fields))
	for i, field := range fields {
		row[field.Name] = reflect.ValueOf(placeholders[i]).Elem().Interface()
	}
	return row, nil
}
