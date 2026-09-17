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
			err = urlErr.Err
		}
		return nil, errors.Wrapf(err, "parse dsn %s failed", utils.AddURLMosaic(dsn))
	}
	return options, nil
}
