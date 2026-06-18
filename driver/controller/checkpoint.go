package controller

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/sparsify"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/pkg/errors"
)

type TemplateInstance struct {
	TemplateID   int32
	TemplateName string
	Address      string
	Labels       string

	Removed bool

	BlockRange
}

func (t TemplateInstance) UniqID() string {
	if t.Labels == "" {
		return fmt.Sprintf("%d/%s", t.TemplateID, t.Address)
	}
	return fmt.Sprintf("%d/%s/%s", t.TemplateID, t.Address, t.Labels)
}

func (t TemplateInstance) String() string {
	firstPart := strconv.FormatInt(int64(t.TemplateID), 10)
	if t.TemplateName != "" {
		firstPart += "/" + t.TemplateName
	}
	var labelPart string
	if t.Labels != "" {
		labelPart = "::" + t.Labels
	}
	onOffPart := utils.Select(t.Removed, "OFF", "ON")
	return fmt.Sprintf("%s::%s%s::%s-%s", firstPart, t.Address, labelPart, onOffPart, t.BlockRange.String())
}

type TplWithCreated struct {
	TemplateInstance

	CreatedBlock uint64
}

func (t TplWithCreated) String() string {
	return fmt.Sprintf("%d#%s", t.CreatedBlock, t.TemplateInstance.String())
}

func CountTemplatesByID(orig map[uint64][]TemplateInstance) map[int32]int {
	sum := make(map[int32]int)
	for _, templates := range orig {
		for _, tpl := range templates {
			sum[tpl.TemplateID] += 1
		}
	}
	return sum
}

type CheckpointData struct {
}

type Checkpoint struct {
	BlockNumber     uint64
	BlockHash       string `json:"BlockHash,omitempty"`
	BlockParentHash string `json:"BlockParentHash,omitempty"`
	BlockTime       time.Time

	TotalBindings uint64

	LatestBlockNumber     uint64
	LatestBlockHash       string `json:"LatestBlockHash,omitempty"`
	LatestBlockParentHash string `json:"LatestBlockParentHash,omitempty"`
	LatestBlockTime       time.Time

	FullBlockRange BlockRange

	Data map[string]string `json:"Data,omitempty"`
}

func (c Checkpoint) GetBlockNumber() uint64 {
	return c.BlockNumber
}

func (c Checkpoint) GetBlockParentHash() string {
	return c.BlockParentHash
}

func (c Checkpoint) GetBlockHash() string {
	return c.BlockHash
}

func (c Checkpoint) GetBlockTime() time.Time {
	return c.BlockTime
}

func (c Checkpoint) InWatching() bool {
	return c.LatestBlockTime.Sub(c.BlockTime) < WatchingDelay
}

func (c Checkpoint) AllDone() bool {
	return c.FullBlockRange.EndBlock != nil && *c.FullBlockRange.EndBlock == c.BlockNumber
}

func (c Checkpoint) CurrentLastBlockNumber() uint64 {
	if c.FullBlockRange.EndBlock != nil && *c.FullBlockRange.EndBlock < c.LatestBlockNumber {
		return *c.FullBlockRange.EndBlock
	}
	return c.LatestBlockNumber
}

func (c Checkpoint) Rate() float64 {
	return float64(c.BlockNumber-c.FullBlockRange.StartBlock+1) /
		float64(c.CurrentLastBlockNumber()-c.FullBlockRange.StartBlock+1)
}

func (c Checkpoint) RateOrDelay() string {
	if c.InWatching() {
		return fmt.Sprintf("[Delay:%s]", time.Since(c.BlockTime).String())
	} else {
		return fmt.Sprintf("[%.1f%%]", c.Rate()*100)
	}
}

func (c Checkpoint) Snapshot() any {
	latest := Checkpoint{
		BlockNumber:     c.LatestBlockNumber,
		BlockHash:       c.LatestBlockHash,
		BlockParentHash: c.LatestBlockParentHash,
		BlockTime:       c.LatestBlockTime,
	}
	return map[string]any{
		"fullBlockRange": c.FullBlockRange.String(),
		"current":        GetBlockFullText(c),
		"latest":         GetBlockFullText(&latest),
		"totalBindings":  c.TotalBindings,
		"data":           utils.MapMapNoError(c.Data, utils.StringSummaryV2),
	}
}

func (c Checkpoint) String() string {
	return GetBlockFullText(c)
}

