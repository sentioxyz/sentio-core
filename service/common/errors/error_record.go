package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/service/common/protos"
)

type ErrorRecord struct {
	ID            string // `gorm:"primarykey"`
	Namespace     ErrorNamespace
	Code          int32
	NamespaceCode int32
	Message       string
	CreatedAt     time.Time `gorm:"autoCreateTime:false"`
}

func (e *ErrorRecord) Error() string {
	return fmt.Sprintf("ErrorRecord for %d: [%d] %s", e.Namespace, e.Code, e.Message)
}

func (e *ErrorRecord) ToPB() *protos.ErrorRecord {
	newErrMode := e.Code >= 100
	var msg string
	if newErrMode || e.Namespace == PROCESSOR {
		msg = e.Message
	} else {
		if !e.CreatedAt.IsZero() {
			msg = "INTERNAL ERROR: This is likely a transient error and the system is trying to recover itself " +
				"(Sentio team will also be notified). Please check back later."
		}
	}
	ret := &protos.ErrorRecord{
		Id:            e.ID,
		Namespace:     int32(e.Namespace),
		Message:       msg,
		CreatedAt:     timestamppb.New(e.CreatedAt),
		Code:          e.Code,
		NamespaceCode: e.NamespaceCode,
	}
	return ret
}

func (e *ErrorRecord) FromPB(err *protos.ErrorRecord) {
	e.ID = err.Id
	e.Namespace = ErrorNamespace(err.Namespace)
	e.Message = err.Message
	e.CreatedAt = err.CreatedAt.AsTime()
	e.Code = err.Code
	e.NamespaceCode = err.NamespaceCode
}

type ErrorNamespace = uint32

const (
	DRIVER    = 0
	PROCESSOR = 1
	// Could be extended to other errors
)

func NewErrorRecord(namespace ErrorNamespace, err error) ErrorRecord {
	var message string

	code := codes.Unknown
	if errors.Is(err, context.DeadlineExceeded) {
		code = codes.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		code = codes.Canceled
	}

	st, ok := status.FromError(err)
	if ok {
		message = st.Message()
	} else {
		message = err.Error()
	}

	id, _ := gonanoid.GenerateLongID()
	return ErrorRecord{
		ID:        id,
		Namespace: namespace,
		Message:   message,
		Code:      int32(code),
		CreatedAt: time.Now(),
	}
}
