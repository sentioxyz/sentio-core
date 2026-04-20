package clientpool

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"math"
	"math/rand"
	"reflect"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/pool"
	"sentioxyz/sentio-core/common/queue"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/timewin"
	"sort"
	"sync"
	"time"
)

type entryStatus[CLIENT pool.Status] struct {
	Client CLIENT

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

type entryExtra struct {
	tags set.Set[string]
	*ban
	*active
}

type consumer struct {
	theme     string
	enterAt   time.Time
	doing     string
	doingFrom time.Time
}

func (c consumer) Snapshot() any {
	return map[string]any{
		"theme":     c.theme,
		"enterAt":   c.enterAt.String(),
		"doing":     c.doing,
		"doingFrom": c.doingFrom.String(),
	}
}

// ClientPool
// member methods that begin with '_' are called only within the critical section constructed by p.mu.
type ClientPool[CONFIG EntryConfig[CONFIG], CLIENT Client] struct {
	pool *pool.Pool[ClientConfig[CONFIG], entryStatus[CLIENT], poolStatus]

	clientBuilder func(CONFIG) CLIENT

	statDowngrade *timewin.TimeWindowsManager[*downgradeStatWindow]

	// protect all properties below
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
	clientBuilder func(CONFIG) CLIENT,
) *ClientPool[CONFIG, CLIENT] {
	p := &ClientPool[CONFIG, CLIENT]{
		clientBuilder: clientBuilder,
		entryExtra:    make(map[string]*entryExtra),
		consumer:      make(map[uint64]consumer),
		statDowngrade: timewin.NewTimeWindowsManager[*downgradeStatWindow](time.Minute),
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
	// push ps.LatestBlock to ps.LatestQueue and trim ps.LatestQueue
	ps.LatestQueue = pre.LatestQueue
	if ps.LatestQueue == nil {
		ps.LatestQueue = queue.NewQueue[Block]()
	}
	if bc, has := ps.LatestQueue.Back(); !has || bc.Number < ps.LatestBlock.Number {
		ps.LatestQueue.PushBack(ps.LatestBlock)
	}
	for {
		fr, _ := ps.LatestQueue.Front()
		if ps.LatestBlock.Timestamp.Sub(fr.Timestamp) <= config.CheckSpeedInterval {
			break
		}
		ps.LatestQueue.PopFront()
	}
	// calculate ps.BlockInterval
	logger := log.With("pool", p.pool.Name(), "latestQueueLen", ps.LatestQueue.Len(), "psi", psi+1)
	ps.BlockInterval = pre.BlockInterval
	fr, _ := ps.LatestQueue.Front()
	if fr.Number < ps.LatestBlock.Number && fr.Timestamp.Before(ps.LatestBlock.Timestamp) {
		ps.BlockInterval = ps.LatestBlock.Timestamp.Sub(fr.Timestamp) / time.Duration(ps.LatestBlock.Number-fr.Number)
	}
	// report
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
		es.Client = p.clientBuilder(config.Config)
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
				logger.Warnf("client latest block stopped from %s to %s", es.LatestBlock, latest)
			} else {
				logger.Debugf("client latest block increased from %s to %s", es.LatestBlock, latest)
			}
			es.LatestBlock = latest
			if !pushChan(ctx, ch, es) {
				return
			}
		}
	}()
	es.Client.SubscribeLatest(ctx, es.LatestBlock.Number, latestChan)
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

type Result struct {
	Err           error
	Broken        bool
	BrokenForTask bool
	AddTags       []string
}

func (p *ClientPool[CONFIG, CLIENT]) consumerCome(theme string) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.consumerCounter
	p.consumerCounter++
	p.consumer[id] = consumer{theme: theme, enterAt: time.Now()}
	return id
}

func (p *ClientPool[CONFIG, CLIENT]) consumerDoing(id uint64, doing string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.consumer[id]
	c.doing = doing
	c.doingFrom = time.Now()
	p.consumer[id] = c
}

func (p *ClientPool[CONFIG, CLIENT]) consumerLeave(id uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.consumer, id)
}

