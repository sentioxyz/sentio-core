package chx

import (
	"context"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	chproto "github.com/ClickHouse/clickhouse-go/v2/lib/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClickhouseManagerConn struct {
	execErr func(sql string) error
	execs   []string
}

func (f *fakeClickhouseManagerConn) GetClickhouseConn() clickhouse.Conn {
	return nil
}

func (f *fakeClickhouseManagerConn) GetDatabase() string {
	return "db"
}

func (f *fakeClickhouseManagerConn) GetCluster() string {
	return "user-part-a"
}

func (f *fakeClickhouseManagerConn) GetHost() string {
	return "clickhouse-test"
}

func (f *fakeClickhouseManagerConn) GetPassword() string {
	return ""
}

func (f *fakeClickhouseManagerConn) GetUsername() string {
	return "default"
}

func (f *fakeClickhouseManagerConn) GetSettings() clickhouse.Settings {
	return nil
}

func (f *fakeClickhouseManagerConn) Close() {}

func (f *fakeClickhouseManagerConn) Exec(_ context.Context, sql string, _ ...any) error {
	f.execs = append(f.execs, sql)
	if f.execErr != nil {
		return f.execErr(sql)
	}
	return nil
}

func (f *fakeClickhouseManagerConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	panic("unexpected query")
}

func (f *fakeClickhouseManagerConn) QueryRow(context.Context, string, ...any) driver.Row {
	panic("unexpected query row")
}

func (f *fakeClickhouseManagerConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	panic("unexpected prepare batch")
}

func TestCreateViewRepairsCommentAfterTableAlreadyExists(t *testing.T) {
	conn := &fakeClickhouseManagerConn{
		execErr: func(sql string) error {
			if strings.HasPrefix(sql, "CREATE VIEW ") {
				return &chproto.Exception{
					Code:    57,
					Name:    "TABLE_ALREADY_EXISTS",
					Message: "Table db.view already exists",
				}
			}
			return nil
		},
	}
	ctrl := New(conn)

	err := ctrl.Create(context.Background(), View{
		Name:    "view",
		Select:  "SELECT 1",
		Comment: "SCHEMA_HASH(xxx)",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"CREATE VIEW `db`.`view` ON CLUSTER 'user-part-a' AS SELECT 1",
		"ALTER TABLE `db`.`view` ON CLUSTER 'user-part-a' MODIFY COMMENT 'SCHEMA_HASH(xxx)'",
	}, conn.execs)
}
