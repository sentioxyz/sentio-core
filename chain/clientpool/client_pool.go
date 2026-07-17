package clientpool

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/pool"
	"sentioxyz/sentio-core/common/queue"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
)

type entryStatus[CLIENT pool.Status] struct {
	Client     CLIENT
	ClientName string

	Initialized            bool
	InitializeFailedTimes  int
	InitializeFailedReason string
	InitializeFailedAt     time.Time
	InitializedAt          time.Time
	LatestBlock            Block
}

func (es entryStatus[CLIENT]) Snapshot() any {
	sn := map[string]any{
		"initialized": es.Initialized,
	}
	if !reflect.ValueOf(es.Client).IsNil() {
		sn["client"] = es.Client.Snapshot()
	}
	if es.InitializeFailedTimes > 0 {
		sn["initializeFailedTimes"] = es.InitializeFailedTimes
		sn["initializeFailedReason"] = es.InitializeFailedReason
		sn["initializeFailedÅt"] = es.InitializeFailedAt.String()
	}
	if es.Initialized {
		sn["initializedAt"] = es.InitializedAt.String()
		sn["latestBlock"] = es.LatestBlock.String()
	}
	return sn
}

type poolStatus struct {
	Ready         bool
	LatestBlock   Block
	LatestQueue   queue.Queue[Block]
	BlockInterval time.Duration
}

func (ps poolStatus) Snapshot() any {
	if !ps.Ready {
		return map[string]any{"ready": ps.Ready}
	}
	return map[string]any{
		"ready":         ps.Ready,
		"latestBlock":   ps.LatestBlock.String(),
		"blockInterval": ps.BlockInterval.String(),
	}
}

type ban struct {
	reason error
	from   time.Time
	dur    time.Duration
}

func (b *ban) enable(now time.Time) bool {
	return !now.Before(b.from) && now.Sub(b.from) < b.dur
}

func (b *ban) extend(c BanConfig, now time.Time, reason error) {
	passed := now.Sub(b.from)
	b.dur = passed + min(time.Duration(float64(passed)*c.ExtendRate), c.ExtendMax)
	b.reason = reason
}

func (b *ban) String() string {
	return fmt.Sprintf("from %s to %s because %v", b.from, b.from.Add(b.dur), b.reason)
}

type active struct {
	theme string
	time  time.Time
}

func (a active) String() string {
	return fmt.Sprintf("%s at %s", a.theme, a.time)
}

type tagInfo struct {
	startAt          time.Time
	lastActiveAt     time.Time
	lastActiveReason error
}

type entryExtra struct {
	tags map[string]tagInfo
	*ban
	*active
}

func (e *entryExtra) hasTag(tag string, validFrom time.Time) bool {
	info, has := e.tags[tag]
	if has && info.lastActiveAt.Before(validFrom) {
		has = false
		delete(e.tags, tag)
	}
	return has
}

const (
	consumerWaiting   = "waiting"
	consumerExecuting = "executing"
)

type consumer struct {
	theme            string
	blackList        map[string]error
	enterAt          time.Time
	executed         int
	executedDuration time.Duration
	executedLast     Result
	waitIndex        int
	useClient        string
	doing            string
	waitingPSI       uint64
	doingFrom        time.Time
}

func (c consumer) Snapshot() any {
	sn := map[string]any{
		"theme":              c.theme,
		"enterAt":            c.enterAt.String(),
		"enterDuration":      time.Since(c.enterAt).String(),
		"executed":           c.executed,
		"executedDuration":   c.executedDuration.String(),
		"executedLastResult": c.executedLast.String(),
		"waitIndex":          c.waitIndex,
		"doing":              c.doing,
		"blackList": utils.MapMapNoError(c.blackList, func(err error) string {
			return fmt.Sprintf("%v", err)
		}),
	}
	switch c.doing {
	case consumerWaiting:
		sn["executedLastClient"] = c.useClient
		sn["waitingPoolStatusIndex"] = c.waitingPSI
		sn["waitingDuration"] = time.Since(c.doingFrom).String()
	case consumerExecuting:
		sn["executingUseClient"] = c.useClient
		sn["executingDuration"] = time.Since(c.doingFrom).String()
	}
	return sn
}

