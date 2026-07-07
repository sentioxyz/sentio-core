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
	if light {
		// Lightweight deletes leave patch parts behind that background merges may never reclaim;
		// materialize the backlog once it piles up past a threshold (see the function comment).
		if err := c.applyPatchesIfPiledUp(ctx, table); err != nil {
			return 0, errors.Wrapf(err, "apply piled-up patch parts on %s failed", table)
		}
	} else {
		// A previous interrupted call (ctx timeout, network cut) may have left its DELETE mutation
		// still running server-side. Wait it out BEFORE probing and submitting: the probe then sees
		// the settled state, so a retry converges to a no-op instead of stacking one more mutation
		// onto the table on every attempt.
		if err := c.waitDeleteMutations(ctx, table); err != nil {
			return 0, errors.Wrapf(err, "wait for previous delete mutation on %s failed", table)
		}
	}
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
		// Note that a plain `DELETE FROM` WITHOUT the LightDeleteCtx settings is not a middle
		// ground: the statement form, not the settings, decides the delete's nature. Under the
		// default lightweight_delete_mode='alter_update' it is rewritten into an
		// `UPDATE _row_exists = 0` mutation — a mutation, but still a masking one: parts are
		// rewritten with only the mask column added (data columns are hardlinked, rows stay), and
		// the projection rebuild reads the part's physical rows with the mask ignored, so
		// projections keep serving the deleted rows just the same. Only `ALTER TABLE ... DELETE`
		// physically drops rows and rebuilds projections from the survivors.
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

// Thresholds on a table's live patch-part backlog past which applyPatchesIfPiledUp materializes
// it. The bytes threshold measures uncompressed bytes — the same metric the server cap
// max_uncompressed_bytes_in_patches (30 GiB by default) is enforced on — and stays far enough
// below the cap that the backlog is reclaimed long before lightweight deletes start being
// rejected. The parts count threshold bounds read amplification: every SELECT on the table has to
// apply all live patch parts on the fly.
const (
	applyPatchesPartsThreshold = 1000
	applyPatchesBytesThreshold = 4 << 30
)

// applyPatchesIfPiledUp materializes the table's patch parts with `ALTER TABLE ... APPLY PATCHES`
// once they pile up past a threshold.
//
// A lightweight `DELETE FROM` (under lightweight_delete_mode='lightweight_update', see
// LightDeleteCtx) does not touch the stored rows: it writes small patch parts carrying just the
// _row_exists mask, which every read applies on the fly, and relies on regular background merges
// to fold into the base parts (apply_patches_on_merge). That reclamation only happens when base
// parts merge, and merges are driven by inserts — so on a table (or an old, long-settled
// partition) that no longer receives writes, patch parts can survive indefinitely. The backlog is
// capped server-side by max_uncompressed_bytes_in_patches; a table that reaches the cap has every
// further lightweight delete rejected (code 755), and a caller whose write path aborts on that
// failure stops inserting too, so no merge ever materializes the backlog — a deadlock only manual
// action used to break.
//
// Hence this probe before every lightweight delete: once the table's live patch parts exceed a
// threshold, submit `ALTER TABLE ... APPLY PATCHES` — a mutation folding the patches into the
// base parts (for delete patches only the _row_exists mask is applied, no full-column rewrite, so
// it is fast) after which the patch parts are collected. Running BEFORE the delete rather than
// after also makes the cap state self-healing: a table that already hit the cap gets its backlog
// applied first, and the delete that kept being rejected then succeeds.
//
// APPLY PATCHES materializes base parts only — projections inside them are NOT rebuilt, so on a
// table with projections it would leave them permanently diverged once the patch parts are
// collected. Lightweight deletes are only safe on tables without projections in the first place
// (see the comment in Delete), so hitting a projection table here means the caller is already
// broken; skip the materialization to at least not widen the damage, and leave the backlog to
// apply_patches_on_merge.
func (c Controller) applyPatchesIfPiledUp(ctx context.Context, table string) error {
	var parts, bytes uint64
	// Patch parts live in synthetic partitions whose id is prefixed `patch-` (followed by a hash
	// of the patched columns and the source partition id), and a part's name starts with its
	// partition id.
	sql := "SELECT count(), sum(data_uncompressed_bytes) FROM system.parts " +
		"WHERE database = ? AND table = ? AND active AND startsWith(name, 'patch-')"
	err := c.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&parts, &bytes)
	}, sql, c.database, c.tableNamePrefix+table)
	if err != nil {
		return errors.Wrapf(err, "probe patch parts of %s failed", table)
	}
	if parts < applyPatchesPartsThreshold && bytes < applyPatchesBytesThreshold {
		return nil
	}
	projections, err := c.QueryCount(ctx,
		"SELECT count() FROM system.projections WHERE database = ? AND table = ?",
		c.database, c.tableNamePrefix+table)
	if err != nil {
		return errors.Wrapf(err, "probe projections of %s failed", table)
	}
	_, logger := log.FromContext(ctx, "table", table)
	if projections > 0 {
		logger.Errorf("%d patch parts (%d uncompressed bytes) piled up on a table with projections, "+
			"where neither lightweight deletes nor APPLY PATCHES are safe; leaving them to merges", parts, bytes)
		return nil
	}
	logger.Infof("%d patch parts (%d uncompressed bytes) piled up, applying them", parts, bytes)
	// Async submit + poll, for the same reason as the heavyweight branch of Delete: the mutation
	// can outlive the client-side read timeout.
	sql = fmt.Sprintf("ALTER TABLE %s APPLY PATCHES", c.FullLogicName(table))
	if err = c.Exec(AsyncMutationCtx(ctx), sql); err != nil {
		return errors.Wrapf(err, "apply patches on %s failed", table)
	}
	return c.waitMutations(ctx, table, "apply patches", "%APPLY PATCHES%")
}

