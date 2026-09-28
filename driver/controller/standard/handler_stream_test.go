package standard

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// echoProcessor serves ProcessBindingsStream by reading until the client goes away. It is only here
// so PrepareExecute has a real grpc peer to open streams against.
type echoProcessor struct {
	protos.UnimplementedProcessorV3Server
}

func (echoProcessor) ProcessBindingsStream(stream protos.ProcessorV3_ProcessBindingsStreamServer) error {
	for {
		if _, err := stream.Recv(); err != nil {
			return nil
		}
	}
}

// newTestProcessorClient starts an in-process ProcessorV3 server and returns a client for it.
func newTestProcessorClient(t *testing.T) protos.ProcessorV3Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	protos.RegisterProcessorV3Server(server, echoProcessor{})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return protos.NewProcessorV3Client(conn)
}

func newTestHandlerController(
	client protos.ProcessorV3Client,
) *BaseHandlerController[controller.Client, controller.BlockHeader, HandlerAgent[controller.BlockHeader]] {
	return &BaseHandlerController[controller.Client, controller.BlockHeader, HandlerAgent[controller.BlockHeader]]{
		processorClients: []protos.ProcessorV3Client{client},
	}
}

// TestFinishExecuteTerminatesStreams pins down that FinishExecute really ends the streams instead of
// only half-closing them: CloseSend leaves the stream context live, which is what used to keep grpc's
// per-stream goroutine parked forever.
func TestFinishExecuteTerminatesStreams(t *testing.T) {
	c := newTestHandlerController(newTestProcessorClient(t))

	require.Nil(t, c.PrepareExecute(context.Background()))

	// hold on to one stream so its context can be inspected after the pool is torn down
	stream := <-c.processStreams
	c.processStreams <- stream
	require.NoError(t, stream.Context().Err(), "stream context should still be live before FinishExecute")

	c.FinishExecute()

	require.Error(t, stream.Context().Err(), "FinishExecute must cancel the stream context")
	require.Nil(t, c.cancelProcessStreams, "FinishExecute should not cancel the same pool twice")
}

// TestPrepareExecuteDoesNotLeakGoroutines covers the actual regression: a driver re-runs Prologue
// every time the processor reports a new template, so a per-run stream leak accumulates without
// bound. Each cycle opens ProcessConcurrency streams, so a leak shows up as growth proportional to
// the number of cycles.
func TestPrepareExecuteDoesNotLeakGoroutines(t *testing.T) {
	c := newTestHandlerController(newTestProcessorClient(t))
	ctx := context.Background()

	// one warm-up cycle, so the connection and grpc's own goroutines are not counted as growth
	require.Nil(t, c.PrepareExecute(ctx))
	c.FinishExecute()

	const cycles = 10
	streamsPerCycle := int(controller.ProcessConcurrency)
	before := goroutineBaseline()

	for i := 0; i < cycles; i++ {
		require.Nil(t, c.PrepareExecute(ctx))
		c.FinishExecute()
	}

	// allow one cycle's worth of slack for goroutines grpc has not reaped yet
	after := settledGoroutines(t, before+streamsPerCycle)
	require.LessOrEqual(t, after, before+streamsPerCycle,
		"leaked roughly %d goroutines per cycle, expected the streams to be released",
		(after-before)/cycles)
}

// goroutineBaseline reports the goroutine count once it stops moving, so leftovers from the warm-up
// cycle are not mistaken for growth.
func goroutineBaseline() int {
	last := -1
	for i := 0; i < 100; i++ {
		runtime.GC()
		if n := runtime.NumGoroutine(); n == last {
			return n
		} else {
			last = n
		}
		time.Sleep(20 * time.Millisecond)
	}
	return last
}

// settledGoroutines waits for the goroutine count to fall to target before returning it, so the test
// does not depend on how promptly grpc tears its stream goroutines down.
func settledGoroutines(t *testing.T, target int) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n <= target || time.Now().After(deadline) {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
}
