package pool

import (
	"context"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/utils"
	"sync"
)

type Status interface {
	Snapshot() any
}

type EntryConfig[C any] interface {
	Equal(ano C) bool
}

type Entry[EC any, ES any] struct {
	Config EC
	Status ES
}

type entryInPool[EC any, ES any] struct {
	Entry[EC, ES]

	refreshCancel context.CancelFunc
	refreshDone   chan struct{}
}

type Pool[EC EntryConfig[EC], ES Status, PS Status] struct {
	name string

	poolStatusBuilder    func(map[string]Entry[EC, ES]) PS
	entryStatusRefresher func(context.Context, EC, chan<- ES)

	mu      sync.Mutex
	entries map[string]*entryInPool[EC, ES]

	status       PS
	statusIndex  uint64
	statusWaiter *concurrency.StatusWaiter[uint64]
}

func NewPool[EC EntryConfig[EC], ES Status, PS Status](
	name string,
	poolStatusBuilder func(map[string]Entry[EC, ES]) PS,
	entryStatusRefresher func(context.Context, EC, chan<- ES),
) *Pool[EC, ES, PS] {
	return &Pool[EC, ES, PS]{
		name:                 name,
		poolStatusBuilder:    poolStatusBuilder,
		entryStatusRefresher: entryStatusRefresher,
		entries:              make(map[string]*entryInPool[EC, ES]),
		statusWaiter:         concurrency.NewStatusWaiter[uint64](0),
	}
}

func (p *Pool[EC, ES, PS]) refreshPoolStatus() {
	entries := make(map[string]Entry[EC, ES])
	for name, ent := range p.entries {
		entries[name] = ent.Entry
	}
	p.status = p.poolStatusBuilder(entries)
	p.statusIndex++
	p.statusWaiter.NewStatus(p.statusIndex)
}

func (p *Pool[EC, ES, PS]) Add(name string, config EC) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	exist, has := p.entries[name]
	if has && exist.Config.Equal(config) {
		// dup add, just return false
		return false
	}
	if has {
		// config updated, remove old entry first
		p.remove(name)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	ent := &entryInPool[EC, ES]{
		Entry: Entry[EC, ES]{
			Config: config,
		},
		refreshCancel: cancel,
		refreshDone:   done,
	}
	p.entries[name] = ent
	p.refreshPoolStatus()
	ch := make(chan ES)
	go func() {
		defer close(done)
		defer close(ch)
		p.entryStatusRefresher(ctx, config, ch)
	}()
	go func() {
		for status := range ch {
			p.mu.Lock()
			ent.Status = status
			p.refreshPoolStatus() // here may be p.entries[name] was deleted
			p.mu.Unlock()
		}
	}()
	return true
}

func (p *Pool[EC, ES, PS]) remove(name string) bool {
	if ent, has := p.entries[name]; has {
		ent.refreshCancel()
		<-ent.refreshDone
		delete(p.entries, name)
		return true
	}
	return false
}

func (p *Pool[EC, ES, PS]) Remove(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.remove(name) {
		p.refreshPoolStatus()
		return true
	}
	return false
}

func (p *Pool[EC, ES, PS]) RemoveAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name := range p.entries {
		p.remove(name)
	}
	p.refreshPoolStatus()
}

func (p *Pool[EC, ES, PS]) Fetch(
	checker func(name string, entry Entry[EC, ES], poolStatus PS) bool,
) (map[string]Entry[EC, ES], PS, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[string]Entry[EC, ES])
	for name, ent := range p.entries {
		if checker(name, ent.Entry, p.status) {
			result[name] = ent.Entry
		}
	}
	return result, p.status, p.statusIndex
}

func (p *Pool[EC, ES, PS]) Wait(ctx context.Context, statusIndexGT uint64) error {
	_, err := p.statusWaiter.Wait(ctx, func(statusIndex uint64) bool {
		return statusIndex > statusIndexGT
	})
	return err
}

func (p *Pool[EC, ES, PS]) Snapshot() any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return map[string]any{
		"name":        p.name,
		"status":      p.status.Snapshot(),
		"statusIndex": p.statusIndex,
		"entries": utils.MapMapNoError(p.entries, func(ent *entryInPool[EC, ES]) map[string]any {
			return map[string]any{
				"config": ent.Config,
				"status": ent.Status.Snapshot(),
			}
		}),
	}
}
