package controller

import (
	"fmt"
	"strconv"
	"strings"
)

// processedSegment is a run of consecutive blocks that all have bindings.
type processedSegment struct {
	from, to uint64
	bindings []uint64
}

// processedWindow accumulates the blocks processed since the last "Processed" log line. Blocks without bindings
// only count towards the window range, blocks with bindings keep their individual binding count so the line can
// list them as "[from-to][b1 b2 ...]+[block][b]+...". The number of blocks with bindings per line is capped by
// PrintProcessedMaxBindingBlocks, and the window is always flushed when a save starts.
type processedWindow struct {
	last          Checkpoint // the last block added, provides the rate and the block range of the line
	blocks        uint64
	from          uint64
	totalBindings uint64
	bindingBlocks uint64
	segments      []processedSegment
}

func (w *processedWindow) add(ck Checkpoint) {
	if w.blocks == 0 {
		w.from = ck.BlockNumber
	}
	w.blocks++
	w.last = ck
	if ck.TotalBindings == 0 {
		return
	}
	w.totalBindings += ck.TotalBindings
	w.bindingBlocks++
	if n := len(w.segments); n > 0 && w.segments[n-1].to+1 == ck.BlockNumber {
		w.segments[n-1].to = ck.BlockNumber
		w.segments[n-1].bindings = append(w.segments[n-1].bindings, ck.TotalBindings)
		return
	}
	w.segments = append(w.segments, processedSegment{
		from:     ck.BlockNumber,
		to:       ck.BlockNumber,
		bindings: []uint64{ck.TotalBindings},
	})
}

func (w *processedWindow) full() bool {
	return w.bindingBlocks >= PrintProcessedMaxBindingBlocks
}

// take renders the window as a log message and resets it.
func (w *processedWindow) take() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "Processed %s[%d/%d-%d/%d] with %d bindings in %d blocks",
		w.last.RateOrDelay(),
		w.last.FullBlockRange.StartBlock,
		w.from,
		w.last.BlockNumber,
		w.last.CurrentLastBlockNumber(),
		w.totalBindings,
		w.bindingBlocks)
	for i, seg := range w.segments {
		if i == 0 {
			buf.WriteString(": ")
		} else {
			buf.WriteByte('+')
		}
		if seg.from == seg.to {
			fmt.Fprintf(&buf, "[%d][", seg.from)
		} else {
			fmt.Fprintf(&buf, "[%d-%d][", seg.from, seg.to)
		}
		for j, b := range seg.bindings {
			if j > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString(strconv.FormatUint(b, 10))
		}
		buf.WriteByte(']')
	}
	*w = processedWindow{}
	return buf.String()
}
