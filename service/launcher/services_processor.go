package main

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/service/processor/driverjob"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/rpc"
	"sentioxyz/sentio-core/service/common/storagesystem"
	processorservice "sentioxyz/sentio-core/service/processor"
	coreprotos "sentioxyz/sentio-core/service/processor/protos"
	"sentioxyz/sentio-core/service/processor/repository"
	"sentioxyz/sentio-core/service/processor/storage"
)

// ProcessorService implements the Service interface for the processor service
type ProcessorService struct{}

// NewProcessorService creates a new processor service factory
func NewProcessorService() Service {
	return &ProcessorService{}
}

// Create creates a new processor service instance
func (ps *ProcessorService) Create(name string, serviceConfig *ServiceConfig, sharedConfig *SharedConfig) (ServiceInstance, error) {
	return &ProcessorServiceInstance{
		name:          name,
		serviceConfig: serviceConfig,
		sharedConfig:  sharedConfig,
		status:        StatusStopped,
	}, nil
}

// ProcessorServiceInstance represents a running processor service instance
type ProcessorServiceInstance struct {
	name              string
	serviceConfig     *ServiceConfig
	sharedConfig      *SharedConfig
	status            ServiceStatus
	processorSvc      *processorservice.Service
	graphNodeService  interface{}
	fileStorageSystem storagesystem.FileStorageSystemInterface
	cancelFunc        context.CancelFunc
	mutex             sync.RWMutex
}

// Initialize initializes the processor service dependencies
func (psi *ProcessorServiceInstance) Initialize(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusRunning {
		return fmt.Errorf("service %s is already initialized", psi.name)
	}

	psi.status = StatusStarting
	log.Infof("Initializing processor service %s", psi.name)

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

	// Create Redis repository factory
	repoFactory := repository.NewRedisRepositoryFactoryWithClient(redisClient)
	processorRepo := repoFactory.CreateProcessorRepo()
	chainStateRepo := repoFactory.CreateChainStateRepo()
	log.Infof("Created Redis repositories for processor service %s", psi.name)

	// Setup storage system based on configuration
	var fileStorageSystem storagesystem.FileStorageSystemInterface
	storageType := psi.sharedConfig.Storage.DefaultStorage
	if storageType == "" {
		storageType = "ipfs" // Default to IPFS for backward compatibility
	}

	switch storageType {
	case "local", "localstorage": // Support both naming conventions
		// Setup local storage system (HTTP handlers registered via localstorage service)
		localStoragePath := psi.sharedConfig.Storage.LocalStoragePath
		localStorageSystem, err := storage.NewLocalStorageSystem(storage.LocalStorageSystemConfig{
			LocalStorageConfig: storage.LocalStorageConfig{
				BasePath: localStoragePath,
				BaseURL:  psi.sharedConfig.Storage.LocalStorageBaseURL,
			},
		})
		if err != nil {
			psi.status = StatusError
			return errors.Wrapf(err, "failed to create local storage system")
		}

		fileStorageSystem = localStorageSystem
		log.Infof("Created local storage system with path: %s", localStoragePath)

	case "ipfs":
		// Setup IPFS storage system
		ipfsStorageSystem, err := storage.NewDefaultIPFSStorageSystem(storage.DefaultIPFSStorageSystemConfig{
			IPFSConfig: storage.IPFSConfig{
				ApiURL:     psi.sharedConfig.Storage.IpfsApiUrl,
				GatewayURL: psi.sharedConfig.Storage.IpfsGatewayUrl,
			},
		})
		if err != nil {
			psi.status = StatusError
			return errors.Wrapf(err, "failed to create IPFS storage system")
		}
		fileStorageSystem = ipfsStorageSystem
		log.Infof("Created IPFS storage system with API: %s, Gateway: %s",
			psi.sharedConfig.Storage.IpfsApiUrl,
			psi.sharedConfig.Storage.IpfsGatewayUrl)

	default:
		psi.status = StatusError
		return fmt.Errorf("unsupported storage type: %s (supported: local, ipfs)", storageType)
	}

	driverJobManager, err := driverjob.NewDockerSwarmManager(ctx, psi.sharedConfig.Driver)
	if err != nil {
		psi.status = StatusError
		return errors.Wrapf(err, "failed to create driver job manager")
	}

	processorFactory := processorservice.NewDefaultProcessorFactory(processorRepo)

	// Create processor service
	processorSvc := processorservice.NewService(
		processorRepo,
		chainStateRepo,
		driverJobManager,
		fileStorageSystem,
		processorFactory,
		nil, // lifecycleHook
		redisClient,
	)

	// Store processor service for later use
	psi.processorSvc = processorSvc
	psi.graphNodeService = nil

	// Store file storage system for later use
	psi.fileStorageSystem = fileStorageSystem

	psi.status = StatusStopped
	log.Infof("%s initialized successfully", psi.name)

	return nil
}

