package ckhmanager

import (
	"net/url"
)

// maskedSecret replaces every credential we strip out of a DSN before it reaches a log line.
const maskedSecret = "xxxxx"

// MaskDSN hides the password of a ClickHouse DSN so that the DSN can safely be logged. Both the
// userinfo password ("clickhouse://user:password@host:9000/db") and the "password" query parameter
// are masked, since clickhouse.ParseDSN accepts the credentials in either place. A DSN that cannot
// be parsed is reported as a placeholder instead of being returned as is: it may still carry a
// password, and there is no reliable way to locate it.
func MaskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "<unparsable dsn>"
	}
	if query := parsed.Query(); query.Has("password") {
		query.Set("password", maskedSecret)
		parsed.RawQuery = query.Encode()
	}
	return parsed.Redacted()
}