type CheckpointController interface {
	// GetLatestCheckpoint get the latest checkpoint
	GetLatestCheckpoint() *Checkpoint

	// GetSavedLatestCheckpoint get the latest checkpoint
	GetSavedLatestCheckpoint() *Checkpoint

	// GetTemplates return all templates
	// There may be template instances later than the checkpoint, which are generated when the binding data of the
	// later block is processed, but these are only temporary.
	GetTemplates() map[uint64][]TemplateInstance

	// Ready Clean up invalid time series data and entity data by checkpoint, executed at the start of a round
	Ready(ctx context.Context, agentStat map[string]int) *ExternalError

	// CleanCheckpoint Delete checkpoints and templates greater than or equal to blockNumberGE, executed after reorg
	CleanCheckpoint(ctx context.Context, curBlockNumber, blockNumberGE uint64) *ExternalError

	// MakeCheckpoint Try to construct a checkpoint at the blockData block, indicating that all bindings of this block
	// have been processed.
	MakeCheckpoint(
		ctx context.Context,
		blockData BlockDataSummary,
		progressBar ProgressBar,
	) (hasNewTemplate bool, err *ExternalError)

	// Save Actually try to save checkpoints
	Save(ctx context.Context, saveAll bool) *ExternalError

	// KeepSave Continuously try to save checkpoints,
	// if all checkpoint was made, allMade will be closed, and KeepSave will return nil after all Checkpoint was saved.
	KeepSave(ctx context.Context, allMade chan struct{}) error

	// SaveError An error occurred during processing, save the error information
	SaveError(ctx context.Context, err *ExternalError) error

	// NewTemplateInstance Declare a new template instance in the specified task
	NewTemplateInstance(ctx context.Context, task Task, templates []TemplateInstance) *ExternalError

	// InsertTimeSeriesData Insert time series data into the specified block
	InsertTimeSeriesData(blockNumber uint64, taskIndex TaskIndex, data []timeseries.Dataset)

	// InsertWebhookData Insert webhook data in the specified block
	InsertWebhookData(blockNumber uint64, taskIndex TaskIndex, messages []WebhookMessage)

	// GetEntityOrInterfaceType Get entity or interface declaration by name
	GetEntityOrInterfaceType(entity string) schema.EntityOrInterface
	// GetEntityType Get entity declaration by name
	GetEntityType(entity string) *schema.Entity
	// GetEntity get entity
	GetEntity(
		ctx context.Context,
		typ schema.EntityOrInterface,
		id string,
		blockNumber uint64,
	) (box *persistent.EntityBox, err *ExternalError)
	// GetEntityInBlock get entity
	GetEntityInBlock(
		ctx context.Context,
		typ schema.EntityOrInterface,
		id string,
		blockNumber uint64,
	) (box *persistent.EntityBox, err *ExternalError)
	// ListEntity list entity
	ListEntity(
		ctx context.Context,
		entityType *schema.Entity,
		filters []persistent.EntityFilter,
		cursor string,
		limit int,
		blockNumber uint64,
	) (boxes []*persistent.EntityBox, next *string, err *ExternalError)
	// ListRelated list related entity to <entityType>/<id> with the relationship definition in <entityType>.<fieldName>
	ListRelated(
		ctx context.Context,
		entityType *schema.Entity,
		id string,
		fieldName string,
		blockNumber uint64,
	) ([]*persistent.EntityBox, schema.EntityOrInterface, *ExternalError)

	// SetEntity set or delete entity
	SetEntity(ctx context.Context, entityType *schema.Entity, box persistent.UncommittedEntityBox) *ExternalError

	Snapshot() map[string]any
}

type CheckpointStore interface {
	Load(ctx context.Context) ([]Checkpoint, map[uint64][]TemplateInstance, error)
	Save(
		ctx context.Context,
		checkpoints []Checkpoint,
		templates map[uint64][]TemplateInstance,
		agentStat map[string]int,
	) error
	SaveError(ctx context.Context, err *ExternalError) error
}

var ErrCheckpointsTooBig = errors.New("checkpoints too big")

var _ CheckpointController = &checkpointController{}

