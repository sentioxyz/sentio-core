package persistent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// ChainStore is the chain-bound storage interface for entity data.
// Each ChainStore instance is bound to a single chain.
//
// Schema initialisation (InitEntitySchema) is intentionally excluded from this
// interface: it is a one-time setup operation that belongs to the storage
// backend (e.g. clickhouse.Store) and must be called once before any ChainStore
// is created, not once per chain.
type ChainStore interface {
	GetChain() string
	GetEntityType(entity string) *schema.Entity
	GetEntityOrInterfaceType(name string) schema.EntityOrInterface

	// GetEntity returns the entity with the given id.
	// fromCache is true when the result came entirely from in-memory cache.
	GetEntity(ctx context.Context, entityType *schema.Entity, id string) (*EntityBox, bool, error)

	// ListEntities returns entities matching the given filters.
	// fromCache is true when the results came entirely from in-memory cache.
	ListEntities(
		ctx context.Context,
		entityType *schema.Entity,
		filters []EntityFilter,
		limit int,
	) ([]*EntityBox, bool, error)

	GetTimeSeriesEntityMaxID(ctx context.Context, entityType *schema.Entity) (int64, error)
	SetEntities(ctx context.Context, entityType *schema.Entity, boxes []EntityBox) (int, error)
	GrowthAggregation(ctx context.Context, curBlockTime time.Time) error
	Reorg(ctx context.Context, blockNumber int64) error

	// CheckValue checks whether values in data are valid for the storage backend.
	CheckValue(entityType *schema.Entity, data map[string]any) error

	// Snapshot returns a snapshot of cache and store state for debugging/monitoring.
	Snapshot() any
}

type Controller struct {
	mu sync.Mutex

	store     ChainStore // persistent data (chain-bound)
	changes   changeSet // uncommited data
	committed *uint64

	timeStat *timewin.TimeWindowsManager[*timeStatWindow]

	monitor Monitor
}

func NewController(store ChainStore, monitor Monitor) *Controller {
	return &Controller{
		store:    store,
		changes:  make(changeSet),
		timeStat: timewin.NewTimeWindowsManager[*timeStatWindow](time.Minute),
		monitor:  monitor,
	}
}

func (t *Controller) GetEntityOrInterfaceType(entity string) schema.EntityOrInterface {
	return t.store.GetEntityOrInterfaceType(entity)
}

func (t *Controller) GetEntityType(entity string) *schema.Entity {
	return t.store.GetEntityType(entity)
}

// GetEntity returns the latest version of the entity at or before
// the given block number. May return ErrInvalidFieldValue.
func (t *Controller) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *EntityBox, err error) {
	return t.getEntityOrInterface(ctx, typ, id, blockNumber, false)
}

// GetEntityInBlock returns the entity only if it was created or
// updated in the given block number. May return ErrInvalidFieldValue.
func (t *Controller) GetEntityInBlock(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *EntityBox, err error) {
	return t.getEntityOrInterface(ctx, typ, id, blockNumber, true)
}

func (t *Controller) getEntityOrInterface(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
	inBlock bool,
) (box *EntityBox, err error) {
	for _, entityType := range typ.ListEntities() {
		box, err = t.getEntity(ctx, entityType, id, blockNumber, inBlock)
		if err != nil {
			return
		}
		if box != nil && box.Data != nil {
			return // found
		}
	}
	return // not found, return the last get result
}

