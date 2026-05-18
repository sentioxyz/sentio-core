package registry

import (
	"context"
	"strings"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/go-faster/errors"
)

type dbRegistry struct {
	mirror           statemirror.Mirror
	databaseMirror   statemirror.MirrorReadOnlyState[string, state.DatabaseInfo]
	permissionMirror statemirror.MirrorReadOnlyState[string, map[string]string]
}

func NewDbRegistry(m statemirror.Mirror) DbRegistry {
	return &dbRegistry{
		mirror: m,
		databaseMirror: statemirror.NewTypedMirror(m, statemirror.MappingDatabases, statemirror.JSONCodec[string, state.DatabaseInfo]{
			FieldFunc: func(db string) (string, error) {
				return db, nil
			},
			ParseFunc: func(field string) (string, error) {
				return field, nil
			},
		}),
		permissionMirror: statemirror.NewTypedMirror(m, statemirror.MappingDatabasePermissions, statemirror.JSONCodec[string, map[string]string]{
			FieldFunc: func(account string) (string, error) {
				return account, nil
			},
			ParseFunc: func(field string) (string, error) {
				return field, nil
			},
		}),
	}
}

func (r *dbRegistry) RetrieveDatabaseInfo(ctx context.Context, database Database) (state.DatabaseInfo, error) {
	if r.mirror == nil {
		return state.DatabaseInfo{}, errors.New("database mirror is not initialized")
	}
	_, logger := log.FromContext(ctx, "database", database)
	info, ok, err := r.databaseMirror.Get(ctx, string(database))
	if err != nil {
		logger.Errorf("failed to get database info for %s: %s", database, err.Error())
		return state.DatabaseInfo{}, errors.Wrap(err, "failed to get database info")
	}
	if !ok || info.PendingDelete {
		logger.Debugf("database not found: %s", database)
		return state.DatabaseInfo{}, errors.Errorf("database not found: %s", database)
	}
	return info, nil
}

func (r *dbRegistry) RetrieveAllDatabaseInfos(ctx context.Context) (map[Database]state.DatabaseInfo, error) {
	if r.mirror == nil {
		return nil, errors.New("database mirror is not initialized")
	}
	_, logger := log.FromContext(ctx)
	databaseInfos, err := r.databaseMirror.GetAll(ctx)
	if err != nil {
		logger.Errorf("failed to get all database infos: %s", err.Error())
		return nil, errors.Wrap(err, "failed to get all database infos")
	}
	result := make(map[Database]state.DatabaseInfo, len(databaseInfos))
	for db, info := range databaseInfos {
		if info.PendingDelete {
			continue
		}
		result[Database(db)] = info
	}
	return result, nil
}

// mirrorPermissionSource implements PermissionSource against a
// statemirror-backed hash of account → {dbId → authStr} entries.
type mirrorPermissionSource struct {
	mirror statemirror.MirrorReadOnlyState[string, map[string]string]
}

func (m *mirrorPermissionSource) GetAccountPermissions(ctx context.Context, account string) (map[string]string, bool, error) {
	return m.mirror.Get(ctx, account)
}

func (r *dbRegistry) RetrievePermissionsByAccount(ctx context.Context, address Address) (map[Database]DbAuth, error) {
	if r.mirror == nil {
		return nil, errors.New("mirror is not initialized")
	}
	address = Address(strings.ToLower(string(address)))
	_, logger := log.FromContext(ctx, "address", address)
	result, err := MergeAccountPermissions(ctx, &mirrorPermissionSource{r.permissionMirror}, string(address))
	if err != nil {
		logger.Errorf("failed to merge permissions for address %s: %s", address, err.Error())
		return nil, err
	}
	for db := range result {
		info, ok, err := r.databaseMirror.Get(ctx, string(db))
		if err != nil {
			logger.Errorf("failed to get database info for %s: %s", db, err.Error())
			return nil, errors.Wrap(err, "failed to get database info")
		}
		if !ok || info.PendingDelete {
			delete(result, db)
		}
	}
	return result, nil
}

func (r *dbRegistry) AccountHasPermission(ctx context.Context, address Address, database Database, action Action) (bool, error) {
	if r.mirror == nil {
		return false, errors.New("mirror is not initialized")
	}
	address = Address(strings.ToLower(string(address)))
	_, logger := log.FromContext(ctx, "address", address)

	info, ok, err := r.databaseMirror.Get(ctx, string(database))
	if err != nil {
		logger.Errorf("failed to get database info for %s: %s", database, err.Error())
		return false, errors.Wrap(err, "failed to get database info")
	}
	if !ok || info.PendingDelete {
		logger.Debugf("database not found: %s", database)
		return false, errors.Errorf("database not found: %s", database)
	}

	perms, err := MergeAccountPermissions(ctx, &mirrorPermissionSource{r.permissionMirror}, string(address))
	if err != nil {
		return false, err
	}
	effectiveAuth := perms[database]
	hasPermission := effectiveAuth&DbAuth(action) != 0
	logger.Debugf("permission check for %s on %s: effective=%d, action=%d, has=%v", address, database, effectiveAuth, action, hasPermission)
	return hasPermission, nil
}
