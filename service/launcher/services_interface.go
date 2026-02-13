package launcher

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

// Service represents a service factory that can create service instances
type Service interface {
	// Create creates a new service instance with the given configuration
	Create(name string, serviceConfig *ServiceConfig, sharedConfig *SharedConfig) (ServiceInstance, error)
}

// ServiceInstance represents a running service instance
type ServiceInstance interface {
	// Register registers the service on the provided gRPC server and HTTP mux
	Register(grpcServer *grpc.Server, mux *runtime.ServeMux, httpPort int) error

	// Initialize initializes the service (setup dependencies, etc.)
	Initialize(ctx context.Context) error

	// Start starts any background processes for the service
	Start(ctx context.Context) error

	// Stop stops the service gracefully
	Stop(ctx context.Context) error

	// Status returns the current status of the service
	Status() string

	// Name returns the name of the service instance
	Name() string

	// Type returns the type of the service
	Type() string
}

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	StatusStarting ServiceStatus = "starting"
	StatusRunning  ServiceStatus = "running"
	StatusStopping ServiceStatus = "stopping"
	StatusStopped  ServiceStatus = "stopped"
	StatusError    ServiceStatus = "error"
)