func (t *Controller) executeEntityOperator(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
) (from string, err error) {
	from = "uncommitted"
	history, _ := utils.GetFromK2Map(t.changes, entityType.Name, id)
	for i, box := range history {
		if box.GenBlockNumber > blockNumber {
			break
		}
		if len(box.Operator) == 0 {
			continue
		}
		var preBox *EntityBox
		if i == 0 {
			var fromCache bool
			preBox, fromCache, err = t.store.GetEntity(ctx, entityType, id)
			from = utils.Select(fromCache, "cache", "persistent")
			if err != nil {
				return from, fmt.Errorf("execute entity operator for %s with id %s failed: get entity failed: %w",
					entityType.GetFullName(), id, err)
			}
		} else {
			preBox = history[i-1]
		}
		var preData map[string]any
		if preBox != nil {
			preData = preBox.Data
		} else {
			preData = make(map[string]any)
		}
		for fieldName, op := range box.Operator {
			field := entityType.GetFieldByName(fieldName)
			originVal, has := preData[fieldName]
			if !has {
				_, originVal = buildType(field.Type)
			}
			box.Data[fieldName] = calcOperator(field.Type, originVal, op)
		}
		if err = t.store.CheckValue(entityType, box.Data); err != nil {
			return from, fmt.Errorf(
				"%w: entity operator result for %s with id %s: %v",
				ErrInvalidFieldValue, entityType.GetFullName(), id, err,
			)
		}
		box.Operator = nil
	}
	return
}

func (t *Controller) executeAllEntityOperator(ctx context.Context, blockNumber uint64) error {
	for entity, set := range t.changes {
		entityType := t.store.GetEntityType(entity)
		for id := range set {
			if _, err := t.executeEntityOperator(ctx, entityType, id, blockNumber); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Controller) getEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
	inBlock bool,
) (box *EntityBox, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	start := time.Now()
	_, logger := log.FromContext(ctx, "entityType", entityType.Name, "id", id, "blockNumber", blockNumber)

	// get from s.changes
	from := "uncommitted"
	history, _ := utils.GetFromK2Map(t.changes, entityType.Name, id)
	box = history.Latest(blockNumber)
	if box != nil { // has uncommitted change
		if inBlock && box.GenBlockNumber < blockNumber {
			box = nil
		}
		if box != nil && len(box.Operator) > 0 {
			// calculate operators
			// box will be changed,  box.Operator will be set to nil and box.Data will be filled
			if from, err = t.executeEntityOperator(ctx, entityType, id, blockNumber); err != nil {
				logger.Errorfe(err, "execute operator failed")
				return
			}
		}
	} else if !inBlock {
		var fromCache bool
		box, fromCache, err = t.store.GetEntity(ctx, entityType, id) // all changes in store will before block number
		if err != nil {
			logger.Errore(err, "get entity from store failed")
			return
		}
		from = utils.Select(fromCache, "cache", "persistent")
	}

	used := time.Since(start)
	logger.Debugw("got entity", "box", box.String(), "from", from, "used", used)
	t.monitor.OnGet(ctx, entityType.GetName(), id, blockNumber, inBlock, from, used)
	t.timeStat.Append(&timeStatWindow{
		startAt: time.Now(),
		entityStat: map[string]entityTimeStat{
			entityType.Name: {
				get:      map[string]timehist.Histogram{from: timehist.Histogram{}.Incr(used)},
				getTotal: map[string]time.Duration{from: used},
			},
		},
	})
	return
}

func splitListCursor(cursor string) (persistent bool, id string) {
	if cursor == "" {
		return false, ""
	}
	return cursor[0] == '@', cursor[1:]
}

func buildListCursor(persistent bool, id string) string {
	return utils.Select(persistent, "@", "#") + id
}

var (
	ErrInvalidField      = errors.New("invalid field")
	ErrInvalidListFilter = errors.New("invalid list filter")
	ErrUpdateImmutable   = errors.New("update immutable")
	ErrInvalidFieldValue = errors.New("invalid field value")
)

// ListRelated returns entities related to the given entity via a reverse foreign key field.
// May return ErrInvalidField.
func (t *Controller) ListRelated(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	fieldName string,
	blockNumber uint64,
) (boxes []*EntityBox, target schema.EntityOrInterface, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	field := entityType.GetForeignKeyFieldByName(fieldName)
	if field == nil {
		return nil, nil, fmt.Errorf("%w: %s.%s is not exists", ErrInvalidField, entityType.GetName(), fieldName)
	}
	fieldTitle := fmt.Sprintf("%s.%s %s", entityType.GetName(), fieldName, field.Type.String())
	target = field.GetTarget()
	if target == nil || !field.IsReverseField() {
		return nil, nil, fmt.Errorf("%w: %s is not a reverse foreign key field", ErrInvalidField, fieldTitle)
	}
	targetFieldTitle := fmt.Sprintf("%s.%s", target.GetName(), field.GetReverseFieldName())
	if targetField := target.GetForeignKeyFieldByName(field.GetReverseFieldName()); targetField == nil {
		return nil, nil, fmt.Errorf("%w: %s is a reverse foreign key for %s, but %s is not exists",
			ErrInvalidField, fieldTitle, targetFieldTitle, targetFieldTitle)
	}

	for _, targetEntityType := range target.ListEntities() {
		targetField := targetEntityType.GetFieldByName(field.GetReverseFieldName())
		many := schema.BreakType(targetField.Type).CountListLayer() > 0
		filter := EntityFilter{
			Field: targetField,
			Op:    utils.Select(many, EntityFilterOpHasAny, EntityFilterOpEq),
			Value: []any{id},
		}
		var targetBoxes []*EntityBox
		targetBoxes, _, err = t.listEntity(
			ctx,
			targetEntityType,
			[]EntityFilter{filter},
			"",
			math.MaxInt,
			true,
			blockNumber)
		if err != nil {
			return nil, nil, err
		}
		boxes = append(boxes, targetBoxes...)
	}
	return
}

// ListEntity returns entities matching the given filters.
// May return ErrInvalidFieldValue or ErrInvalidListFilter.
func (t *Controller) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) (boxes []*EntityBox, next *string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.listEntity(ctx, entityType, filters, cursor, limit, false, blockNumber)
}

