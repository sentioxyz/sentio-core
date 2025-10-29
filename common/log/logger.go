package log

import (
	"context"
	"flag"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"sentioxyz/sentio-core/common/version"
)

var (
	LogFormat = flag.String("log-format", "", "either empty (choose automatic), console or json")
	LogFile   = flag.String("log-file", "", "log file path")
)

var levelFlag = zap.LevelFlag("verbose", DefaultLevel(), "-1: debug, 0: info, 1: warning, 2: error")

// GlobalLogConfig TODO This is just for chainserver to easily config, change change server framework
var GlobalLogConfig *zap.Config = nil

func DefaultLevel() zapcore.Level {
	envLogLevel := os.Getenv("LOG_LEVEL")
	if envLogLevel != "" {
		level, err := zapcore.ParseLevel(envLogLevel)
		if err == nil {
			return level
		}
	}
	return zap.InfoLevel
}

func ManuallySetEncoder(encoder string) {
	*LogFormat = encoder
}

func ManuallySetLevel(level zapcore.Level) {
	*levelFlag = level
}

func lumberJackWriter() zapcore.WriteSyncer {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   *LogFile,
		MaxSize:    1024,
		MaxBackups: 10,
		MaxAge:     14,
		Compress:   true,
	}
	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(lumberjackLogger))
}

func zapDevelopmentConfig() zap.Config {
	c := zap.NewDevelopmentConfig()
	c.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	c.EncoderConfig.EncodeLevel = zapcore.LowercaseColorLevelEncoder
	c.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	return c
}

func zapDevelopmentEncoder() zapcore.Encoder {
	c := zapDevelopmentConfig()
	return zapcore.NewJSONEncoder(c.EncoderConfig)
}

func NewZapToFile() *zap.Logger {
	core := zapcore.NewCore(zapDevelopmentEncoder(), lumberJackWriter(), zap.NewAtomicLevelAt(*levelFlag))
	logger := zap.New(core, zap.AddCaller())
	zap.ReplaceGlobals(logger)
	return logger
}

func NewZap() *zap.Logger {
	// if GlobalLogConfig == nil {
	var c zap.Config
	if *LogFormat == "json" || version.IsProduction() && *LogFormat == "" {
		c = zap.NewProductionConfig()
		c.EncoderConfig = zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		// c = zapdriver.NewProductionConfig()
	} else {
		c = zapDevelopmentConfig()
	}

	// manual control level, only do stacktrace for error
	c.DisableStacktrace = true
	c.Level = zap.NewAtomicLevelAt(*levelFlag)
	GlobalLogConfig = &c

	zapLogger, _ := GlobalLogConfig.Build(zap.AddStacktrace(zap.ErrorLevel))
	return zapLogger
}

func NewRaw() *zap.Logger {
	if *LogFile != "" {
		return NewZapToFile()
	}
	return NewZap()
}

func BuildMetadata() {
	timestamp, err := strconv.ParseInt(version.BuildTimestamp, 10, 64)
	var timeStr string
	if err != nil {
		timeStr = "No BuildTimestamp"
	} else {
		timeStr = time.Unix(timestamp, 0).String()
	}

	Infow("Build Metadata",
		"Version", version.Version,
		"CommitSha", version.CommitSha,
		"BuildTime", timeStr,
	)
}

// Format string dont use assignment ot make static analysis happier
func Debugf(template string, args ...interface{}) {
	global.Debugf(template, args...)
}

func Debug(msg string, args ...interface{}) {
	global.Debug(msg, args...)
}

func Debuge(err error, args ...interface{}) {
	global.Debuge(err, args...)
}

func Debugfe(err error, template string, args ...interface{}) {
	global.Debugfe(err, template, args...)
}

func Debugw(msg string, keysAndValues ...interface{}) {
	global.Debugw(msg, keysAndValues...)
}

func DebugIf(cond bool, template string, args ...interface{}) {
	global.DebugIf(cond, template, args...)
}

func DebugIfF(cond bool, template string, args ...interface{}) {
	global.DebugIfF(cond, template, args...)
}

