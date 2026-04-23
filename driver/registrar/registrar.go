// Package registrar defines the hook drivers use to mirror their physical
// ClickHouse (database, table) layout as on-chain records before issuing DDL.
// The concrete implementation lives outside sentio-core (e.g., a gRPC client
// that forwards the request to sentio-node, which signs the transaction).
// A nil value disables on-chain registration entirely — callers that target
// the sentio cloud should pass nil.
package registrar

import "context"

// OnChain mirrors (database, table) creations to the on-chain Databases
// contract. Implementations must be idempotent: repeated calls for an
// already-registered identifier must return nil without error.
//
// Drivers address databases by (processorID, replicaIndex); the on-chain
// database_id is composed server-side. Drivers do not know their indexerId —
// the server fills that from its own identity.
type OnChain interface {
	EnsureDatabase(ctx context.Context, processorID string, replicaIndex uint32) error
	EnsureTable(ctx context.Context, processorID string, replicaIndex uint32, tableID, tableType string) error
}