func (t *Controller) listEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	cursor string,
	limit int,
	loadRelated bool,
	blockNumber uint64,
) (boxes []*EntityBox, next *string, err error) {
	var persistentPart []*EntityBox
	var persistentPartFromCache bool
	var from = "uncommitted"

	start := time.Now()
	_, logger := log.FromContext(ctx)

	defer func() {
		if err != nil {
			return
		}
		used := time.Since(start)
		t.monitor.OnList(ctx, entityType.GetName(), blockNumber, loadRelated, from, len(boxes), len(persistentPart), used)
		t.timeStat.Append(&timeStatWindow{
			startAt: time.Now(),
			entityStat: map[string]entityTimeStat{
				entityType.Name: {
					list:      map[string]timehist.Histogram{from: timehist.Histogram{}.Incr(used)},
					listTotal: map[string]time.Duration{from: used},
				},
			},
		})
		logger.Debugw("list entity",
			"loadRelated", loadRelated,
			"entityType", entityType.GetName(),
			"filters", EntityFiltersString(filters),
			"cursor", cursor,
			"limit", limit,
			"next", next,
			"count", len(boxes),
			"persistentCount", len(persistentPart),
			"from", from,
			"used", used)
	}()

	if limit == 0 {
		return nil, nil, nil
	}

	// get uncommitted part result
	cp, cid := splitListCursor(cursor)
	checked := make(map[string]bool)
	for _, change := range t.changes[entityType.Name] {
		box := change.Latest(blockNumber)
		if box == nil {
			continue
		}
		checked[box.ID] = true
		if cp || box.ID <= cid {
			continue // before the cursor
		}
		if box.Data == nil {
			continue // deleted
		}
		if len(box.Operator) > 0 {
			// calculate operators
			// box will be changed,  box.Operator will be set to nil and box.Data will be filled
			if _, err = t.executeEntityOperator(ctx, entityType, box.ID, blockNumber); err != nil {
				logger.Errorfe(err, "execute operator failed")
				return
			}
		}
		if pass, cke := CheckFilters(filters, *box); cke != nil {
			logger.With("used", time.Since(start).String()).Errore(err, "check filters failed")
			return nil, nil, cke
		} else if !pass {
			continue // not match the filter
		}
		boxes = append(boxes, box)
	}
	SortEntityBoxes(boxes)
	if len(boxes) >= limit {
		boxes = boxes[:limit]
		next = utils.WrapPointer(buildListCursor(false, boxes[limit-1].ID))
		return
	}
	limit -= len(boxes)

	// get persistent part result
	primaryField := entityType.GetFieldByName(schema.EntityPrimaryFieldName)
	filters = append(filters, EntityFilter{
		Field: primaryField,
		Op:    EntityFilterOpNotIn,
		Value: utils.ToAnyArray(utils.GetOrderedMapKeys(checked)),
		idSet: checked,
	})
	if cp {
		filters = append(filters, EntityFilter{
			Field: primaryField,
			Op:    EntityFilterOpGt,
			Value: []any{cid},
		})
	}
	persistentPart, persistentPartFromCache, err = t.store.ListEntities(ctx, entityType, filters, limit)
	if err != nil {
		logger.With("used", time.Since(start).String()).Errore(err, "list entity in store failed")
		return nil, nil, err
	}
	from = utils.Select(persistentPartFromCache, "cache", "persistent")

	// merge result and return
	boxes = append(boxes, persistentPart...)
	if len(persistentPart) == limit {
		next = utils.WrapPointer(buildListCursor(true, boxes[len(boxes)-1].ID))
	}
	return
}