type Notifier[CONFIG EntryConfig[CONFIG]] interface {
	CurrentPriority(currentPriority int)
	EntryLatestBlock(c ClientConfig[CONFIG], latest Block)
	EntryUsed(c ClientConfig[CONFIG], what string, dur time.Duration, hasErr bool)
}

type ConfigModifier[CONFIG any] func(CONFIG) CONFIG

// ClientPool
// member methods that begin with '_' are called only within the critical section constructed by p.mu.
type ClientPool[CONFIG EntryConfig[CONFIG], CLIENT Client] struct {
	pool *pool.Pool[ClientConfig[CONFIG], entryStatus[CLIENT], poolStatus]

	clientBuilder func(CONFIG, UsedNotifier) CLIENT
	notifier      Notifier[CONFIG]
	confModifiers []ConfigModifier[CONFIG]

	statDowngrade *timewin.TimeWindowsManager[*downgradeStatWindow]
	statEntryUsed *timewin.TimeWindowsManager[*usedStatWindow]

	// protect all properties below.
	// Lock ordering: never call pool methods while holding mu.
	// pool.mu can call back into ClientPool (e.g. via poolStatusBuilder, Enable's status goroutine),
	// which will attempt to acquire mu — so acquiring mu and then calling any pool method that
	// acquires pool.mu creates a deadlock cycle. Always release mu before calling any pool methods.
	mu sync.Mutex

	config           PoolConfig[CONFIG]
	configUpdateAt   time.Time
	configVersion    int
	configEntries    map[string]ClientConfig[CONFIG] // dict of config.ClientConfigs, key is CONFIG.GetName()
	configPriorities []uint32

	priorityCursor int
	entryExtra     map[string]*entryExtra

	consumer        map[uint64]consumer
	consumerCounter uint64
}

func NewClientPool[CONFIG EntryConfig[CONFIG], CLIENT Client](
	name string,
	clientBuilder func(CONFIG, UsedNotifier) CLIENT,
	notifier Notifier[CONFIG],
	confModifiers ...ConfigModifier[CONFIG],
) *ClientPool[CONFIG, CLIENT] {
	p := &ClientPool[CONFIG, CLIENT]{
		clientBuilder: clientBuilder,
		notifier:      notifier,
		confModifiers: confModifiers,
		entryExtra:    make(map[string]*entryExtra),
		consumer:      make(map[uint64]consumer),
		statDowngrade: timewin.NewTimeWindowsManager[*downgradeStatWindow](time.Minute),
		statEntryUsed: timewin.NewTimeWindowsManager[*usedStatWindow](time.Minute),
	}
	p.pool = pool.NewPool[ClientConfig[CONFIG], entryStatus[CLIENT], poolStatus](
		name,
		p.poolStatusBuilder,
		p.entryStatusRefresher,
	)
	return p
}

func (p *ClientPool[CONFIG, CLIENT]) poolStatusBuilder(
	entries map[string]pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]],
	pre poolStatus,
	psi uint64,
) (ps poolStatus) {
	p.mu.Lock()
	config := p.config
	p.mu.Unlock()

	// select valid entries
	var valid []pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]]
	for _, ent := range entries {
		if ent.Enable && ent.Status.Initialized && ent.Status.LatestBlock.Number >= pre.LatestBlock.Number {
			valid = append(valid, ent)
		}
	}
	if len(valid) == 0 {
		return pre // no valid entries, just return pre status
	}
	ps.Ready = true
	// calculate ps.LatestBlock
	latest := valid[0].Status.LatestBlock
	for i := 1; i < len(valid); i++ {
		if valid[i].Status.LatestBlock.Number > latest.Number {
			latest = valid[i].Status.LatestBlock
		}
	}
	ps.LatestBlock = latest
	for _, ent := range valid {
		if latest.Timestamp.Sub(ent.Status.LatestBlock.Timestamp) > config.BrokenFallBehind {
			continue // fall behind
		}
		if ent.Status.LatestBlock.Number < ps.LatestBlock.Number {
			ps.LatestBlock = ent.Status.LatestBlock
		}
	}
	// push ps.LatestBlock to ps.LatestQueue and trim ps.LatestQueue and calculate ps.BlockInterval
	ps.LatestQueue, ps.BlockInterval = pushLatestQueue(pre.LatestQueue, ps.LatestBlock, config.CheckSpeedInterval)
	if ps.BlockInterval == 0 {
		ps.BlockInterval = pre.BlockInterval
	}
	// report
	logger := log.With("pool", p.pool.Name(), "latestQueueLen", ps.LatestQueue.Len(), "psi", psi+1)
	logger.Debugf("pool latest %s => %s [%s]", pre.LatestBlock, ps.LatestBlock, ps.BlockInterval)
	return ps
}

