package log

import (
	"context"
	"fmt"
	"hash/fnv"
	"reflect"
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
	// onceBloom is a best-effort, process-wide Bloom filter used by Once and *Once helpers
	// to avoid printing duplicate log messages. It is intentionally approximate: collisions
	// may cause some distinct messages to be suppressed, which is acceptable for logging.
	onceBloom = newBloomFilter()
)

// bloomFilter is a small concurrent-safe Bloom filter using two FNV-based hash functions.
// It is tuned for best-effort suppression of duplicate log call sites/template pairs.
type bloomFilter struct {
	bits []uint64
	mask uint64
}

const (
	// onceBloomBits defines the number of bits in the Bloom filter. It must be a power of two
	// to allow using a simple mask instead of modulo. 1<<18 == 262144 bits (~32 KiB).
	onceBloomBits = 1 << 18
)

func newBloomFilter() *bloomFilter {
	nWords := onceBloomBits / 64
	return &bloomFilter{
		bits: make([]uint64, nWords),
		mask: onceBloomBits - 1,
	}
}

// maybeAdd returns true if the key is probably already present, and false if this is the
// first time we've seen it (in which case the key is added to the filter).
func (b *bloomFilter) maybeAdd(key []byte) bool {
	if b == nil || len(b.bits) == 0 {
		return false
	}

	h1 := fnv1a64(key)
	h2 := fnv1a64Alt(key)

	idx1 := (h1 & b.mask) >> 6
	bit1 := uint64(1) << (h1 & 63)

	idx2 := (h2 & b.mask) >> 6
	bit2 := uint64(1) << (h2 & 63)

	// Load current words atomically.
	w1 := atomic.LoadUint64(&b.bits[idx1])
	w2 := atomic.LoadUint64(&b.bits[idx2])

	already := (w1&bit1 != 0) && (w2&bit2 != 0)

	// Set bits atomically; races are acceptable as long as we eventually set the bits.
	if w1&bit1 == 0 {
		atomic.StoreUint64(&b.bits[idx1], w1|bit1)
	}
	if w2&bit2 == 0 {
		atomic.StoreUint64(&b.bits[idx2], w2|bit2)
	}

	return already
}

func fnv1a64(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

// fnv1a64Alt derives a second hash from FNV-1a with a different offset basis.
func fnv1a64Alt(b []byte) uint64 {
	const offset64Alt = 1469598103934665603 ^ 0x9e3779b97f4a7c15
	const prime64 = 1099511628211

	h := uint64(offset64Alt)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

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

func invokeAnyFunc(v any) (any, bool) {
	t := reflect.TypeOf(v)
	if t == nil || t.Kind() != reflect.Func || t.NumIn() != 0 || t.NumOut() != 1 {
		return nil, false
	}
	if out := t.Out(0); out.NumMethod() != 0 {
		return nil, false
	}

	result := reflect.ValueOf(v).Call(nil)
	return result[0].Interface(), true
}

func (l *SentioLogger) lazy(args ...any) []any {
	var values []any
	for _, arg := range args {
		if f, ok := invokeAnyFunc(arg); ok {
			values = append(values, f)
		} else {
			values = append(values, arg)
		}
	}
	return values
}

func (l *SentioLogger) EveryN(n int, f func(template string, args ...interface{}), template string, args ...interface{}) {
	if n <= 0 {
		return
	}
	callSite := getCallSite(2)
	counter := getOrCreateCounter(callSite)

	count := atomic.AddInt64(counter, 1)
	if count%int64(n) == 0 {
		f(template, l.lazy(args...)...)
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
		f(msg, l.lazy(keyAndValues...)...)
	}
}

func (l *SentioLogger) IfF(cond bool, f func(template string, args ...interface{}), template string, args ...any) {
	if !cond {
		return
	}
	f(template, l.lazy(args...)...)
}

func (l *SentioLogger) If(cond bool, f func(template string, args ...interface{}), template string, args ...any) {
	if !cond {
		return
	}
	f(template, args...)
}

func (l *SentioLogger) Once(f func(template string, args ...interface{}), template string, args ...any) {
	// Build a stable key from call site and template so Once is scoped per call-site/template
	// pair across the process. We use skip=2 to be consistent with EveryN/EveryNw helpers.
	callSite := getCallSite(2)
	key := []byte(callSite + "|" + template)
	if onceBloom.maybeAdd(key) {
		return
	}
	f(template, args...)
}

func (l *SentioLogger) OnceF(f func(template string, args ...interface{}), template string, args ...any) {
	// Build a stable key from call site and template so Once is scoped per call-site/template
	// pair across the process. We use skip=2 to be consistent with EveryN/EveryNw helpers.
	callSite := getCallSite(2)
	key := []byte(callSite + "|" + template)
	if onceBloom.maybeAdd(key) {
		return
	}
	f(template, l.lazy(args...)...)
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

func (l *SentioLogger) DebugIf(cond bool, template string, args ...interface{}) {
	l.If(cond, l.AddCallerSkip(2).Debugf, template, args...)
}

func (l *SentioLogger) DebugIfF(cond bool, template string, args ...interface{}) {
	l.IfF(cond, l.AddCallerSkip(2).Debugf, template, args...)
}

func (l *SentioLogger) DebugEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.AddCallerSkip(2).Debugf, template, args...)
}

func (l *SentioLogger) DebugEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.AddCallerSkip(2).Debugw, msg, keyAndValues...)
}

