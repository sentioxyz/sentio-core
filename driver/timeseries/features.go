package timeseries

import "fmt"

// Features controls schema-affecting behaviors of timeseries (metric/event)
// storage. It is built from the processor's TimeseriesSchemaVersion bits and
// must stay constant for the lifetime of a processor, since it determines the
// column types of the tables created for it.
type Features struct {
	// false: BigFloat use Decimal(76, 30) (Decimal256) in clickhouse
	// true:  BigFloat use Decimal(154, 60) (Decimal512) in clickhouse
	BigFloatUseDecimal512 bool
}

func BuildFeatures(schemaVersion int32) Features {
	fea := Features{
		BigFloatUseDecimal512: (schemaVersion & 1) > 0,
	}
	if schemaVersion < 0 || schemaVersion > 1 {
		panic(fmt.Errorf("timeseries schema version is %d, must be in [0,1]", schemaVersion))
	}
	return fea
}