func pushChan[ELEM any](ctx context.Context, ch chan<- ELEM, elem ELEM) bool {
	select {
	case ch <- elem:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *ClientPool[CONFIG, CLIENT]) entryStatusRefresher(
	ctx context.Context,
	config ClientConfig[CONFIG],
	es entryStatus[CLIENT],
	ch chan<- entryStatus[CLIENT],
) {
	ctx, logger := log.FromContext(ctx, "pool", p.pool.Name(), "client", config.Config.GetName())
	logger.Infow("client status refresh started", "config", config)
	defer func() {
		logger.Infow("client status refresh finished", "config", config)
	}()

	if !es.Initialized {
		es.Client = p.clientBuilder(config.Config, func(what string, dur time.Duration, hasErr bool) {
			key := fmt.Sprintf("P%d(%s)%s", config.Priority, config.Config.GetName(), what)
			p.statEntryUsed.Append(newUsedStatWin(key, dur, hasErr))
			if p.notifier != nil {
				p.notifier.EntryUsed(config, what, dur, hasErr)
			}
		})
		es.ClientName = es.Client.GetName()
		var latest Block
		err := backoff.RetryNotify(
			func() (err error) {
				latest, err = es.Client.Init(ctx)
				if err == nil {
					return
				}
				es.InitializeFailedTimes++
				es.InitializeFailedReason = err.Error()
				es.InitializeFailedAt = time.Now()
				pushChan(ctx, ch, es)
				if errors.Is(err, ErrInvalidConfig) {
					logger.With("config", config).Warne(err, "client config is invalid")
					return backoff.Permanent(err)
				}
				return err
			},
			backoff.WithContext(backoff.NewExponentialBackOff(backoff.WithMaxElapsedTime(0)), ctx), // retry forever
			func(err error, duration time.Duration) {
				logger.Warnfe(err, "client init failed, will retry after %s", duration)
			})
		if err != nil {
			return // because ctx canceled or has invalid config
		}
		es.Initialized = true
		es.InitializedAt = time.Now()
		es.LatestBlock = latest
		logger.Infof("client initialized, latest block %s", latest)
		if !pushChan(ctx, ch, es) {
			return
		}
		if p.notifier != nil {
			p.notifier.EntryLatestBlock(config, latest)
		}
	}

	latestChan := make(chan Block)
	defer close(latestChan)
	go func() {
		for latest := range latestChan {
			if latest.Number < es.LatestBlock.Number {
				logger.Warnf("client latest block backed off from %s to %s, will be ignored", es.LatestBlock, latest)
				continue
			}
			if latest.Number == es.LatestBlock.Number {
				logger.Debugf("client latest block stopped from %s to %s", es.LatestBlock, latest)
			} else {
				logger.Debugf("client latest block increased from %s to %s", es.LatestBlock, latest)
			}
			es.LatestBlock = latest
			if !pushChan(ctx, ch, es) {
				return
			}
			if p.notifier != nil {
				p.notifier.EntryLatestBlock(config, latest)
			}
		}
	}()
	es.Client.SubscribeLatest(ctx, latestChan)
}

var rd = rand.New(rand.NewSource(time.Now().UnixNano()))

func (p *ClientPool[CONFIG, CLIENT]) chooseOne(
	set map[string]pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]],
) (string, pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]]) {
	var target pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]]
	var targetName string
	var targetPriority uint32
	var targetRK int
	for name, ent := range set {
		entRK := rd.Int()
		switch {
		case targetName == "":
		case ent.Config.Priority < targetPriority:
		case ent.Config.Priority == targetPriority && entRK < targetRK:
		default:
			continue
		}
		target, targetName, targetRK, targetPriority = ent, name, entRK, ent.Config.Priority
	}
	return targetName, target
}

