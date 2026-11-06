package sui

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	"sentioxyz/sentio-core/common/memsize"
)

func decodedCheckpoint(t *testing.T, seq uint64) *rpcv2.Checkpoint {
	bcs := func(n int) *rpcv2.Bcs {
		return &rpcv2.Bcs{Name: proto.String("bcs"), Value: []byte(strings.Repeat("b", n))}
	}
	src := &rpcv2.Checkpoint{
		SequenceNumber: proto.Uint64(seq),
		Digest:         proto.String(fmt.Sprintf("checkpoint-digest-%d", seq)),
		Summary:        &rpcv2.CheckpointSummary{Bcs: bcs(300), Epoch: proto.Uint64(900)},
	}
	for i := 0; i < 40; i++ {
		tx := &rpcv2.ExecutedTransaction{
			Digest:      proto.String(fmt.Sprintf("tx-digest-%d-%d", seq, i)),
			Transaction: &rpcv2.Transaction{Bcs: bcs(400 + i), Digest: proto.String(fmt.Sprintf("tx-%d", i))},
			Effects: &rpcv2.TransactionEffects{
				Bcs:     bcs(200),
				GasUsed: &rpcv2.GasCostSummary{ComputationCost: proto.Uint64(1000), StorageCost: proto.Uint64(2000)},
			},
			Events: &rpcv2.TransactionEvents{},
		}
		for j := 0; j < 5; j++ {
			tx.Events.Events = append(tx.Events.Events, &rpcv2.Event{
				PackageId: proto.String(fmt.Sprintf("0x%064x", j)),
				Module:    proto.String("pool"),
				Sender:    proto.String(fmt.Sprintf("0x%064x", i)),
				EventType: proto.String(fmt.Sprintf("0x%064x::pool::SwapEvent", j)),
				Contents:  bcs(150),
			})
		}
		src.Transactions = append(src.Transactions, tx)
	}
	raw, err := proto.Marshal(src)
	assert.NoError(t, err)
	var cp rpcv2.Checkpoint
	assert.NoError(t, proto.Unmarshal(raw, &cp))
	return &cp
}

func TestMemsizeOfDecodedCheckpoint(t *testing.T) {
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	const n = 30
	_ = decodedCheckpoint(t, 0) // initialize the shared protobuf type info out of the measurement
	keep := make([]*Slot, n)
	before := heap()
	for i := range keep {
		keep[i] = &Slot{GrpcCheckpoint: decodedCheckpoint(t, uint64(i))}
	}
	real := float64(heap()-before) / n
	var est float64
	for _, s := range keep {
		est += float64(memsize.Of(s))
	}
	est /= n
	runtime.KeepAlive(keep)
	t.Logf("real=%.0f estimate=%.0f estimate/real=%.3f", real, est, est/real)
	assert.InDelta(t, 1, est/real, 0.1)
}

func TestMemsizeOfCheckpointConcurrentWithMarshal(t *testing.T) {
	// the shared protobuf type info is initialized lazily on first use of each message type,
	// run with -race to check sizing a checkpoint never reads it
	slot := &Slot{GrpcCheckpoint: decodedCheckpoint(t, 1)}
	others := []proto.Message{
		&rpcv2.Object{ObjectId: proto.String("0x1"), Version: proto.Uint64(1)},
		&rpcv2.ObjectSet{Objects: []*rpcv2.Object{{ObjectId: proto.String("0x2")}}},
		&rpcv2.Owner{Address: proto.String("0x3")},
		&rpcv2.ChangedObject{ObjectId: proto.String("0x4")},
		&rpcv2.ValidatorAggregatedSignature{Epoch: proto.Uint64(1)},
		&rpcv2.CheckpointContents{Digest: proto.String("d")},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, m := range others {
			_, _ = proto.Marshal(m)
			_ = proto.Size(m)
		}
	}()
	for {
		select {
		case <-done:
			assert.Positive(t, memsize.Of(slot))
			return
		default:
			assert.Positive(t, memsize.Of(slot))
		}
	}
}
