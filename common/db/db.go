package db

import (
	"flag"
	"time"

	"gorm.io/gorm/schema"

	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sentioxyz/sentio-core/common/log"
)

var dbVerbose = flag.Bool("db-verbose", false, "Weather to do detail db client log")

func ConnectDB(dbURL string) *gorm.DB {
	db, err := ConnectDBWithPrepare(dbURL, true)
	if err != nil {
		log.Fatale(err)
	}
	return db
}

func ConnectDBWithPrepare(dbURL string, prepare bool, opts ...func(*gorm.Config)) (*gorm.DB, error) {
	schema.RegisterSerializer("protojson", &ProtoJSONSerializer{})

	logConfig := logger.Config{
		SlowThreshold:             2 * time.Second, // Slow SQL threshold
		LogLevel:                  logger.Warn,     // Log level
		IgnoreRecordNotFoundError: true,            // Ignore ErrRecordNotFound error for logger
	}

	if *dbVerbose {
		logConfig.LogLevel = logger.Info
	}

	c := &gorm.Config{
		Logger:      NewLogger(log.NewZap(), logConfig),
		PrepareStmt: prepare,
	}
	for _, opt := range opts {
		opt(c)
	}

	db, err := gorm.Open(postgres.Dialector{Config: &postgres.Config{DSN: dbURL, PreferSimpleProtocol: !prepare}}, c)

	if err != nil {
		log.Errore(err)
		return nil, err
	}
	err = db.Use(otelgorm.NewPlugin())
	if err != nil {
		log.Error("Failed to install gorm plugin")
		return nil, err
	}
	return db, nil
}
