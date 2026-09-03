package controller

import (
	"context"
	"time"

	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// EntityController manages entity reads and uncommitted entity changes, and commits them on demand.
//
// Implementations must be safe for concurrent use: the entity read/write methods may be called
// concurrently with each other and with Commit, CachedTooMuch and Snapshot without any external
// locking (checkpointController forwards to them without holding its own mutex). Callers guarantee
// that SetEntity is only invoked for block numbers greater than the block number of any Commit
// that has started, so implementations do not need to handle writes at or below the committing
// block number (persistent.Controller enforces this with a panic).
type EntityController interface {
	Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError
	CachedTooMuch(blockNumber uint64) bool
	Commit(
		ctx context.Context,
		blockNumber uint64,
		blockTime time.Time,
	) (created, updated map[string]int, err *ExternalError)

	GetEntityOrInterfaceType(entity string) schema.EntityOrInterface
	GetEntityType(entity string) *schema.Entity
	GetEntity(
		ctx context.Context,
		typ schema.EntityOrInterface,
		id string,
		blockNumber uint64,
	) (box *persistent.EntityBox, err *ExternalError)
	GetEntityInBlock(
		ctx context.Context,
		typ schema.EntityOrInterface,
		id string,
		blockNumber uint64,
	) (box *persistent.EntityBox, err *ExternalError)
	ListEntity(
		ctx context.Context,
		entityType *schema.Entity,
		filters []persistent.EntityFilter,
		cursor string,
		limit int,
		blockNumber uint64,
	) (boxes []*persistent.EntityBox, next *string, err *ExternalError)
	ListRelated(
		ctx context.Context,
		entityType *schema.Entity,
		id string,
		fieldName string,
		blockNumber uint64,
	) ([]*persistent.EntityBox, schema.EntityOrInterface, *ExternalError)
	SetEntity(ctx context.Context, entityType *schema.Entity, box persistent.UncommittedEntityBox) *ExternalError

	Snapshot() any
}

type EmptyEntityController struct{}

func (c EmptyEntityController) Reset(ctx context.Context, checkpoint *Checkpoint) *ExternalError {
	return nil
}

func (c EmptyEntityController) CachedTooMuch(blockNumber uint64) bool {
	return false
}

func (c EmptyEntityController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (map[string]int, map[string]int, *ExternalError) {
	return nil, nil, nil
}

func (c EmptyEntityController) GetEntityOrInterfaceType(entity string) schema.EntityOrInterface {
	return nil
}

func (c EmptyEntityController) GetEntityType(entity string) *schema.Entity {
	return nil // all are unknown entity type
}

func (c EmptyEntityController) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *persistent.EntityBox, err *ExternalError) {
	return nil, nil
}

func (c EmptyEntityController) GetEntityInBlock(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *persistent.EntityBox, err *ExternalError) {
	return nil, nil
}

func (c EmptyEntityController) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) (boxes []*persistent.EntityBox, next *string, err *ExternalError) {
	return nil, nil, nil
}

func (c EmptyEntityController) ListRelated(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	fieldName string,
	blockNumber uint64,
) ([]*persistent.EntityBox, schema.EntityOrInterface, *ExternalError) {
	return nil, nil, nil
}

func (c EmptyEntityController) SetEntity(context.Context, *schema.Entity, persistent.UncommittedEntityBox) *ExternalError {
	return nil
}

func (c EmptyEntityController) Snapshot() any {
	return nil
}