// Register registers the processor service on the provided gRPC server and HTTP mux
func (psi *ProcessorServiceInstance) Register(grpcServer *grpc.Server, mux *runtime.ServeMux, httpPort int) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.processorSvc == nil {
		return fmt.Errorf("service %s not initialized", psi.name)
	}

	// Register processor service on the gRPC server
	coreprotos.RegisterProcessorServiceServer(grpcServer, psi.processorSvc)
	err := coreprotos.RegisterProcessorServiceHandlerFromEndpoint(context.Background(),
		mux,
		fmt.Sprintf(":%d", httpPort),
		rpc.GRPCGatewayDialOptions)
	if err != nil {
		return err
	}

	// Register processor runtime service on the gRPC server
	coreprotos.RegisterProcessorRuntimeServiceServer(grpcServer, psi.processorSvc)
	return coreprotos.RegisterProcessorRuntimeServiceHandlerFromEndpoint(context.Background(),
		mux,
		fmt.Sprintf(":%d", httpPort),
		rpc.GRPCGatewayDialOptions)

}

// Start starts any background processes for the processor service
func (psi *ProcessorServiceInstance) Start(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusRunning {
		return fmt.Errorf("service %s is already running", psi.name)
	}

	// Create context for background processes
	_, cancel := context.WithCancel(ctx)
	psi.cancelFunc = cancel

	// Start GraphNodeService if enabled
	/*	if cfg.GraphNodeService.Enabled && cfg.GraphNodeService.Port > 0 && psi.graphNodeService != nil {
		if graphNodeSvc, ok := psi.graphNodeService.(*processorservice.GraphNodeService); ok {
			go func() {
				bufPool := jsonrpc.NewBufPool(64<<10, 100, 10000, time.Second)
				handler := jsonrpc.NewHandler("graph-node", false, bufPool, nil, nil)
				handler.Register("subgraph", "1.0", graphNodeSvc)
				h := monitoring.NewWrappedHandler(handler, "subgraph")
				addr := fmt.Sprintf(":%d", cfg.GraphNodeService.Port)
				sgCtx, _ := log.FromContext(serviceCtx, "svrName", "subgraph")
				if err := jsonrpc.ListenAndServe(sgCtx, addr, h); err != nil {
					log.Errorf("GraphNode service for service %s failed: %v", psi.name, err)
				}
			}()
			log.Infof("GraphNode service started on port %d for service %s", cfg.GraphNodeService.Port, psi.name)
		}
	}*/

	psi.status = StatusRunning
	log.Infof("Service %s started successfully", psi.name)

	return nil
}

// Stop stops the processor service
func (psi *ProcessorServiceInstance) Stop(ctx context.Context) error {
	psi.mutex.Lock()
	defer psi.mutex.Unlock()

	if psi.status == StatusStopped {
		return nil
	}

	psi.status = StatusStopping
	log.Infof("Stopping processor service %s", psi.name)

	// Cancel the service context
	if psi.cancelFunc != nil {
		psi.cancelFunc()
	}

	psi.status = StatusStopped
	log.Infof("Processor service %s stopped", psi.name)

	return nil
}

// Status returns the current status of the service
func (psi *ProcessorServiceInstance) Status() string {
	psi.mutex.RLock()
	defer psi.mutex.RUnlock()
	return string(psi.status)
}

// Name returns the name of the service instance
func (psi *ProcessorServiceInstance) Name() string {
	return psi.name
}

// Type returns the type of the service
func (psi *ProcessorServiceInstance) Type() string {
	return "processor"
}

// ProcessorConfig represents processor-specific configuration
type ProcessorConfig struct {
	GraphNodeService GraphNodeServiceConfig `yaml:"graph_node_service"`
}

// GraphNodeServiceConfig represents configuration for the GraphNode service
type GraphNodeServiceConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// extractConfig extracts processor-specific configuration
func (psi *ProcessorServiceInstance) extractConfig() (*ProcessorConfig, error) {
	cfg := &ProcessorConfig{
		GraphNodeService: GraphNodeServiceConfig{
			Enabled: false, // Default disabled
			Port:    10090, // Default port from the research
		},
	}

	// Extract configuration from service config map
	if psi.serviceConfig.Config != nil {
		// Extract GraphNodeService configuration
		if val, ok := psi.serviceConfig.Config["graph_node_service"]; ok {
			if graphNodeConfig, ok := val.(map[string]interface{}); ok {
				if enabled, ok := graphNodeConfig["enabled"].(bool); ok {
					cfg.GraphNodeService.Enabled = enabled
				}
				if port, ok := graphNodeConfig["port"].(int); ok {
					cfg.GraphNodeService.Port = port
				}
			}
		}
	}

	return cfg, nil
}
