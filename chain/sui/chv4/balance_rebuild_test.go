package chv4

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/common/chx"
)

// rebuildConn records the statements of a balance rebuild in the order issued, and answers them
// as an empty database does.
type rebuildConn struct {
	mu  sync.Mutex
	log []string
}

func (c *rebuildConn) record(sql string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log = append(c.log, sql)
}

func (c *rebuildConn) GetClickhouseConn() clickhouse.Conn { return nil }
func (c *rebuildConn) GetDatabase() string                { return "chain_db" }
func (c *rebuildConn) GetCluster() string                 { return "" }
func (c *rebuildConn) GetHost() string                    { return "" }
func (c *rebuildConn) GetPassword() string                { return "" }
func (c *rebuildConn) GetUsername() string                { return "" }
func (c *rebuildConn) GetSettings() clickhouse.Settings   { return nil }
func (c *rebuildConn) Close()                             {}

func (c *rebuildConn) Exec(_ context.Context, sql string, _ ...any) error {
	c.record(sql)
	return nil
}

func (c *rebuildConn) Query(_ context.Context, sql string, _ ...any) (driver.Rows, error) {
	c.record(sql)
	switch {
	case strings.HasPrefix(sql, "SELECT count(), max(latest_fail_reason) FROM system.mutations"):
		// no delete is still running on the table
		return &rebuildRows{rows: [][]any{{uint64(0), ""}}, at: -1}, nil
	case strings.HasPrefix(sql, "SELECT COUNT(*) FROM"):
		// nothing to delete
		return &rebuildRows{rows: [][]any{{uint64(0)}}, at: -1}, nil
	}
	return &rebuildRows{at: -1}, nil
}

func (c *rebuildConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }

func (c *rebuildConn) PrepareBatch(_ context.Context, sql string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	c.record(sql)
	return &rebuildBatch{}, nil
}

type rebuildRows struct {
	driver.Rows
	rows [][]any
	at   int
}

func (r *rebuildRows) Next() bool {
	r.at++
	return r.at < len(r.rows)
}

func (r *rebuildRows) Scan(dest ...any) error {
	row := r.rows[r.at]
	if len(dest) != len(row) {
		return errors.Errorf("scan %d values from a row of %d", len(dest), len(row))
	}
	for i, d := range dest {
		v := reflect.ValueOf(d).Elem()
		v.Set(reflect.ValueOf(row[i]).Convert(v.Type()))
	}
	return nil
}

func (r *rebuildRows) Close() error { return nil }
func (r *rebuildRows) Err() error   { return nil }

type rebuildBatch struct {
	driver.Batch
}

func (b *rebuildBatch) Append(...any) error { return nil }
func (b *rebuildBatch) Flush() error        { return nil }
func (b *rebuildBatch) Send() error         { return nil }
func (b *rebuildBatch) Abort() error        { return nil }

// A rebuild of a page ends the INSERTs still running over its checkpoints before it deletes them,
// the ones of the slot store and of an earlier attempt of its own alike.
func Test_rebuildBalancePageEndsInserts(t *testing.T) {
	conn := &rebuildConn{}
	ctrl := chx.New(conn)
	var s balanceController
	require.NoError(t, s.Init(ctrl, t.TempDir()))

	ctx := context.Background()
	_, tooBig, err := s.rebuildBalancePage(ctx, 100, 199)
	require.NoError(t, err)
	assert.False(t, tooBig)

	at := func(prefix string) int {
		for i, sql := range conn.log {
			if strings.HasPrefix(sql, prefix) {
				return i
			}
		}
		return -1
	}
	kill, count, insert := at("KILL QUERY"), at("SELECT COUNT(*) FROM"), at("INSERT INTO")
	require.NotEqual(t, -1, kill, conn.log)
	require.NotEqual(t, -1, count, conn.log)
	require.NotEqual(t, -1, insert, conn.log)
	assert.Less(t, kill, count, "the inserts are ended before the delete counts what to remove")
	assert.Less(t, count, insert)

	// The statement is the one that ends the INSERTs of the scope the slot store tags its INSERTs
	// into the table with, over the same checkpoints; the rebuild tags its own INSERT with that scope.
	issued := conn.log[kill]
	require.NoError(t, chx.NewInsertScope(ctrl, tableNameBalances).End(ctx, 100, 199))
	assert.Equal(t, conn.log[len(conn.log)-1], issued)
	assert.Equal(t, chx.NewInsertScope(ctrl, tableNameBalances), s.inserts())
}
