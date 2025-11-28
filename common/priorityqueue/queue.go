package priorityqueue

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

type (
	Priority     int
	ScheduleType int
)

const (
	StrictSchedule ScheduleType = iota // execute all high priority tasks before low priority tasks
)

const (
	Normal Priority = iota
	Low
	Medium
	High
)

func FromString(s string) Priority {
	switch s {
	case "low":
		return Low
	case "medium":
		return Medium
	case "high":
		return High
	case "normal":
		return Normal
	}
	return Low
}

func (p Priority) String() string {
	switch p {
	case Low:
		return "low"
	case Medium:
		return "medium"
	case High:
		return "high"
	case Normal:
		return "normal"
	}
	return "normal"
}

type PriorityQueue[T any] interface {
	RegisterPriority(Priority) PriorityQueue[T]
	RegisterPriorityWithWeight(Priority, float64) PriorityQueue[T]
	Push(context.Context, T, Priority) (int64, error)
	Pop(context.Context) (*T, error)
	PopSelectPriority(context.Context, Priority) (*T, error)
	PopByPriorityDescending(context.Context, Priority) (*T, error)
	PopByPriorityAscending(context.Context, Priority) (*T, error)
	BlockUntilPop(context.Context, time.Duration) (*T, Priority, error)
	BlockUntilPopSelectPriority(context.Context, time.Duration, Priority) (*T, error)
	Len() int64
	LenByPriority(Priority) int64
}

type queuePriority struct {
	priority Priority
	weight   float64
}

type queuePriorities []queuePriority

func (q queuePriorities) Len() int {
	return len(q)
}

func (q queuePriorities) Less(i, j int) bool {
	return q[i].priority < q[j].priority
}

func (q queuePriorities) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

type queueImpl[T any] struct {
	redisClient   *redis.Client
	priorities    []queuePriority
	prioritiesMap map[Priority]struct{}
	scheduleType  ScheduleType
}

func NewPriorityQueue[T any](redisClient *redis.Client, scheduleType ScheduleType) PriorityQueue[T] {
	q := &queueImpl[T]{
		redisClient:   redisClient,
		priorities:    []queuePriority{},
		prioritiesMap: make(map[Priority]struct{}),
		scheduleType:  scheduleType,
	}
	return q
}

func (q *queueImpl[T]) prefix() string {
	return "sentio_priority_queue:"
}

func (q *queueImpl[T]) queueName(p Priority) string {
	return q.prefix() + strconv.FormatInt(int64(p), 10)
}

func (q *queueImpl[T]) queuePriority(name string) Priority {
	p, _ := strconv.ParseUint(strings.TrimPrefix(name, q.prefix()), 10, 64)
	return Priority(p)
}

func (q *queueImpl[T]) RegisterPriority(p Priority) PriorityQueue[T] {
	if _, ok := q.prioritiesMap[p]; ok {
		panic(errors.Errorf("priority %d already exists", p))
	}
	q.priorities = append(q.priorities, queuePriority{
		priority: p,
		weight:   1.0,
	})
	q.prioritiesMap[p] = struct{}{}
	sort.Sort(queuePriorities(q.priorities))
	return q
}

func (q *queueImpl[T]) RegisterPriorityWithWeight(p Priority, w float64) PriorityQueue[T] {
	if _, ok := q.prioritiesMap[p]; ok {
		panic(errors.Errorf("priority %d already exists", p))
	}
	q.priorities = append(q.priorities, queuePriority{
		priority: p,
		weight:   w,
	})
	q.prioritiesMap[p] = struct{}{}
	sort.Sort(queuePriorities(q.priorities))
	return q
}

func (q *queueImpl[T]) Push(ctx context.Context, item T, priority Priority) (int64, error) {
	if _, ok := q.prioritiesMap[priority]; !ok {
		return 0, errors.Errorf("priority %d not registered", priority)
	}
	encodedItem, err := json.Marshal(item)
	if err != nil {
		return 0, errors.Wrap(err, "failed to marshal item")
	}
	queueName := q.queueName(priority)
	length, err := q.redisClient.LPush(ctx, queueName, encodedItem).Result()
	if err != nil {
		return 0, errors.Wrap(err, "failed to push item to queue")
	}
	return length, nil
}

func (q *queueImpl[T]) unmarshal(data string) (*T, error) {
	var item T
	err := json.Unmarshal([]byte(data), &item)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal item")
	}
	return &item, nil
}

