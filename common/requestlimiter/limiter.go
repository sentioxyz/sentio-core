package requestlimiter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"sentioxyz/sentio-core/common/configmanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/protos"

	kyaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

var (
	ErrAcquireFailed = errors.New("acquire failed")
)

type RequestData interface {
	String() string
}

type VarsAttributesKey string

const (
	VarsUsernameKey    = "username"
	VarsProjectNameKey = "project_name"
	VarsApiKey         = "api_key"
)

type RequestVars struct {
	OwnerID    string
	ProjectID  string
	RequestIP  string
	Tier       protos.Tier
	Data       RequestData `json:"-"`
	Attributes map[VarsAttributesKey]string
}

type Limiter interface {
	Acquire(ctx context.Context, vars RequestVars) (string, bool, error)
	Release(ctx context.Context, vars RequestVars, limiterID string)
}

type LimiterConfig struct {
	ConcurrentQuotaPerUser    int            `yaml:"concurrent_quota_per_user"`
	ConcurrentQuotaPerIP      int            `yaml:"concurrent_quota_per_ip"`
	ConcurrentQuotaPerProject int            `yaml:"concurrent_quota_per_project"`
	ConcurrentQuotaByTier     map[string]int `yaml:"concurrent_quota_by_tier"`
	ConcurrentQuotaUserTier   map[string]int `yaml:"concurrent_quota_user_tier"`
}

type metrics struct {
	acquiredCount metric.Int64Counter
	releasedCount metric.Int64Counter
	inUseGauge    metric.Int64Gauge
}

type limiter struct {
	*redis.Client
	timeout        time.Duration
	localConf      LimiterConfig
	remoteConf     configmanager.Config
	id             atomic.Uint64
	acquireScripts string
	releaseScripts string
	name           string

	meter   metric.Meter
	metrics metrics
}

func (l *limiter) initMetrics() {
	l.metrics.acquiredCount, _ = l.meter.Int64Counter(l.name+"_limiter.acquired_total",
		metric.WithDescription("Counter of acquired limiters"))
	l.metrics.releasedCount, _ = l.meter.Int64Counter(l.name+"_limiter.released_total",
		metric.WithDescription("Counter of released limiters"))
	l.metrics.inUseGauge, _ = l.meter.Int64Gauge(l.name + "_limiter.in_use")
}

func (l *limiter) recordGauge(ctx context.Context, vars RequestVars, counts []string) {
	mustInt64 := func(s string) int64 {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0
		}
		return i
	}
	if len(counts) != 4 {
		return
	}
	user, project, ip, tier := mustInt64(counts[0]), mustInt64(counts[1]), mustInt64(counts[2]), mustInt64(counts[3])

	if vars.OwnerID != "" {
		l.metrics.inUseGauge.Record(ctx, user, metric.WithAttributes(
			attribute.String("scope", "user"),
			attribute.String("user", vars.OwnerID),
			attribute.String("user_name", lo.If(vars.Attributes != nil, vars.Attributes[VarsUsernameKey]).Else("")),
			attribute.String("api_key", lo.If(vars.Attributes != nil, vars.Attributes[VarsApiKey]).Else("")),
		))
	}
	if vars.ProjectID != "" {
		l.metrics.inUseGauge.Record(ctx, project, metric.WithAttributes(
			attribute.String("scope", "project"),
			attribute.String("project", vars.ProjectID),
			attribute.String("project_slug", lo.If(vars.Attributes != nil, vars.Attributes[VarsProjectNameKey]).Else("")),
		))
	}
	if vars.RequestIP != "" {
		l.metrics.inUseGauge.Record(ctx, ip, metric.WithAttributes(
			attribute.String("scope", "ip"),
			attribute.String("ip", vars.RequestIP),
		))
	}
	if vars.Tier.String() != "" {
		l.metrics.inUseGauge.Record(ctx, tier, metric.WithAttributes(
			attribute.String("scope", "tier"),
			attribute.String("tier", vars.Tier.String()),
		))
	}
}

