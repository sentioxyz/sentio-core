package persistent

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/metric"
	"math"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type NoticeController interface {
	NoticeGet(
		ctx context.Context,
		entity string,
		id string,
		blockNumber uint64,
		inBlock bool,
		from string,
		used time.Duration)
	NoticeList(
		ctx context.Context,
		entity string,
		blockNumber uint64,
		loadRelated bool,
		from string,
		resultLen int,
		resultPersistentLen int,
		used time.Duration)
	NoticeSet(
		ctx context.Context,
		entity string,
		id string,
		blockNumber uint64,
		remove bool,
		hasOperator bool,
		used time.Duration)
	NoticeCommit(
		ctx context.Context,
		blockNumber uint64,
		created map[string]int,
		updated map[string]int,
		used time.Duration)
}

type SimpleNoticeController struct {
	UsedMetric metric.Float64Histogram
}

func (c SimpleNoticeController) recordMetric(ctx context.Context, used time.Duration, attrs ...attribute.KeyValue) {
	if c.UsedMetric == nil {
		return
	}
	c.UsedMetric.Record(ctx, float64(used.Nanoseconds())/1e6, metric.WithAttributes(attrs...))
}

func (c SimpleNoticeController) NoticeGet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	inBlock bool,
	from string,
	used time.Duration,
) {
	c.recordMetric(ctx, used,
		attribute.String("operation", "get"),
		attribute.String("entity_type", entity),
		attribute.String("from", from),
		attribute.Bool("in_block", inBlock))
}

func (c SimpleNoticeController) NoticeList(
	ctx context.Context,
	entity string,
	blockNumber uint64,
	loadRelated bool,
	from string,
	resultLen int,
	resultPersistentLen int,
	used time.Duration,
) {
	c.recordMetric(ctx, used,
		attribute.String("operation", "list"),
		attribute.String("entity_type", entity),
		attribute.String("from", from),
		attribute.Bool("load_related", loadRelated))
}

func (c SimpleNoticeController) NoticeSet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	remove bool,
	hasOperator bool,
	used time.Duration,
) {
	if remove {
		c.recordMetric(ctx, used,
			attribute.String("operation", "delete"),
			attribute.String("entity_type", entity))
	} else {
		c.recordMetric(ctx, used,
			attribute.String("operation", "upsert"),
			attribute.String("entity_type", entity),
			attribute.Bool("partly_set", hasOperator))
	}
}

func (c SimpleNoticeController) NoticeCommit(
	ctx context.Context,
	blockNumber uint64,
	created map[string]int,
	updated map[string]int,
	used time.Duration,
) {
}

type changeHistory []*EntityBox
type changeSet map[string]map[string]changeHistory // key is [entity][id]

func (cs changeSet) Count(blockNumberLE uint64) (total int) {
	for _, set := range cs {
		for _, history := range set {
			total += history.Count(blockNumberLE)
		}
	}
	return total
}

func (cs changeSet) Split(blockNumber uint64) changeSet {
	ret := make(changeSet)
	for entity, set := range cs {
		newSet := make(map[string]changeHistory)
		for id, history := range set {
			after := history.Split(blockNumber)
			if len(history) == 0 {
				delete(set, id)
			} else {
				set[id] = history
			}
			if len(after) > 0 {
				newSet[id] = after
			}
		}
		if len(set) == 0 {
			delete(cs, entity)
		}
		if len(newSet) > 0 {
			ret[entity] = newSet
		}
	}
	return ret
}

func (cs changeSet) Snapshot() any {
	st := make(map[string]any)
	for entity, changes := range cs {
		var changeCount int
		for _, history := range changes {
			changeCount += len(history)
		}
		st[entity] = map[string]any{
			"idCount":     len(changes),
			"changeCount": changeCount,
		}
	}
	return st
}

func (ch *changeHistory) Count(blockNumberLE uint64) int {
	if ch == nil {
		return 0
	}
	n := len(*ch)
	if n == 0 {
		return 0
	}
	// let (*ch)[n].GenBlockNumber == +INF
	// so  (*ch)[n].GenBlockNumber > blockNumberLE
	// sort.Search return c so
	//     (*ch)[c-1].GenBlockNumber <= blockNumberLE &&
	//     (*ch)[c].GenBlockNumber > blockNumberLE
	// so count is c
	return sort.Search(n, func(i int) bool {
		return (*ch)[i].GenBlockNumber > blockNumberLE
	})
}

func (ch *changeHistory) Latest(blockNumber uint64) *EntityBox {
	if p := ch.Count(blockNumber); p > 0 {
		return (*ch)[p-1]
	}
	return nil
}

func (ch *changeHistory) Split(blockNumber uint64) changeHistory {
	i := ch.Count(blockNumber)
	if i == len(*ch) {
		return nil
	}
	ret := make(changeHistory, len(*ch)-i)
	copy(ret, (*ch)[i:])
	*ch = (*ch)[:i]
	return ret
}

func (ch *changeHistory) Push(entityType *schema.Entity, nw *EntityBox) {
	i := ch.Count(nw.GenBlockNumber)
	if i > 0 && (*ch)[i-1].GenBlockNumber == nw.GenBlockNumber {
		// just override (*ch)[i-1]
		(*ch)[i-1].Merge(entityType, nw)
		return
	}
	// rebuild the history by [ch[:i] + nw + ch[i:]]
	if i == len(*ch) {
		*ch = append(*ch, nw)
		return
	}
	*ch = append(*ch, nil)
	for j := len(*ch) - 1; j > i; j-- {
		(*ch)[j] = (*ch)[j-1]
	}
	(*ch)[i] = nw
}

