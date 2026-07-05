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
			logger.With("sqlArgs", sqlArgs).Warnfe(e, "clickhouse query failed")
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

func (c Controller) Delete(ctx context.Context, table string, condition string, light bool) (uint64, error) {
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", c.FullLogicName(table), condition)
	count, err := c.QueryCount(DisableProjectionCtx(ctx), sql)
	if err != nil {
		return 0, errors.Wrapf(err, "query count for deleting from %s failed", table)
	} else if count == 0 {
		return 0, nil
	}
	if light {
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", c.FullLogicName(table), condition)
		if err = c.Exec(LightDeleteCtx(ctx), sql); err != nil {
			return 0, errors.Wrapf(err, "delete from %s failed", table)
		}
	} else {
		// Heavyweight `ALTER TABLE ... DELETE` differs from the lightweight `DELETE FROM` above in
		// WHERE the delete happens, and that difference is what makes it mandatory for tables with
		// projections:
		//
		//   - `DELETE FROM` (lightweight) never touches the stored rows. It only attaches a
		//     _row_exists mask (or patch parts) that the BASE read path applies on the fly.
		//     Projections have no mask concept, so on a table with projections they keep serving
		//     the deleted rows forever — even lightweight_mutation_projection_mode='rebuild'
		//     rebuilds the projection from the part's physical rows with the mask ignored. Base and
		//     projection permanently diverge; lightweight is therefore only safe on tables without
		//     projections.
		//   - `ALTER TABLE ... DELETE` (heavyweight) deletes by atomically rewriting whole parts:
		//     each affected part is rewritten from its surviving rows, the projections inside the
		//     part are rebuilt from those same surviving rows in the same rewrite, and the new part
		//     atomically replaces the old one. Base and projection always come from the same write,
		//     so no base-updated-but-projection-stale state is ever observable.
		//
		// That per-part atomicity also means a failure of any kind (mutation failing halfway, KILL
		// MUTATION, client disconnect) is only ever a PROGRESS problem, never a consistency problem:
		// every active part is either the old one (rows still present in both base and projection)
		// or the new one (rows deleted from both), and an unfinished mutation stays queued and
		// retries server-side until done or killed.
		//
		// The mutation rewrites every matching part, which on a large table can far exceed the
		// client-side read timeout. Waiting on the statement itself would then time out client-side
		// while the server keeps executing, and a retry would submit yet another mutation on top of
		// the running one. Submit asynchronously instead (the statement returns once the mutation is
		// queued) and poll system.mutations until it finishes: each poll is a cheap short query, so
		// the total wait is bounded only by ctx.
		sql = fmt.Sprintf("ALTER TABLE %s DELETE WHERE %s", c.FullLogicName(table), condition)
		if err = c.Exec(AsyncMutationCtx(ctx), sql); err != nil {
			return 0, errors.Wrapf(err, "delete from %s failed", table)
		}
		if err = c.waitDeleteMutations(ctx, table); err != nil {
			return 0, errors.Wrapf(err, "wait for delete mutation on %s failed", table)
		}
	}
	return count, nil
}

// waitDeleteMutations blocks until the table has no unfinished DELETE mutation, polling
// system.mutations. Transient poll failures are tolerated (the mutation keeps running server-side
// regardless), so the wait survives network hiccups and is bounded only by ctx. A mutation that
// reports a failure reason is surfaced as an error instead of waiting forever on it.
func (c Controller) waitDeleteMutations(ctx context.Context, table string) error {
	const pollInterval = time.Second * 5
	startAt := time.Now()
	sql := "SELECT count(), max(latest_fail_reason) FROM system.mutations " +
		"WHERE database = ? AND table = ? AND NOT is_done AND command LIKE '%DELETE WHERE%'"
	for {
		var pending uint64
		var failReason string
		err := c.Query(ctx, func(rows driver.Rows) error {
			return rows.Scan(&pending, &failReason)
		}, sql, c.database, c.tableNamePrefix+table)
		if err == nil {
			if failReason != "" {
				return errors.Errorf("delete mutation on %s keeps failing: %s", table, failReason)
			}
			if pending == 0 {
				used := time.Since(startAt)
				if used >= slowQueryLimit {
					_, logger := log.FromContext(ctx, "table", table, "used", used.String())
					logger.Warnf("delete mutation done, but waited > %s", slowQueryLimit)
				}
				return nil
			}
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (c Controller) AlterTable(ctx context.Context, table string, sql string, args ...any) error {
	return c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s %s", c.FullLogicNameWithOnCluster(table), sql), args...)
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
