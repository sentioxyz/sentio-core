package utils

import (
	"net/url"
)

// MaskedSecret replaces every credential that is stripped out of a DSN before it reaches a log line.
const MaskedSecret = "xxxxx"

// MaskDSN hides the password of a database DSN so that the DSN can safely be logged. Both the
// userinfo password ("clickhouse://user:password@host:9000/db") and the "password" query parameter
// are masked, since ClickHouse accepts the credentials in either place. A DSN that cannot be parsed
// is reported as a placeholder instead of being returned as is: it may still carry a password, and
// there is no reliable way to locate it.
func MaskDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "<unparsable dsn>"
	}
	if query := parsed.Query(); query.Has("password") {
		query.Set("password", MaskedSecret)
		parsed.RawQuery = query.Encode()
	}
	return parsed.Redacted()
}