func (l *SentioLogger) DebugOnce(template string, args ...interface{}) {
	l.Once(l.AddCallerSkip(2).Debugf, template, args...)
}

func (l *SentioLogger) DebugOnceF(template string, args ...interface{}) {
	l.OnceF(l.AddCallerSkip(2).Debugf, template, args...)
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

func (l *SentioLogger) InfoIf(cond bool, template string, args ...interface{}) {
	l.If(cond, l.AddCallerSkip(2).Infof, template, args...)
}

func (l *SentioLogger) InfoIfF(cond bool, template string, args ...interface{}) {
	l.IfF(cond, l.AddCallerSkip(2).Infof, template, args...)
}

func (l *SentioLogger) InfoEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.AddCallerSkip(2).Infof, template, args...)
}

func (l *SentioLogger) InfoEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.AddCallerSkip(2).Infow, msg, keyAndValues...)
}

func (l *SentioLogger) InfoOnce(template string, args ...interface{}) {
	l.Once(l.AddCallerSkip(2).Infof, template, args...)
}

func (l *SentioLogger) InfoOnceF(template string, args ...interface{}) {
	l.OnceF(l.AddCallerSkip(2).Infof, template, args...)
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

func (l *SentioLogger) WarnIf(cond bool, template string, args ...interface{}) {
	l.If(cond, l.AddCallerSkip(2).Warnf, template, args...)
}

func (l *SentioLogger) WarnIfF(cond bool, template string, args ...interface{}) {
	l.IfF(cond, l.AddCallerSkip(2).Warnf, template, args...)
}

func (l *SentioLogger) WarnEveryN(n int, template string, args ...interface{}) {
	l.EveryN(n, l.AddCallerSkip(2).Warnf, template, args...)
}

func (l *SentioLogger) WarnEveryNw(n int, msg string, keyAndValues ...interface{}) {
	l.EveryNw(n, l.AddCallerSkip(2).Warnw, msg, keyAndValues...)
}

func (l *SentioLogger) WarnOnce(template string, args ...interface{}) {
	l.Once(l.AddCallerSkip(2).Warnf, template, args...)
}

func (l *SentioLogger) WarnOnceF(template string, args ...interface{}) {
	l.OnceF(l.AddCallerSkip(2).Warnf, template, args...)
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

func (l *SentioLogger) ErrorOnce(template string, args ...interface{}) {
	l.Once(l.AddCallerSkip(2).Errorf, template, args...)
}

func (l *SentioLogger) ErrorOnceF(template string, args ...interface{}) {
	l.OnceF(l.AddCallerSkip(2).Errorf, template, args...)
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