var uniqTimeSeriesID atomic.Int64

// SetEntity stores an entity into the uncommitted change set.
// May return ErrInvalidFieldValue or ErrUpdateImmutable.
func (t *Controller) SetEntity(ctx context.Context, entityType *schema.Entity, box EntityBox) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if box.GenBlockChain != t.store.GetChain() {
		// unreachable
		panic(fmt.Errorf("GenBlockChain %s not match the store chain %s", box.GenBlockChain, t.store.GetChain()))
	}
	box.Entity = entityType.Name

	if box.Data != nil {
		if err := t.store.CheckValue(entityType, box.Data); err != nil {
			return fmt.Errorf(
				"%w: set entity %s/%s in chain %s failed: %v",
				ErrInvalidFieldValue, entityType.Name,
				box.ID, box.GenBlockChain, err,
			)
		}
	}

	start := time.Now()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "box", box.String())

	if entityType.IsTimeSeries() {
		if box.Data == nil {
			return fmt.Errorf("%w: delete timeseries entity %s in chain %s",
				ErrUpdateImmutable, entityType.Name, box.GenBlockChain)
		}
		// id of time series entity can be auto-incremented
		// sea: https://thegraph.com/docs/en/subgraphs/best-practices/timeseries/#defining-timeseries-entities
		if idNum, _ := strconv.ParseInt(box.ID, 10, 64); idNum <= 0 {
			// Need auto gen box.ID.
			// Just make sure that box.ID does not conflict with others here.
			// It will be reset when commit.
			box.ID = "@" + strconv.FormatInt(uniqTimeSeriesID.Add(1), 10)
		}
		box.Data[schema.EntityTimestampFieldName] = box.GenBlockTime.UnixMicro()
	}

	history, _ := utils.GetFromK2Map(t.changes, entityType.Name, box.ID)
	if latest := history.Latest(math.MaxUint64); latest != nil && entityType.IsImmutable() {
		logger.Errorw("update immutable entity", "latest", latest.String())
		return fmt.Errorf("invalid update for %s/%s in chain %s, latest is %s: %w",
			entityType.Name, box.ID, box.GenBlockChain, latest.String(), ErrUpdateImmutable)
	}

	// put into t.changes
	history.Push(entityType, &box)
	utils.PutIntoK2Map(t.changes, entityType.Name, box.ID, history)

	remove := box.Data == nil
	hasOperator := len(box.Operator) > 0
	mode := "update"
	if remove {
		mode = "delete"
	} else if hasOperator {
		mode = "updateWithOperator"
	}
	used := time.Since(start)
	logger.Debugw("set entity", "hasOperator", hasOperator, "remove", remove, "used", used)
	t.monitor.OnSet(ctx, entityType.GetName(), box.ID, box.GenBlockNumber, remove, hasOperator, used)
	t.timeStat.Append(&timeStatWindow{
		startAt: time.Now(),
		entityStat: map[string]entityTimeStat{
			entityType.Name: {
				set:      map[string]timehist.Histogram{mode: timehist.Histogram{}.Incr(used)},
				setTotal: map[string]time.Duration{mode: used},
			},
		},
	})
	return nil
}

