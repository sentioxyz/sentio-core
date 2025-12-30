package ckhmanager

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Conn interface {
	GetClickhouseConn() clickhouse.Conn
	GetDatabase() string
	GetCluster() string
	GetHost() string
	GetPassword() string
	GetUsername() string
	GetSettings() clickhouse.Settings

	Close()
	Exec(context.Context, string, ...any) error
	Query(context.Context, string, ...any) (driver.Rows, error)
	QueryRow(context.Context, string, ...any) driver.Row
}
