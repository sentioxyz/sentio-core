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

	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
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
// Implementations must be safe for concurrent use: Controller.Commit calls
// SetEntities and GrowthAggregation without holding the Controller mutex, so
// they run concurrently with the read methods (which the Controller still
// serialises against each other).
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

	store   ChainStore // persistent data (chain-bound); must be safe for concurrent use
	changes changeSet  // uncommitted data
	// committing is the highest block number for which a Commit has started
	// (converged by Reorg). SetEntity asserts new boxes are above it: a write at
	// or below a started commit would be lost by Commit's snapshot-write-prune
	// sequence, so it panics instead.
	committing *uint64
	committed  *uint64

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

// prefetchKey identifies the store version of one entity.
type prefetchKey struct {
	entity string
	id     string
}

// storeVersion is the store version of one entity, read without c.mu.
type storeVersion struct {
	box       *EntityBox
	fromCache bool
}

// prefetchWant names a store version to read.
type prefetchWant struct {
	entityType *schema.Entity
	id         string
}

const (
	// storeReadConcurrency bounds the parallel store reads of prefetchStoreVersions.
	storeReadConcurrency = 16
	// maxStoreVersionRounds bounds how often withStoreVersions goes back to the store for versions
	// that concurrent writes made necessary since its previous round.
	maxStoreVersionRounds = 4
)

// storeVersionNeeded reports whether resolving history up to blockNumber reads the store: only the
// first pending version does, and only when it exists and is not resolved yet.
func storeVersionNeeded(history changeHistory, blockNumber uint64) bool {
	if len(history) == 0 {
		return false
	}
	first := history[0]
	return first.GenBlockNumber <= blockNumber && first.Data != nil && !first.Resolved()
}

// executeEntityOperator resolves the pending operators of one entity up to blockNumber. The first
// pending version reads the store version from prefetched; every later version reads the resolved
// version before it. Must be called under c.mu.
//
// A version missing from prefetched is read under c.mu as a last resort; withStoreVersions only
// lets that happen after maxStoreVersionRounds rounds of concurrent writes on the entities read.
func (c *Controller) executeEntityOperator(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
	prefetched map[prefetchKey]storeVersion,
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
		if box.Resolved() {
			continue
		}
		var preBox *EntityBox
		if i == 0 {
			if pre, has := prefetched[prefetchKey{entity: entityType.Name, id: id}]; has {
				preBox = pre.box
				from = utils.Select(pre.fromCache, "cache", "persistent")
			} else {
				_, logger := log.FromContext(ctx, "entityType", entityType.Name, "id", id, "blockNumber", blockNumber)
				logger.Warn("store version was not prefetched, reading it under the controller lock")
				var fromCache bool
				preBox, fromCache, err = c.store.GetEntity(ctx, entityType, id)
				from = utils.Select(fromCache, "cache", "persistent")
				if err != nil {
					return from, fmt.Errorf("execute entity operator for %s with id %s failed: get entity failed: %w",
						entityType.GetFullName(), id, err)
				}
			}
		} else {
			preBox = &history[i-1].EntityBox // always no Operator
		}
		row := expRow{exists: preBox != nil && preBox.Data != nil}
		if row.exists {
			row.data = preBox.Data
		}
		// resolve round by round, each round reads the resolved result of the one before it
		for _, round := range box.Operator {
			next := utils.CopyMap(row.data)
			if next == nil {
				next = make(map[string]any)
			}
			if err = resolveRound(entityType, round, row, next); err != nil {
				return from, fmt.Errorf("%w: resolve %s with id %s failed: %v",
					ErrInvalidFieldValue, entityType.GetFullName(), id, err)
			}
			row = expRow{exists: true, data: next}
		}
		// validate before touching the box, so that a rejected result leaves it pending and a later
		// read does not return the invalid row as resolved
		if err = c.store.CheckValue(entityType, row.data); err != nil {
			return from, fmt.Errorf(
				"%w: entity operator result for %s with id %s: %v",
				ErrInvalidFieldValue, entityType.GetFullName(), id, err,
			)
		}
		box.Data, box.Operator = row.data, nil
	}
	return
}