type checkpointController struct {
	chainID                string
	saveDelay              time.Duration
	saveInterval           time.Duration
	maxKeepCheckpointCount uint64

	checkpointStore              CheckpointStore
	checkpoints                  []Checkpoint
	savedCheckpoints             int // checkpoints[:savedCheckpoints] are all saved in checkpointStore
	checkpointSparsifyMultiplier uint64

	// lastSavedAt and lastSavedCheckpoint are used to estimate Watching needed,
	lastSavedAt         time.Time
	lastSavedCheckpoint *Checkpoint
	actuallyLastSavedAt time.Time

	templates        map[uint64][]TemplateInstance // key is blockNumber
	unsavedTemplates map[uint64][]TemplateInstance // key is blockNumber

	agentStat map[string]int

	quotaService QuotaService

	commitCtxBuilder func(ctx context.Context, chainID string, cur Checkpoint) context.Context

	timeSeriesCtrl TimeSeriesController
	entityCtrl     EntityController
	webhookCtrl    WebhookController

	stopped                bool
	printProcessedExecutor *timer.MinimumIntervalExecutor

	stat *timewin.TimeWindowsManager[*checkpointStatWindow]

	mu sync.Mutex
}

func NewCheckpointController(
	ctx context.Context,
	chainID string,
	saveDelay time.Duration,
	saveInterval time.Duration,
	maxKeepCheckpointCount uint64,
	checkpointStore CheckpointStore,
	quotaService QuotaService,
	timeSeriesCtrl TimeSeriesController,
	entityCtrl EntityController,
	webhookCtrl WebhookController,
	commitCtxBuilder func(ctx context.Context, chainID string, cur Checkpoint) context.Context,
) (CheckpointController, error) {
	c := checkpointController{
		chainID:                chainID,
		saveDelay:              saveDelay,
		saveInterval:           saveInterval,
		maxKeepCheckpointCount: maxKeepCheckpointCount,
		checkpointStore:        checkpointStore,
		quotaService:           quotaService,
		commitCtxBuilder:       commitCtxBuilder,
		timeSeriesCtrl:         timeSeriesCtrl,
		entityCtrl:             entityCtrl,
		webhookCtrl:            webhookCtrl,
		printProcessedExecutor: timer.NewMinimumIntervalExecutor(PrintProcessedInterval),
		stat:                   timewin.NewTimeWindowsManager[*checkpointStatWindow](time.Minute),
	}
	checkpoints, templates, err := checkpointStore.Load(ctx)
	if err != nil {
		return nil, err
	}
	c.checkpoints = checkpoints
	c.savedCheckpoints = len(checkpoints)
	if templates != nil {
		c.templates = templates
	} else {
		c.templates = make(map[uint64][]TemplateInstance)
	}
	c.unsavedTemplates = make(map[uint64][]TemplateInstance)
	return &c, nil
}

func (c *checkpointController) findCheckpoint(blockNumberLE uint64) int {
	var p int
	for p < len(c.checkpoints) && c.checkpoints[p].BlockNumber <= blockNumberLE {
		p++
	}
	return p - 1
}

func (c *checkpointController) getLatestCheckpoint(saved bool) *Checkpoint {
	cc := len(c.checkpoints)
	if saved {
		cc = c.savedCheckpoints
	}
	if cc == 0 {
		return nil
	}
	return &c.checkpoints[cc-1]
}

func (c *checkpointController) GetLatestCheckpoint() *Checkpoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLatestCheckpoint(false)
}

func (c *checkpointController) GetSavedLatestCheckpoint() *Checkpoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getLatestCheckpoint(true)
}

func (c *checkpointController) getTemplates(savedOnly bool) map[uint64][]TemplateInstance {
	if savedOnly {
		if c.savedCheckpoints == 0 {
			return make(map[uint64][]TemplateInstance)
		}
		savedBlockNumber := c.checkpoints[c.savedCheckpoints-1].BlockNumber
		return utils.FilterMap(c.templates, func(u uint64) bool {
			return u <= savedBlockNumber
		})
	}
	return c.templates
}

func (c *checkpointController) GetTemplates() map[uint64][]TemplateInstance {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getTemplates(false)
}

