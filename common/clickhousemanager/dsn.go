package ckhmanager

import (
	"net/url"

	"sentioxyz/sentio-core/common/utils"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
)

// ParseDSN parses a ClickHouse DSN the way clickhouse.ParseDSN does, but reports a failure with the
// password masked. The driver returns a *url.Error that embeds the raw DSN, so propagating that
// error as is leaks the credentials into whatever ends up logging it.
func ParseDSN(dsn string) (*clickhouse.Options, error) {
	options, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			// Neither half of a malformed URL's error can be shown: it embeds the raw DSN, and its
			// cause quotes whatever it choked on - a password holding an unescaped "?" comes back
			// as an invalid port - so report the category alone. Nor can the DSN be masked, since
			// locating the password in a URL that does not parse is exactly what failed here.
			return nil, errors.New("parse dsn failed: malformed url")
		}
		// Any other cause names the option it rejected, and the password is never one of them.
		return nil, errors.Wrapf(err, "parse dsn %s failed", utils.AddURLMosaic(dsn))
	}
	return options, nil
}
