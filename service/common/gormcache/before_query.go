package gormcache

import (
	"context"
	"sentioxyz/sentio-core/common/log"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

type ParentKey string

const ParentCacheKey ParentKey = "parentCacheKey"

const NoCacheKey string = "noCache"

func (p *Plugin) BeforeQuery() func(db *gorm.DB) {
	return func(db *gorm.DB) {
		callbacks.BuildQuerySQL(db)
		tableName := ""
		if db.Statement.Schema != nil {
			tableName = db.Statement.Schema.Table
		} else {
			tableName = db.Statement.Table
		}
		ctx := db.Statement.Context

		//log.Debugw("before query", "table", tableName, "sql", db.Statement.SQL.String(), "values", db.Statement.Vars)

		if noCache, ok := db.Get(NoCacheKey); ok && noCache.(bool) {
			return
		}

		cacheKey := p.cache.GetCacheKey(tableName, db.Statement.SQL.String(), db.Statement.Vars, db.Statement.Preloads)

		// if parentCacheKey is not set, this is the top level query
		if _, ok := ctx.Value(ParentCacheKey).(string); !ok {
			newCtx := context.WithValue(ctx, ParentCacheKey, cacheKey)
			db.Statement.Context = newCtx
		} else {
			// this is a nested query, no need to query cache
			return
		}

		cacheValue, err := p.cache.GetQuery(ctx, cacheKey)

		if err != nil { // cache miss or other error
			p.cache.IncrCacheCount(false)

			if !errors.Is(err, ErrCacheMiss) {
				log.Errore(err, "cache error")
			}
			db.Error = nil
			return
		}
		// cache hit
		p.cache.IncrCacheCount(true)
		if cacheValue.ErrString != "" {
			// special error
			if cacheValue.ErrString == gorm.ErrRecordNotFound.Error() {
				// set this back to gorm.ErrRecordNotFound, in case someone use == to compare
				db.Error = gorm.ErrRecordNotFound
			} else {
				db.Error = errors.New(cacheValue.ErrString)
			}
			return
		}
		err = p.cache.Decode(ctx, cacheValue.Data, db.Statement.Dest)
		if err != nil {
			log.Errore(err, "cache decode error")
			db.Error = nil
			return
		}
		db.RowsAffected = cacheValue.AffectRows

		db.Error = ErrCacheHit
	}
}