type option[CONFIG any] struct {
	noTags       []string
	withTags     []string
	maxPriority  *uint32
	configFilter func(c CONFIG) bool
}

type Option[CONFIG any] func(c *option[CONFIG])

func WithTags[CONFIG any](tags ...string) Option[CONFIG] {
	return func(c *option[CONFIG]) {
		if len(c.withTags) == 0 {
			c.withTags = tags
		} else {
			s := set.New[string](tags...)
			s.Add(c.withTags...)
			c.withTags = s.DumpValues()
		}
	}
}

func WithoutTags[CONFIG any](tags ...string) Option[CONFIG] {
	return func(c *option[CONFIG]) {
		if len(c.noTags) == 0 {
			c.noTags = tags
		} else {
			s := set.New[string](tags...)
			s.Add(c.noTags...)
			c.noTags = s.DumpValues()
		}
	}
}

func WithConfigFilter[CONFIG any](f func(c CONFIG) bool) Option[CONFIG] {
	return func(c *option[CONFIG]) {
		if c.configFilter == nil {
			c.configFilter = f
		} else {
			pre := c.configFilter
			c.configFilter = func(conf CONFIG) bool {
				return pre(conf) && f(conf)
			}
		}
	}
}

func WithMaxPriority[CONFIG any](priority uint32) Option[CONFIG] {
	return func(c *option[CONFIG]) {
		c.maxPriority = &priority
	}
}

type Result struct {
	Err error
	// Broken indicates the endpoint itself is unhealthy; other requests should avoid it too.
	Broken bool
	// BrokenForTask indicates the endpoint is not suitable for this particular request and should be retried with another endpoint.
	// Broken and BrokenForTask are independent: both can be set at the same time.
	BrokenForTask bool
	AddTags       []string
}

func (r Result) String() string {
	var buf bytes.Buffer
	if r.Err != nil {
		buf.WriteString("Err[")
		buf.WriteString(r.Err.Error())
		buf.WriteString("]")
	}
	if r.Broken {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("Broken")
	}
	if r.BrokenForTask {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("BrokenForTask")
	}
	if len(r.AddTags) > 0 {
		if buf.Len() > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("AddTags[")
		for i, tag := range r.AddTags {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(tag)
		}
		buf.WriteString("]")
	}
	return buf.String()
}

type Report struct {
	Err        error
	ConfigName string
	ClientName string
}

func (p *ClientPool[CONFIG, CLIENT]) consumerCome(theme string) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.consumerCounter
	p.consumerCounter++
	p.consumer[id] = consumer{theme: theme, enterAt: time.Now()}
	return id
}

func (p *ClientPool[CONFIG, CLIENT]) consumerWaitTooLong(id uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.config.ConsumerMaxWait <= 0 {
		return false
	}
	c := p.consumer[id]
	return time.Since(c.enterAt)-c.executedDuration > p.config.ConsumerMaxWait
}

func (p *ClientPool[CONFIG, CLIENT]) consumerWaiting(id uint64, psi uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.consumer[id]
	c.doing = consumerWaiting
	c.doingFrom = time.Now()
	c.waitIndex += 1
	c.waitingPSI = psi
	p.consumer[id] = c
}

func (p *ClientPool[CONFIG, CLIENT]) consumerExecuting(id uint64, client string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.consumer[id]
	c.doing = consumerExecuting
	c.doingFrom = time.Now()
	c.useClient = client
	p.consumer[id] = c
}

func (p *ClientPool[CONFIG, CLIENT]) consumerExecuted(id uint64, used time.Duration, result Result) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.consumer[id]
	c.executed += 1
	c.executedDuration += used
	c.executedLast = result
	p.consumer[id] = c
}

func (p *ClientPool[CONFIG, CLIENT]) consumerCollectDoing(doing string) (themes []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.consumer {
		if c.doing == doing {
			themes = append(themes, c.theme)
		}
	}
	return themes
}