// checkBox validates every value a write carries before it is stored: the concrete Data and the
// Set corrections of the last pending round (earlier rounds were validated when they were stored).
// Expression and affine results are validated when they resolve.
func (c *Controller) checkBox(entityType *schema.Entity, box *UncommittedEntityBox) error {
	if box.Data != nil {
		if err := c.store.CheckValue(entityType, box.Data); err != nil {
			return err
		}
	}
	if !box.Resolved() {
		return c.store.CheckValue(entityType, box.LastRoundSetValues())
	}
	return nil
}

func (c *Controller) executeAllEntityOperator(
	ctx context.Context,
	blockNumber uint64,
	prefetched map[prefetchKey]storeVersion,
) error {
	for entity, set := range c.changes {
		entityType := c.store.GetEntityType(entity)
		for id := range set {
			if _, err := c.executeEntityOperator(ctx, entityType, id, blockNumber, prefetched); err != nil {
				return err
			}
		}
	}
	return nil
}

// pendingWants lists, under c.mu, the entities of entityType (all of them when ids is nil) whose
// resolution up to blockNumber reads the store and whose store version is not in prefetched yet.
func (c *Controller) pendingWants(
	entityType *schema.Entity,
	blockNumber uint64,
	prefetched map[prefetchKey]storeVersion,
	keep func(id string, history changeHistory) bool,
) (wants []prefetchWant) {
	for id, history := range c.changes[entityType.Name] {
		if !storeVersionNeeded(history, blockNumber) || (keep != nil && !keep(id, history)) {
			continue
		}
		if _, has := prefetched[prefetchKey{entity: entityType.Name, id: id}]; has {
			continue
		}
		wants = append(wants, prefetchWant{entityType: entityType, id: id})
	}
	return
}

// prefetchStoreVersions reads wants from the store without c.mu, in parallel, into prefetched.
func (c *Controller) prefetchStoreVersions(
	ctx context.Context,
	wants []prefetchWant,
	prefetched map[prefetchKey]storeVersion,
) error {
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	concurrency.RunWithTaskArray(g, gctx, storeReadConcurrency, wants,
		func(ctx context.Context, w prefetchWant) error {
			box, fromCache, err := c.store.GetEntity(ctx, w.entityType, w.id)
			if err != nil {
				return fmt.Errorf("prefetch %s with id %s failed: %w", w.entityType.GetFullName(), w.id, err)
			}
			mu.Lock()
			prefetched[prefetchKey{entity: w.entityType.Name, id: w.id}] = storeVersion{box: box, fromCache: fromCache}
			mu.Unlock()
			return nil
		})
	return g.Wait()
}

// withStoreVersions runs body under c.mu once every store version it needs is at hand, so that no
// store round trip happens while the lock is held. needs, called under c.mu, lists the versions
// still missing from prefetched; they are read without c.mu, and the check repeats because a
// concurrent write can make another version necessary. After maxStoreVersionRounds rounds body runs
// anyway (see executeEntityOperator for what it then does with a missing version).
//
// The store is only written by Commit, which runs alone, so a version read in an earlier round
// cannot go stale before body uses it.
func (c *Controller) withStoreVersions(
	ctx context.Context,
	needs func(prefetched map[prefetchKey]storeVersion) []prefetchWant,
	body func(prefetched map[prefetchKey]storeVersion) error,
) error {
	prefetched := make(map[prefetchKey]storeVersion)
	for round := 0; ; round++ {
		c.mu.Lock()
		wants := needs(prefetched)
		if len(wants) == 0 || round >= maxStoreVersionRounds {
			err := body(prefetched)
			c.mu.Unlock()
			return err
		}
		c.mu.Unlock()
		if err := c.prefetchStoreVersions(ctx, wants, prefetched); err != nil {
			return err
		}
	}
}

