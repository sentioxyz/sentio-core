package ckhmanager

import (
	"context"
	"sync"

	"sentioxyz/sentio-core/common/clickhousemanager/helper"
	"sentioxyz/sentio-core/common/log"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type conn struct {
	conn        clickhouse.Conn
	connOptions *clickhouse.Options
	dsn         string
	cluster     string
	once        sync.Once
}

func (c *conn) GetClickhouseConn() clickhouse.Conn {
	return c.conn
}

func (c *conn) GetDatabase() string {
	return c.connOptions.Auth.Database
}

func (c *conn) GetUsername() string {
	return c.connOptions.Auth.Username
}

func (c *conn) GetPassword() string {
	return c.connOptions.Auth.Password
}

func (c *conn) GetCluster() string {
	c.once.Do(func() {
		c.cluster = helper.MustAutoGetCluster(context.Background(), c.conn)
	})
	return c.cluster
}

func (c *conn) GetHost() string {
	if len(c.connOptions.Addr) == 0 {
		return ""
	}
	return c.connOptions.Addr[0]
}

func (c *conn) GetSettings() clickhouse.Settings {
	return c.connOptions.Settings
}

func (c *conn) Close() {
	if err := c.conn.Close(); err != nil {
		log.Errorf("close clickhouse connection failed: %v", err)
	}
}

func (c *conn) Exec(ctx context.Context, sql string, args ...any) error {
	return c.conn.Exec(ctx, sql, args...)
}

func (c *conn) Query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	return c.conn.Query(ctx, sql, args...)
}

func (c *conn) QueryRow(ctx context.Context, sql string, args ...any) driver.Row {
	return c.conn.QueryRow(ctx, sql, args...)
}

func parseDSN(dsn string, connectOptions ...func(*Options)) *clickhouse.Options {
	ckhOptions := &clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "my_database",
			Username: "default",
			Password: "password",
		},
		Settings:     newConnSettingsMacro(),
		MaxIdleConns: *maxIdleConns,
		MaxOpenConns: *maxOpenConns,
		ReadTimeout:  *readTimeout,
		DialTimeout:  *dialTimeout,
	}
	if len(dsn) > 0 {
		var err error
		ckhOptions, err = clickhouse.ParseDSN(dsn)
		if err != nil {
			log.Errorf("parse dsn failed: %v", err)
			panic(err)
		}
		for k, v := range newConnSettingsMacro() {
			ckhOptions.Settings[k] = v
		}
		ckhOptions.ReadTimeout = *readTimeout
		ckhOptions.DialTimeout = *dialTimeout
		ckhOptions.MaxIdleConns = *maxIdleConns
		ckhOptions.MaxOpenConns = *maxOpenConns
	}
	var connOptions = &Options{}
	for _, opt := range connectOptions {
		opt(connOptions)
	}
	if len(connOptions.settings) > 0 {
		for k, v := range connOptions.settings {
			ckhOptions.Settings[k] = v
		}
	}
	if connOptions.maxIdleConns > 0 {
		ckhOptions.MaxIdleConns = connOptions.maxIdleConns
	}
	if connOptions.maxOpenConns > 0 {
		ckhOptions.MaxOpenConns = connOptions.maxOpenConns
	}
	if connOptions.readTimeout > 0 {
		ckhOptions.ReadTimeout = connOptions.readTimeout
	}
	if connOptions.dialTimeout > 0 {
		ckhOptions.DialTimeout = connOptions.dialTimeout
	}
	return ckhOptions
}

func connect(dsn string, connectOptions ...func(*Options)) Conn {
	ckhOptions := parseDSN(dsn, connectOptions...)
	ckhConn, err := clickhouse.Open(ckhOptions)
	if err != nil {
		log.Errorf("connect to clickhouse failed: %v", err)
		panic(err)
	}
	return &conn{
		conn:        ckhConn,
		connOptions: ckhOptions,
		dsn:         dsn,
	}
}

var (
	connections      = make(map[string]Conn)
	connectionsMutex sync.RWMutex
)

func NewOrGetConn(dsn string, connectOptions ...func(*Options)) Conn {
	var connOptions = &Options{}
	for _, opt := range connectOptions {
		opt(connOptions)
	}
	connectionsMutex.RLock()
	conn, ok := connections[dsn+connOptions.Serialization()]
	if ok {
		connectionsMutex.RUnlock()
		return conn
	}
	connectionsMutex.RUnlock()

	return NewConn(dsn, connectOptions...)
}

func NewConn(dsn string, connectOptions ...func(*Options)) Conn {
	var connOptions = &Options{}
	for _, opt := range connectOptions {
		opt(connOptions)
	}

	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	conn := connect(dsn, connectOptions...)
	connections[dsn+connOptions.Serialization()] = conn
	return conn
}