func DebugEveryN(n int, template string, args ...interface{}) {
	global.DebugEveryN(n, template, args...)
}

func DebugEveryNw(n int, msg string, keyAndValues ...interface{}) {
	global.DebugEveryNw(n, msg, keyAndValues...)
}

func Infof(template string, args ...interface{}) {
	global.Infof(template, args...)
}

func Info(msg string, args ...interface{}) {
	global.Info(msg, args...)
}

func Infoe(err error, args ...interface{}) {
	global.Infoe(err, args...)
}

func Infofe(err error, template string, args ...interface{}) {
	global.Infofe(err, template, args...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	global.Infow(msg, keysAndValues...)
}

func InfoIf(cond bool, template string, args ...interface{}) {
	global.InfoIf(cond, template, args...)
}

func InfoIfF(cond bool, template string, args ...interface{}) {
	global.InfoIfF(cond, template, args...)
}

func InfoEveryN(n int, template string, args ...interface{}) {
	global.InfoEveryN(n, template, args...)
}

func InfoEveryNw(n int, msg string, keyAndValues ...interface{}) {
	global.InfoEveryNw(n, msg, keyAndValues...)
}

func Warnf(template string, args ...interface{}) {
	global.Warnf(template, args...)
}

func Warn(msg string, args ...interface{}) {
	global.Warn(msg, args...)
}

func Warne(err error, args ...interface{}) {
	global.Warne(err, args...)
}

func Warnfe(err error, template string, args ...interface{}) {
	global.Warnfe(err, template, args...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	global.Warnw(msg, keysAndValues...)
}

func WarnIf(cond bool, template string, args ...interface{}) {
	global.WarnIf(cond, template, args...)
}

func WarnIfF(cond bool, template string, args ...interface{}) {
	global.WarnIfF(cond, template, args...)
}

func WarnEveryN(n int, template string, args ...interface{}) {
	global.WarnEveryN(n, template, args...)
}

func WarnEveryNw(n int, msg string, keyAndValues ...interface{}) {
	global.WarnEveryNw(n, msg, keyAndValues...)
}

func Errorfe(err error, template string, args ...interface{}) {
	global.Errorfe(err, template, args...)
}

func Errore(err error, args ...interface{}) {
	global.Errore(err, args...)
}

func Errorf(template string, args ...interface{}) {
	global.Errorf(template, args...)
}

func Error(msg string, args ...interface{}) {
	global.Error(msg, args...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	global.Errorw(msg, keysAndValues...)
}

func Fatalf(template string, args ...interface{}) {
	global.Fatalf(template, args...)
}

func Fatal(msg string, args ...interface{}) {
	global.Fatal(msg, args...)
}

func Fatalfe(err error, template string, args ...interface{}) {
	global.Fatalfe(err, template, args...)
}

func Fatale(err error, args ...interface{}) {
	global.Fatale(err, args...)
}

func Fatalw(msg string, keysAndValues ...interface{}) {
	global.Fatalw(msg, keysAndValues...)
}

func With(args ...interface{}) *SentioLogger {
	return fromSugar(globalRaw.Sugar().With(args...))
}

func UserVisible() *SentioLogger {
	return fromSugar(globalRaw.Sugar()).UserVisible()
}

var globalRaw *zap.Logger
var global *SentioLogger

func init() {
	BindFlag()
}

func BindFlag() {
	globalRaw = NewRaw()
	global = fromRaw(globalRaw.WithOptions(zap.AddCallerSkip(1)))
	// GlobalLogConfig.Level.SetLevel(*levelFlag)
}

func Sync() error {
	// if global != nil {
	//	global.Sync()
	// }
	if globalRaw != nil {
		globalRaw.Sync()
	}
	return nil
}

func WithContext(ctx context.Context) *SentioLogger {
	return fromRaw(WithContextFromParent(ctx, globalRaw))
}

func WithContextFromParent(ctx context.Context, parent *zap.Logger) *zap.Logger {
	spanCtx := trace.SpanContextFromContext(ctx)

	if spanCtx.IsValid() && spanCtx.IsSampled() {
		return parent.With(
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()))
	}
	return parent
}