func (c *checkpointController) Ready(ctx context.Context, agentStat map[string]int) *ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agentStat = agentStat
	var checkpoint *Checkpoint
	if len(c.checkpoints) > 0 {
		checkpoint = &c.checkpoints[len(c.checkpoints)-1]
	}
	_, logger := log.FromContext(ctx, "checkpoint", utils.NullOrToString(checkpoint))
	logger.Debug("will clean data")
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if extErr := c.entityCtrl.Reset(gctx, checkpoint); extErr != nil {
			logger.Errorfe(extErr, "clean data in entity controller failed")
			return extErr
		}
		return nil
	})
	g.Go(func() error {
		if extErr := c.timeSeriesCtrl.Reset(gctx, checkpoint); extErr != nil {
			logger.Errorfe(extErr, "clean data in time series controller failed")
			return extErr
		}
		return nil
	})
	g.Go(func() error {
		if extErr := c.webhookCtrl.Reset(gctx, checkpoint); extErr != nil {
			logger.Errorfe(extErr, "clean data in webhook controller failed")
			return extErr
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		var extErr *ExternalError
		if errors.As(err, &extErr) {
			return extErr
		}
		// unreachable
		return NewExternalError(ErrCodeSystem, err)
	}
	c.stopped = false
	return nil
}

func (c *checkpointController) CleanCheckpoint(
	ctx context.Context,
	curBlockNumber,
	blockNumberGE uint64,
) *ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, logger := log.FromContext(ctx)
	logger = logger.UserVisible()
	detectedMsg := fmt.Sprintf("Reorg detected when processing block %d, all blocks from block %d are invalid",
		curBlockNumber, blockNumberGE)
	c.stopped = true
	var cc int
	for cc < len(c.checkpoints) && c.checkpoints[cc].BlockNumber < blockNumberGE {
		cc++
	}
	// c.checkpoints[cc:] need to delete
	if cc > 0 {
		// we have at least one checkpoint
		tcc := len(c.checkpoints)
		c.checkpoints = c.checkpoints[:cc]
		backMsg := fmt.Sprintf("progress will back to %s", c.checkpoints[cc-1].String())
		if cc < c.savedCheckpoints {
			logger.Warnf("%s, will remove all %d unsaved checkpoints and %d saved checkpoints in checkpoint store, %s",
				detectedMsg, tcc-c.savedCheckpoints, c.savedCheckpoints-cc, backMsg)
			// saved checkpoint should rollback
			if err := c.checkpointStore.Save(ctx, c.checkpoints, c.getTemplates(false), c.agentStat); err != nil {
				return NewExternalError(ErrCodeSaveCheckpointFailed, err)
			}
			c.savedCheckpoints = len(c.checkpoints)
		} else if cc == c.savedCheckpoints {
			logger.Warnf("%s, will remove all %d unsaved checkpoints, %s", detectedMsg, tcc-cc, backMsg)
		} else {
			logger.Warnf("%s, will remove %d unsaved checkpoints, %s", detectedMsg, tcc-cc, backMsg)
		}
		deleteFilter := func(bn uint64) bool {
			return bn > c.checkpoints[cc-1].BlockNumber
		}
		utils.MapDelete(c.templates, deleteFilter)
		utils.MapDelete(c.unsavedTemplates, deleteFilter)
	} else {
		logger.Warnf("%s, will remove all checkpoints", detectedMsg)
		// all checkpoints are invalid, it means all data are useless
		if err := c.checkpointStore.Save(ctx, nil, nil, c.agentStat); err != nil {
			return NewExternalError(ErrCodeSaveCheckpointFailed, err)
		}
		c.checkpoints = nil
		c.savedCheckpoints = 0
		c.templates = make(map[uint64][]TemplateInstance)
		c.unsavedTemplates = make(map[uint64][]TemplateInstance)
	}
	return nil
}

