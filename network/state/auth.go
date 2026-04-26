package state

import "strings"

// WritePermission is the permission string the database event handler
// materializes for any writer (Owner or Operator) into the per-account
// permission map. New write-style permissions can be added later; this
// constant is the canonical spelling for "may execute write DDL/DML on a
// user database."
const WritePermission = "write"

// DatabaseAuthSource is the narrow interface IsDatabaseWriter consumes.
// State satisfies it, but downstream callers (e.g. clickhouse-proxy
// which exposes only a subset of State via NetworkState) can implement
// just these two methods without taking on the full State surface.
type DatabaseAuthSource interface {
	GetDatabase(databaseId string) (DatabaseInfo, bool)
	GetDatabasePermissions() map[string]map[string]string
}

// IsDatabaseWriter reports whether addr is authorized to perform write
// operations (DROP DATABASE, INSERT/UPDATE/DELETE, ALTER, CREATE TABLE
// inside the DB) on the given user database.
//
// Authorization is granted when either:
//
//  1. addr matches DatabaseInfo.Owner (case-insensitive hex compare).
//     Owner is checked directly so the predicate works even before the
//     event handler has projected Owner into the permission map (e.g.
//     immediately after a fresh CreateUserDatabase tx).
//  2. addr appears in the materialized per-account permission map with
//     WritePermission. The permission map is the canonical "writer set"
//     populated by syncDatabaseWriters from {Owner} ∪ Operators, so this
//     branch covers Operators without callers needing to access
//     DatabaseInfo.Operators directly.
//
// Address comparison is case-insensitive throughout — Owner is stored in
// EIP-55 checksum form (decoded.Owner.Hex()) but callers may pass either
// case.
func IsDatabaseWriter(s DatabaseAuthSource, addr, databaseId string) bool {
	if addr == "" || databaseId == "" {
		return false
	}
	if db, ok := s.GetDatabase(databaseId); ok {
		if strings.EqualFold(db.Owner, addr) {
			return true
		}
	}
	for account, perms := range s.GetDatabasePermissions() {
		if !strings.EqualFold(account, addr) {
			continue
		}
		return perms[databaseId] == WritePermission
	}
	return false
}
