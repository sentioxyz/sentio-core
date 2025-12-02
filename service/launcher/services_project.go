package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/rpc"
	projectservice "sentioxyz/sentio-core/service/project"
	projectprotos "sentioxyz/sentio-core/service/project/protos"
	"sentioxyz/sentio-core/service/project/repository"
)

// ProjectServiceFactory implements the Service interface for the project service
type ProjectServiceFactory struct{}

// NewProjectServiceFactory creates a new project service factory
func NewProjectServiceFactory() Service {
	return &ProjectServiceFactory{}
}

// Create creates a new project service instance
func (ps *ProjectServiceFactory) Create(name string, serviceConfig *ServiceConfig, sharedConfig *SharedConfig) (ServiceInstance, error) {
	return &ProjectServiceInstance{
		name:          name,
		serviceConfig: serviceConfig,
		sharedConfig:  sharedConfig,
		status:        StatusStopped,
	}, nil
}

// ProjectServiceInstance represents a running project service instance
type ProjectServiceInstance struct {
	name          string
	serviceConfig *ServiceConfig
	sharedConfig  *SharedConfig
	status        ServiceStatus
	projectSvc    *projectservice.ProjectService
	cancelFunc    context.CancelFunc
	mutex         sync.RWMutex
}

// Initialize initializes the project service dependencies
func (psi *ProjectServiceInstance) Initialize(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusRunning {
		return fmt.Errorf("service %s is already initialized", psi.name)
	}

	psi.status = StatusStarting
	log.Infof("Initializing service %s", psi.name)

	// Setup Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     psi.sharedConfig.Redis.URL,
		Password: psi.sharedConfig.Redis.Password,
		DB:       psi.sharedConfig.Redis.DB,
	})

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		psi.status = StatusError
		return errors.Wrapf(err, "failed to connect to Redis")
	}
	log.Infof("Connected to Redis at %s", psi.sharedConfig.Redis.URL)

	// Create Redis repository
	projectRepo := repository.NewRedisProjectRepository(redisClient)
	// Create project service
	projectSvc := projectservice.NewProjectService(projectRepo)

	// Store project service for later use
	psi.projectSvc = projectSvc

	psi.status = StatusStopped
	log.Infof("%s initialized successfully", psi.name)

	return nil
}

// Register registers the project service on the provided gRPC server and HTTP mux
func (psi *ProjectServiceInstance) Register(grpcServer *grpc.Server, mux *runtime.ServeMux, httpPort int) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.projectSvc == nil {
		return fmt.Errorf("project service %s not initialized", psi.name)
	}

	// Register project service on the gRPC server
	projectprotos.RegisterProjectServiceServer(grpcServer, psi.projectSvc)

	err := projectprotos.RegisterProjectServiceHandlerFromEndpoint(context.Background(),
		mux,
		fmt.Sprintf(":%d", httpPort),
		rpc.GRPCGatewayDialOptions)
	if err != nil {
		return err
	}

	log.Infof("%s registered on gRPC server and HTTP mux", psi.name)
	return nil
}

// Start starts any background processes for the project service
func (psi *ProjectServiceInstance) Start(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusRunning {
		return fmt.Errorf("service %s is already running", psi.name)
	}

	psi.status = StatusRunning
	return nil
}

// Stop stops the project service
func (psi *ProjectServiceInstance) Stop(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusStopped {
		return nil
	}

	psi.status = StatusStopping
	log.Infof("Stopping project service %s", psi.name)

	// Cancel the service context
	if psi.cancelFunc != nil {
		psi.cancelFunc()
	}

	psi.status = StatusStopped
	log.Infof("Project service %s stopped", psi.name)

	return nil
}

// Status returns the current status of the service
func (psi *ProjectServiceInstance) Status() string {
	psi.mutex.RLock()
	defer psi.mutex.RUnlock()
	return string(psi.status)
}

// Name returns the name of the service instance
func (psi *ProjectServiceInstance) Name() string {
	return psi.name
}

// Type returns the type of the service
func (psi *ProjectServiceInstance) Type() string {
	return "project"
}
