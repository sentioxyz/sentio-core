package controller

import (
	"context"
	"time"

	"sentioxyz/sentio-core/service/processor/models"
)

// TaskInfo identifies the handler task an event is attributed to.
type TaskInfo struct {
	Processor  *models.Processor
	ChainID    string
	Handler    string
	Category   string
	DataSource string
}

// Notifier is the controller's outbound side channel. Each method describes
// something that happened in the controller; what to do with it (record an
// OpenTelemetry metric, call the processor service, ...) is up to the
// implementation, which lives in the driver binary. This keeps the controller
// free of the binary's metric and service-client packages so it can be hosted
// in sentio-core.
//
// No method returns an error: the controller treats notifications as
// best-effort, so the implementation logs and swallows any failure.
//
// The implementation is installed once at startup via SetNotifier.
type Notifier interface {
	// Chain lifecycle.

	// DriverCreated reports that a chain's main controller has been created (once
	// per chain). get returns the latest processed block number and whether it is
	// available yet, for continuous observation.
	DriverCreated(processor *models.Processor, chainID string, get func() (int64, bool))
	// DriverStarted reports that a chain's main stream has (re)started, carrying
	// the current instance count of each template keyed by template id.
	DriverStarted(ctx context.Context, processor *models.Processor, chainID string, templateInstanceCounts map[int32]int)
	// ReorgDetected reports that a chain reorg was detected while fetching blocks.
	ReorgDetected(ctx context.Context, processor *models.Processor, chainID string)
	// ReorgDone reports that a detected reorg has been applied: the chain state was
	// rolled back and persisted. reorgBlocks is the number of blocks rolled back;
	// reduceToBlock is the processed block number afterwards (-1 if the rollback
	// went below the start of the processed range).
	ReorgDone(ctx context.Context, processor *models.Processor, chainID string, reorgBlocks uint64, reduceToBlock int64)

	// Per task.

	// BeforeEntityOperation returns a context tagged with the task, so the entity
	// store can attribute the events it reports while serving the task.
	BeforeEntityOperation(ctx context.Context, task TaskInfo) context.Context
	// TaskDone reports that a handler task finished.
	TaskDone(ctx context.Context, task TaskInfo, succeed bool, used time.Duration)
	// SubgraphTaskDone reports that a subgraph handler task finished, together with
	// the extra resource usage a subgraph task exposes: the time spent in each
	// import function and the wasm memory used.
	SubgraphTaskDone(ctx context.Context, task TaskInfo, succeed bool, used time.Duration,
		importFuncUsed map[string]time.Duration, memoryUsed uint32)
	// SubgraphRPCDone reports that an RPC call issued by a subgraph handler finished.
	SubgraphRPCDone(ctx context.Context, task TaskInfo, succeed bool, used time.Duration)
	// DataEmitted reports data a task produced. dataType/subtype/name describe the
	// data point, e.g. ("event", "", name), ("metric", "gauge", name) or
	// ("entity", op, name).
	DataEmitted(ctx context.Context, task TaskInfo, dataType, subtype, name string, count int64)

	// On commit.

	// DataSaved reports data persisted for a processor on a chain. dataType/subtype/name
	// describe the data point as in DataEmitted.
	DataSaved(ctx context.Context, processor *models.Processor, chainID, dataType, subtype, name string, count int64)
}

// N is the process-wide notifier. It defaults to a no-op so that tests and tools
// that build controllers without a driver binary do not panic; the driver binary
// installs the real implementation via SetNotifier before any controller runs.
var N Notifier = noopNotifier{}

// SetNotifier installs the process-wide notifier. Called once by the driver
// binary at startup.
func SetNotifier(n Notifier) {
	if n != nil {
		N = n
	}
}

type noopNotifier struct{}

func (noopNotifier) DriverCreated(*models.Processor, string, func() (int64, bool))           {}
func (noopNotifier) DriverStarted(context.Context, *models.Processor, string, map[int32]int) {}
func (noopNotifier) ReorgDetected(context.Context, *models.Processor, string)                {}
func (noopNotifier) ReorgDone(context.Context, *models.Processor, string, uint64, int64)     {}
func (noopNotifier) BeforeEntityOperation(ctx context.Context, _ TaskInfo) context.Context {
	return ctx
}
func (noopNotifier) TaskDone(context.Context, TaskInfo, bool, time.Duration) {}
func (noopNotifier) SubgraphTaskDone(context.Context, TaskInfo, bool, time.Duration, map[string]time.Duration, uint32) {
}
func (noopNotifier) SubgraphRPCDone(context.Context, TaskInfo, bool, time.Duration)       {}
func (noopNotifier) DataEmitted(context.Context, TaskInfo, string, string, string, int64) {}
func (noopNotifier) DataSaved(context.Context, *models.Processor, string, string, string, string, int64) {
}
