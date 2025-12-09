package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/repository"
	"sentioxyz/sentio-core/service/common/rpc"
)

// ServiceManager manages the lifecycle of multiple servers and their services
type ServiceManager struct {
	config   *Config
	services map[string]Service
	servers  map[string]*ServerInstance
	mutex    sync.RWMutex
}

// ServerInstance represents a running gRPC server with its services
type ServerInstance struct {
	Name         string
	Config       *ServerConfig
	Services     map[string]ServiceInstance
	Status       string
	grpcServer   *grpc.Server
	httpMux      *runtime.ServeMux
	grpcListener net.Listener
	httpListener net.Listener
	httpServer   *http.Server
	cancelFunc   context.CancelFunc
	mutex        sync.RWMutex
}

// NewServiceManager creates a new service manager
func NewServiceManager(cfg *Config) *ServiceManager {
	return &ServiceManager{
		config:   cfg,
		services: make(map[string]Service),
		servers:  make(map[string]*ServerInstance),
	}
}

// StartAll starts all enabled servers and their services
func (sm *ServiceManager) StartAll(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Register available service types
	if err := sm.registerServices(); err != nil {
		return errors.Wrapf(err, "failed to register services")
	}

	// Start enabled servers
	var wg sync.WaitGroup
	errChan := make(chan error, len(sm.config.Servers))

	for _, serverConfig := range sm.config.Servers {
		if !serverConfig.Enabled {
			log.Infof("Server %s is disabled, skipping", serverConfig.Name)
			continue
		}

		wg.Add(1)
		go func(cfg ServerConfig) {
			defer wg.Done()
			if err := sm.startServer(ctx, cfg); err != nil {
				errChan <- errors.Wrapf(err, "failed to start server %s", cfg.Name)
			}
		}(serverConfig)
	}

	// Wait for all servers to start
	go func() {
		wg.Wait()
		close(errChan)
	}()
	var startErrors []error
	// Check for errors
	for err := range errChan {
		if err != nil {
			log.Errorfe(err, "Server startup error")
			// Don't return immediately, let other servers start
		}
		startErrors = append(startErrors, err)
	}
	if len(startErrors) > 0 {
		return fmt.Errorf("errors occurred while starting servers: %v", startErrors)
	}

	return nil
}

// StopAll stops all running servers and their services
func (sm *ServiceManager) StopAll(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(sm.servers))

	for name, server := range sm.servers {
		wg.Add(1)
		go func(serverName string, serverInstance *ServerInstance) {
			defer wg.Done()

			log.Infof("Stopping server %s...", serverName)
			if err := sm.stopServer(ctx, serverInstance); err != nil {
				errChan <- errors.Wrapf(err, "failed to stop server %s", serverName)
			} else {
				log.Infof("Server %s stopped", serverName)
			}
		}(name, server)
	}

	// Wait for all servers to stop
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors
	var stopErrors []error
	for err := range errChan {
		if err != nil {
			log.Errorf("Server stop error: %v", err)
			stopErrors = append(stopErrors, err)
		}
	}

	// Clear servers
	sm.servers = make(map[string]*ServerInstance)

	if len(stopErrors) > 0 {
		return fmt.Errorf("errors occurred while stopping servers: %v", stopErrors)
	}

	log.Info("All servers stopped")
	return nil
}

// RestartServer restarts a specific server and all its services
func (sm *ServiceManager) RestartServer(ctx context.Context, serverName string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Find server config
	var serverConfig *ServerConfig
	for _, cfg := range sm.config.Servers {
		if cfg.Name == serverName {
			serverConfig = &cfg
			break
		}
	}

	if serverConfig == nil {
		return fmt.Errorf("server %s not found in configuration", serverName)
	}

	// Stop existing server if running
	if server, exists := sm.servers[serverName]; exists {
		log.Infof("Stopping server %s for restart...", serverName)
		if err := sm.stopServer(ctx, server); err != nil {
			log.Errorf("Error stopping server %s: %v", serverName, err)
		}
		delete(sm.servers, serverName)
	}

	// Start server again
	return sm.startServer(ctx, *serverConfig)
}

// GetServerStatus returns the status of a server
func (sm *ServiceManager) GetServerStatus(serverName string) (string, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	server, exists := sm.servers[serverName]
	if !exists {
		return "stopped", nil
	}

	return server.Status, nil
}

// ListServers returns a list of all configured servers and their status
func (sm *ServiceManager) ListServers() map[string]string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]string)

	for _, cfg := range sm.config.Servers {
		if server, exists := sm.servers[cfg.Name]; exists {
			result[cfg.Name] = server.Status
		} else {
			result[cfg.Name] = "stopped"
		}
	}

	return result
}

// registerServices registers all available service types
func (sm *ServiceManager) registerServices() error {
	// Register processor service
	sm.services["processor"] = NewProcessorService()
	sm.services["localstorage"] = NewLocalStorageService()
	sm.services["project"] = NewProjectServiceFactory()

	return nil
}

func (sm *ServiceManager) configSharedDB() (*gorm.DB, error) {
	conn, err := repository.SetupDB(sm.config.Shared.Database.URL)
	return conn, err
}