func (p *ClientPool[CONFIG, CLIENT]) consumerLeave(id uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.consumer, id)
}

func (p *ClientPool[CONFIG, CLIENT]) consumerBlackListAdd(id uint64, entName string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.consumer[id]
	if c.blackList == nil {
		c.blackList = make(map[string]error)
	}
	c.blackList[entName] = err
	p.consumer[id] = c
}

func (p *ClientPool[CONFIG, CLIENT]) findEntries(blackList set.Set[string], opt option[CONFIG]) (
	entries map[string]pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]],
	backup int,
	psi uint64,
) {
	now := time.Now()
	entries, _, psi = p.pool.Fetch(
		func(name string, entry pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]], poolStatus poolStatus) bool {
			if blackList.Contains(name) {
				return false
			}
			if opt.configFilter != nil && !opt.configFilter(entry.Config.Config) {
				return false
			}
			if opt.maxPriority != nil && entry.Config.Priority > *opt.maxPriority {
				return false
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			tagValidFrom := time.Now().Add(-p.config.TagDuration)
			extra, has := p.entryExtra[name]
			for _, tag := range opt.withTags {
				if !has || !extra.hasTag(tag, tagValidFrom) {
					return false
				}
			}
			for _, tag := range opt.noTags {
				if has && extra.hasTag(tag, tagValidFrom) {
					return false
				}
			}
			backup++ // at least this is a backup client
			if has && extra.ban != nil && extra.ban.enable(now) {
				return false // can wait for this client to recover
			}
			if !entry.Enable {
				return false // can wait for a downgrade
			}
			if !entry.Status.Initialized {
				return false // can wait for this client to initialize
			}
			if entry.Status.LatestBlock.Number < poolStatus.LatestBlock.Number {
				return false // can wait for this client to catch up
			}
			return true
		},
	)
	return entries, backup, psi
}