// UseClient will return ErrNoValidClient or error returned by fn
func (p *ClientPool[CONFIG, CLIENT]) UseClient(
	ctx context.Context,
	theme string,
	fn func(ctx context.Context, cli CLIENT) Result,
	opts ...Option[CONFIG],
) error {
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
		now, backup := time.Now(), 0
		entries, _, psIndex := p.pool.Fetch(
			func(name string, entry pool.Entry[ClientConfig[CONFIG], entryStatus[CLIENT]], poolStatus poolStatus) bool {
				if blackList.Contains(name) {
					return false
				}
				if c.configFilter != nil && !c.configFilter(entry.Config.Config) {
					return false
				}
				p.mu.Lock()
				extra, has := p.entryExtra[name]
				p.mu.Unlock()
				for _, tag := range c.withTags {
					if !has || !extra.tags.Contains(tag) {
						return false
					}
				}
				for _, tag := range c.noTags {
					if has && extra.tags.Contains(tag) {
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
		if len(entries) == 0 {
			if backup == 0 {
				return ErrNoValidClient
			}
			p.consumerDoing(cid, "waiting")
			if err := p.pool.Wait(ctx, psIndex); err != nil {
				return err
			}
			continue
		}
		entName, ent := p.chooseOne(entries)
		logger.Debugw("choose client", "client", entName, "count", len(entries))
		p.consumerDoing(cid, "executing")
		result := fn(ctx, ent.Status.Client)
		for _, tag := range result.AddTags {
			p.clientAddTag(curCtx, entName, tag)
		}
		if result.Broken {
			p.clientBan(curCtx, entName, result.Err)
			continue
		}
		if result.BrokenForTask {
			blackList.Add(entName)
			continue
		}
		p.clientActive(curCtx, entName, theme)
		return result.Err
	}
}

func (p *ClientPool[CONFIG, CLIENT]) clientAddTag(ctx context.Context, name string, tag string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, has := p.configEntries[name]; !has {
		return // the client is already removed
	}
	extra, has := p.entryExtra[name]
	if !has {
		extra = &entryExtra{tags: set.New[string]()}
		p.entryExtra[name] = extra
	}
	if !extra.tags.Contains(tag) {
		extra.tags.Add(tag)
		_, logger := log.FromContext(ctx, "client", name)
		logger.Infof("client add tag %s", tag)
	}
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
		extra = &entryExtra{tags: set.New[string]()}
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
		extra = &entryExtra{tags: set.New[string]()}
		p.entryExtra[name] = extra
	}
	if extra.active == nil {
		extra.active = &active{}
	}
	extra.active.theme, extra.active.time, extra.ban = theme, time.Now(), nil
	logger.Debug("client active")
}

func (p *ClientPool[CONFIG, CLIENT]) enablePriority() {
	p.mu.Lock()
	var current uint32 = math.MaxUint32
	if len(p.configPriorities) > 0 {
		current = p.configPriorities[p.priorityCursor]
	}
	var enableList, disableList []string
	logger := log.With("pool", p.pool.Name())
	for name, cc := range p.configEntries {
		if cc.Priority <= current {
			enableList = append(enableList, name)
		} else {
			disableList = append(disableList, name)
		}
	}
	p.mu.Unlock()

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
	p.mu.Lock()
	p.configVersion++
	p.configUpdateAt = time.Now()
	logger := log.With("pool", p.pool.Name(), "configVersion", p.configVersion)
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

func (p *ClientPool[CONFIG, CLIENT]) adjustPriority() {
	p.mu.Lock()
	if p.pool.Waiting() > 0 {
		// has waiting consumer, try to downgrade
		if p.priorityCursor+1 < len(p.configPriorities) {
			p.priorityCursor++
			log.With("pool", p.pool.Name()).Infof("pool downgrade to %d", p.configPriorities[p.priorityCursor])
		}
	} else if p.priorityCursor > 0 {
		// no waiting consumer, may be can upgrade
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
			log.With("pool", p.pool.Name()).Infof("pool upgrade to %d", p.configPriorities[p.priorityCursor])
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
	for {
		var next <-chan time.Time
		p.mu.Lock()
		if p.config.AdjustPriorityInterval > 0 {
			next = time.After(p.config.AdjustPriorityInterval)
		} else {
			next = make(chan time.Time) // never closed chan
		}
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case cfg := <-ch:
			p.updateConfig(cfg.Trim())
		case <-next:
			p.adjustPriority()
		}
	}
}

func (p *ClientPool[CONFIG, CLIENT]) GetState() (latest Block, blockInterval time.Duration, ready bool, psi uint64) {
	ps, psi := p.pool.Status()
	return ps.LatestBlock, ps.BlockInterval, ps.Ready, psi
}

func (p *ClientPool[CONFIG, CLIENT]) WaitState(ctx context.Context, psiGT uint64) error {
	return p.pool.Wait(ctx, psiGT)
}

func (p *ClientPool[CONFIG, CLIENT]) WaitBlock(ctx context.Context, numberGE uint64) (Block, error) {
	var psi uint64
	var ready bool
	var latest Block
	for !ready || latest.Number < numberGE {
		if err := p.WaitState(ctx, psi); err != nil {
			return latest, err
		}
		latest, _, ready, psi = p.GetState()
	}
	return latest, nil
}

func (p *ClientPool[CONFIG, CLIENT]) WaitBlockInterval(ctx context.Context) (time.Duration, error) {
	var psi uint64
	var ready bool
	var interval time.Duration
	for !ready || interval == 0 {
		if err := p.WaitState(ctx, psi); err != nil {
			return interval, err
		}
		_, interval, ready, psi = p.GetState()
	}
	return interval, nil
}