func (c *checkpointController) MakeCheckpoint(
	ctx context.Context,
	blockData BlockDataSummary,
	progressBar ProgressBar,
) (templatesChanged bool, extErr *ExternalError) {
	_, logger := log.FromContext(ctx,
		"current", GetBlockSummary(blockData),
		"latest", GetBlockSummary(progressBar.LatestBlock))

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		logger.Warnf("try to make checkpoint after checkpoint controller stopped")
		return
	}

	ck := Checkpoint{
		BlockNumber:           blockData.GetBlockNumber(),
		BlockHash:             blockData.GetBlockHash(),
		BlockParentHash:       blockData.GetBlockParentHash(),
		BlockTime:             blockData.GetBlockTime(),
		TotalBindings:         uint64(blockData.TaskCount),
		LatestBlockNumber:     progressBar.LatestBlock.GetBlockNumber(),
		LatestBlockHash:       progressBar.LatestBlock.GetBlockHash(),
		LatestBlockParentHash: progressBar.LatestBlock.GetBlockParentHash(),
		LatestBlockTime:       progressBar.LatestBlock.GetBlockTime(),
		FullBlockRange:        progressBar.FullBlockRange,
		Data:                  blockData.CheckpointData,
	}
	processedMsg := fmt.Sprintf("Processed %s[%d/%s/%d] with %d bindings",
		ck.RateOrDelay(),
		ck.FullBlockRange.StartBlock,
		GetBlockSummary(blockData),
		ck.CurrentLastBlockNumber(),
		ck.TotalBindings)

	var templates []TemplateInstance
	if templates, templatesChanged = c.unsavedTemplates[blockData.GetBlockNumber()]; templatesChanged {
		// If there are new template instances, temporarily confirm these template instances,
		// then return ErrInternalHasNewTemplate and wait for the next time.
		c.templates[blockData.GetBlockNumber()] = append(c.templates[blockData.GetBlockNumber()], templates...)
		// New template instances that come later need to be ignored because the current block may have new data
		// at the beginning, and the generation process of those new template instances may change as a result.
		c.unsavedTemplates = make(map[uint64][]TemplateInstance)
		// No more templates will be accepted in this run.
		c.stopped = true
		var minStartBlock uint64 = math.MaxUint64
		for _, tpl := range templates {
			minStartBlock = min(minStartBlock, tpl.StartBlock)
		}
		logger = logger.UserVisible()
		processedMsg += fmt.Sprintf(" and %d new templates [%s]",
			len(templates), strings.Join(utils.MapSliceNoError(templates, TemplateInstance.String), ","))
		if minStartBlock == ck.BlockNumber {
			logger.Warn(processedMsg + ", but this block will be re-processed because has new template from this block")
		} else {
			// The checkpoint of the current block is still acceptable, but because a new template is created later,
			// the BlockBuilder need to be reset and restarted.
			c.checkpoints = append(c.checkpoints, ck)
			logger.Info(processedMsg)
		}
		return
	}
	c.checkpoints = append(c.checkpoints, ck)

	printProcessed := func() {
		logger.UserVisible().Info(processedMsg)
	}
	if ck.InWatching() || ck.TotalBindings > 0 {
		printProcessed()
	} else {
		c.printProcessedExecutor.ExecSimple(printProcessed)
	}

	if c.saveDelay == 0 && ck.InWatching() {
		// realtime mode
		extErr = c.save(ctx, false, true)
	} else if uint64(len(c.checkpoints)) >= c.maxKeepCheckpointCount {
		logger.Info("will try to save checkpoint because there are too many checkpoints")
		extErr = c.save(ctx, false, false)
	} else if c.webhookCtrl.CachedTooMuch(ck.BlockNumber) {
		logger.Info("will try to save checkpoint because there are too many uncommitted webhook message")
		extErr = c.save(ctx, false, false)
	} else if c.timeSeriesCtrl.CachedTooMuch(ck.BlockNumber) {
		logger.Info("will try to save checkpoint because there are too many uncommitted time series data")
		extErr = c.save(ctx, false, false)
	} else if c.entityCtrl.CachedTooMuch(ck.BlockNumber) {
		logger.Info("will try to save checkpoint because there are too many uncommitted entity changes")
		extErr = c.save(ctx, false, false)
	}
	return
}

func (c *checkpointController) Save(ctx context.Context, saveAll bool) *ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.save(ctx, saveAll, true)
}

func (c *checkpointController) estimateWatchingNeed(cur *Checkpoint) string {
	if cur == nil {
		return "<calculating...>"
	}
	if cur.InWatching() {
		return "0"
	}
	passed := time.Since(c.lastSavedAt)
	if c.lastSavedCheckpoint == nil || passed < time.Minute {
		return "<calculating...>"
	}
	last := *c.lastSavedCheckpoint
	growthSpeed := float64(cur.CurrentLastBlockNumber()-last.CurrentLastBlockNumber()) / passed.Seconds()
	processSpeed := float64(cur.BlockNumber-last.BlockNumber) / passed.Seconds()
	if processSpeed <= growthSpeed {
		return "INF"
	}
	eta := time.Second * time.Duration(float64(cur.CurrentLastBlockNumber()-cur.BlockNumber)/(processSpeed-growthSpeed))
	return eta.String()
}