func NewLimiterWithConfig(name string, client *redis.Client, timeout time.Duration, localConf LimiterConfig, remoteConf configmanager.Config) Limiter {
	l := &limiter{
		Client:     client,
		name:       name,
		timeout:    timeout,
		localConf:  localConf,
		remoteConf: remoteConf,
		meter:      otel.Meter(name),
	}
	if err := l.scriptsLoad(context.Background()); err != nil {
		panic(err)
	}
	l.initMetrics()
	return l
}

func NewLimiter(name string, db *gorm.DB, client *redis.Client, timeout time.Duration, configPath string) Limiter {
	data, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	var config LimiterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Errorf("Failed to unmarshal config: %v", err)
		panic(err)
	}
	var (
		remoteConf configmanager.Config = nil
		ok         bool
	)
	if err := configmanager.Set(name,
		configmanager.NewPgProvider(db, configmanager.WithPgKey(name)),
		kyaml.Parser(), &configmanager.LoadParams{
			EnableReload: true,
			ReloadPeriod: time.Second * 60,
		}); err != nil {
		log.Errorf("Failed to load config: %v", err)
	} else {
		remoteConf, ok = configmanager.Get(name)
		if !ok {
			log.Errorf("Failed to get config from config manager")
			remoteConf = nil
		} else {
			log.Infof("Loaded remote config: %s", remoteConf.Sprint())
		}
	}
	log.Infof("Limiter config: %+v", config)
	return NewLimiterWithConfig(name, client, timeout, config, remoteConf)
}

func NewLimiterWithDefaultConfig(name string, db *gorm.DB, client *redis.Client, timeout time.Duration) Limiter {
	return NewLimiter(name, db, client, timeout, "common/requestlimiter/limiter_config.yaml")
}

func (l *limiter) scriptsLoad(ctx context.Context) error {
	acquireSha, err := l.ScriptLoad(ctx, acquireTemplate).Result()
	if err != nil {
		return errors.Wrap(err, "failed to load acquire script")
	}
	releaseSha, err := l.ScriptLoad(ctx, releaseTemplate).Result()
	if err != nil {
		return errors.Wrap(err, "failed to load release script")
	}
	l.acquireScripts = acquireSha
	l.releaseScripts = releaseSha
	return nil
}

func (l *limiter) generateLimiterID(data RequestData) (string, error) {
	buf := new(bytes.Buffer)
	buf.WriteString(data.String())
	buf.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
	buf.WriteString(strconv.Itoa(os.Getpid()))
	buf.WriteString(strconv.FormatUint(l.id.Add(1), 10))
	return buf.String(), nil
}

func (l *limiter) keyName(key string) string {
	if key == "" {
		return ""
	}
	return "requestlimiter:" + key
}

func (l *limiter) concurrentQuotaPerIP() int {
	switch {
	case l.remoteConf != nil:
		q := l.remoteConf.Int("concurrent_quota_per_ip")
		return lo.If(q <= 0, 0).Else(q)
	default:
		return lo.If(l.localConf.ConcurrentQuotaPerIP <= 0, 0).Else(l.localConf.ConcurrentQuotaPerIP)
	}
}

func (l *limiter) concurrentQuotaPerProject() int {
	switch {
	case l.remoteConf != nil:
		q := l.remoteConf.Int("concurrent_quota_per_project")
		return lo.If(q <= 0, 0).Else(q)
	default:
		return lo.If(l.localConf.ConcurrentQuotaPerProject <= 0, 0).Else(l.localConf.ConcurrentQuotaPerProject)
	}
}

func (l *limiter) concurrentQuotaPerUser(tier string) int {
	switch {
	case l.remoteConf != nil:
		userQuota := l.remoteConf.Int("concurrent_quota_per_user")
		userTierQuota := l.remoteConf.MustIntMap("concurrent_quota_user_tier")
		if quota, ok := userTierQuota[tier]; ok {
			userQuota = quota
		}
		return lo.If(userQuota <= 0, 0).Else(userQuota)
	default:
		userQuota := l.localConf.ConcurrentQuotaPerUser
		if l.localConf.ConcurrentQuotaUserTier != nil {
			quota, ok := l.localConf.ConcurrentQuotaUserTier[tier]
			if ok {
				userQuota = quota
			}
		}
		return lo.If(userQuota <= 0, 0).Else(userQuota)
	}
}

