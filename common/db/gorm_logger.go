package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"sentioxyz/sentio-core/common/log"
)

type Logger struct {
	ZapLogger                 *zap.Logger
	LogLevel                  gormlogger.LogLevel
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
}

func NewLogger(zapLogger *zap.Logger, logConfig gormlogger.Config) Logger {
	return Logger{
		ZapLogger:                 zapLogger,
		LogLevel:                  logConfig.LogLevel,
		SlowThreshold:             logConfig.SlowThreshold,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: logConfig.IgnoreRecordNotFoundError,
	}
}

func (l Logger) SetAsDefault() {
	gormlogger.Default = l
}

func (l Logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return Logger{
		ZapLogger:                 l.ZapLogger,
		SlowThreshold:             l.SlowThreshold,
		LogLevel:                  level,
		SkipCallerLookup:          l.SkipCallerLookup,
		IgnoreRecordNotFoundError: l.IgnoreRecordNotFoundError,
	}
}

func (l Logger) Info(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Info {
		return
	}
	l.logger(ctx).Sugar().Infof(str, args...)
}

func (l Logger) Warn(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Warn {
		return
	}
	l.logger(ctx).Sugar().Warnf(str, args...)
}

func (l Logger) Error(ctx context.Context, str string, args ...interface{}) {
	if l.LogLevel < gormlogger.Error {
		return
	}
	l.logger(ctx).Sugar().Errorf(str, args...)
}

func (l Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= 0 {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= gormlogger.Error &&
		(!l.IgnoreRecordNotFoundError || !errors.Is(err, gorm.ErrRecordNotFound)):
		sql, rows := fc()
		l.logger(ctx).Error(sql, zap.Error(err), zap.Duration("elapsed", elapsed), zap.Int64("rows", rows))
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= gormlogger.Warn:
		sql, rows := fc()
		if len(sql) > 512 {
			sql = sql[:512] + "..."
		}
		slowLog := fmt.Sprintf("SLOW SQL >= %v %s", l.SlowThreshold, sql)
		l.logger(ctx).Warn(slowLog, zap.Duration("elapsed", elapsed), zap.Int64("rows", rows))
	case l.LogLevel >= gormlogger.Info:
		sql, rows := fc()
		l.logger(ctx).Info(sql, zap.Duration("elapsed", elapsed), zap.Int64("rows", rows))
	}
}

var (
	gormBazelPackage = "io_gorm_gorm"
	gormPackage      = filepath.Join("gorm.io", "gorm")
	zapgormPackage   = filepath.Join("db", "gorm_logger")
)

func (l Logger) logger(ctx context.Context) *zap.Logger {
	loggerFromCtx := log.WithContextFromParent(ctx, l.ZapLogger)
	for i := 2; i < 15; i++ {
		_, file, _, ok := runtime.Caller(i)
		switch {
		case !ok:
		case strings.HasSuffix(file, "_test.go"):
		case strings.Contains(file, gormPackage):
		case strings.Contains(file, gormBazelPackage):
		case strings.Contains(file, zapgormPackage):
		default:
			return loggerFromCtx.WithOptions(zap.AddCallerSkip(i))
		}
	}
	return loggerFromCtx
}
