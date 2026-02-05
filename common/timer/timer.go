package timer

import (
	"bytes"
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"sync"
	"time"
)

type Start struct {
	sub string
	t   time.Time
	tm  *Timer
}

func (s Start) End() time.Duration {
	dur := time.Since(s.t)
	s.tm.mu.Lock()
	defer s.tm.mu.Unlock()
	s.tm.subTotalUsed[s.sub] += dur
	return dur
}

func (s Start) Watch() time.Duration {
	return time.Since(s.t)
}

type Timer struct {
	subTotalUsed map[string]time.Duration
	mu           sync.Mutex
}

func NewTimer() *Timer {
	return &Timer{
		subTotalUsed: make(map[string]time.Duration),
	}
}

func (t *Timer) Start(sub string) Start {
	return Start{
		sub: sub,
		t:   time.Now(),
		tm:  t,
	}
}

func (t *Timer) ReportDistribution(mainSub string, subs string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var buf bytes.Buffer
	buf.WriteString(t.subTotalUsed[mainSub].String())
	buf.WriteRune('[')
	subArr := strings.Split(subs, ",")
	if subs == "*" {
		subArr = utils.GetOrderedMapKeys(t.subTotalUsed)
	}
	for i, sub := range subArr {
		if i > 0 {
			buf.WriteRune(',')
		}
		buf.WriteString(fmt.Sprintf("%s:%d%%", sub, t.subTotalUsed[sub]*100/t.subTotalUsed[mainSub]))
	}
	buf.WriteRune(']')
	return buf.String()
}

func Wait(
	ctx context.Context,
	timeout time.Duration,
	warn time.Duration,
	main func() error,
	warnFn func(used time.Duration),
) error {
	done := make(chan struct{})
	start := time.Now()
	var err error
	go func() {
		defer close(done)
		err = main()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(warn)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return err
		case <-ctx.Done():
			ctxErr := ctx.Err()
			return ctxErr
		case <-timer.C:
			return context.DeadlineExceeded
		case <-ticker.C:
			warnFn(time.Since(start))
		}
	}
}

// MinimumIntervalExecutor Used to control the minimum interval for doing something, such as printing some kind of
// alarm log, or some kind of circular printing report
type MinimumIntervalExecutor struct {
	interval time.Duration
	last     *time.Time

	sync.Mutex
}

func NewMinimumIntervalExecutor(interval time.Duration) *MinimumIntervalExecutor {
	return &MinimumIntervalExecutor{interval: interval}
}

func (s *MinimumIntervalExecutor) Reset() {
	if s != nil {
		s.Lock()
		defer s.Unlock()
		s.last = nil
	}
}

func (s *MinimumIntervalExecutor) Exec(f func() error) error {
	if s == nil || s.interval == 0 {
		// not enable
		return f()
	}
	s.Lock()
	defer s.Unlock()
	now := time.Now()
	if s.last != nil && now.Sub(*s.last) < s.interval {
		// The interval from the last execution is too small
		return nil
	}
	err := f()
	if err == nil {
		s.last = &now
	}
	return err
}

func (s *MinimumIntervalExecutor) ExecSimple(f func()) {
	_ = s.Exec(func() error {
		f()
		return nil
	})
}