// methodVetoedByAuthority reports whether any method-authority entry in the pool currently
// carries one of the given MethodNotSupported tags. Method-authority entries (typically the
// chain's own full nodes) define the pool's supported method set: once one of them rejected a
// method, probing the remaining endpoints for it is pointless — even ones that would answer —
// so the request fails fast instead of cascading through the pool and triggering a priority
// downgrade. Returns false when the pool has no method-authority entries or noTags carries no
// MethodNotSupported tag.
func (p *ClientPool[CONFIG, CLIENT]) methodVetoedByAuthority(noTags []string) bool {
	var methodTags []string
	for _, tag := range noTags {
		if strings.HasPrefix(tag, MethodNotSupportedTagPrefix) {
			methodTags = append(methodTags, tag)
		}
	}
	if len(methodTags) == 0 {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tagValidFrom := time.Now().Add(-p.config.TagDuration)
	for name, cc := range p.configEntries {
		if !cc.MethodAuthority {
			continue
		}
		extra, has := p.entryExtra[name]
		if !has {
			continue
		}
		for _, tag := range methodTags {
			if extra.hasTag(tag, tagValidFrom) {
				return true
			}
		}
	}
	return false
}

// UseClient will return ErrNoValidClient or error returned by fn
func (p *ClientPool[CONFIG, CLIENT]) UseClient(
	ctx context.Context,
	theme string,
	fn func(ctx context.Context, cli CLIENT) Result,
	opts ...Option[CONFIG],
) Report {
	cid := p.consumerCome(theme)
	curCtx, logger := log.FromContext(ctx, "pool", p.pool.Name(), "pcid", cid, "theme", theme)
	defer func() {
		p.consumerLeave(cid)
	}()
	var c option[CONFIG]
	for _, opt := range opts {
		opt(&c)
	}
	blackList := set.New[string]()
	for {
		if p.methodVetoedByAuthority(c.noTags) {
			return Report{Err: errors.Wrap(ErrNoValidClient, "the method is rejected by a method-authority endpoint")}
		}
		entries, backup, psi := p.findEntries(blackList, c)
		if len(entries) == 0 {
			if backup == 0 || p.consumerWaitTooLong(cid) {
				return Report{Err: ErrNoValidClient}
			}
			// doing stays "waiting" until the next consumerDoing(executing) call below,
			// so consumerCollectDoing may briefly over-count after Wait returns and before
			// entries are found on the next iteration. This is safe: it only causes a
			// conservative (spurious) downgrade signal, never a missed one.
			p.consumerWaiting(cid, psi)
			if err := p.pool.Wait(ctx, psi); err != nil {
				return Report{Err: err} // only because ctx canceled
			}
			continue
		}
		entName, ent := p.chooseOne(entries)
		logger.Debugw("choose client", "client", entName, "count", len(entries))
		p.consumerExecuting(cid, entName)
		startAt := time.Now()
		result := fn(ctx, ent.Status.Client)
		p.consumerExecuted(cid, time.Since(startAt), result)
		logger.Debugw("got use result", "client", entName, "result", result.String())
		for _, tag := range result.AddTags {
			p.clientAddTag(curCtx, entName, tag, result.Err)
		}
		if result.Broken {
			p.clientBan(curCtx, entName, result.Err)
		}
		if result.BrokenForTask {
			blackList.Add(entName)
			p.consumerBlackListAdd(cid, entName, result.Err)
		}
		if !result.Broken && !result.BrokenForTask {
			p.clientActive(curCtx, entName, theme)
			return Report{Err: result.Err, ConfigName: entName, ClientName: ent.Status.ClientName}
		}
	}
}

func (p *ClientPool[CONFIG, CLIENT]) clientAddTag(ctx context.Context, name string, tag string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, has := p.configEntries[name]; !has {
		return // the client is already removed
	}
	extra, has := p.entryExtra[name]
	if !has {
		extra = &entryExtra{}
		p.entryExtra[name] = extra
	}
	if extra.tags == nil {
		extra.tags = make(map[string]tagInfo)
	}
	now := time.Now()
	var info tagInfo
	if info, has = extra.tags[tag]; !has {
		info = tagInfo{startAt: now}
	}
	info.lastActiveAt = now
	info.lastActiveReason = err
	extra.tags[tag] = info
	_, logger := log.FromContext(ctx, "client", name, "reason", err)
	logger.Infof("client add tag %s", tag)
}

func (p *ClientPool[CONFIG, CLIENT]) clientBan(ctx context.Context, name string, reason error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, has := p.configEntries[name]; !has {
		return // the client is already removed
	}
	_, logger := log.FromContext(ctx, "client", name)
	now := time.Now()
	extra, has := p.entryExtra[name]
	if !has {
		extra = &entryExtra{}
		p.entryExtra[name] = extra
	}
	if extra.ban == nil {
		extra.ban = &ban{from: now, reason: reason, dur: p.config.BanConfig.Min}
		logger.Infof("client initial ban %s", extra.ban)
	} else if !extra.ban.enable(now) {
		extra.ban.extend(p.config.BanConfig, now, reason)
		logger.Infof("client continous ban %s", extra.ban)
	}
}

func (p *ClientPool[CONFIG, CLIENT]) clientActive(ctx context.Context, name string, theme string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, has := p.configEntries[name]; !has {
		return // the client is already removed
	}
	_, logger := log.FromContext(ctx, "client", name)
	extra, has := p.entryExtra[name]
	if !has {
		extra = &entryExtra{}
		p.entryExtra[name] = extra
	}
	if extra.active == nil {
		extra.active = &active{}
	}
	extra.active.theme, extra.active.time, extra.ban = theme, time.Now(), nil
	logger.Debug("client active")
}

func (p *ClientPool[CONFIG, CLIENT]) enablePriority() {
	logger := log.With("pool", p.pool.Name())
	p.mu.Lock()
	var current uint32 = math.MaxUint32
	if len(p.configPriorities) > 0 {
		current = p.configPriorities[p.priorityCursor]
	}
	var enableList, disableList []string
	for name, cc := range p.configEntries {
		if cc.Priority <= current {
			enableList = append(enableList, name)
		} else {
			disableList = append(disableList, name)
		}
	}
	p.mu.Unlock()

	if p.notifier != nil {
		if current == math.MaxUint32 {
			p.notifier.CurrentPriority(-1)
		} else {
			p.notifier.CurrentPriority(int(current))
		}
	}

	for _, name := range enableList {
		if p.pool.Enable(name) {
			logger.Infow("client enabled", "client", name)
		}
	}
	for _, name := range disableList {
		if p.pool.Disable(name) {
			logger.Infow("client disabled", "client", name)
		}
	}
	p.statDowngrade.Append(newDowngradeStatWindow(current))
}

func (p *ClientPool[CONFIG, CLIENT]) updateConfig(c PoolConfig[CONFIG]) {
	poolName := p.pool.Name()
	p.mu.Lock()
	p.configVersion++
	p.configUpdateAt = time.Now()
	logger := log.With("pool", poolName, "configVersion", p.configVersion)
	logger.Infow("pool got new config", "config", c)
	exists := p.configEntries
	p.configEntries = make(map[string]ClientConfig[CONFIG])
	for _, cc := range c.ClientConfigs {
		name := cc.Config.GetName()
		p.configEntries[name] = cc
		pcc, has := exists[name]
		if has {
			delete(exists, name) // make sure that `exists` only contains the clients need to be deleted
		}
		if pcc.Equal(cc) {
			continue // not changed
		}
		delete(p.entryExtra, name)
		if has {
			logger.Infow("pool will update client", "client", name, "preConfig", pcc, "config", cc)
		} else {
			logger.Infow("pool will add client", "client", name, "config", cc)
		}
	}
	for name, cc := range exists {
		delete(p.entryExtra, name)
		logger.Infow("pool will remove client", "client", name, "preConfig", cc)
	}
	p.config = c
	p.priorityCursor = 0
	p.configPriorities = nil
	if len(c.ClientConfigs) > 0 {
		prioritySet := set.New[uint32]()
		for _, cc := range c.ClientConfigs {
			prioritySet.Add(cc.Priority)
		}
		p.configPriorities = prioritySet.DumpValues()
		sort.Slice(p.configPriorities, func(i, j int) bool {
			return p.configPriorities[i] < p.configPriorities[j]
		})
	}
	p.mu.Unlock()

	for _, cc := range c.ClientConfigs {
		p.pool.Add(cc.Config.GetName(), cc)
	}
	for entryName := range exists {
		p.pool.Remove(entryName)
	}

	p.enablePriority()
}

func (p *ClientPool[CONFIG, CLIENT]) shouldDowngrade() string {
	if themes := p.consumerCollectDoing(consumerWaiting); len(themes) > 0 {
		return "has waiting consumer " + utils.ArrSummary(themes)
	}
	ps, _ := p.pool.Status()
	if !ps.Ready || time.Since(ps.LatestBlock.Timestamp) > p.config.BrokenFallBehind {
		return "overall fall behind"
	}
	entries, _, _ := p.findEntries(set.New[string](), option[CONFIG]{})
	if len(entries) == 0 {
		return "no valid entry"
	}
	return ""
}

func (p *ClientPool[CONFIG, CLIENT]) currentPriority() uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.priorityCursor < len(p.configPriorities) {
		return p.configPriorities[p.priorityCursor]
	}
	return 0
}

