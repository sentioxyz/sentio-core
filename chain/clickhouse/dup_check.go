package clickhouse

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
)

// A save can leave a second copy of a flush behind: a round that fails after some of its flushes
// landed is retried, and the row-count probe that truncates a range before saving it can miss rows
// that are still being committed or not yet replicated to the replica it asks. Nothing notices
// until a query returns the same row twice, so the store looks itself over for it.

// DuplicateReport describes the rows sharing a unique key that a check found in one table.
type DuplicateReport struct {
	Table     string
	Groups    uint64 // distinct unique keys carried by more than one row
	ExtraRows uint64 // rows beyond the first one of every duplicated key
	First     uint64 // lowest number field value among the duplicated keys
	Last      uint64 // highest number field value among the duplicated keys
}

// KeepCheckingDuplicates looks the store over every interval until ctx ends, reporting every table
// that holds rows sharing a unique key as an error log. A check that fails is logged and left for
// the next one, so the task outlives a ClickHouse hiccup.
func (s *SimpleSlotStore[SLOT]) KeepCheckingDuplicates(
	ctx context.Context,
	history *RangeStore,
	interval time.Duration,
) error {
	ctx, logger := log.FromContext(ctx)
	logger.Infof("duplicate check begin, every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		startAt := time.Now()
		window, reports, err := s.CheckDuplicates(ctx, history)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				logger.Warnfe(err, "duplicate check failed, the window will be checked again next time")
			}
		case window.IsEmpty():
			logger.Info("duplicate check skipped, the destination has no recorded range yet")
		default:
			checkLogger := logger.With("window", window.String())
			for _, r := range reports {
				checkLogger.Errorw("detected duplicate rows",
					"table", r.Table,
					"dupGroups", r.Groups,
					"extraRows", r.ExtraRows,
					"firstSlot", r.First,
					"lastSlot", r.Last)
			}
			checkLogger.Infow("duplicate check finished",
				"tablesWithDuplicates", len(reports), "used", time.Since(startAt).String())
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// CheckDuplicates scans the slots the range store still has a record of, and reports the window it
// covered. Taking the window from that record rather than from a remembered watermark means
// consecutive checks overlap, a restart loses nothing, and a fork needs no special handling:
// rolling the destination back records an end below the ones before it, which pulls the slots it is
// about to have rewritten back into the window on their own. Anything older than what the record
// reaches is not this check's business; it belongs to whoever goes looking through history by hand.
func (s *SimpleSlotStore[SLOT]) CheckDuplicates(
	ctx context.Context,
	history *RangeStore,
) (rg.Range, []DuplicateReport, error) {
	oldest, recorded, err := history.OldestRecordedEnd(ctx)
	if err != nil {
		return rg.EmptyRange, nil, err
	}
	cur, err := history.Get(ctx)
	if err != nil {
		return rg.EmptyRange, nil, errors.Wrapf(err, "get current range failed")
	}
	window := duplicateCheckWindow(oldest, recorded, cur)
	if window.IsEmpty() {
		return window, nil, nil
	}
	reports, err := s.scanDuplicates(ctx, window)
	return window, reports, err
}

// duplicateCheckWindow is the window a check covers: from the oldest end the range store has kept
// up to the end the destination holds now, and never below the start of what it holds.
//
// That lower edge is exactly what the recorded history reaches, and no further. Slots written
// before the oldest recorded end are outside a check, the same way anything older than the
// retention is: on a destination that started empty, that is the batch that filled it, since the
// first range it ever recorded already ends at that batch's last slot. There is nothing in the
// history to anchor them to — how far one recorded end sits above the one before it is decided by
// the order concurrent flushes happen to complete in, not by any batch size — so a destination
// bootstrapped from nothing needs its first stretch looked at by hand, once.
func duplicateCheckWindow(oldest uint64, recorded bool, cur rg.Range) rg.Range {
	if !recorded || cur.IsEmpty() {
		return rg.EmptyRange
	}
	return rg.NewRange(min(max(oldest, cur.Start), *cur.End), *cur.End)
}

// nullableUniqueKeyColumns returns the unique key columns of the table that may hold NULL.
func (t TableSchema) nullableUniqueKeyColumns() []string {
	var columns []string
	for _, column := range t.UniqueKey {
		for _, field := range t.Table.Fields {
			if field.Name != column {
				continue
			}
			if _, nullable := field.Type.(chx.FieldTypeNullable); nullable {
				columns = append(columns, column)
			}
			break
		}
	}
	return columns
}

// duplicateCheckSQL counts the unique keys of the table carried by more than one row of
// rangeWhere, along with the extra rows and the slot range those keys span.
//
// A key column added to a table after the fact is NULL on every row written before, which says
// the identity of the row is unknown, not that it is shared. Such rows are left out of the scan:
// keeping them would turn one old range into a single enormous fake duplicate and hide the real
// ones. They stay unchecked until their range is re-synced.
func (t TableSchema) duplicateCheckSQL(fullName, rangeWhere string) string {
	where := rangeWhere
	for _, column := range t.nullableUniqueKeyColumns() {
		where += fmt.Sprintf(" AND `%s` IS NOT NULL", column)
	}
	return fmt.Sprintf(
		"SELECT count(), toUInt64(sum(c - 1)), toUInt64(min(n)), toUInt64(max(n)) FROM ("+
			"SELECT count() AS c, min(`%s`) AS n FROM %s WHERE %s GROUP BY `%s` HAVING c > 1)",
		t.NumberField,
		fullName,
		where,
		strings.Join(t.UniqueKey, "`, `"),
	)
}

// Validate rejects a table meta that would leave one of its tables out of the duplicate check.
// It runs when the store is built, so a table added without an identity fails the syncer at
// startup instead of quietly going unchecked.
func (m TablesMeta) Validate() error {
	for _, table := range m.Tables {
		if err := table.checkUniqueKey(); err != nil {
			return err
		}
	}
	return nil
}

// checkUniqueKey rejects a table that would drop out of the duplicate check without saying so: one
// that declares neither an identity for its rows nor a reason to go without one, and one that
// declares an identity the check could never scope a window for. A declared key has to mean the
// table is really scanned, otherwise it reads as coverage that is not there.
func (t TableSchema) checkUniqueKey() error {
	if len(t.UniqueKey) == 0 {
		if t.UniqueKeyExemption == "" {
			return errors.Errorf("table %s declares neither a unique key nor a reason to go without "+
				"one, see TableSchema.UniqueKey", t.Table.Name)
		}
		return nil
	}
	if t.UniqueKeyExemption != "" {
		return errors.Errorf("table %s declares both a unique key and a reason to go without one",
			t.Table.Name)
	}
	for _, column := range t.UniqueKey {
		if !slices.Contains(t.Table.Fields.Names(), column) {
			return errors.Errorf("unique key column %q is not a column of table %s", column, t.Table.Name)
		}
	}
	if t.NumberField == "" {
		return errors.Errorf("table %s declares a unique key but has no number field to scope a "+
			"duplicate check window with", t.Table.Name)
	}
	return nil
}

// duplicateCheckPageRows is about how many rows one scan aggregates over, and
// duplicateCheckBuckets how finely the rows of a window are counted to make those pages.
//
// A scan groups by the unique key, so its memory grows with the rows it covers, while the window
// it is handed does not: it is as wide as whatever the range store still remembers. Measured on a
// production sui mainnet window, scanning it whole (44M rows) cost 10 GiB and 29s, while pages of
// this size cost around 1 GiB and a fraction of a second each. Sizing the pages from a row count
// per bucket rather than from the total matters because rows are not spread evenly over a window:
// a busy stretch would otherwise make some pages several times heavier than the rest.
const (
	duplicateCheckPageRows = 5_000_000
	duplicateCheckBuckets  = 1_000
)

// scanDuplicates looks over one window: every table is scanned for unique keys carried by more
// than one row. Construction guarantees that a table without a key has said why it needs none, and
// that a table with one has a number field to scope a window with, so the only tables left out
// here are the ones that declared themselves out.
func (s *SimpleSlotStore[SLOT]) scanDuplicates(
	ctx context.Context,
	interval rg.Range,
) ([]DuplicateReport, error) {
	if interval.End == nil {
		return nil, errors.Errorf("interval %s is infinity", interval)
	}
	var reports []DuplicateReport
	for _, table := range s.tablesMeta.Tables {
		if len(table.UniqueKey) == 0 {
			continue // an exempt table, see TableSchema.UniqueKeyExemption
		}
		report, err := s.scanTableDuplicates(ctx, table, interval)
		if err != nil {
			return nil, err
		}
		if report.Groups > 0 {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

// scanTableDuplicates scans one table page by page and sums up what the pages found.
func (s *SimpleSlotStore[SLOT]) scanTableDuplicates(
	ctx context.Context,
	table TableSchema,
	interval rg.Range,
) (DuplicateReport, error) {
	report := DuplicateReport{Table: table.Table.Name}
	pages, err := s.duplicateCheckPages(ctx, table, interval)
	if err != nil {
		return report, err
	}
	for _, page := range pages {
		where, whereErr := s.tableRangeWhere(ctx, table, page)
		if whereErr != nil {
			return report, whereErr
		}
		var found DuplicateReport
		sql := table.duplicateCheckSQL(s.ctrl.FullLogicName(table.Table.Name), where)
		if err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
			return rows.Scan(&found.Groups, &found.ExtraRows, &found.First, &found.Last)
		}, sql); err != nil {
			return report, errors.Wrapf(err, "check duplicates of table %s in %s failed", table.Table.Name, page)
		}
		if found.Groups == 0 {
			continue
		}
		if report.Groups == 0 || found.First < report.First {
			report.First = found.First
		}
		if found.Last > report.Last {
			report.Last = found.Last
		}
		report.Groups += found.Groups
		report.ExtraRows += found.ExtraRows
	}
	return report, nil
}

// duplicateCheckPages counts the rows of the table per bucket of the window and packs consecutive
// buckets into pages of about duplicateCheckPageRows rows. Counting them costs next to nothing
// (70ms and 2 MiB over a two-day window of the widest table in production): only the number field
// is read, and it leads the sorting key of every table.
func (s *SimpleSlotStore[SLOT]) duplicateCheckPages(
	ctx context.Context,
	table TableSchema,
	interval rg.Range,
) ([]rg.Range, error) {
	where, err := s.tableRangeWhere(ctx, table, interval)
	if err != nil {
		return nil, err
	}
	bucket := max(*interval.Size()/duplicateCheckBuckets, 1)
	sql := fmt.Sprintf("SELECT intDiv(`%s`, %d) AS b, count() FROM %s WHERE %s GROUP BY b ORDER BY b",
		table.NumberField, bucket, s.ctrl.FullLogicName(table.Table.Name), where)
	var counted []bucketRows
	if err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var b bucketRows
		if scanErr := rows.Scan(&b.Bucket, &b.Rows); scanErr != nil {
			return scanErr
		}
		counted = append(counted, b)
		return nil
	}, sql); err != nil {
		return nil, errors.Wrapf(err, "count rows of table %s per bucket in %s failed", table.Table.Name, interval)
	}
	return packDuplicateCheckPages(interval, bucket, counted), nil
}

// bucketRows is how many rows of a table fall in one bucket of a window.
type bucketRows struct {
	Bucket uint64
	Rows   uint64
}

// packDuplicateCheckPages turns the row count per bucket into pages of about
// duplicateCheckPageRows rows. Buckets holding no row are absent, so the pages skip the empty
// stretches of the window instead of scanning them; a single bucket heavier than a page is left
// whole, since a bucket cannot be split any further.
func packDuplicateCheckPages(interval rg.Range, bucket uint64, counted []bucketRows) []rg.Range {
	var pages []rg.Range
	var start, end, rows uint64
	var open bool
	for _, b := range counted {
		if !open {
			start, rows, open = max(b.Bucket*bucket, interval.Start), 0, true
		}
		end = min(b.Bucket*bucket+bucket-1, *interval.End)
		rows += b.Rows
		if rows >= duplicateCheckPageRows {
			pages = append(pages, rg.NewRange(start, end))
			open = false
		}
	}
	if open {
		pages = append(pages, rg.NewRange(start, end))
	}
	return pages
}
