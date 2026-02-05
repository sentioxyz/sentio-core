package timewin

import (
	"sentioxyz/sentio-core/common/sparsify"
	"sync"
	"time"
)

type Window[WIN any] interface {
	GetStartAt() time.Time
	Merge(WIN)
	Snapshot(endAt time.Time) any
}

type TimeWindowsManager[WIN Window[WIN]] struct {
	minWindowWidth time.Duration

	mu   sync.Mutex
	wins []WIN
}

func NewTimeWindowsManager[WIN Window[WIN]](minWindowWidth time.Duration) *TimeWindowsManager[WIN] {
	return &TimeWindowsManager[WIN]{minWindowWidth: minWindowWidth}
}

func (m *TimeWindowsManager[WIN]) shrink() {
	remove := sparsify.Remove(m.wins, func(w WIN) int64 {
		return w.GetStartAt().UnixNano() / m.minWindowWidth.Nanoseconds()
	})
	if len(remove) == 0 {
		return
	}
	n := 1
	for i := 1; i < len(m.wins); i++ {
		if remove[i] {
			m.wins[n-1].Merge(m.wins[i])
		} else {
			m.wins[n] = m.wins[i]
			n++
		}
	}
	m.wins = m.wins[:n]
}

func (m *TimeWindowsManager[WIN]) Append(w WIN) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.wins) > 0 && w.GetStartAt().Sub(m.wins[len(m.wins)-1].GetStartAt()) < m.minWindowWidth {
		m.wins[len(m.wins)-1].Merge(w)
	} else {
		m.wins = append(m.wins, w)
		m.shrink()
	}
}

func (m *TimeWindowsManager[WIN]) Snapshot() []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.wins) == 0 {
		return nil
	}
	sp := make([]any, len(m.wins))
	for i := 0; i < len(m.wins); i++ {
		if i+1 < len(m.wins) {
			sp[i] = m.wins[i].Snapshot(m.wins[i+1].GetStartAt())
		} else {
			sp[i] = m.wins[i].Snapshot(time.Now())
		}
	}
	return sp
}
