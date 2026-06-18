package controller

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
)

var (
	ErrInternalNeedUpgrade    = errors.New("need upgrade")
	ErrInternalHasNewTemplate = errors.New("has new template")
	ErrInternalReorgDetected  = errors.New("reorg detected")
)

func NewExternalError(code int, err error) *ExternalError {
	if err == nil {
		panic(errors.Errorf("NewExternalError with code %d and nil err", code))
	}
	return &ExternalError{code: code, error: err}
}

type ExternalError struct {
	code int
	error
}

func (e *ExternalError) Code() int {
	return e.code
}

func (e *ExternalError) Wrapped() error {
	return e.error
}

func (e *ExternalError) Wrapf(fmt string, args ...any) *ExternalError {
	return NewExternalError(e.code, errors.Wrapf(e.error, fmt, args...))
}

func (e *ExternalError) Error() string {
	return fmt.Sprintf("ERR%03d: %s", e.code, e.error.Error())
}

func (e *ExternalError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "ERR%03d: %+v\n", e.code, e.error)
			return
		}
		fallthrough
	case 's', 'q':
		_, _ = io.WriteString(s, e.Error())
	}
}

func (e *ExternalError) IsUserError() bool {
	return e.code >= 300
}

func (e *ExternalError) IsUserRuntimeError() bool {
	switch e.code {
	case ErrCodeProcessFailed:
		return true
	default:
		return false
	}
}

func (e *ExternalError) IsSystemError() bool {
	return e.code < 200
}

func (e *ExternalError) IsDriverError() bool {
	return e.code >= 200 && e.code < 300
}

func (e *ExternalError) IsBillingError() bool {
	return e.code >= 400
}

// other error
const (
	ErrCodeSystem = iota + 100
	ErrCodeNeedUpgrade
	ErrCodeOOM
)

// driver error
const (
	ErrCodeCallProcessorFailed = iota + 200
	ErrCodeResetWasmInstanceFailed
	ErrCodeWasmError
	ErrCodeGetContractStartBlockFailed
	ErrCodeFetchDataFailed
	ErrCodeSubgraphEthCallFailed
	ErrCodeSubgraphIpfsCatFailed

	ErrCodeInvalidCheckpointData
	ErrCodeSaveCheckpointFailed
	ErrCodeQuotaServiceError

	ErrCodeCleanTimeSeriesDataFailed
	ErrCodeSaveTimeSeriesDataFailed

	ErrCodeSendWebhookDataFailed

	ErrCodeInitEntityFailed
	ErrCodeCleanEntityDataFailed
	ErrCodeSaveEntityDataFailed
	ErrCodeGetEntityFromDBFailed
	ErrCodeListEntityFromDBFailed
	ErrCodeInvalidEntityData
)

// processor error
const (
	ErrCodeUnexpectedProcessorConfig = iota + 300
	ErrCodeInvalidEntitySchema
	ErrCodeProcessorConfigsHasDiff
	ErrCodeProcessFailed
	ErrCodeCallWasmExportFunctionFailed
	ErrCodeWasmInitFailed
	ErrCodeWasmStackOverFlow
	ErrCodeCreateTemplateFailed
	ErrCodeInvalidSubgraphManifest

	ErrCodeGetUnknownEntity
	ErrCodeListUnknownEntity
	ErrCodeListRelatedEntityWithInvalidField
	ErrCodeInvalidListEntityFilter
	ErrCodeInvalidUpsertEntityRequest
	ErrCodeUpsertUnknownEntity
	ErrCodeInvalidUpdateEntityRequest
	ErrCodeUpdateUnknownEntity
	ErrCodeInvalidDeleteEntityRequest
	ErrCodeDeleteUnknownEntity
	ErrCodeUpdateImmutableEntity
	ErrCodeInvalidEntityFieldValue

	ErrCodeInvalidTimeSeriesData
	ErrCodeTimeSeriesDataSchemaChanged

	ErrCodeTooManyWebhookMsgEntity

	ErrCodeSubgraphEthCallWithInvalidParam
)

// billing error
const (
	ErrCodeOverQuota = iota + 400
)