func (c *checkpointController) save(ctx context.Context, saveAll bool, checkInterval bool) (extErr *ExternalError) {
	_, logger := log.FromContext(ctx)

	if c.stopped {
		return
	}

	if !saveAll && checkInterval && time.Since(c.actuallyLastSavedAt) < c.saveInterval/2 {
		// This is not the final save, and not long enough since the last save, just ignore
		return
	}

	tm := timer.NewTimer()
	startTm := tm.Start("ALL")

	// Find the checkpoints should be saved
	var cc int
	if saveAll {
		cc = len(c.checkpoints)
	} else {
		cc = c.savedCheckpoints
		for cc < len(c.checkpoints) && time.Since(c.checkpoints[cc].BlockTime) > c.saveDelay {
			cc++
		}
	}
	if cc == 0 || cc == c.savedCheckpoints {
		// No new checkpoints or all new checkpoints are too close to the current time, so there is nothing to save.
		return
	}

	cur := c.checkpoints[cc-1]
	pre := cur.FullBlockRange.StartBlock
	if c.savedCheckpoints > 0 {
		pre = c.checkpoints[c.savedCheckpoints-1].BlockNumber + 1
	}
	var totalBindings uint64
	for i := c.savedCheckpoints; i < cc; i++ {
		totalBindings += c.checkpoints[i].TotalBindings
	}
	if cc > c.savedCheckpoints { // if c.allDone() is true, cc may be equal to c.savedCheckpoints
		// checkpoints[:cc] should be saved.
		// However, considering that checkpoint data is only generated when each data store commits,
		// so only c.checkpoints[cc-1] should to save,
		// so unsaved checkpoints in c.checkpoints[c.savedCheckpoints:cc-1] should be removed
		c.checkpoints = utils.RemoveSubSeq(c.checkpoints, c.savedCheckpoints, cc-1-c.savedCheckpoints)
		cc = c.savedCheckpoints + 1
	}

	win := checkpointStatWindow{startAt: time.Now(), count: 1}
	defer func() {
		if extErr == nil {
			win.totalBinding = totalBindings
		} else {
			win.failedCount = 1
		}
		c.stat.Append(&win)
	}()

	checkOverQuotaTm := tm.Start("O")
	over, err := c.quotaService.CheckOverQuota(ctx)
	win.checkOverQuotaUsed = checkOverQuotaTm.End()
	if err != nil {
		return NewExternalError(ErrCodeQuotaServiceError, err)
	} else if over != nil {
		logger.UserVisible().Errorf("Over quota: %s", over.Detail)
		return NewExternalError(ErrCodeOverQuota, errors.Errorf("over quota: %s", over.Msg))
	}

	var usage Usage

	defer func() {
		startTm.End()
		logger = logger.UserVisible().With("used", tm.ReportDistribution("ALL", "*"))
		progress := fmt.Sprintf("%s[%d/%d-%d/%d] with %d bindings",
			cur.RateOrDelay(),
			cur.FullBlockRange.StartBlock,
			pre,
			cur.BlockNumber,
			cur.CurrentLastBlockNumber(),
			totalBindings)
		if extErr != nil {
			// commits below may be completed partly, so all unsaved checkpoint should be clean, and need to re-init
			c.checkpoints = c.checkpoints[:c.savedCheckpoints]
			c.templates = c.getTemplates(true)
			c.unsavedTemplates = make(map[uint64][]TemplateInstance)
			c.stopped = true
			if extErr.IsUserError() {
				logger.Warnf("Save %s failed: %s", progress, extErr.Error())
			} else {
				logger.Warnf("Save %s failed, progress will back to %s", progress, c.getLatestCheckpoint(true))
			}
		} else {
			// succeed
			logger.Infof("Saved %s and %s", progress, usage.String())
			c.actuallyLastSavedAt = time.Now()
			if c.lastSavedCheckpoint == nil || time.Since(c.lastSavedAt) > time.Minute*5 {
				c.lastSavedAt, c.lastSavedCheckpoint = c.actuallyLastSavedAt, &cur
			}
		}
	}()

	if c.commitCtxBuilder != nil {
		ctx = c.commitCtxBuilder(ctx, c.chainID, cur)
	}

	// Save the time series data up to cur.BlockNumber
	commitTimeSeriesTm := tm.Start("T")
	usage.TimeSeries, extErr = c.timeSeriesCtrl.Commit(ctx, cur.BlockNumber, cur.BlockTime)
	win.commitTimeSeriesUsed = commitTimeSeriesTm.End()
	if extErr != nil {
		return extErr
	}

	// Save entity data up to cur.BlockNumber
	commitEntityTm := tm.Start("E")
	usage.EntityCreated, usage.EntityUpdated, extErr = c.entityCtrl.Commit(ctx, cur.BlockNumber, cur.BlockTime)
	win.commitEntityUsed = commitEntityTm.End()
	if extErr != nil {
		return extErr
	}

	// Save webhook data up to cur.BlockNumber
	commitWebhookTm := tm.Start("W")
	usage.Export, extErr = c.webhookCtrl.Commit(ctx, cur.BlockNumber, cur.BlockTime)
	win.commitWebhookUsed = commitWebhookTm.End()
	if extErr != nil {
		return extErr
	}

	// Save usage
	saveUsageTm := tm.Start("U")
	err = c.quotaService.SaveUsage(ctx, usage, cur.InWatching())
	win.saveUsageUsed = saveUsageTm.End()
	if err != nil {
		return NewExternalError(ErrCodeQuotaServiceError, err)
	}

	// Sparse these cc checkpoints and save them
	for {
		multiplier := max(c.checkpointSparsifyMultiplier, 2)
		saveCheckpointTm := tm.Start("S")
		remove := sparsify.RemoveWithMultiplier(c.checkpoints[:cc], Checkpoint.GetBlockNumber, multiplier/2, multiplier)
		c.checkpoints = utils.RemoveByIndex(c.checkpoints, remove)
		cc -= len(remove)
		c.savedCheckpoints = cc
		err = c.checkpointStore.Save(ctx, c.checkpoints[:c.savedCheckpoints], c.getTemplates(true), c.agentStat)
		win.saveCheckpointUsed = saveCheckpointTm.End()
		if err != nil {
			if errors.Is(err, ErrCheckpointsTooBig) {
				logger.With("sparsifyMultiplier", multiplier).Warnfe(err, "checkpoints too big, will reduce it")
				c.checkpointSparsifyMultiplier = multiplier * 2
				continue
			}
			return NewExternalError(ErrCodeSaveCheckpointFailed, err)
		}
		return nil
	}
}

