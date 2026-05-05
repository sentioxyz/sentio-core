package registry

import "context"

// PermissionSource retrieves raw per-database permission strings for a
// lowercased Ethereum address. Values are decimal int64 strings (see
// parseAuth). Returning (nil, false, nil) means "no row on file" — not
// an error.
type PermissionSource interface {
	GetAccountPermissions(ctx context.Context, account string) (map[string]string, bool, error)
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
