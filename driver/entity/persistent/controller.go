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
//
// Implementations are not required to be thread-safe; the caller (e.g.
// Controller) is responsible for serialising concurrent access.
type ChainStore interface {
	GetChain() string
	GetEntityType(entity string) *schema.Entity
	GetEntityOrInterfaceType(name string) schema.EntityOrInterface

	// GetEntity returns the entity with the given id.
	// fromCache is true when the result came entirely from in-memory cache.
	// Will got nil if entity not exists or deleted before.
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

	// SetEntities writes a batch of entity boxes to persistent storage.
	//
	// Ordering contract: within boxes, entries that share the same ID are
	// ordered by ascending GenBlockNumber — later writes for the same ID always
	// appear after earlier ones in the slice.
	//
	// TimeSeries ID contract: for TimeSeries entity types, every box ID is
	// guaranteed to be greater than the value returned by GetTimeSeriesEntityMaxID
	// at the time of the call, so implementations need not perform a duplicate-ID
	// check for TimeSeries entities.
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
	changes   changeSet  // uncommitted data
	committed *uint64

	timeStat *timewin.TimeWindowsManager[*timeStatWindow]

	monitor Monitor
}

func NewController(store ChainStore, monitor Monitor) *Controller {
	if monitor == nil {
		monitor = emptyMonitor{}
	}
	return &Controller{
		store:    store,
		changes:  make(changeSet),
		timeStat: timewin.NewTimeWindowsManager[*timeStatWindow](time.Minute),
		monitor:  monitor,
	}
}

func (c *Controller) GetEntityOrInterfaceType(entity string) schema.EntityOrInterface {
	return c.store.GetEntityOrInterfaceType(entity)
}

func (c *Controller) GetEntityType(entity string) *schema.Entity {
	return c.store.GetEntityType(entity)
}

// GetEntity returns the latest version of the entity at or before
// the given block number. May return ErrInvalidFieldValue.
func (c *Controller) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *EntityBox, err error) {
	return c.getEntityOrInterface(ctx, typ, id, blockNumber, false)
}

// GetEntityInBlock returns the entity only if it was created or
// updated in the given block number. May return ErrInvalidFieldValue.
func (c *Controller) GetEntityInBlock(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *EntityBox, err error) {
	return c.getEntityOrInterface(ctx, typ, id, blockNumber, true)
}

func (c *Controller) getEntityOrInterface(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
	inBlock bool,
) (box *EntityBox, err error) {
	for _, entityType := range typ.ListEntities() {
		box, err = c.getEntity(ctx, entityType, id, blockNumber, inBlock)
		if err != nil {
			return
		}
		if box != nil && box.Data != nil {
			return // found
		}
	}
	return // not found, return the last get result
}

