package ckhmanager

import (
	"context"
	"crypto/ecdsa"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/clickhousemanager/helper"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mitchellh/hashstructure/v2"
)

var (
	rawConnections *utils.SafeMap[uint64, driver.Conn]
	connections    *utils.SafeMap[string, Conn]
)

func init() {
	rawConnections = utils.NewSafeMap[uint64, driver.Conn]()
	connections = utils.NewSafeMap[string, Conn]()
}

type conn struct {
	conn        clickhouse.Conn
	connOptions *clickhouse.Options
	dsn         string
	cluster     string
	once        sync.Once
	privateKey  *ecdsa.PrivateKey
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

func (c *conn) sign(ctx context.Context, query string) context.Context {
	if c.privateKey == nil {
		return ctx
	}
	signature, err := crypto.Sign(crypto.Keccak256([]byte(query)), c.privateKey)
	if err != nil {
		log.Errorf("sign query failed: %v", err)
		return ctx
	}
	return clickhouse.Context(ctx, clickhouse.WithQuotaKey(string(signature)))
}

func (c *conn) Exec(ctx context.Context, sql string, args ...any) error {
	return c.conn.Exec(c.sign(ctx, sql), sql, args...)
}

func (c *conn) Query(ctx context.Context, sql string, args ...any) (driver.Rows, error) {
	return c.conn.Query(c.sign(ctx, sql), sql, args...)
}

func (c *conn) QueryRow(ctx context.Context, sql string, args ...any) driver.Row {
	return c.conn.QueryRow(c.sign(ctx, sql), sql, args...)
}

func (c *conn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return c.conn.PrepareBatch(c.sign(ctx, query), query, opts...)
}

func parseDSNAndOptions(dsn string, connectOptions ...func(*Options)) (*clickhouse.Options, *ecdsa.PrivateKey) {
	ckhOptions := &clickhouse.Options{
		Addr: []string{"localhost:9000"},
		Auth: clickhouse.Auth{
			Database: "my_database",
			Username: "default",
			Password: "password",
		},
		Settings: newConnSettingsMacro(),
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
	return ckhOptions, connOptions.privateKey
}

type ckhHashStruct struct {
	Addr         []string
	Auth         clickhouse.Auth
	Settings     clickhouse.Settings
	ReadTimeout  time.Duration
	DialTimeout  time.Duration
	MaxIdleConns int
	MaxOpenConns int
}

func connect(dsn string, connectOptions ...func(*Options)) Conn {
	ckhOptions, privateKey := parseDSNAndOptions(dsn, connectOptions...)
	ckhHash, err := hashstructure.Hash(ckhHashStruct{
		Addr:         ckhOptions.Addr,
		Auth:         ckhOptions.Auth,
		Settings:     ckhOptions.Settings,
		ReadTimeout:  ckhOptions.ReadTimeout,
		DialTimeout:  ckhOptions.DialTimeout,
		MaxIdleConns: ckhOptions.MaxIdleConns,
		MaxOpenConns: ckhOptions.MaxOpenConns,
	}, hashstructure.FormatV2, nil)
	if err != nil {
		log.Errorf("hash clickhouse options failed: %v", err)
		panic(err)
	}
	ckhConn, ok := rawConnections.Get(ckhHash)
	if ok {
		return &conn{
			conn:        ckhConn.(clickhouse.Conn),
			connOptions: ckhOptions,
			dsn:         dsn,
			privateKey:  privateKey,
		}
	}
	ckhConn, err = clickhouse.Open(ckhOptions)
	if err != nil {
		log.Errorf("connect to clickhouse failed: %v", err)
		panic(err)
	}
	rawConnections.Put(ckhHash, ckhConn)
	return &conn{
		conn:        ckhConn,
		connOptions: ckhOptions,
		dsn:         dsn,
		privateKey:  privateKey,
	}
}

func NewOrGetConn(dsn string, connectOptions ...func(*Options)) Conn {
	var connOptions = &Options{}
	for _, opt := range connectOptions {
		opt(connOptions)
	}
	conn, ok := connections.Get(dsn + connOptions.Serialization())
	if ok {
		return conn
	}
	return NewConn(dsn, connectOptions...)
}

func NewConn(dsn string, connectOptions ...func(*Options)) Conn {
	var connOptions = &Options{}
	for _, opt := range connectOptions {
		opt(connOptions)
	}

	conn := connect(dsn, connectOptions...)
	connections.Put(dsn+connOptions.Serialization(), conn)
	return conn
}