func (p *ClientPool[CONFIG, CLIENT]) adjustPriority() {
	shouldDowngrade := p.shouldDowngrade()

	var hasValidEntryAfterUpgrade bool
	if curPriority := p.currentPriority(); curPriority > 0 {
		maxPriority := curPriority - 1
		entries, _, _ := p.findEntries(set.New[string](), option[CONFIG]{maxPriority: &maxPriority})
		hasValidEntryAfterUpgrade = len(entries) > 0
	}

	logger := log.With("pool", p.pool.Name())

	p.mu.Lock()
	if shouldDowngrade != "" {
		// should downgrade, try to downgrade
		if p.priorityCursor+1 < len(p.configPriorities) {
			p.priorityCursor++
			logger.Infof("pool downgrade to %d because %s", p.configPriorities[p.priorityCursor], shouldDowngrade)
		} else {
			logger.Infof("pool want to downgrade because %s but already in the lowest priority %d",
				shouldDowngrade, p.configPriorities[p.priorityCursor])
		}
	} else if p.priorityCursor > 0 {
		// should not downgrade, may be can upgrade
		if hasValidEntryAfterUpgrade {
			priority := make(map[string]uint32)
			for entName, cc := range p.configEntries {
				priority[entName] = cc.Priority
			}
			var lastActiveAt time.Time
			for entryName, extra := range p.entryExtra {
				if priority[entryName] != p.configPriorities[p.priorityCursor] {
					continue
				}
				if extra.active != nil && extra.active.time.After(lastActiveAt) {
					lastActiveAt = extra.active.time
				}
			}
			if time.Since(lastActiveAt) > p.config.UpgradeSensitivity {
				p.priorityCursor--
				logger.Infof("pool upgrade to %d", p.configPriorities[p.priorityCursor])
			} else {
				logger.Debugf("pool want to upgrade but current priority entries last active at %s", lastActiveAt)
			}
		} else {
			logger.Debugf("pool want to upgrade but no valid entry after upgrade")
		}
	}
	p.mu.Unlock()

	p.enablePriority()
}