func (t *Controller) CountUncommittedChanges(blockNumber uint64) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.changes.Count(blockNumber)
}

// Commit persists all uncommitted changes up to the given block
// number. May return ErrInvalidFieldValue or ErrUpdateImmutable.
func (t *Controller) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (created map[string]int, updated map[string]int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	start := time.Now()
	_, logger := log.FromContext(ctx, "blockNumber", blockNumber)

	if err = t.executeAllEntityOperator(ctx, blockNumber); err != nil {
		logger.Errorfe(err, "execute all entity operators failed")
		return
	}

	created, updated = make(map[string]int), make(map[string]int)
	newChanges := t.changes.Split(blockNumber)
	for entity, set := range t.changes {
		entityLogger := logger.With("entity", entity)
		// save to persistent
		entityStart := time.Now()
		entityType := t.store.GetEntityType(entity)
		var entities []EntityBox
		if entityType.IsTimeSeries() {
			// set timestamp and reset id for all boxes
			var maxID int64
			maxID, err = t.store.GetTimeSeriesEntityMaxID(ctx, entityType)
			if err != nil {
				entityLogger.Errorfe(err, "commit changes of entity failed: get count of time series entity %q failed", entity)
				return
			}
			manualIDSet := make(map[string]bool)
			var tmp []EntityBox
			for id, history := range set {
				if !strings.HasPrefix(id, "@") {
					manualIDSet[id] = true
				}
				for _, box := range history {
					tmp = append(tmp, *box)
				}
			}

			sort.Slice(tmp, func(i, j int) bool {
				return tmp[i].GenBlockNumber < tmp[j].GenBlockNumber
			})
			for _, box := range tmp {
				if strings.HasPrefix(box.ID, "@") {
					// need reset box.ID
					maxID++
					box.ID = strconv.FormatInt(maxID, 10)
					for manualIDSet[box.ID] {
						// skip manual ID
						maxID++
						box.ID = strconv.FormatInt(maxID, 10)
					}
				}
				entities = append(entities, box)
			}
		} else {
			for _, history := range set {
				for _, box := range history {
					entities = append(entities, *box)
				}
			}
		}
		created[entity], err = t.store.SetEntities(ctx, entityType, entities)
		updated[entity] = len(entities) - created[entity]
		entityLogger = entityLogger.With("used", time.Since(entityStart))
		if err != nil {
			entityLogger.Errore(err, "commit changes of entity failed")
			return
		}
		entityLogger.Debugw("commit changes of entity succeed", "created", created[entity], "updated", updated[entity])
	}
	t.changes = newChanges
	t.committed = &blockNumber

	if err = t.store.GrowthAggregation(ctx, blockTime); err != nil {
		logger.Errorfe(err, "growth aggregation failed")
		return
	}

	used := time.Since(start)
	logger.Debugw("committed changes", "created", created, "updated", updated, "used", used)
	t.monitor.OnCommit(ctx, blockNumber, created, updated, used)
	t.timeStat.Append(&timeStatWindow{startAt: time.Now(), commit: timehist.Histogram{}.Incr(used)})
	return
}

func (t *Controller) Reorg(ctx context.Context, blockNumberGT int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	startAt := time.Now()
	defer func() {
		t.timeStat.Append(&timeStatWindow{
			startAt: time.Now(),
			reorg:   timehist.Histogram{}.Incr(time.Since(startAt)),
		})
	}()

	if blockNumberGT < 0 {
		t.changes = make(changeSet)
	} else {
		for _, set := range t.changes {
			for id, history := range set {
				_ = history.Split(uint64(blockNumberGT))
				set[id] = history
			}
		}
	}
	return t.store.Reorg(ctx, blockNumberGT)
}

func (t *Controller) Snapshot() any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]any{
		"store":      t.store.Snapshot(),
		"committed":  t.committed,
		"uncommited": t.changes.Snapshot(),
		"statistics": t.timeStat.Snapshot(),
	}
}
