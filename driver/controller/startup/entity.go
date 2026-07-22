package startup

import (
	"context"
	"errors"
	"time"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/entity/clickhouse"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

type entityController struct {
	*persistent.Controller
}

func newEntityController(
	store *clickhouse.Store,
	chainID string,
	storeCacheSize int,
	storeFullCacheSize int,
	storeFullIDCacheMaxCount int,
	monitor persistent.MetricsMonitor,
) *entityController {
	return &entityController{
		Controller: persistent.NewController(
			clickhouse.NewChainStore(store, chainID, storeCacheSize, storeFullCacheSize, storeFullIDCacheMaxCount),
			monitor,
		),
	}
}

func (c entityController) Reset(ctx context.Context, checkpoint *controller.Checkpoint) *controller.ExternalError {
	var blockNumber int64 = -1
	if checkpoint != nil {
		blockNumber = int64(checkpoint.BlockNumber)
	}
	if err := c.Controller.Reorg(ctx, blockNumber); err != nil {
		return controller.NewExternalError(controller.ErrCodeCleanEntityDataFailed, err)
	}
	return nil
}

var maxUncommitedEntityChanges = envconf.LoadUInt64("SENTIO_MAX_UNCOMMITED_ENTITY_CHANGES", 1000000,
	envconf.WithMin(10000), envconf.WithMax(1000000))

func (c entityController) CachedTooMuch(blockNumber uint64) bool {
	return uint64(c.Controller.CountUncommittedChanges(blockNumber)) > maxUncommitedEntityChanges
}

func (c entityController) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (map[string]int, map[string]int, *controller.ExternalError) {
	created, updated, err := c.Controller.Commit(ctx, blockNumber, blockTime)
	if err != nil {
		if errors.Is(err, persistent.ErrUpdateImmutable) {
			return created, updated, controller.NewExternalError(controller.ErrCodeUpdateImmutableEntity, err)
		}
		if errors.Is(err, persistent.ErrInvalidFieldValue) {
			return created, updated, controller.NewExternalError(controller.ErrCodeInvalidEntityFieldValue, err)
		}
		return created, updated, controller.NewExternalError(controller.ErrCodeSaveEntityDataFailed, err)
	}
	return created, updated, nil
}

func (c entityController) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (*persistent.EntityBox, *controller.ExternalError) {
	box, err := c.Controller.GetEntity(ctx, typ, id, blockNumber)
	if err != nil {
		if errors.Is(err, persistent.ErrInvalidFieldValue) {
			return box, controller.NewExternalError(controller.ErrCodeInvalidEntityFieldValue, err)
		}
		return box, controller.NewExternalError(controller.ErrCodeGetEntityFromDBFailed, err)
	}
	return box, nil
}

func (c entityController) GetEntityInBlock(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (*persistent.EntityBox, *controller.ExternalError) {
	box, err := c.Controller.GetEntityInBlock(ctx, typ, id, blockNumber)
	if err != nil {
		if errors.Is(err, persistent.ErrInvalidFieldValue) {
			return box, controller.NewExternalError(controller.ErrCodeInvalidEntityFieldValue, err)
		}
		return box, controller.NewExternalError(controller.ErrCodeGetEntityFromDBFailed, err)
	}
	return box, nil
}

func (c entityController) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) ([]*persistent.EntityBox, *string, *controller.ExternalError) {
	boxes, next, err := c.Controller.ListEntity(ctx, entityType, filters, cursor, limit, blockNumber)
	if err != nil {
		if errors.Is(err, persistent.ErrInvalidListFilter) {
			return boxes, next, controller.NewExternalError(controller.ErrCodeInvalidListEntityFilter, err)
		}
		if errors.Is(err, persistent.ErrInvalidFieldValue) {
			return boxes, next, controller.NewExternalError(controller.ErrCodeInvalidEntityFieldValue, err)
		}
		return boxes, next, controller.NewExternalError(controller.ErrCodeListEntityFromDBFailed, err)
	}
	return boxes, next, nil
}

func (c entityController) ListRelated(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	fieldName string,
	blockNumber uint64,
) ([]*persistent.EntityBox, schema.EntityOrInterface, *controller.ExternalError) {
	boxes, target, err := c.Controller.ListRelated(ctx, entityType, id, fieldName, blockNumber)
	if err != nil {
		if errors.Is(err, persistent.ErrInvalidField) {
			return boxes, target, controller.NewExternalError(controller.ErrCodeListRelatedEntityWithInvalidField, err)
		}
		return boxes, target, controller.NewExternalError(controller.ErrCodeListEntityFromDBFailed, err)
	}
	return boxes, target, nil
}

func (c entityController) SetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	box persistent.UncommittedEntityBox,
) *controller.ExternalError {
	if err := c.Controller.SetEntity(ctx, entityType, box); err != nil {
		if errors.Is(err, persistent.ErrUpdateImmutable) {
			return controller.NewExternalError(controller.ErrCodeUpdateImmutableEntity, err)
		}
		if errors.Is(err, persistent.ErrInvalidFieldValue) {
			return controller.NewExternalError(controller.ErrCodeInvalidEntityFieldValue, err)
		}
		return controller.NewExternalError(controller.ErrCodeSaveEntityDataFailed, err)
	}
	return nil
}
