package chx

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"sentioxyz/sentio-core/common/log"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
)

const slowQueryLimit = time.Second * 5

func (c Controller) Query(
	ctx context.Context,
	scanner func(rows driver.Rows) error,
	sql string,
	sqlArgs ...any,
) (e error) {
	startAt := time.Now()
	var rowsNum int
	defer func() {
		used := time.Since(startAt)
		_, logger := log.FromContext(ctx, "sql", sql, "rows", rowsNum, "used", used.String())
		if e != nil {
			logger.Warnfe(e, "clickhouse query failed")
		} else if used >= slowQueryLimit {
			logger.Warnf("clickhouse query succeed, but used > %s", slowQueryLimit)
		} else {
			logger.Debug("clickhouse query succeed")
		}
	}()
	rows, err := c.conn.Query(ctx, sql, sqlArgs...)
	if err != nil {
		return errors.Wrapf(err, "execute sql failed")
	}
	for rows.Next() {
		rowsNum++
		if err = scanner(rows); err != nil {
			_ = rows.Close()
			return errors.Wrapf(err, "scan result failed")
		}
	}
	if err = rows.Close(); err != nil {
		return errors.Wrapf(err, "close clickhouse rows failed")
	}
	return nil
}

func (c Controller) Exec(ctx context.Context, sql string, args ...any) error {
	startAt := time.Now()
	err := c.conn.Exec(ctx, sql, args...)
	_, logger := log.FromContext(ctx, "sql", sql, "used", time.Since(startAt).String())
	if err != nil {
		logger.Warnfe(err, "clickhouse execute failed")
		return err
	}
	logger.Debug("clickhouse execute succeed")
	return nil
}

func (c Controller) QueryCount(ctx context.Context, sql string, args ...any) (count uint64, err error) {
	err = c.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&count)
	}, sql, args...)
	return count, err
}

func (c Controller) BatchInsert(
	ctx context.Context,
	sql string,
	batchSize int,
	getter func() ([]any, bool),
) (e error) {
	startAt := time.Now()
	var rowsNum int
	var batchNum int
	defer func() {
		_, logger := log.FromContext(ctx, "sql", sql, "rows", rowsNum, "batches", batchNum, "used", time.Since(startAt).String())
		if e != nil {
			logger.Warnfe(e, "clickhouse batch insert failed")
		} else {
			logger.Debug("clickhouse batch insert succeed")
		}
	}()
	has := true
	for has {
		uniqToken := strconv.FormatUint(rand.Uint64(), 16)
		batch, err := c.conn.PrepareBatch(InsertCtx(ctx, uniqToken), sql)
		if err != nil {
			return errors.Wrapf(err, "prepare batch failed")
		}
		var thisBatchSize int
		for thisBatchSize = 0; thisBatchSize < batchSize; thisBatchSize++ {
			var columns []any
			columns, has = getter()
			if !has {
				break
			}
			if err = batch.Append(columns...); err != nil {
				return errors.Wrapf(err, "batch append failed")
			}
			rowsNum++
		}
		if thisBatchSize > 0 {
			if err = batch.Send(); err != nil {
				return errors.Wrapf(err, "batch send failed")
			}
			batchNum++
		} else if err = batch.Close(); err != nil {
			return errors.Wrapf(err, "batch close failed")
		}
	}
	return nil
}

func (c Controller) Delete(ctx context.Context, table string, condition string) (uint64, error) {
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", c.FullLogicName(table), condition)
	count, err := c.QueryCount(DisableProjectionCtx(ctx), sql)
	if err != nil {
		return 0, errors.Wrapf(err, "query count for deleting from %s failed", table)
	} else if count == 0 {
		return 0, nil
	}
	sql = fmt.Sprintf("DELETE FROM %s WHERE %s", c.FullLogicName(table), condition)
	if err = c.Exec(ctx, sql); err != nil {
		return 0, errors.Wrapf(err, "delete from %s failed", table)
	}
	return count, nil
}

func (c Controller) AlterTable(ctx context.Context, table string, sql string, args ...any) error {
	return c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s %s", c.FullNameWithOnCluster(table), sql), args...)
}

type PartitionMeta struct {
	Partition             string
	Rows                  uint64
	BytesOnDisk           uint64
	DataCompressedBytes   uint64
	DataUncompressedBytes uint64
}

func (c Controller) ListPartitions(ctx context.Context, table string) (partitions []PartitionMeta, err error) {
	sql := "SELECT " +
		"partition," +
		"sum(rows)," +
		"sum(bytes_on_disk)," +
		"sum(data_compressed_bytes)," +
		"sum(data_uncompressed_bytes) " +
		"FROM system.parts " +
		"WHERE database = ? AND table = ? AND active = 1 " +
		"GROUP BY partition " +
		"ORDER BY partition"
	err = c.Query(ctx, func(rows driver.Rows) error {
		var p PartitionMeta
		scanErr := rows.Scan(&p.Partition, &p.Rows, &p.BytesOnDisk, &p.DataCompressedBytes, &p.DataUncompressedBytes)
		if scanErr != nil {
			return scanErr
		}
		partitions = append(partitions, p)
		return nil
	}, sql, c.database, c.tableNamePrefix+table)
	return partitions, err
}
