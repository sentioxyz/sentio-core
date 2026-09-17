package ckhmanager

import (
	"net/url"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
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

// ParseDSN parses a ClickHouse DSN the way clickhouse.ParseDSN does, but reports a failure with the
// password masked. The driver returns a *url.Error that embeds the raw DSN, so propagating that
// error as is leaks the credentials into whatever ends up logging it.
func ParseDSN(dsn string) (*clickhouse.Options, error) {
	options, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, errors.Wrapf(err, "parse dsn %s failed", MaskDSN(dsn))
	}
	return options, nil
}