func (p *ClientPool[CONFIG, CLIENT]) Start(ctx context.Context, ch <-chan PoolConfig[CONFIG]) {
	logger := log.With("pool", p.pool.Name())
	logger.Infof("pool started")
	defer func() {
		logger.Infof("pool stopped")
	}()
	neverCloseNext := make(chan time.Time)        // never closed chan
	neverCloseCh := make(chan PoolConfig[CONFIG]) // never closed chan
	for {
		var next <-chan time.Time
		p.mu.Lock()
		if p.config.AdjustPriorityInterval > 0 {
			next = time.After(p.config.AdjustPriorityInterval)
		} else {
			next = neverCloseNext
		}
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case cfg, has := <-ch:
			if !has {
				// ch is closed, meaning no more config updates will arrive.
				// Replace with a never-closing, never-sending channel to keep the select alive for ctx and next.
				ch = neverCloseCh
				continue
			}
			p.updateConfig(cfg.Trim(p.confModifiers))
		case <-next:
			p.adjustPriority()
		}
	}
}

type Shell interface {
	GetState() (latest Block, blockInterval time.Duration, ready bool, psi uint64)
	WaitState(ctx context.Context, psiGT uint64) error
	WaitReady(ctx context.Context) error
	WaitBlock(ctx context.Context, numberGE uint64) (Block, error)
	WaitBlockInterval(ctx context.Context) (time.Duration, error)
}

func (p *ClientPool[CONFIG, CLIENT]) GetState() (latest Block, blockInterval time.Duration, ready bool, psi uint64) {
	ps, psi := p.pool.Status()
	return ps.LatestBlock, ps.BlockInterval, ps.Ready, psi
}

func (p *ClientPool[CONFIG, CLIENT]) WaitState(ctx context.Context, psiGT uint64) error {
	return p.pool.Wait(ctx, psiGT)
}

func (p *ClientPool[CONFIG, CLIENT]) wait(ctx context.Context, ok func(status poolStatus) bool) (poolStatus, error) {
	for {
		ps, psi := p.pool.Status()
		if ok(ps) {
			return ps, nil
		}
		if err := p.WaitState(ctx, psi); err != nil {
			return ps, err
		}
	}
}

func (p *ClientPool[CONFIG, CLIENT]) WaitReady(ctx context.Context) error {
	_, err := p.wait(ctx, func(ps poolStatus) bool {
		return ps.Ready
	})
	return err
}

func (p *ClientPool[CONFIG, CLIENT]) WaitBlock(ctx context.Context, numberGE uint64) (Block, error) {
	ps, err := p.wait(ctx, func(ps poolStatus) bool {
		return ps.Ready && ps.LatestBlock.Number >= numberGE
	})
	if err != nil {
		return Block{}, err
	}
	return ps.LatestBlock, nil
}

func (p *ClientPool[CONFIG, CLIENT]) WaitBlockInterval(ctx context.Context) (time.Duration, error) {
	ps, err := p.wait(ctx, func(ps poolStatus) bool {
		return ps.Ready && ps.BlockInterval > 0
	})
	if err != nil {
		return 0, err
	}
	return ps.BlockInterval, nil
}
