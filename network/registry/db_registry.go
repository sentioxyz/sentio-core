package registry

import (
	"context"
	"strconv"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/go-faster/errors"
)

type Database string
type Address string

type DbAuth int64

const (
	DbAuthRead DbAuth = 1 << iota
	DbAuthWrite
	DbAuthAdmin
)

type Action int64

const (
	Read Action = 1 << iota
	Write
)

type DbRegistry interface {
	RetrieveDatabaseInfo(ctx context.Context, database Database) (state.DatabaseInfo, error)
	RetrievePermissionsByAccount(ctx context.Context, address Address) (map[Database]DbAuth, error)
	AccountHasPermission(ctx context.Context, address Address, database Database, action Action) (bool, error)
}

type userDbRegistry struct {
	mirror           statemirror.Mirror
	databaseMirror   statemirror.MirrorReadOnlyState[string, state.DatabaseInfo]
	permissionMirror statemirror.MirrorReadOnlyState[string, map[string]string]
}

var databaseCodec = statemirror.JSONCodec[string, state.DatabaseInfo]{
	FieldFunc: func(db string) (string, error) {
		return db, nil
	},
	ParseFunc: func(field string) (string, error) {
		return field, nil
	},
}

var permissionCodec = statemirror.JSONCodec[string, map[string]string]{
	FieldFunc: func(account string) (string, error) {
		return account, nil
	},
	ParseFunc: func(field string) (string, error) {
		return field, nil
	},
}

func NewUserDbRegistry(m statemirror.Mirror) DbRegistry {
	return &userDbRegistry{
		mirror:           m,
		databaseMirror:   statemirror.NewTypedMirror(m, statemirror.MappingDatabases, databaseCodec),
		permissionMirror: statemirror.NewTypedMirror(m, statemirror.MappingDatabasePermissions, permissionCodec),
	}
}

func (r *userDbRegistry) RetrieveDatabaseInfo(ctx context.Context, database Database) (state.DatabaseInfo, error) {
	if r.databaseMirror == nil {
		return state.DatabaseInfo{}, errors.New("database mirror is not initialized")
	}
	_, logger := log.FromContext(ctx)
	info, ok, err := r.databaseMirror.Get(ctx, string(database))
	if err != nil {
		logger.Errorf("failed to get database info for %s: %s", database, err.Error())
		return state.DatabaseInfo{}, errors.Wrap(err, "failed to get database info")
	}
	if !ok {
		logger.Errorf("database not found: %s", database)
		return state.DatabaseInfo{}, errors.Errorf("database not found: %s", database)
	}
	return info, nil
}

func (r *userDbRegistry) RetrievePermissionsByAccount(ctx context.Context, address Address) (map[Database]DbAuth, error) {
	if r.permissionMirror == nil {
		return nil, errors.New("permission mirror is not initialized")
	}
	_, logger := log.FromContext(ctx)
	authMap, ok, err := r.permissionMirror.Get(ctx, string(address))
	if err != nil {
		logger.Errorf("failed to get permissions for address %s: %s", address, err.Error())
		return nil, errors.Wrap(err, "failed to get permissions")
	}
	if !ok {
		return make(map[Database]DbAuth), nil
	}

	result := make(map[Database]DbAuth, len(authMap))
	for db, authStr := range authMap {
		auth, err := parseAuth(authStr)
		if err != nil {
			logger.Errorf("failed to parse auth for database %s: %s", db, err.Error())
			return nil, errors.Wrap(err, "failed to parse auth")
		}
		result[Database(db)] = auth
	}
	return result, nil
}

func (r *userDbRegistry) AccountHasPermission(ctx context.Context, address Address, database Database, action Action) (bool, error) {
	if r.databaseMirror == nil || r.permissionMirror == nil {
		return false, errors.New("mirror is not initialized")
	}
	_, logger := log.FromContext(ctx)
	info, ok, err := r.databaseMirror.Get(ctx, string(database))
	if err != nil {
		logger.Errorf("failed to get database info for %s: %s", database, err.Error())
		return false, errors.Wrap(err, "failed to get database info")
	}
	if !ok {
		logger.Errorf("database not found: %s", database)
		return false, errors.Errorf("database not found: %s", database)
	}

	ownerAuth := DbAuth(0)
	if info.Owner == string(address) {
		ownerAuth = DbAuthAdmin
	}

	auth := DbAuth(0)
	authMap, ok, err := r.permissionMirror.Get(ctx, string(address))
	if err != nil {
		logger.Warnf("failed to get permissions for %s, using owner-only: %s", address, err.Error())
	} else if ok {
		authStr, hasDb := authMap[string(database)]
		if hasDb {
			auth, err = parseAuth(authStr)
			if err != nil {
				logger.Warnf("failed to parse auth for %s on %s: %s", address, database, err.Error())
				auth = DbAuth(0)
			}
		}
	}

	effectiveAuth := auth | ownerAuth
	hasPermission := effectiveAuth&DbAuth(action) != 0
	logger.Debugf("permission check for %s on %s: effective=%d, action=%d, has=%v", address, database, effectiveAuth, action, hasPermission)
	return hasPermission, nil
}

func parseAuth(s string) (DbAuth, error) {
	if s == "" {
		return 0, nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, errors.Errorf("invalid permission format: %s", s)
	}
	return DbAuth(i), nil
}
