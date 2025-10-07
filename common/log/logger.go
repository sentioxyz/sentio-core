package log

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"sentioxyz/sentio-core/common/version"
)

var (
	LogFormat      = flag.String("log-format", "console", "Log format, support console or json")
	LogLevel       = flag.String("log-level", "info", "Log level")
	DisableCaller  = flag.Bool("log-disable-caller", false, "Disable caller in log")
	DisableColor   = flag.Bool("log-disable-color", false, "Disable color in log")
	LogFileEnabled = flag.Bool("log-file-enabled", true, "Whether to enable log file to be written to file system or not")
	LogFilePath    = flag.String("log-file-path", "", "Log file path, if set to empty, logs won't be persisted to file")
	LogFileMaxSizeMB = flag.Int("log-file-max-size", 100, "Log file max size in MB")
	LogFileMaxAge = flag.Int("log-file-max-age", 30, "Log file max age in days")
	LogFileMaxBackups = flag.Int("log-file-max-backups", 3, "Log file max backups")
)

var globalRaw *zap.Logger
var global *SentioLogger

func BindFlag() {
	production := version.IsProduction()
	config := buildZapConfig(production)

	var err error
	globalRaw, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	global = &SentioLogger{globalRaw.Sugar()}
}

func buildZapConfig(production bool) zap.Config {
	var config zap.Config

	if *LogFormat == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
		if !*DisableColor {
			config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}
	}

	config.DisableCaller = *DisableCaller

	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(*LogLevel)); err != nil {
		log.Fatalf("Error parsing log level: %v", err)
	}
	config.Level = level

	if *LogFileEnabled && *LogFilePath != "" {
		w := zapcore.AddSync(&lumberjack.Logger{
			Filename:   *LogFilePath,
			MaxSize:    *LogFileMaxSizeMB,
			MaxAge:     *LogFileMaxAge,
			MaxBackups: *LogFileMaxBackups,
		})
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(config.EncoderConfig),
			w,
			config.Level,
		)
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}

		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(config.EncoderConfig),
			zapcore.AddSync(os.Stdout),
			config.Level,
		)

		multiCore := zapcore.NewTee(consoleCore, core)
		globalRaw = zap.New(multiCore, zap.AddCaller(), zap.AddCallerSkip(1))
		global = &SentioLogger{globalRaw.Sugar()}
		return config
	}

	return config
}

func Init() {
	BindFlag()
}

func Debug(args ...interface{}) {
	global.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	global.Debugf(template, args...)
}

func Info(args ...interface{}) {
	global.Info(args...)
}

func Infof(template string, args ...interface{}) {
	global.Infof(template, args...)
}

func Warn(args ...interface{}) {
	global.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	global.Warnf(template, args...)
}

func Error(args ...interface{}) {
	global.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	global.Errorf(template, args...)
}

func Errore(err error, msg string) {
	global.Errore(err, msg)
}

func Fatal(args ...interface{}) {
	global.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	global.Fatalf(template, args...)
}

func Panic(args ...interface{}) {
	global.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	global.Panicf(template, args...)
}

func With(fields ...zap.Field) *SentioLogger {
	return &SentioLogger{globalRaw.With(fields...).Sugar()}
}

func GetRawLogger() *zap.Logger {
	return globalRaw
}

func GetLogger() *SentioLogger {
	return global
}

func withDetail(err error) string {
	type stackTracer interface {
		StackTrace() interface{}
	}

	var st interface{}
	if err, ok := err.(stackTracer); ok {
		st = err.StackTrace()
	}

	m := map[string]interface{}{
		"error":      err.Error(),
		"stacktrace": st,
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func Sync() error {
	if globalRaw != nil {
		time.Sleep(time.Second)
		return globalRaw.Sync()
	}
	return nil
}