type entityTimeStat struct {
	get       map[string]timehist.Histogram // map[from]
	list      map[string]timehist.Histogram // map[from]
	set       map[string]timehist.Histogram // map[mode]
	getTotal  map[string]time.Duration      // map[from]
	listTotal map[string]time.Duration      // map[from]
	setTotal  map[string]time.Duration      // map[mode]
}

func (s entityTimeStat) Merge(a entityTimeStat) (r entityTimeStat) {
	r.get = utils.CopyMap(s.get)
	for from, hist := range a.get {
		r.get[from] = r.get[from].Add(hist)
	}
	r.list = utils.CopyMap(s.list)
	for from, hist := range a.list {
		r.list[from] = r.list[from].Add(hist)
	}
	r.set = utils.CopyMap(s.set)
	for mode, hist := range a.set {
		r.set[mode] = r.set[mode].Add(hist)
	}
	r.getTotal = utils.MapAdd(s.getTotal, a.getTotal)
	r.listTotal = utils.MapAdd(s.listTotal, a.listTotal)
	r.setTotal = utils.MapAdd(s.setTotal, a.setTotal)
	return r
}

func (s entityTimeStat) Snapshot() any {
	themeSnapshot := func(hist map[string]timehist.Histogram, total map[string]time.Duration) map[string]any {
		sn := map[string]any{}
		for k := range hist {
			ic, it := hist[k], total[k]
			count := ic.Sum()
			var avg time.Duration
			if count > 0 {
				avg = it / time.Duration(count)
			}
			sn[k] = map[string]any{
				"count": count,
				"dist":  ic.String(),
				"total": it.String(),
				"avg":   avg.String(),
			}
		}
		return sn
	}
	return map[string]any{
		"get":  themeSnapshot(s.get, s.getTotal),
		"list": themeSnapshot(s.list, s.listTotal),
		"set":  themeSnapshot(s.set, s.setTotal),
	}
}

type timeStatWindow struct {
	startAt    time.Time
	reorg      timehist.Histogram
	commit     timehist.Histogram
	entityStat map[string]entityTimeStat
}

func (w *timeStatWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *timeStatWindow) Merge(a *timeStatWindow) {
	w.reorg = w.reorg.Add(a.reorg)
	w.commit = w.commit.Add(a.commit)
	if w.entityStat == nil {
		w.entityStat = make(map[string]entityTimeStat)
	}
	for entity, stat := range a.entityStat {
		w.entityStat[entity] = w.entityStat[entity].Merge(stat)
	}
}

func (w *timeStatWindow) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt":  w.startAt.String(),
		"endAt":    endAt.String(),
		"duration": endAt.Sub(w.startAt).String(),
		"reorg":    w.reorg.Snapshot(),
		"commit":   w.commit.Snapshot(),
		"entity":   utils.MapMapNoError(w.entityStat, entityTimeStat.Snapshot),
	}
}

type Controller struct {
	mu sync.Mutex

	store     *CachedStore // persistent data
	changes   changeSet    // uncommited data
	committed *uint64

	timeStat *timewin.TimeWindowsManager[*timeStatWindow]

	noticeCtl NoticeController
}

func NewController(store *CachedStore, noticeCtl NoticeController) *Controller {
	return &Controller{
		store:     store,
		changes:   make(changeSet),
		timeStat:  timewin.NewTimeWindowsManager[*timeStatWindow](time.Minute),
		noticeCtl: noticeCtl,
	}
}

func (t *Controller) GetEntityOrInterfaceType(entity string) schema.EntityOrInterface {
	return t.store.GetEntityOrInterfaceType(entity)
}

func (t *Controller) GetEntityType(entity string) *schema.Entity {
	return t.store.GetEntityType(entity)
}

func (t *Controller) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *EntityBox, err error) {
	return t.getEntityOrInterface(ctx, typ, id, blockNumber, false)
}

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
	t.noticeCtl.NoticeGet(ctx, entityType.GetName(), id, blockNumber, inBlock, from, used)
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
		t.noticeCtl.NoticeList(ctx, entityType.GetName(), blockNumber, loadRelated, from, len(boxes), len(persistentPart), used)
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
		if pass, cke := checkFilters(filters, *box); cke != nil {
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

func (t *Controller) SetEntity(ctx context.Context, entityType *schema.Entity, box EntityBox) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if box.GenBlockChain != t.store.GetChain() {
		// unreachable
		panic(fmt.Errorf("GenBlockChain %s not match the store chain %s", box.GenBlockChain, t.store.GetChain()))
	}
	box.Entity = entityType.Name

	if err := box.CheckValue(entityType); err != nil {
		return fmt.Errorf("%w: set entity %s/%s in chain %s failed: %v",
			ErrInvalidFieldValue, entityType.Name, box.ID, box.GenBlockChain, err)
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
	t.noticeCtl.NoticeSet(ctx, entityType.GetName(), box.ID, box.GenBlockNumber, remove, hasOperator, used)
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
			maxID, err = t.store.GetTimeSeriesMaxID(ctx, entityType)
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
	t.noticeCtl.NoticeCommit(ctx, blockNumber, created, updated, used)
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