func (c *checkpointController) KeepSave(ctx context.Context, allMade chan struct{}) error {
	_, logger := log.FromContext(ctx)
	logger.Info("keep save checkpoint started")
	defer func() {
		logger.Info("keep save checkpoint finished")
	}()
	ticker := time.NewTicker(c.saveInterval)
	defer ticker.Stop()
	for {
		if extErr := c.Save(ctx, false); extErr != nil {
			return extErr
		}
		select {
		case <-allMade:
			// last save
			if extErr := c.Save(ctx, true); extErr != nil {
				return extErr
			}
			return nil
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *checkpointController) SaveError(ctx context.Context, err *ExternalError) error {
	return c.checkpointStore.SaveError(ctx, err)
}

func (c *checkpointController) NewTemplateInstance(
	ctx context.Context,
	task Task,
	templates []TemplateInstance,
) *ExternalError {
	blockNumber := task.GetBlockNumber()
	c.mu.Lock()
	defer c.mu.Unlock()
	_, logger := log.FromContext(ctx)
	if c.stopped {
		logger.Warnw("try to make new template instance after checkpoint controller stopped",
			"blockNumber", blockNumber,
			"templates", utils.MapSliceNoError(templates, TemplateInstance.String))
		return nil
	}
	exists := make(map[string][]TplWithCreated) // key is <TemplateID>/<Address>/<Labels>
	for _, bn := range utils.GetOrderedMapKeys(c.templates) {
		for _, tpl := range c.templates[bn] {
			exists[tpl.UniqID()] = append(exists[tpl.UniqID()], TplWithCreated{TemplateInstance: tpl, CreatedBlock: bn})
		}
	}
	for _, bn := range utils.GetOrderedMapKeys(c.unsavedTemplates) {
		if bn > blockNumber {
			break
		}
		for _, tpl := range c.unsavedTemplates[bn] {
			exists[tpl.UniqID()] = append(exists[tpl.UniqID()], TplWithCreated{TemplateInstance: tpl, CreatedBlock: bn})
		}
	}
	for _, tpl := range templates {
		if tpl.StartBlock < blockNumber && !tpl.Removed {
			logger.Warnf("start block of the template instance %s less then the block %d created it, will be reset to %d",
				tpl, blockNumber, blockNumber)
			tpl.StartBlock = blockNumber
		}
		if exist := exists[tpl.UniqID()]; len(exist) > 0 {
			existText := strings.Join(utils.MapSliceNoError(exist, TplWithCreated.String), ",")
			on := EmptyBlockRangeSet
			for _, ex := range exist {
				if ex.Removed {
					on = on.Remove(ex.BlockRange)
				} else {
					on = on.Union(ex.BlockRange)
				}
			}
			on = on.Intersection(BlockRange{StartBlock: blockNumber})
			intersection := on.Intersection(tpl.BlockRange)
			if tpl.Removed && intersection.IsEmpty() || !tpl.Removed && intersection.Include(tpl.BlockRange) {
				logger.Warnf("try to create new template instance %s at %s, but already created %s, will be ignored",
					tpl, task.Summary(), existText)
				continue
			}
			if tpl.Removed {
				on = on.Remove(tpl.BlockRange)
			} else {
				on = on.Union(tpl.BlockRange)
			}
			if len(on.Holes) > 0 {
				return NewExternalError(ErrCodeCreateTemplateFailed, errors.Errorf(
					"new template instance %s at %s is invalid, already created %s, "+
						"after created the enable block range will have hole", tpl, task.Summary(), existText))
			}
			logger.Infof("has new template %s at %s, and already created %s", tpl, task.Summary(), existText)
		} else {
			logger.Infof("has new template %s at %s", tpl, task.Summary())
		}
		c.unsavedTemplates[blockNumber] = append(c.unsavedTemplates[blockNumber], tpl)
	}
	return nil
}

func (c *checkpointController) InsertTimeSeriesData(
	blockNumber uint64,
	taskIndex TaskIndex,
	data []timeseries.Dataset,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeSeriesCtrl.Insert(blockNumber, taskIndex, data)
}

func (c *checkpointController) InsertWebhookData(blockNumber uint64, taskIndex TaskIndex, messages []WebhookMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.webhookCtrl.Insert(blockNumber, taskIndex, messages)
}

func (c *checkpointController) GetEntityOrInterfaceType(entity string) schema.EntityOrInterface {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.GetEntityOrInterfaceType(entity)
}

func (c *checkpointController) GetEntityType(entity string) *schema.Entity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.GetEntityType(entity)
}

func (c *checkpointController) GetEntity(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *persistent.EntityBox, err *ExternalError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.GetEntity(ctx, typ, id, blockNumber)
}

func (c *checkpointController) GetEntityInBlock(
	ctx context.Context,
	typ schema.EntityOrInterface,
	id string,
	blockNumber uint64,
) (box *persistent.EntityBox, err *ExternalError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.GetEntityInBlock(ctx, typ, id, blockNumber)
}

func (c *checkpointController) ListEntity(
	ctx context.Context,
	entityType *schema.Entity,
	filters []persistent.EntityFilter,
	cursor string,
	limit int,
	blockNumber uint64,
) (boxes []*persistent.EntityBox, next *string, err *ExternalError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.ListEntity(ctx, entityType, filters, cursor, limit, blockNumber)
}

func (c *checkpointController) ListRelated(
	ctx context.Context,
	entityType *schema.Entity,
	id string,
	fieldName string,
	blockNumber uint64,
) ([]*persistent.EntityBox, schema.EntityOrInterface, *ExternalError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.ListRelated(ctx, entityType, id, fieldName, blockNumber)
}

func (c *checkpointController) SetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	box persistent.UncommittedEntityBox,
) *ExternalError {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entityCtrl.SetEntity(ctx, entityType, box)
}