// getEntity returns the version of the entity at blockNumber: the latest uncommitted change (only
// one made in that very block when inBlock), else the store version. The store is read without
// c.mu (see withStoreVersions).
func (c *Controller) getEntity(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	blockNumber uint64,
	inBlock bool,
) (box *EntityBox, err error) {
	start := time.Now()
	_, logger := log.FromContext(ctx, "entityType", entityType.Name, "id", id, "blockNumber", blockNumber)
	key := prefetchKey{entity: entityType.Name, id: id}
	from := "uncommitted"

	// latest is the uncommitted change the read is about, nil when the store answers; under c.mu
	latest := func() (*UncommittedEntityBox, changeHistory) {
		history, _ := utils.GetFromK2Map(c.changes, entityType.Name, id)
		uctBox := history.Latest(blockNumber)
		if uctBox != nil && inBlock && uctBox.GenBlockNumber < blockNumber {
			uctBox = nil
		}
		return uctBox, history
	}
	err = c.withStoreVersions(ctx, func(prefetched map[prefetchKey]storeVersion) []prefetchWant {
		if _, has := prefetched[key]; has {
			return nil
		}
		uctBox, history := latest()
		if uctBox != nil {
			if !uctBox.Resolved() && storeVersionNeeded(history, blockNumber) {
				return []prefetchWant{{entityType: entityType, id: id}}
			}
			return nil
		}
		if inBlock {
			return nil // no change in that block, the store is not consulted
		}
		return []prefetchWant{{entityType: entityType, id: id}} // the store version is the answer
	}, func(prefetched map[prefetchKey]storeVersion) error {
		uctBox, _ := latest()
		if uctBox != nil { // has uncommitted change
			if !uctBox.Resolved() {
				// calculate operators
				// uctBox will be changed, uctBox.Operator will be set to nil and uctBox.Data will be filled
				if from, err = c.executeEntityOperator(ctx, entityType, id, blockNumber, prefetched); err != nil {
					logger.Errorfe(err, "execute operator failed")
					return err
				}
			}
			box = &uctBox.EntityBox
		} else if !inBlock {
			pre := prefetched[key] // all changes in store will before block number
			box = pre.box
			from = utils.Select(pre.fromCache, "cache", "persistent")
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
		return nil
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

	// each listEntity takes c.mu for its own uncommitted part only
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
func (c *Controller) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) (boxes []*EntityBox, next *string, err error) {
	return c.listEntity(ctx, entityType, filters, cursor, limit, false, blockNumber)
}

// listEntity merges the uncommitted changes with the store. The uncommitted part is computed under
// c.mu, with the store versions its pending entities need read beforehand (see withStoreVersions);
// the store query for the rest runs without c.mu.
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

	if limit == 0 {
		return nil, nil, nil
	}

	// get uncommitted part result
	cp, cid := splitListCursor(cursor)
	checked := make(map[string]bool)
	inCursor := func(id string) bool { return !cp && id > cid }
	enough := false
	err = c.withStoreVersions(ctx, func(prefetched map[prefetchKey]storeVersion) []prefetchWant {
		return c.pendingWants(entityType, blockNumber, prefetched, func(id string, history changeHistory) bool {
			uctBox := history.Latest(blockNumber)
			return uctBox != nil && uctBox.Data != nil && inCursor(uctBox.ID)
		})
	}, func(prefetched map[prefetchKey]storeVersion) error {
		for _, change := range c.changes[entityType.Name] {
			uctBox := change.Latest(blockNumber)
			if uctBox == nil {
				continue
			}
			checked[uctBox.ID] = true
			if !inCursor(uctBox.ID) {
				continue // before the cursor
			}
			if uctBox.Data == nil {
				continue // deleted
			}
			if !uctBox.Resolved() {
				// calculate operators
				// uctBox will be changed, uctBox.Operator will be set to nil and uctBox.Data will be filled
				if _, err := c.executeEntityOperator(ctx, entityType, uctBox.ID, blockNumber, prefetched); err != nil {
					logger.Errorfe(err, "execute operator failed")
					return err
				}
			}
			if pass, cke := CheckFilters(filters, uctBox.EntityBox); cke != nil {
				logger.With("used", time.Since(start).String()).Errore(cke, "check filters failed")
				return cke
			} else if !pass {
				continue // not match the filter
			}
			boxes = append(boxes, &uctBox.EntityBox)
		}
		SortEntityBoxes(boxes)
		if len(boxes) >= limit {
			boxes = boxes[:limit]
			next = utils.WrapPointer(buildListCursor(false, boxes[limit-1].ID))
			enough = true
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if !enough {
		limit -= len(boxes)

		// get persistent part result, without c.mu
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
	}

	used := time.Since(start)
	c.mu.Lock() // monitor callbacks are serialized by c.mu
	c.monitor.OnList(ctx, entityType.GetName(), blockNumber, loadRelated, from, len(boxes), len(persistentPart), used)
	c.mu.Unlock()
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
	return
}

var uniqTimeSeriesID atomic.Int64

// SetEntity stores an entity into the uncommitted change set.
// May return ErrInvalidFieldValue or ErrUpdateImmutable.
func (c *Controller) SetEntity(ctx context.Context, entityType *schema.Entity, box UncommittedEntityBox) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.committing != nil && box.GenBlockNumber <= *c.committing {
		panic(fmt.Errorf("set entity %s/%s at block %d, but a commit at block %d has already started",
			entityType.Name, box.ID, box.GenBlockNumber, *c.committing))
	}
	box.Entity = entityType.Name

	start := time.Now()
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "box", box.String())

	if entityType.IsImmutable() && !box.Resolved() {
		// an update reads and corrects the previous version, which an immutable entity must not
		// have; an update that sets every field is already an upsert (see FromEntityUpdateData)
		return fmt.Errorf("%w: update immutable entity %s in chain %s, use upsert",
			ErrUpdateImmutable, entityType.Name, c.store.GetChain())
	}
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

	// put into c.changes, validated first so that a rejected write changes nothing
	validate := func(stored *UncommittedEntityBox) error { return c.checkBox(entityType, stored) }
	if _, _, err := history.Push(entityType, &box, validate); err != nil {
		return fmt.Errorf(
			"%w: set entity %s/%s in chain %s failed: %v",
			ErrInvalidFieldValue, entityType.Name,
			box.ID, c.store.GetChain(), err,
		)
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

// commitBatch is the phase-1 snapshot of one entity type's changes to commit.
type commitBatch struct {
	entityType *schema.Entity
	boxes      []EntityBox
	manualIDs  set.Set[string] // time series entities only: ids that were not auto-generated
}

// assignTimeSeriesIDs replaces the auto-generated placeholder IDs ("@n") in a
// time series batch with sequential numeric IDs above the store's current max
// ID, ordering boxes by ascending GenBlockNumber. It works on the phase-1
// snapshot only and must not touch c.changes: it runs without holding c.mu.
// May return ErrUpdateImmutable.
func (c *Controller) assignTimeSeriesIDs(
	ctx context.Context,
	entityType *schema.Entity,
	boxes []EntityBox,
	manualIDs set.Set[string],
) ([]EntityBox, error) {
	maxID, err := c.store.GetTimeSeriesEntityMaxID(ctx, entityType)
	if err != nil {
		return nil, err
	}
	for _, id := range manualIDs.DumpValues() {
		if manualID, _ := strconv.ParseInt(id, 10, 64); manualID <= maxID {
			return nil, fmt.Errorf("%w: manual id %s for time series entity %q is too small, less than max id in store %d",
				ErrUpdateImmutable, id, entityType.Name, maxID)
		}
	}
	entities := make([]EntityBox, len(boxes))
	copy(entities, boxes)
	sort.Slice(entities, func(i, j int) bool {
		return entities[i].GenBlockNumber < entities[j].GenBlockNumber
	})
	for i := range entities {
		if strings.HasPrefix(entities[i].ID, "@") {
			// need reset the ID
			maxID++
			entities[i].ID = strconv.FormatInt(maxID, 10)
			for manualIDs.Contains(entities[i].ID) {
				// skip manual ID
				maxID++
				entities[i].ID = strconv.FormatInt(maxID, 10)
			}
		}
	}
	return entities, nil
}

// Commit persists all uncommitted changes up to the given block
// number. May return ErrInvalidFieldValue or ErrUpdateImmutable.
//
// Commit runs in three phases so the expensive persistent writes do not block
// concurrent entity reads and writes:
//  1. under c.mu: resolve operators and snapshot every change at or below
//     blockNumber — the changes stay in c.changes, so readers keep seeing them
//     while the writes are in flight;
//  2. without c.mu: write the snapshot to the store (SetEntities and
//     GrowthAggregation), which therefore must be safe for concurrent use;
//  3. under c.mu: prune the committed changes and advance the committed
//     watermark.
//
// Before phase 1, the store versions the pending operators read are prefetched
// without c.mu (withStoreVersions), so that the lock is held for in-memory
// work only and the handlers' SetEntity calls are not stalled behind store reads.
//
// Only one Commit runs at a time (the checkpoint controller serialises
// save/reset), and SetEntity panics on writes at or below a started commit, so
// the phase-1 snapshot cannot go stale before phase 3 prunes it.
func (c *Controller) Commit(
	ctx context.Context,
	blockNumber uint64,
	blockTime time.Time,
) (created map[string]int, updated map[string]int, err error) {
	start := time.Now()
	_, logger := log.FromContext(ctx, "blockNumber", blockNumber)

	// Phase 1: resolve operators and snapshot the changes to commit. The store versions the
	// operators read are fetched beforehand, without c.mu (see withStoreVersions).
	var batches []commitBatch
	err = c.withStoreVersions(ctx, func(prefetched map[prefetchKey]storeVersion) (wants []prefetchWant) {
		for entity := range c.changes {
			wants = append(wants, c.pendingWants(c.store.GetEntityType(entity), blockNumber, prefetched, nil)...)
		}
		return wants
	}, func(prefetched map[prefetchKey]storeVersion) error {
		// new with a value expression allocates a copy and returns its pointer — Go 1.26+ syntax
		// (this module requires go 1.26.3); the other new(...) watermark assignments in this
		// package rely on it too.
		c.committing = new(blockNumber)
		if err := c.executeAllEntityOperator(ctx, blockNumber, prefetched); err != nil {
			return err
		}
		for entity, entityChanges := range c.changes {
			batch := commitBatch{entityType: c.store.GetEntityType(entity)}
			if batch.entityType.IsTimeSeries() {
				batch.manualIDs = set.New[string]()
			}
			for id, history := range entityChanges {
				cnt := history.Count(blockNumber)
				if cnt == 0 {
					continue
				}
				for _, box := range history[:cnt] {
					batch.boxes = append(batch.boxes, box.EntityBox)
				}
				if batch.manualIDs != nil && !strings.HasPrefix(id, "@") {
					batch.manualIDs.Add(id)
				}
			}
			if len(batch.boxes) > 0 {
				batches = append(batches, batch)
			}
		}
		return nil
	})
	if err != nil {
		logger.Errorfe(err, "execute all entity operators failed")
		return
	}

	// Phase 2: write the snapshot to the persistent store.
	created, updated = make(map[string]int), make(map[string]int)
	for _, batch := range batches {
		entity := batch.entityType.Name
		entityLogger := logger.With("entity", entity)
		entityStart := time.Now()
		entities := batch.boxes
		if batch.entityType.IsTimeSeries() {
			// set timestamp and reset id for all boxes
			if entities, err = c.assignTimeSeriesIDs(ctx, batch.entityType, batch.boxes, batch.manualIDs); err != nil {
				entityLogger.Errorfe(err, "commit changes of entity failed: assign time series entity ids failed")
				return
			}
		}
		created[entity], err = c.store.SetEntities(ctx, batch.entityType, entities)
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

	// Phase 3: prune the committed changes and advance the watermark.
	c.mu.Lock()
	c.changes = c.changes.Split(blockNumber)
	c.committed = new(blockNumber)
	used := time.Since(start)
	c.timeStat.Append(&timeStatWindow{startAt: time.Now(), commit: timehist.Histogram{}.Incr(used)})
	// OnCommit must stay under c.mu: monitor callbacks are serialized by it (see Monitor),
	// and implementations like ReportMonitor read the whole mutable report here while the
	// data-plane callbacks mutate it.
	c.monitor.OnCommit(ctx, blockNumber, created, updated, used)
	c.mu.Unlock()
	logger.Debugw("committed changes", "created", created, "updated", updated, "used", used)
	return
}

// Reorg drops the changes above blockNumberGT and reorgs the store. Callers run it while no
// handler is active; the store part still runs without c.mu, like every other store round trip.
func (c *Controller) Reorg(ctx context.Context, blockNumberGT int64) error {
	c.mu.Lock()
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
		c.committing = nil
	} else {
		_ = c.changes.Split(uint64(blockNumberGT)) // discard changes above blockNumberGT, clean up empty entries
		if c.committed != nil && *c.committed > uint64(blockNumberGT) {
			c.committed = new(uint64(blockNumberGT))
		}
		if c.committing != nil && *c.committing > uint64(blockNumberGT) {
			c.committing = new(uint64(blockNumberGT))
		}
	}
	c.mu.Unlock()
	return c.store.Reorg(ctx, blockNumberGT)
}

func (c *Controller) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"store":       c.store.Snapshot(),
		"committing":  c.committing,
		"committed":   c.committed,
		"uncommitted": c.changes.Snapshot(),
		"statistics":  c.timeStat.Snapshot(),
	}
}
