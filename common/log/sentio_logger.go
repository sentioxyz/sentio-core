package log

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const ErrorSeparator = ": "

type SentioLogger struct {
	s *zap.SugaredLogger
}

func fromRaw(l *zap.Logger) *SentioLogger {
	return &SentioLogger{
		l.WithOptions(zap.AddCallerSkip(1)).Sugar(),
	}
}

func fromSugar(l *zap.SugaredLogger) *SentioLogger {
	return &SentioLogger{
		l.WithOptions(zap.AddCallerSkip(1)),
	}
}

type logKey struct {
}

var (
	ctxLogKey      = logKey{}
	loggerCounters sync.Map
)

// getCallSite returns a unique identifier for the calling location
func getCallSite(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// getOrCreateCounter gets or creates a counter for a specific call site
func getOrCreateCounter(callSite string) *int64 {
	if counter, ok := loggerCounters.Load(callSite); ok {
		return counter.(*int64)
	}

	counter := new(int64)
	actual, loaded := loggerCounters.LoadOrStore(callSite, counter)
	if loaded {
		return actual.(*int64)
	}
	return counter
}

func ToContext(ctx context.Context, logger *SentioLogger) context.Context {
	return context.WithValue(ctx, ctxLogKey, logger)
}

func FromContext(ctx context.Context, args ...any) (context.Context, *SentioLogger) {
	var logger *SentioLogger
	lany := ctx.Value(ctxLogKey)
	if lany != nil {
		l, ok := lany.(*SentioLogger)
		if ok {
			logger = l
		}
	}
	if logger == nil {
		logger = fromRaw(globalRaw)
	}
	if len(args) == 0 {
		return ctx, logger
	}
	logger = logger.With(args...)
	return context.WithValue(ctx, ctxLogKey, logger), logger
}

func FromContextWithTrace(ctx context.Context, args ...any) (context.Context, *SentioLogger) {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() && spanCtx.IsSampled() {
		args = append(args,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()))
	}
	return FromContext(ctx, args...)
}

func mergeArgs(err error, args []any) []any {
	if err == nil {
		return args
	}
	if len(args) == 0 {
		return []any{err.Error()}
	}
	return append(args, ErrorSeparator, err.Error())
}

func appendErr(template string, err error) string {
	if err != nil {
		return template + ErrorSeparator + err.Error()
	}
	return template
}

func (l *SentioLogger) WithContext(ctx context.Context) *SentioLogger {
	return &SentioLogger{
		WithContextFromParent(ctx, l.s.Desugar()).Sugar(),
	}
}

func (l *SentioLogger) AddCallerSkip(skip int) *SentioLogger {
	return &SentioLogger{l.s.WithOptions(zap.AddCallerSkip(skip))}
}

func (l *SentioLogger) UserVisible() *SentioLogger {
	return l.With("user_visible", true)
}

func (l *SentioLogger) With(args ...interface{}) *SentioLogger {
	return &SentioLogger{l.s.With(args...)}
}

func (l *SentioLogger) withError(err error) *SentioLogger {
	return &SentioLogger{l.s.With(zap.Error(err))}
}

func (l *SentioLogger) Sync() error {
	return l.s.Sync()
}

func (l *SentioLogger) EveryN(n int, f func(template string, args ...interface{}), template string, args ...interface{}) {
	if n <= 0 {
		return
	}
	callSite := getCallSite(2)
	counter := getOrCreateCounter(callSite)

	count := atomic.AddInt64(counter, 1)
	if count%int64(n) == 0 {
		f(template, args...)
	}
}

func (l *SentioLogger) EveryNw(n int, f func(msg string, keyAndValues ...interface{}), msg string, keyAndValues ...interface{}) {
	if n <= 0 {
		return
	}
	callSite := getCallSite(2)
	counter := getOrCreateCounter(callSite)

	count := atomic.AddInt64(counter, 1)
	if count%int64(n) == 0 {
		f(msg, keyAndValues...)
	}
}

func (l *SentioLogger) Debug(msg string, args ...interface{}) {
	l.s.Debug(append([]interface{}{msg}, args...)...)
}

func (l *SentioLogger) Debugf(template string, args ...interface{}) {
	l.s.Debugf(template, args...)
}

