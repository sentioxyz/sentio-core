package ckhmanager

import "flag"

var (
	ReadTimeout     = flag.Int("clickhouse-read-timeout", 0, "Clickhouse read timeout")
	DialTimeout     = flag.Int("clickhouse-dial-timeout", 0, "Clickhouse dial timeout")
	MaxIdleConns    = flag.Int("clickhouse-max-idle-conns", 0, "Clickhouse max idle conns")
	MaxOpenConns    = flag.Int("clickhouse-max-open-conns", 0, "Clickhouse max open conns")
	EnableSignQuery = flag.Bool("clickhouse-enable-sign-query", false, "Enable Clickhouse sign query")
)
