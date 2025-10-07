package log

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type SentioLogger struct {
	*zap.SugaredLogger
}

type contextKey struct{}

var logContextKey = contextKey{}

func FromContext(ctx context.Context) (context.Context, *SentioLogger) {
	if ctx == nil {
		ctx = context.Background()
	}

	logger, ok := ctx.Value(logContextKey).(*SentioLogger)
	if !ok {
		logger = global
	}

	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()
		logger = logger.With(zap.String("trace_id", traceID), zap.String("span_id", spanID))
	}

	ctx = context.WithValue(ctx, logContextKey, logger)
	return ctx, logger
}

func WithContext(ctx context.Context, logger *SentioLogger) context.Context {
	return context.WithValue(ctx, logContextKey, logger)
}

func (logger *SentioLogger) With(args ...interface{}) *SentioLogger {
	return &SentioLogger{logger.SugaredLogger.With(args...)}
}

func (logger *SentioLogger) WithOptions(opts ...zap.Option) *SentioLogger {
	return &SentioLogger{logger.SugaredLogger.WithOptions(opts...)}
}

func (logger *SentioLogger) AddCallerSkip(skip int) *SentioLogger {
	return logger.WithOptions(zap.AddCallerSkip(skip))
}

func (logger *SentioLogger) Debugw(msg string, keysAndValues ...interface{}) {
	logger.SugaredLogger.Debugw(msg, keysAndValues...)
}

func (logger *SentioLogger) Infow(msg string, keysAndValues ...interface{}) {
	logger.SugaredLogger.Infow(msg, keysAndValues...)
}

func (logger *SentioLogger) Warnw(msg string, keysAndValues ...interface{}) {
	logger.SugaredLogger.Warnw(msg, keysAndValues...)
}

func (logger *SentioLogger) Errorw(msg string, keysAndValues ...interface{}) {
	logger.SugaredLogger.Errorw(msg, keysAndValues...)
}

func (logger *SentioLogger) Errore(err error, msg string) {
	logger.SugaredLogger.Errorw(msg, "error", withDetail(err))
}

type logEveryNState struct {
	mu      sync.Mutex
	counter atomic.Uint64
}

var logEveryNStates sync.Map

func (logger *SentioLogger) InfoEveryN(n int, msg string) {
	key := fmt.Sprintf("%p_%s", logger, msg)

	value, _ := logEveryNStates.LoadOrStore(key, &logEveryNState{})
	state := value.(*logEveryNState)

	count := state.counter.Add(1)

	if count%uint64(n) == 1 {
		logger.Info(msg)
	}
}

func (logger *SentioLogger) DebugEveryN(n int, msg string) {
	key := fmt.Sprintf("%p_%s", logger, msg)

	value, _ := logEveryNStates.LoadOrStore(key, &logEveryNState{})
	state := value.(*logEveryNState)

	count := state.counter.Add(1)

	if count%uint64(n) == 1 {
		logger.Debug(msg)
	}
}

func (logger *SentioLogger) WarnEveryN(n int, msg string) {
	key := fmt.Sprintf("%p_%s", logger, msg)

	value, _ := logEveryNStates.LoadOrStore(key, &logEveryNState{})
	state := value.(*logEveryNState)

	count := state.counter.Add(1)

	if count%uint64(n) == 1 {
		logger.Warn(msg)
	}
}

func (logger *SentioLogger) ErrorEveryN(n int, msg string) {
	key := fmt.Sprintf("%p_%s", logger, msg)

	value, _ := logEveryNStates.LoadOrStore(key, &logEveryNState{})
	state := value.(*logEveryNState)

	count := state.counter.Add(1)

	if count%uint64(n) == 1 {
		logger.Error(msg)
	}
}

func (logger *SentioLogger) Check(lvl zapcore.Level, msg string) *zapcore.CheckedEntry {
	return logger.SugaredLogger.Desugar().Check(lvl, msg)
}
