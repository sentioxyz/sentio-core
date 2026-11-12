package timeseries

import "fmt"

// Features controls schema-affecting behaviors of timeseries (metric/event)
// storage. It is built from the processor's TimeseriesSchemaVersion bits and
// must stay constant for the lifetime of a processor, since it determines the
// column types of the tables created for it. No feature bit is defined yet.
type Features struct{}

func BuildFeatures(schemaVersion int32) Features {
	if schemaVersion != 0 {
		panic(fmt.Errorf("timeseries schema version is %d, must be 0", schemaVersion))
	}
	return Features{}
}