func (c *checkpointController) unsavedSnapshot() any {
	count := len(c.checkpoints) - c.savedCheckpoints
	sn := map[string]any{
		"checkpointCount": count,
	}
	if count > 0 {
		var tb uint64
		for i := c.savedCheckpoints; i < len(c.checkpoints); i++ {
			tb += c.checkpoints[i].TotalBindings
		}
		sn["totalBindings"] = tb
		sn["latestCheckpoint"] = c.checkpoints[len(c.checkpoints)-1].Snapshot()
	}
	return sn
}

func (c *checkpointController) Snapshot() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	lastSaved := c.getLatestCheckpoint(true)
	watching := lastSaved != nil && lastSaved.InWatching()
	return map[string]any{
		"chainID": c.chainID,
		"config": map[string]any{
			"saveDelay":              c.saveDelay.String(),
			"saveInterval":           c.saveInterval.String(),
			"maxKeepCheckpointCount": c.maxKeepCheckpointCount,
			"watchingDelay":          WatchingDelay.String(),
		},
		"checkpointCount": len(c.checkpoints),
		"stores": map[string]any{
			"timeSeries": c.timeSeriesCtrl.Snapshot(),
			"entity":     c.entityCtrl.Snapshot(),
			"webhook":    c.webhookCtrl.Snapshot(),
		},
		"saved": map[string]any{
			"checkpoints":                  utils.MapSliceNoError(c.checkpoints[:c.savedCheckpoints], Checkpoint.Snapshot),
			"inWatching":                   watching,
			"estimateWatchingNeed":         c.estimateWatchingNeed(lastSaved),
			"checkpointSparsifyMultiplier": c.checkpointSparsifyMultiplier,
		},
		"unsaved":    c.unsavedSnapshot(),
		"templates":  c.getTemplates(false),
		"statistics": c.stat.Snapshot(),
	}
}