func (l *SentioLogger) Debugw(msg string, keysAndValues ...interface{}) {
	l.s.Debugw(msg, keysAndValues...)
}

func (l *SentioLogger) Debuge(err error, args ...interface{}) {
	l.withError(err).s.Debug(mergeArgs(err, args)...)
}

func (l *SentioLogger) Debugfe(err error, template string, args ...interface{}) {
	l.withError(err).s.Debugf(appendErr(template, err), args...)
}

func (l *SentioLogger) DebugEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.Debugf, template, args...)
}

func (l *SentioLogger) DebugEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.Debugw, msg, keyAndValues...)
}

func (l *SentioLogger) Info(msg string, args ...interface{}) {
	l.s.Info(append([]interface{}{msg}, args...)...)
}

func (l *SentioLogger) Infof(template string, args ...interface{}) {
	l.s.Infof(template, args...)
}

func (l *SentioLogger) Infow(msg string, keysAndValues ...interface{}) {
	l.s.Infow(msg, keysAndValues...)
}

func (l *SentioLogger) Infoe(err error, args ...interface{}) {
	l.withError(err).s.Info(mergeArgs(err, args)...)
}

func (l *SentioLogger) Infofe(err error, template string, args ...interface{}) {
	l.withError(err).s.Infof(appendErr(template, err), args...)
}

func (l *SentioLogger) InfoEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.Infof, template, args...)
}

func (l *SentioLogger) InfoEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.Infow, msg, keyAndValues...)
}

func (l *SentioLogger) Warn(msg string, args ...interface{}) {
	l.s.Warn(append([]interface{}{msg}, args...)...)
}

func (l *SentioLogger) Warnf(template string, args ...interface{}) {
	l.s.Warnf(template, args...)
}

func (l *SentioLogger) Warnw(msg string, keysAndValues ...interface{}) {
	l.s.Warnw(msg, keysAndValues...)
}

func (l *SentioLogger) Warne(err error, args ...interface{}) {
	l.withError(err).s.Warn(mergeArgs(err, args)...)
}

func (l *SentioLogger) Warnfe(err error, template string, args ...interface{}) {
	l.withError(err).s.Warnf(appendErr(template, err), args...)
}

func (l *SentioLogger) WarnEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.Warnf, template, args...)
}

func (l *SentioLogger) WarnEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.Warnw, msg, keyAndValues...)
}

func (l *SentioLogger) Error(msg string, args ...interface{}) {
	l.s.Error(append([]interface{}{msg}, args...)...)
}

func (l *SentioLogger) Errorf(template string, args ...interface{}) {
	l.s.Errorf(template, args...)
}

func (l *SentioLogger) Errorw(msg string, keysAndValues ...interface{}) {
	l.s.Errorw(msg, keysAndValues...)
}

func (l *SentioLogger) Errore(err error, args ...interface{}) {
	l.withError(err).s.Error(mergeArgs(err, args)...)
}

func (l *SentioLogger) Errorfe(err error, template string, args ...interface{}) {
	l.withError(err).s.Errorf(appendErr(template, err), args...)
}

func (l *SentioLogger) Fatal(msg string, args ...interface{}) {
	l.s.Fatal(append([]interface{}{msg}, args...)...)
}

func (l *SentioLogger) Fatalf(template string, args ...interface{}) {
	l.s.Fatalf(template, args...)
}

func (l *SentioLogger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.s.Fatalw(msg, keysAndValues...)
}

func (l *SentioLogger) Fatale(err error, args ...interface{}) {
	l.withError(err).s.Fatal(mergeArgs(err, args)...)
}

func (l *SentioLogger) Fatalfe(err error, template string, args ...interface{}) {
	l.withError(err).s.Fatalf(appendErr(template, err), args...)
}

func (l *SentioLogger) LogTimeUsed(start time.Time, warnLimit time.Duration, msg string, keysAndValues ...interface{}) {
	used := time.Since(start)
	if used <= warnLimit {
		l.s.Debugw(msg, append(keysAndValues, "used", used.String())...)
	} else {
		msg = fmt.Sprintf("%s, but used > %s", msg, warnLimit.String())
		l.s.Warnw(msg, append(keysAndValues, "used", used.String())...)
	}
}
