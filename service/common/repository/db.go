package repository

import (
	"flag"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/db"
	"sentioxyz/sentio-core/service/common/gormcache"
	"sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/common/redis"
)

var skipMigration = flag.Bool("skip-migration", false, "Skip migration")
var maxOpenConnsFlag = flag.Int("max-open-conns", 16, "max open conns")
var maxIdleConnsFlag = flag.Int("max-idle-conns", 16, "max idle conns")
var maxConnLifetimeFlag = flag.Duration("max-conn-lifetime", 5*time.Minute, "max connection lifetime, 0 means no limit")
var noGormCache = flag.Bool("no-gorm-cache", false, "no cache")

func Connect(dbURL string) (*gorm.DB, error) {
	conn := db.ConnectDB(dbURL)
	// set connection pool size
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(*maxOpenConnsFlag)
	sqlDB.SetMaxIdleConns(*maxIdleConnsFlag)
	sqlDB.SetConnMaxLifetime(*maxConnLifetimeFlag)

	err = conn.SetupJoinTable(&models.Project{}, "Members", &models.ProjectMember{})
	if err != nil {
		return nil, err
	}
	err = conn.SetupJoinTable(&models.User{}, "SharedProjects", &models.ProjectMember{})
	if err != nil {
		return nil, err
	}
	// err = conn.SetupJoinTable(&models.Organization{}, "Users", &models.UserOrganization{})
	// if err != nil {
	//	return nil, err
	// }
	err = conn.SetupJoinTable(&models.User{}, "Organizations", &models.UserOrganization{})
	if err != nil {
		return nil, err
	}
	err = conn.SetupJoinTable(&models.User{}, "ViewedProjects", &models.ViewedProject{})
	if err != nil {
		return nil, err
	}
	err = conn.SetupJoinTable(&models.User{}, "StarredProjects", &models.StarredProject{})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func SetupDBWithoutCache(dbURL string, extraModels ...any) (*gorm.DB, error) {
	conn, err := Connect(dbURL)
	if err != nil {
		return nil, err
	}

	gormModels := []any{
		&models.User{},
		&models.Identity{},
		&models.Organization{},
		&models.Owner{},
		&models.Project{},
		&models.CommunityProject{},
		&models.APIKey{},
		&models.Notification{},
		&models.Account{},
		&models.ProjectView{},
		&models.ImportedProject{},
	}

	if !(*skipMigration) {
		err = conn.AutoMigrate(append(gormModels, extraModels...)...)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

func SetupDB(dbURL string, extraModels ...any) (*gorm.DB, error) {
	if *noGormCache {
		return SetupDBWithoutCache(dbURL, extraModels...)
	}

	return SetupDBWithRedis(dbURL, redis.NewClientWithDefaultOptions(), extraModels...)
}

func SetupDBWithRedis(dbURL string, redisClient *redisv9.Client, extraModels ...any) (*gorm.DB, error) {
	conn, err := SetupDBWithoutCache(dbURL, extraModels...)
	if err != nil {
		return nil, err
	}
	err = gormcache.SetupDBWithRedisCache(conn, redisClient)
	return conn, err
}
