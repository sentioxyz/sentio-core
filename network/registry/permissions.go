package registry

import (
	"context"
	"strconv"

	"github.com/go-faster/errors"
)

// PermissionSource retrieves raw per-database permission strings for a
// lowercased Ethereum address. Values are decimal int64 strings (see
// parseAuth). Returning (nil, false, nil) means "no row on file" — not
// an error.
type PermissionSource interface {
	GetAccountPermissions(ctx context.Context, account string) (map[string]string, bool, error)
}

// expandAuth applies the permission hierarchy: Owner ⇒ Admin|Write|Read, Write ⇒ Read.
// Admin alone does NOT imply Write or Read.
func expandAuth(auth DbAuth) DbAuth {
	if auth&DbAuthOwner != 0 {
		auth |= DbAuthAdmin | DbAuthWrite | DbAuthRead
	}
	if auth&DbAuthWrite != 0 {
		auth |= DbAuthRead
	}
	return auth
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

// MergeAccountPermissions returns the effective per-database permission
// bitmaps for account by ORing in WildcardAddress permissions and running
// expandAuth on each entry. account must already be lowercase.
func MergeAccountPermissions(ctx context.Context, src PermissionSource, account string) (map[Database]DbAuth, error) {
	result := make(map[Database]DbAuth)
	if err := collectInto(ctx, src, account, result); err != nil {
		return nil, err
	}
	if account != string(WildcardAddress) {
		if err := collectInto(ctx, src, string(WildcardAddress), result); err != nil {
			return nil, err
		}
	}
	for db, auth := range result {
		result[db] = expandAuth(auth)
	}
	return result, nil
}

func collectInto(ctx context.Context, src PermissionSource, account string, result map[Database]DbAuth) error {
	perms, ok, err := src.GetAccountPermissions(ctx, account)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for db, authStr := range perms {
		auth, err := parseAuth(authStr)
		if err != nil {
			continue
		}
		result[Database(db)] |= auth
	}
	return nil
}
