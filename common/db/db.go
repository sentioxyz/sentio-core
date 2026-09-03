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

// ConnectDBWithPrepare ties two independent settings together: prepare=false both
// drops GORM's statement cache and puts pgx on the simple protocol, which
// interpolates parameters as SQL text. Under the simple protocol pgx renders any
// fmt.Stringer through String(), so a protobuf enum arrives as "ACTIVE" rather
// than its int32, and JSON columns lose their type annotation — both are rejected
// by Postgres. Callers that only want the statement cache gone must use
// ConnectDBWithStatementCache instead.
func ConnectDBWithPrepare(dbURL string, prepare bool, opts ...func(*gorm.Config)) (*gorm.DB, error) {
	return connect(dbURL, prepare, !prepare, opts...)
}

// ConnectDBWithStatementCache controls GORM's prepared statement cache while
// keeping pgx on the extended protocol, so parameters stay binary-encoded.
//
// The cache guards its statement map with a single RWMutex. Under concurrency a
// goroutine that already holds a pooled connection inside a transaction can block
// acquiring the read lock while another waits for the write lock; Go's RWMutex
// favours pending writers, so every subsequent RLock queues behind that writer and
// the cycle never breaks — the transactions never release their connections, the
// pool drains, and every query deadlocks.
func ConnectDBWithStatementCache(dbURL string, statementCache bool, opts ...func(*gorm.Config)) (*gorm.DB, error) {
	return connect(dbURL, statementCache, false, opts...)
}

func connect(dbURL string, statementCache, simpleProtocol bool, opts ...func(*gorm.Config)) (*gorm.DB, error) {
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
		PrepareStmt: statementCache,
	}
	for _, opt := range opts {
		opt(c)
	}

	db, err := gorm.Open(postgres.Dialector{Config: &postgres.Config{DSN: dbURL, PreferSimpleProtocol: simpleProtocol}}, c)

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
