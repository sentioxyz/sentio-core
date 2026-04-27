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
// just these methods without taking on the full State surface.
type DatabaseAuthSource interface {
	GetDatabase(databaseId string) (DatabaseInfo, bool)
	GetDatabasePermissions() map[string]map[string]string
	GetIndexerInfo(indexerId uint64) (IndexerInfo, bool)
}

// IsDatabaseWriter reports whether addr is authorized to perform write
// operations (DROP DATABASE, INSERT/UPDATE/DELETE, ALTER, CREATE TABLE
// inside the DB) on the given user database.
//
// Authorization is granted when any of:
//
//  1. addr matches the bound indexer's current signer (IndexerInfo.Signer),
//     mirroring the on-chain check IndexerRegistry.getSigner(db.indexerId).
//  2. addr matches DatabaseInfo.Owner (case-insensitive hex compare).
//     Owner is checked directly so the predicate works even before the
//     event handler has projected Owner into the permission map (e.g.
//     immediately after a fresh CreateUserDatabase tx).
//  3. addr appears in the materialized per-account permission map with
//     WritePermission. The permission map is the canonical "writer set"
//     populated by syncDatabaseWriters from {Owner} ∪ Operators, so this
//     branch covers Operators without callers needing to access
//     DatabaseInfo.Operators directly.
//
// Address comparison is case-insensitive for robustness — callers may
// pass either case — but all addresses stored in state are normalized
// to lowercase hex (0x-prefixed) by the event handlers.
func IsDatabaseWriter(s DatabaseAuthSource, addr, databaseId string) bool {
	if addr == "" || databaseId == "" {
		return false
	}
	if db, ok := s.GetDatabase(databaseId); ok {
		// Branch 1: bound indexer signer (mirrors on-chain onlyWriter).
		if info, ok := s.GetIndexerInfo(db.IndexerId); ok && info.Signer != "" {
			if strings.EqualFold(info.Signer, addr) {
				return true
			}
		}
		// Branch 2: database owner.
		if strings.EqualFold(db.Owner, addr) {
			return true
		}
	}
	// Branch 3: permission map (covers operators).
	for account, perms := range s.GetDatabasePermissions() {
		if !strings.EqualFold(account, addr) {
			continue
		}
		return perms[databaseId] == WritePermission
	}
	return false
}
