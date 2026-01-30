package ckhmanager

import "flag"

var (
	ReadTimeout  = flag.Int("clickhouse-read-timeout", 60, "Clickhouse read timeout")
	DialTimeout  = flag.Int("clickhouse-dial-timeout", 60, "Clickhouse dial timeout")
	MaxIdleConns = flag.Int("clickhouse-max-idle-conns", 30, "Clickhouse max idle conns")
	MaxOpenConns = flag.Int("clickhouse-max-open-conns", 100, "Clickhouse max open conns")
)
