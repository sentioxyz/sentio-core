package driverjob

import (
	"context"
	"time"

	"sentioxyz/sentio-core/service/processor/models"
)

// DriverJobManager defines the interface for managing driver jobs
type DriverJobManager interface {
	// StartJob starts a driver job for the given processor
	StartJob(ctx context.Context, processor *models.Processor) error

	// RestartJob restarts a driver job for the given processor
	RestartJob(ctx context.Context, processor *models.Processor) error

	// DeleteJob deletes a driver job for the given processor
	DeleteJob(ctx context.Context, processor *models.Processor) error

	// StartOrUpdateDriverJob creates or updates a driver job for the given processor
	StartOrUpdateDriverJob(ctx context.Context, processor *models.Processor) error

	// StartManager starts the k8s manager for watching DriverJob resources
	StartManager(ctx context.Context) error

	// IsProcessorRunning checks if the processor is currently running by examining the driver job status
	IsProcessorRunning(processorID string, k8sClusterID int) bool

	// RestartProcessorByID restarts a specific processor on the given cluster
	RestartProcessorByID(ctx context.Context, processorID string, k8sClusterID int) error

	// RestartWaitingProcessorsByOwner restarts waiting processors owned by the given owner
	RestartWaitingProcessorsByOwner(ctx context.Context, ownerID, ownerType string) error

	// HasCluster checks if the given cluster ID exists
	HasCluster(clusterID int) bool

	// ListDriverJobPodsByProcessor lists driver job pods for the given processor
	ListDriverJobPodsByProcessor(ctx context.Context, processorID string) (map[int][]Pod, error)

	// GetNamespaceForCluster returns the namespace for the given cluster ID
	GetNamespaceForCluster(clusterID int) string

	// GetLogs fetches logs for the given processor
	GetLogs(ctx context.Context, processor *models.Processor, limit int32, after, query string) ([]Log, string, error)
}

type Log interface {
	Message() string
	Timestamp() time.Time
}

type Pod interface {
	Name() string
	Namespace() string
	PodIP() string
}