func (c *Controller) executeEntityOperator(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
) (from string, err error) {
	from = "uncommitted"
	history, _ := utils.GetFromK2Map(c.changes, entityType.Name, id)
	for i, box := range history {
		if box.GenBlockNumber > blockNumber {
			break
		}
		if box.Data == nil {
			continue
		}
		if len(box.Operator) == 0 {
			continue
		}
		var preBox *EntityBox
		if i == 0 {
			var fromCache bool
			preBox, fromCache, err = c.store.GetEntity(ctx, entityType, id)
			from = utils.Select(fromCache, "cache", "persistent")
			if err != nil {
				return from, fmt.Errorf("execute entity operator for %s with id %s failed: get entity failed: %w",
					entityType.GetFullName(), id, err)
			}
		} else {
			preBox = &history[i-1].EntityBox // always no Operator
		}
		var preData map[string]any
		if preBox != nil && preBox.Data != nil {
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
		if err = c.store.CheckValue(entityType, box.Data); err != nil {
			return from, fmt.Errorf(
				"%w: entity operator result for %s with id %s: %v",
				ErrInvalidFieldValue, entityType.GetFullName(), id, err,
			)
		}
		box.Operator = nil
	}
	return
}

func (c *Controller) executeAllEntityOperator(ctx context.Context, blockNumber uint64) error {
	for entity, set := range c.changes {
		entityType := c.store.GetEntityType(entity)
		for id := range set {
			if _, err := c.executeEntityOperator(ctx, entityType, id, blockNumber); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Controller) getEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
	inBlock bool,
) (box *EntityBox, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	start := time.Now()
	_, logger := log.FromContext(ctx, "entityType", entityType.Name, "id", id, "blockNumber", blockNumber)

	// get from s.changes
	from := "uncommitted"
	history, _ := utils.GetFromK2Map(c.changes, entityType.Name, id)
	uctBox := history.Latest(blockNumber)
	if uctBox != nil { // has uncommitted change
		if inBlock && uctBox.GenBlockNumber < blockNumber {
			uctBox = nil
		}
		if uctBox != nil && len(uctBox.Operator) > 0 {
			// calculate operators
			// uctBox will be changed,  uctBox.Operator will be set to nil and uctBox.Data will be filled
			if from, err = c.executeEntityOperator(ctx, entityType, id, blockNumber); err != nil {
				logger.Errorfe(err, "execute operator failed")
				return
			}
		}
		if uctBox != nil {
			box = &uctBox.EntityBox
		}
	} else if !inBlock {
		var fromCache bool
		box, fromCache, err = c.store.GetEntity(ctx, entityType, id) // all changes in store will before block number
		if err != nil {
			logger.Errore(err, "get entity from store failed")
			return
		}
		from = utils.Select(fromCache, "cache", "persistent")
	}

	used := time.Since(start)
	logger.Debugw("got entity", "box", box.String(), "from", from, "used", used)
	c.monitor.OnGet(ctx, entityType.GetName(), id, blockNumber, inBlock, from, used)
	c.timeStat.Append(&timeStatWindow{
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
func (c *Controller) ListRelated(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	fieldName string,
	blockNumber uint64,
) (boxes []*EntityBox, target schema.EntityOrInterface, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
		targetBoxes, _, err = c.listEntity(
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
func (c *Controller) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) (boxes []*EntityBox, next *string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listEntity(ctx, entityType, filters, cursor, limit, false, blockNumber)
}

func (c *Controller) listEntity(
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
		c.monitor.OnList(ctx, entityType.GetName(), blockNumber, loadRelated, from, len(boxes), len(persistentPart), used)
		c.timeStat.Append(&timeStatWindow{
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
	for _, change := range c.changes[entityType.Name] {
		uctBox := change.Latest(blockNumber)
		if uctBox == nil {
			continue
		}
		checked[uctBox.ID] = true
		if cp || uctBox.ID <= cid {
			continue // before the cursor
		}
		if uctBox.Data == nil {
			continue // deleted
		}
		if len(uctBox.Operator) > 0 {
			// calculate operators
			// uctBox will be changed, uctBox.Operator will be set to nil and uctBox.Data will be filled
			if _, err = c.executeEntityOperator(ctx, entityType, uctBox.ID, blockNumber); err != nil {
				logger.Errorfe(err, "execute operator failed")
				return
			}
		}
		if pass, cke := CheckFilters(filters, uctBox.EntityBox); cke != nil {
			logger.With("used", time.Since(start).String()).Errore(err, "check filters failed")
			return nil, nil, cke
		} else if !pass {
			continue // not match the filter
		}
		boxes = append(boxes, &uctBox.EntityBox)
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
	persistentPart, persistentPartFromCache, err = c.store.ListEntities(ctx, entityType, filters, limit)
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
func (c *Controller) SetEntity(ctx context.Context, entityType *schema.Entity, box UncommittedEntityBox) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.committed != nil && box.GenBlockNumber <= *c.committed {
		// unreachable
		panic(fmt.Errorf("GenBlockNumber %d must be greater than last committed block %d", box.GenBlockNumber, *c.committed))
	}
	box.Entity = entityType.Name

	if box.Data != nil {
		if err := c.store.CheckValue(entityType, box.Data); err != nil {
			return fmt.Errorf(
				"%w: set entity %s/%s in chain %s failed: %v",
				ErrInvalidFieldValue, entityType.Name,
				box.ID, c.store.GetChain(), err,
			)
		}
	}

	start := time.Now()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "box", box.String())

	if entityType.IsTimeSeries() {
		if box.Data == nil {
			return fmt.Errorf("%w: delete timeseries entity %s in chain %s",
				ErrUpdateImmutable, entityType.Name, c.store.GetChain())
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

	history, _ := utils.GetFromK2Map(c.changes, entityType.Name, box.ID)
	if latest := history.Latest(math.MaxUint64); latest != nil && entityType.IsImmutable() {
		logger.Errorw("update immutable entity", "latest", latest.String())
		return fmt.Errorf("invalid update for %s/%s in chain %s, latest is %s: %w",
			entityType.Name, box.ID, c.store.GetChain(), latest.String(), ErrUpdateImmutable)
	}

	// put into c.changes
	if merged, mergedBox := history.Push(entityType, &box); merged && mergedBox.Data != nil {
		if err := c.store.CheckValue(entityType, mergedBox.Data); err != nil {
			return fmt.Errorf(
				"%w: set entity %s/%s in chain %s failed: %v",
				ErrInvalidFieldValue, entityType.Name,
				box.ID, c.store.GetChain(), err,
			)
		}
	}
	utils.PutIntoK2Map(c.changes, entityType.Name, box.ID, history)

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
	c.monitor.OnSet(ctx, entityType.GetName(), box.ID, box.GenBlockNumber, remove, hasOperator, used)
	c.timeStat.Append(&timeStatWindow{
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

func (c *Controller) CountUncommittedChanges(blockNumber uint64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.changes.Count(blockNumber)
}

// Commit persists all uncommitted changes up to the given block
// number. May return ErrInvalidFieldValue or ErrUpdateImmutable.
func (c *Controller) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (created map[string]int, updated map[string]int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	start := time.Now()
	_, logger := log.FromContext(ctx, "blockNumber", blockNumber)

	if err = c.executeAllEntityOperator(ctx, blockNumber); err != nil {
		logger.Errorfe(err, "execute all entity operators failed")
		return
	}

	created, updated = make(map[string]int), make(map[string]int)
	newChanges := c.changes.Split(blockNumber)
	for entity, set := range c.changes {
		entityLogger := logger.With("entity", entity)
		// save to persistent
		entityStart := time.Now()
		entityType := c.store.GetEntityType(entity)
		var entities []EntityBox
		if entityType.IsTimeSeries() {
			// set timestamp and reset id for all boxes
			var maxID int64
			maxID, err = c.store.GetTimeSeriesEntityMaxID(ctx, entityType)
			if err != nil {
				entityLogger.Errorfe(err, "commit changes of entity failed: get count of time series entity %q failed", entity)
				return
			}
			manualIDSet := make(map[string]bool)
			var tmp []EntityBox
			for id, history := range set {
				if !strings.HasPrefix(id, "@") {
					manualIDSet[id] = true
					if manualID, _ := strconv.ParseInt(id, 10, 64); manualID <= maxID {
						entityLogger.Errorf("manual id %s for time series entity %q is too small, less than max id in store %d",
							id, entity, maxID)
						err = fmt.Errorf("%w: manual id %s for time series entity %q is too small, less than max id in store %d",
							ErrUpdateImmutable, id, entity, maxID)
						return
					}
				}
				for _, box := range history {
					tmp = append(tmp, box.EntityBox)
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
					entities = append(entities, box.EntityBox)
				}
			}
		}
		created[entity], err = c.store.SetEntities(ctx, entityType, entities)
		updated[entity] = len(entities) - created[entity]
		entityLogger = entityLogger.With("used", time.Since(entityStart))
		if err != nil {
			entityLogger.Errore(err, "commit changes of entity failed")
			return
		}
		entityLogger.Debugw("commit changes of entity succeed", "created", created[entity], "updated", updated[entity])
	}

	if err = c.store.GrowthAggregation(ctx, blockTime); err != nil {
		logger.Errorfe(err, "growth aggregation failed")
		return
	}

	c.changes = newChanges
	c.committed = &blockNumber
	used := time.Since(start)
	logger.Debugw("committed changes", "created", created, "updated", updated, "used", used)
	c.monitor.OnCommit(ctx, blockNumber, created, updated, used)
	c.timeStat.Append(&timeStatWindow{startAt: time.Now(), commit: timehist.Histogram{}.Incr(used)})
	return
}

func (c *Controller) Reorg(ctx context.Context, blockNumberGT int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	startAt := time.Now()
	defer func() {
		c.timeStat.Append(&timeStatWindow{
			startAt: time.Now(),
			reorg:   timehist.Histogram{}.Incr(time.Since(startAt)),
		})
	}()

	if blockNumberGT < 0 {
		c.changes = make(changeSet)
		c.committed = nil
	} else {
		_ = c.changes.Split(uint64(blockNumberGT)) // discard changes above blockNumberGT, clean up empty entries
		if c.committed != nil && *c.committed > uint64(blockNumberGT) {
			n := uint64(blockNumberGT)
			c.committed = &n
		}
	}
	return c.store.Reorg(ctx, blockNumberGT)
}

func (c *Controller) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"store":       c.store.Snapshot(),
		"committed":   c.committed,
		"uncommitted": c.changes.Snapshot(),
		"statistics":  c.timeStat.Snapshot(),
	}
}