func (q *queueImpl[T]) strictSchedulePop(ctx context.Context) (*T, error) {
	for idx := len(q.priorities) - 1; idx >= 0; idx-- {
		data, err := q.redisClient.RPop(ctx, q.queueName(q.priorities[idx].priority)).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, errors.Wrap(err, "failed to pop item from queue")
		}
		if data != "" {
			return q.unmarshal(data)
		}
	}
	return nil, nil
}

func (q *queueImpl[T]) Pop(ctx context.Context) (*T, error) {
	if len(q.priorities) == 0 {
		return nil, errors.New("no priorities registered")
	}
	switch q.scheduleType {
	case StrictSchedule:
		return q.strictSchedulePop(ctx)
	}
	return nil, errors.Errorf("unsupported schedule type %d", q.scheduleType)
}

func (q *queueImpl[T]) PopSelectPriority(ctx context.Context, priority Priority) (*T, error) {
	if _, ok := q.prioritiesMap[priority]; !ok {
		return nil, errors.Errorf("priority %d not registered", priority)
	}
	data, err := q.redisClient.RPop(ctx, q.queueName(priority)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to pop item from queue")
	}
	if data == "" {
		return nil, nil
	}
	return q.unmarshal(data)
}

func (q *queueImpl[T]) PopByPriorityDescending(ctx context.Context, priority Priority) (*T, error) {
	if len(q.priorities) == 0 {
		return nil, errors.New("no priorities registered")
	}
	var startIdx = -1
	for idx := len(q.priorities) - 1; idx >= 0; idx-- {
		if q.priorities[idx].priority >= priority {
			startIdx = idx
		} else {
			break
		}
	}
	if startIdx < 0 {
		return nil, nil
	}
	for idx := startIdx; idx >= 0; idx-- {
		data, err := q.redisClient.RPop(ctx, q.queueName(q.priorities[idx].priority)).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, errors.Wrap(err, "failed to pop item from queue")
		}
		if data != "" {
			return q.unmarshal(data)
		}
	}
	return nil, nil
}

func (q *queueImpl[T]) PopByPriorityAscending(ctx context.Context, priority Priority) (*T, error) {
	if len(q.priorities) == 0 {
		return nil, errors.New("no priorities registered")
	}
	var startIdx = -1
	for idx := 0; idx < len(q.priorities); idx++ {
		if q.priorities[idx].priority >= priority {
			startIdx = idx
			break
		}
	}
	if startIdx < 0 {
		return nil, nil
	}
	for idx := startIdx; idx < len(q.priorities); idx++ {
		data, err := q.redisClient.RPop(ctx, q.queueName(q.priorities[idx].priority)).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, errors.Wrap(err, "failed to pop item from queue")
		}
		if data != "" {
			return q.unmarshal(data)
		}
	}
	return nil, nil
}

func (q *queueImpl[T]) BlockUntilPop(ctx context.Context, timeout time.Duration) (*T, Priority, error) {
	if len(q.priorities) == 0 {
		return nil, 0, errors.New("no priorities registered")
	}
	var queueNames []string
	for idx := len(q.priorities) - 1; idx >= 0; idx-- {
		queueNames = append(queueNames, q.queueName(q.priorities[idx].priority))
	}
	data, err := q.redisClient.BRPop(ctx, timeout, queueNames...).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, 0, nil
		}
		return nil, 0, errors.Wrap(err, "failed to pop item from queue")
	}
	if len(data) < 2 {
		return nil, 0, nil
	}
	value, err := q.unmarshal(data[1])
	if err != nil {
		return nil, 0, err
	}
	return value, q.queuePriority(data[0]), nil
}

func (q *queueImpl[T]) BlockUntilPopSelectPriority(ctx context.Context, timeout time.Duration, priority Priority) (*T, error) {
	if _, ok := q.prioritiesMap[priority]; !ok {
		return nil, errors.Errorf("priority %d not registered", priority)
	}
	data, err := q.redisClient.BRPop(ctx, timeout, q.queueName(priority)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to pop item from queue")
	}
	if len(data) < 2 {
		return nil, nil
	}
	return q.unmarshal(data[1])
}

func (q *queueImpl[T]) Len() int64 {
	var total int64
	for p := range q.prioritiesMap {
		length, err := q.redisClient.LLen(context.Background(), q.queueName(p)).Result()
		if err != nil {
			return 0
		}
		total += length
	}
	return total
}

func (q *queueImpl[T]) LenByPriority(priority Priority) int64 {
	length, err := q.redisClient.LLen(context.Background(), q.queueName(priority)).Result()
	if err != nil {
		return 0
	}
	return length
}
