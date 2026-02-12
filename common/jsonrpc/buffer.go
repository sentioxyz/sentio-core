package jsonrpc

import (
	"sync/atomic"
	"time"
)

type BufPool struct {
	size       uint64
	maxWaiting time.Duration

	pool chan []byte

	newCap atomic.Int64
	using  atomic.Int64

	overSizePut atomic.Uint64
	getCount    atomic.Uint64
	getUsed     atomic.Uint64
	maxPutSize  atomic.Uint64
}

func NewBufPool(size, initCap, maxCap int, maxWaiting time.Duration) *BufPool {
	bp := &BufPool{
		size:       uint64(size),
		maxWaiting: maxWaiting,
		pool:       make(chan []byte, maxCap),
	}
	for i := 0; i < initCap; i++ {
		bp.pool <- make([]byte, 0, size)
	}
	bp.newCap.Swap(int64(maxCap - initCap))
	return bp
}

func (bp *BufPool) Get() []byte {
	startAt := time.Now()
	defer func() {
		bp.using.Add(1)
		bp.getCount.Add(1)
		bp.getUsed.Add(uint64(time.Since(startAt)))
	}()
	if bp.newCap.Load() <= 0 {
		return <-bp.pool
	}
	var b []byte
	for b == nil {
		t := time.NewTimer(bp.maxWaiting)
		select {
		case b = <-bp.pool:
		case <-t.C:
			if bp.newCap.Add(-1) >= 0 {
				b = make([]byte, 0, bp.size)
			}
		}
		t.Stop()
	}
	return b
}

func (bp *BufPool) Put(b []byte) {
	if putCap := uint64(cap(b)); putCap > bp.size {
		bp.overSizePut.Add(1)
		pre := bp.maxPutSize.Load()
		for pre < putCap {
			if bp.maxPutSize.CompareAndSwap(pre, putCap) {
				break
			}
			pre = bp.maxPutSize.Load()
		}
		bp.pool <- make([]byte, 0, bp.size)
	} else {
		bp.pool <- b[:0]
	}
	bp.using.Add(-1)
}

func (bp *BufPool) Snapshot() any {
	return map[string]any{
		"config": map[string]any{
			"size":       bp.size,
			"maxWaiting": bp.maxWaiting.String(),
			"maxCap":     cap(bp.pool),
		},
		"current": map[string]any{
			"having": len(bp.pool),
			"using":  bp.using.Load(),
			"newCap": bp.newCap.Load(),
		},
		"history": map[string]any{
			"getCount":      bp.getCount.Load(),
			"getUsed":       time.Duration(bp.getUsed.Load()).String(),
			"overSizeCount": bp.overSizePut.Load(),
			"maxPutSize":    bp.maxPutSize.Load(),
		},
	}
}