func (l *limiter) concurrentQuotaByTier(tier string) int {
	switch {
	case l.remoteConf != nil:
		m := l.remoteConf.MustIntMap("concurrent_quota_by_tier")
		q := m[tier]
		return lo.If(q <= 0, 0).Else(q)
	default:
		return lo.If(l.localConf.ConcurrentQuotaByTier[tier] <= 0, 0).
			Else(l.localConf.ConcurrentQuotaByTier[tier])
	}
}

func (l *limiter) Acquire(ctx context.Context, vars RequestVars) (string, bool, error) {
	ctx, logger := log.FromContext(ctx)
	now := time.Now()
	limiterID, err := l.generateLimiterID(vars.Data)
	if err != nil {
		logger.Errorf("Failed to generate limiter ID: %v", err)
		return "", false, err
	}
	user, project, ip, tier := l.concurrentQuotaPerUser(vars.Tier.String()),
		l.concurrentQuotaPerProject(),
		l.concurrentQuotaPerIP(),
		l.concurrentQuotaByTier(vars.Tier.String())
	logger.Debugf("Acquire limiter ID: %s, concurrent quota per user: %d, concurrent quota per project: %d, concurrent quota per ip: %d, concurrent quota by tier: %d",
		limiterID, user, project, ip, tier)
	result, err := l.EvalSha(ctx, l.acquireScripts, []string{
		l.keyName(vars.OwnerID), l.keyName(vars.ProjectID),
		l.keyName(vars.RequestIP), l.keyName(vars.Tier.String()),
	},
		now.Unix(), now.Add(-l.timeout).Unix(), limiterID,
		user, project, ip, tier,
	).Result()
	if err != nil {
		logger.Errorf("Failed to eval acquire template: %v", err)
		return "", true, ErrAcquireFailed
	}

	getErrDetail := func(detail string) (int, int) {
		parts := strings.Split(detail, "/")
		if len(parts) != 2 {
			return 0, 0
		}
		current, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0
		}
		limit, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0
		}
		return current, limit
	}

	results := strings.Split(result.(string), ":")
	category, message := results[0], results[1]
	switch category {
	case "ok":
		logger.Debugf("Acquired limiter ID: %s", limiterID)
		l.recordGauge(ctx, vars, strings.Split(message, "/"))
		l.metrics.acquiredCount.Add(ctx, 1)
		return limiterID, true, nil
	case "user_reached_quota":
		current, limit := getErrDetail(message)
		return "", false, fmt.Errorf("current: %d, limit: %d, reject by user quota", current, limit)
	case "project_reached_quota":
		current, limit := getErrDetail(message)
		return "", false, fmt.Errorf("current: %d, limit: %d, reject by project quota", current, limit)
	case "ip_reached_quota":
		current, limit := getErrDetail(message)
		return "", false, fmt.Errorf("current: %d, limit: %d, reject by ip quota", current, limit)
	case "tier_reached_quota":
		current, limit := getErrDetail(message)
		return "", false, fmt.Errorf("current: %d, limit: %d, reject by tier quota", current, limit)
	default:
		return "", false, fmt.Errorf("unknown error: %v", result)
	}
}

func (l *limiter) Release(ctx context.Context, vars RequestVars, limiterID string) {
	ctx, logger := log.FromContext(ctx)
	l.metrics.releasedCount.Add(ctx, 1)
	result, err := l.EvalSha(ctx, l.releaseScripts, []string{
		l.keyName(vars.OwnerID), l.keyName(vars.ProjectID),
		l.keyName(vars.RequestIP), l.keyName(vars.Tier.String()),
	}, limiterID).Result()
	if err != nil {
		logger.Errorf("Failed to eval release template: %v", err)
	}
	logger.Debugf("Released limiter ID: %s", limiterID)
	results := strings.Split(result.(string), ":")
	_, message := results[0], results[1]
	l.recordGauge(ctx, vars, strings.Split(message, "/"))
}
