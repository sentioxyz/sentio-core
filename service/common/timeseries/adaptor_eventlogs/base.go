package adaptor_eventlogs

import (
	"context"
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	processormodels "sentioxyz/sentio-core/service/processor/models"
)

const (
	JsonObjectErr = "Cannot parse JSON object here"
)

type Base struct {
	ctx       context.Context
	logger    *log.SentioLogger
	store     timeseries.Store
	meta      map[string]timeseries.Meta
	processor *processormodels.Processor
	errors    []error
}

func (b Base) Error() error {
	var errorMessage []string
	for _, err := range b.errors {
		errorMessage = append(errorMessage, err.Error())
	}
	if len(errorMessage) > 0 {
		return fmt.Errorf(strings.Join(errorMessage, "\n"))
	}
	return nil
}

func (b Base) Scan(ctx context.Context, scan ScanFunc, sql string, args ...any) (matrix.Matrix, error) {
	if err := b.Error(); err != nil {
		b.logger.Errorf("error: %s", err)
		return nil, err
	}
	rows, err := scan(ctx, sql, args)
	if err != nil {
		b.logger.Warnf("query failed: %v", err)
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return matrix.NewMatrix(rows)
}