// waitDeleteMutations blocks until the table has no unfinished DELETE mutation.
func (c Controller) waitDeleteMutations(ctx context.Context, table string) error {
	// `LIKE '%DELETE%WHERE%'` covers both mutation command forms, `DELETE WHERE ...` and
	// `DELETE IN PARTITION ... WHERE ...`, while still excluding the lightweight
	// `UPDATE _row_exists = 0 WHERE ...` form and projection/column commands.
	return c.waitMutations(ctx, table, "delete", "%DELETE%WHERE%")
}

// waitMutations blocks until the table has no unfinished mutation whose command matches
// commandPattern, polling system.mutations. Transient poll failures are tolerated (the mutation
// keeps running server-side regardless), so the wait survives network hiccups and is bounded only
// by ctx. A mutation failure reason is transient too at first — ClickHouse retries failed
// mutations automatically, and hiccups like a lock conflict resolve on their own — so it is only
// surfaced as an error after persisting for several consecutive polls (a stuck mutation must be
// reported rather than waited on forever).
func (c Controller) waitMutations(ctx context.Context, table, kind, commandPattern string) error {
	const pollInterval = time.Second * 5
	// ~1 minute of a persisting failure reason before giving up on the mutation.
	const failReportThreshold = 12
	startAt := time.Now()
	failStreak := 0
	sql := "SELECT count(), max(latest_fail_reason) FROM system.mutations " +
		"WHERE database = ? AND table = ? AND NOT is_done AND command LIKE ?"
	for {
		var pending uint64
		var failReason string
		err := c.Query(ctx, func(rows driver.Rows) error {
			return rows.Scan(&pending, &failReason)
		}, sql, c.database, c.tableNamePrefix+table, commandPattern)
		if err == nil {
			if pending == 0 {
				used := time.Since(startAt)
				if used >= slowQueryLimit {
					_, logger := log.FromContext(ctx, "table", table, "used", used.String())
					logger.Warnf("%s mutation done, but waited > %s", kind, slowQueryLimit)
				}
				return nil
			}
			if failReason != "" {
				failStreak++
				if failStreak >= failReportThreshold {
					return errors.Errorf("%s mutation on %s keeps failing: %s", kind, table, failReason)
				}
				_, logger := log.FromContext(ctx, "table", table, "failStreak", failStreak)
				logger.Warnf("%s mutation reported a failure, keep waiting: %s", kind, failReason)
			} else {
				failStreak = 0
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