// startServer starts a single server with all its services
func (sm *ServiceManager) startServer(ctx context.Context, cfg ServerConfig) error {
	log.Infof("Starting server %s on ports: %d", cfg.Name, cfg.Port)

	// Create server instance
	server := &ServerInstance{
		Name:     cfg.Name,
		Config:   &cfg,
		Services: make(map[string]ServiceInstance),
		Status:   "starting",
	}
	// Create gRPC server with interceptors
	server.grpcServer = rpc.NewServer()

	// Create HTTP mux for REST endpoints
	server.httpMux = rpc.NewServeMux()

	// Create and initialize all services
	for _, serviceConfig := range cfg.Services {
		if !serviceConfig.Enabled {
			log.Infof("Service %s in server %s is disabled, skipping", serviceConfig.Name, cfg.Name)
			continue
		}

		log.Infof("Initializing service %s (type: %s) in server %s", serviceConfig.Name, serviceConfig.Type, cfg.Name)

		// Get service factory
		serviceFactory, exists := sm.services[serviceConfig.Type]
		if !exists {
			return fmt.Errorf("unknown service type: %s", serviceConfig.Type)
		}

		// Create service instance
		instance, err := serviceFactory.Create(serviceConfig.Name, &serviceConfig, &sm.config.Shared)
		if err != nil {
			return errors.Wrapf(err, "failed to create service instance %s", serviceConfig.Name)
		}

		// Initialize the service
		initCtx, initCancel := context.WithTimeout(ctx, 30*time.Second)
		if err := instance.Initialize(initCtx); err != nil {
			initCancel()
			return errors.Wrapf(err, "failed to initialize service %s", serviceConfig.Name)
		}
		initCancel()

		// Register service on the gRPC server and HTTP mux
		if err := instance.Register(server.grpcServer, server.httpMux, cfg.Port); err != nil {
			return errors.Wrapf(err, "failed to register service %s", serviceConfig.Name)
		}

		// Store the service instance
		server.Services[serviceConfig.Name] = instance
	}

	port := cfg.Port
	if port == 0 {
		port = 10000 // Default port
	}

	server.httpServer = &http.Server{
		Handler: server.httpMux,
	}

	go func() {
		rpc.BindAndServeWithHTTP(server.httpMux, server.grpcServer, port, nil)
	}()

	// Start all service background processes
	for name, instance := range server.Services {
		startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
		if err := instance.Start(startCtx); err != nil {
			startCancel()
			return errors.Wrapf(err, "failed to start service for %s", name)
		}
		startCancel()
		log.Infof("Service %s started in server %s", name, cfg.Name)
	}

	server.Status = "running"
	sm.servers[cfg.Name] = server
	log.Infof("Server %s started successfully with %d services", cfg.Name, len(server.Services))

	return nil
}

// stopServer stops a server and all its services
func (sm *ServiceManager) stopServer(ctx context.Context, server *ServerInstance) error {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	server.Status = "stopping"

	// Cancel server context
	if server.cancelFunc != nil {
		server.cancelFunc()
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(server.Services))

	// Stop all services in this server
	for name, instance := range server.Services {
		wg.Add(1)
		go func(serviceName string, serviceInstance ServiceInstance) {
			defer wg.Done()
			log.Infof("Stopping service %s in server %s...", serviceName, server.Name)
			if err := serviceInstance.Stop(ctx); err != nil {
				errChan <- errors.Wrapf(err, "failed to stop service %s", serviceName)
			} else {
				log.Infof("Service %s in server %s stopped", serviceName, server.Name)
			}
		}(name, instance)
	}

	// Wait for all services to stop
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors
	var stopErrors []error
	for err := range errChan {
		if err != nil {
			log.Errorf("Service stop error in server %s: %v", server.Name, err)
			stopErrors = append(stopErrors, err)
		}
	}

	// Gracefully stop the HTTP server
	if server.httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Warnf("HTTP server %s shutdown error: %v", server.Name, err)
		} else {
			log.Infof("HTTP server for %s gracefully stopped", server.Name)
		}
		shutdownCancel()
	}

	// Gracefully stop the gRPC server
	if server.grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			server.grpcServer.GracefulStop()
			close(stopped)
		}()

		// Wait for graceful stop with timeout
		select {
		case <-stopped:
			log.Infof("gRPC server for %s gracefully stopped", server.Name)
		case <-ctx.Done():
			log.Warnf("Graceful stop of gRPC server %s timed out, forcing stop", server.Name)
			server.grpcServer.Stop()
		}
	}

	// Close listeners
	if server.grpcListener != nil {
		server.grpcListener.Close()
	}
	if server.httpListener != nil {
		server.httpListener.Close()
	}

	// Clear services and server state
	server.Services = make(map[string]ServiceInstance)
	server.grpcServer = nil
	server.httpServer = nil
	server.httpMux = nil
	server.grpcListener = nil
	server.httpListener = nil
	server.Status = "stopped"

	if len(stopErrors) > 0 {
		return fmt.Errorf("errors occurred while stopping services in server %s: %v", server.Name, stopErrors)
	}

	return nil
}
