package launcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pkgerrors "github.com/pkg/errors"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/repository"
	"sentioxyz/sentio-core/service/common/rpc"
)

const (
	// defaultServerPort is used when a server config leaves the port unset.
	defaultServerPort = 10000
	// serviceStepTimeout bounds each service Initialize and Start call.
	serviceStepTimeout = 30 * time.Second
	// rollbackTimeout bounds the stop that undoes a failed Prepare or Start.
	rollbackTimeout = 30 * time.Second
)

// Server status values reported by GetServerStatus and ListServers.
const (
	serverStatusPrepared = "prepared"
	serverStatusRunning  = "running"
	serverStatusStopping = "stopping"
	serverStatusStopped  = "stopped"
)

// ServiceManager manages the lifecycle of multiple servers and their services.
//
// The lifecycle is transactional:
//
//	Prepare -> Start -> Stop -> Wait
//
// Prepare creates, initializes and registers every enabled service and binds
// (or adopts) every server listener without serving any traffic. Start starts
// the services' background work and then the HTTP/gRPC serving loops; when it
// returns nil every serving loop is established. A failure in Prepare or Start
// rolls back every server already prepared or started and closes its listener.
// Stop is idempotent and bounded by its context. Wait joins every serving
// goroutine and reports serve errors that were not caused by Stop.
//
// StartAll, StopAll and RestartServer remain as wrappers over this lifecycle.
type ServiceManager struct {
	config   *Config
	services map[string]Service
	servers  map[string]*ServerInstance
	// retired holds servers stopped since the last Prepare, so Wait can still
	// join them and report their serve errors.
	retired []*ServerInstance
	mutex   sync.RWMutex
}

// ServerInstance represents a server with its services. It owns the actual
// listener, HTTP/gRPC servers and cmux through the prepared endpoint.
type ServerInstance struct {
	Name       string
	Config     *ServerConfig
	Services   map[string]ServiceInstance
	Status     string
	grpcServer *grpc.Server
	httpMux    *runtime.ServeMux
	listener   net.Listener
	endpoint   *rpc.PreparedServer
	// serviceOrder is the order services were initialized in.
	serviceOrder []string
	ctx          context.Context
	cancelFunc   context.CancelFunc
	mutex        sync.RWMutex
}

// PrepareOption customizes Prepare.
type PrepareOption func(*prepareOptions)

type prepareOptions struct {
	listeners map[string]net.Listener
}

// WithListener hands Prepare a caller-bound listener for the named server
// instead of letting the manager bind the configured port. The manager owns
// the listener from then on: it is closed by Stop, by a rollback, and by a
// Prepare that fails for any reason.
func WithListener(serverName string, listener net.Listener) PrepareOption {
	return func(o *prepareOptions) {
		if o.listeners == nil {
			o.listeners = make(map[string]net.Listener)
		}
		o.listeners[serverName] = listener
	}
}

// NewServiceManager creates a new service manager
func NewServiceManager(cfg *Config) *ServiceManager {
	return &ServiceManager{
		config:   cfg,
		services: make(map[string]Service),
		servers:  make(map[string]*ServerInstance),
	}
}

// Prepare initializes and registers every enabled server's services and
// binds its listener, but serves nothing. A bind error (for example an
// occupied port) is returned here instead of exiting the process.
//
// A bound listener is not yet accepting: the kernel may complete TCP
// handshakes into the listen backlog, but no connection is read or answered
// before Start, and Stop closes the listener together with every connection
// still queued in its backlog.
//
// On error every server already prepared is rolled back and its port is
// immediately reusable.
func (sm *ServiceManager) Prepare(ctx context.Context, opts ...PrepareOption) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.prepareLocked(ctx, opts...)
}

func (sm *ServiceManager) prepareLocked(ctx context.Context, opts ...PrepareOption) (err error) {
	var o prepareOptions
	for _, opt := range opts {
		opt(&o)
	}
	// Every injected listener is owned from here on; close the ones that do
	// not end up in a prepared server.
	adopted := make(map[string]bool, len(o.listeners))
	defer func() {
		for name, l := range o.listeners {
			if !adopted[name] {
				_ = l.Close()
			}
		}
	}()

	if len(sm.servers) > 0 {
		return fmt.Errorf("servers are already prepared or running; stop them first")
	}
	enabled := make(map[string]bool, len(sm.config.Servers))
	for _, cfg := range sm.config.Servers {
		if !cfg.Enabled {
			continue
		}
		if enabled[cfg.Name] {
			return fmt.Errorf("duplicate enabled server name %q", cfg.Name)
		}
		enabled[cfg.Name] = true
	}
	for name := range o.listeners {
		if !enabled[name] {
			return fmt.Errorf("listener given for server %q, which is not an enabled server", name)
		}
	}

	if err := sm.registerServices(); err != nil {
		return pkgerrors.Wrapf(err, "failed to register services")
	}
	sm.retired = nil

	defer func() {
		if err != nil {
			sm.rollbackLocked(ctx)
		}
	}()
	for _, serverConfig := range sm.config.Servers {
		if !serverConfig.Enabled {
			log.Infof("Server %s is disabled, skipping", serverConfig.Name)
			continue
		}
		listener := o.listeners[serverConfig.Name]
		adopted[serverConfig.Name] = true
		server, err := sm.prepareServer(ctx, serverConfig, listener)
		if err != nil {
			return pkgerrors.Wrapf(err, "failed to prepare server %s", serverConfig.Name)
		}
		sm.servers[serverConfig.Name] = server
	}
	return nil
}

// Start starts every prepared server: first its services' background work,
// then its HTTP/gRPC serving loops. A nil return is the readiness signal:
// every serving loop is established. On error every server is rolled back.
func (sm *ServiceManager) Start(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.startLocked(ctx)
}

func (sm *ServiceManager) startLocked(ctx context.Context) (err error) {
	if len(sm.servers) == 0 {
		return nil
	}
	for _, server := range sm.servers {
		if server.Status != serverStatusPrepared {
			return fmt.Errorf("server %s is %s, not prepared", server.Name, server.Status)
		}
	}
	defer func() {
		if err != nil {
			sm.rollbackLocked(ctx)
		}
	}()
	for _, cfg := range sm.config.Servers {
		server, ok := sm.servers[cfg.Name]
		if !ok {
			continue
		}
		if err := sm.startServer(ctx, server); err != nil {
			return pkgerrors.Wrapf(err, "failed to start server %s", server.Name)
		}
	}
	return nil
}

// Stop stops every server's services and closes its listener, HTTP server,
// gRPC server and mux. It is idempotent and bounded by ctx: graceful draining
// gives way to a forced close when ctx ends.
func (sm *ServiceManager) Stop(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	return sm.stopAllLocked(ctx)
}

func (sm *ServiceManager) stopAllLocked(ctx context.Context) error {
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var stopErrors []error

	for name, server := range sm.servers {
		wg.Add(1)
		go func(serverName string, serverInstance *ServerInstance) {
			defer wg.Done()

			log.Infof("Stopping server %s...", serverName)
			if err := sm.stopServer(ctx, serverInstance); err != nil {
				log.Errorf("Server stop error: %v", err)
				errMu.Lock()
				stopErrors = append(stopErrors, pkgerrors.Wrapf(err, "failed to stop server %s", serverName))
				errMu.Unlock()
			} else {
				log.Infof("Server %s stopped", serverName)
			}
		}(name, server)
	}
	wg.Wait()

	for _, server := range sm.servers {
		sm.retired = append(sm.retired, server)
	}
	sm.servers = make(map[string]*ServerInstance)

	if len(stopErrors) > 0 {
		return fmt.Errorf("errors occurred while stopping servers: %v", stopErrors)
	}
	return nil
}

// rollbackLocked undoes a failed Prepare or Start. It is bounded by its own
// timeout so a cancelled caller context still gets a graceful attempt.
func (sm *ServiceManager) rollbackLocked(ctx context.Context) {
	rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()
	if err := sm.stopAllLocked(rbCtx); err != nil {
		log.Errorf("rollback of servers failed: %v", err)
	}
}

// Wait joins every serving goroutine of the servers prepared since the last
// Prepare and returns the serve errors that Stop did not cause. While servers
// are running it blocks until they stop or fail.
func (sm *ServiceManager) Wait() error {
	sm.mutex.RLock()
	servers := make([]*ServerInstance, 0, len(sm.servers)+len(sm.retired))
	for _, server := range sm.servers {
		servers = append(servers, server)
	}
	servers = append(servers, sm.retired...)
	sm.mutex.RUnlock()

	var errs []error
	for _, server := range servers {
		server.mutex.RLock()
		endpoint := server.endpoint
		server.mutex.RUnlock()
		if endpoint == nil {
			continue
		}
		if err := endpoint.Wait(); err != nil {
			errs = append(errs, fmt.Errorf("server %s: %w", server.Name, err))
		}
	}
	return errors.Join(errs...)
}

// StartAll prepares and starts all enabled servers and their services. Any
// failure rolls back every server, so a non-nil return leaves no listener
// bound and no service running.
func (sm *ServiceManager) StartAll(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if err := sm.prepareLocked(ctx); err != nil {
		return err
	}
	return sm.startLocked(ctx)
}

// StopAll stops all running servers and their services, then joins their
// serving goroutines. Serve errors found while joining are logged.
func (sm *ServiceManager) StopAll(ctx context.Context) error {
	if err := sm.Stop(ctx); err != nil {
		return err
	}
	if err := sm.waitBounded(ctx); err != nil {
		log.Errorf("Servers reported serve errors: %v", err)
	}
	log.Info("All servers stopped")
	return nil
}

func (sm *ServiceManager) waitBounded(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- sm.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("joining serving goroutines: %w", ctx.Err())
	}
}

// RestartServer restarts a specific server and all its services
func (sm *ServiceManager) RestartServer(ctx context.Context, serverName string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Find server config
	var serverConfig *ServerConfig
	for i := range sm.config.Servers {
		if sm.config.Servers[i].Name == serverName {
			serverConfig = &sm.config.Servers[i]
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
		sm.retired = append(sm.retired, server)
	}

	if err := sm.registerServices(); err != nil {
		return pkgerrors.Wrapf(err, "failed to register services")
	}
	server, err := sm.prepareServer(ctx, *serverConfig, nil)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to prepare server %s", serverName)
	}
	if err := sm.startServer(ctx, server); err != nil {
		rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		if stopErr := sm.stopServer(rbCtx, server); stopErr != nil {
			log.Errorf("rollback of server %s failed: %v", serverName, stopErr)
		}
		sm.retired = append(sm.retired, server)
		return pkgerrors.Wrapf(err, "failed to start server %s", serverName)
	}
	sm.servers[serverName] = server
	return nil
}

// GetServerStatus returns the status of a server
func (sm *ServiceManager) GetServerStatus(serverName string) (string, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	server, exists := sm.servers[serverName]
	if !exists {
		return serverStatusStopped, nil
	}

	server.mutex.RLock()
	defer server.mutex.RUnlock()
	return server.Status, nil
}

// ListServers returns a list of all configured servers and their status
func (sm *ServiceManager) ListServers() map[string]string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]string)

	for _, cfg := range sm.config.Servers {
		if server, exists := sm.servers[cfg.Name]; exists {
			server.mutex.RLock()
			result[cfg.Name] = server.Status
			server.mutex.RUnlock()
		} else {
			result[cfg.Name] = serverStatusStopped
		}
	}

	return result
}

// registerServices registers the built-in service types. Factories already
// registered for a type are kept.
func (sm *ServiceManager) registerServices() error {
	defaults := map[string]func() Service{
		"processor":    NewProcessorService,
		"localstorage": NewLocalStorageService,
		"project":      NewProjectServiceFactory,
	}
	for serviceType, factory := range defaults {
		if _, exists := sm.services[serviceType]; !exists {
			sm.services[serviceType] = factory()
		}
	}
	return nil
}

func (sm *ServiceManager) configSharedDB() (*gorm.DB, error) {
	conn, err := repository.SetupDB(sm.config.Shared.Database.URL)
	return conn, err
}

// prepareServer creates, initializes and registers a server's services and
// binds its listener (or adopts the given one). On error it releases
// everything it acquired, including the given listener.
func (sm *ServiceManager) prepareServer(
	ctx context.Context,
	cfg ServerConfig,
	listener net.Listener,
) (_ *ServerInstance, err error) {
	log.Infof("Preparing server %s on port: %d", cfg.Name, cfg.Port)

	serverCtx, cancel := context.WithCancel(ctx)
	server := &ServerInstance{
		Name:       cfg.Name,
		Config:     &cfg,
		Services:   make(map[string]ServiceInstance),
		Status:     "starting",
		grpcServer: rpc.NewServer(),
		httpMux:    rpc.NewServeMux(),
		listener:   listener,
		ctx:        serverCtx,
		cancelFunc: cancel,
	}
	defer func() {
		if err != nil {
			rbCtx, rbCancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
			defer rbCancel()
			if stopErr := sm.stopServer(rbCtx, server); stopErr != nil {
				log.Errorf("rollback of server %s failed: %v", cfg.Name, stopErr)
			}
		}
	}()

	for _, serviceConfig := range cfg.Services {
		if !serviceConfig.Enabled {
			log.Infof("Service %s in server %s is disabled, skipping", serviceConfig.Name, cfg.Name)
			continue
		}

		log.Infof("Initializing service %s (type: %s) in server %s", serviceConfig.Name, serviceConfig.Type, cfg.Name)

		serviceFactory, exists := sm.services[serviceConfig.Type]
		if !exists {
			return nil, fmt.Errorf("unknown service type: %s", serviceConfig.Type)
		}

		instance, err := serviceFactory.Create(serviceConfig.Name, &serviceConfig, &sm.config.Shared)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to create service instance %s", serviceConfig.Name)
		}

		// Track the instance before Initialize so a rollback stops whatever
		// Initialize acquired.
		server.Services[serviceConfig.Name] = instance
		server.serviceOrder = append(server.serviceOrder, serviceConfig.Name)

		initCtx, initCancel := context.WithTimeout(serverCtx, serviceStepTimeout)
		err = instance.Initialize(initCtx)
		initCancel()
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to initialize service %s", serviceConfig.Name)
		}

		if err := instance.Register(server.grpcServer, server.httpMux, cfg.Port); err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to register service %s", serviceConfig.Name)
		}
	}

	if server.listener == nil {
		port := cfg.Port
		if port == 0 {
			port = defaultServerPort
		}
		l, err := rpc.ListenTCP(port)
		if err != nil {
			return nil, err
		}
		server.listener = l
	}
	server.endpoint = rpc.PrepareServeWithHTTP(server.listener, server.httpMux, server.grpcServer)
	server.Status = serverStatusPrepared
	log.Infof("Server %s prepared on %s with %d services", cfg.Name, server.listener.Addr(), len(server.Services))
	return server, nil
}

// startServer starts a prepared server's services and then serves traffic.
func (sm *ServiceManager) startServer(ctx context.Context, server *ServerInstance) error {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	for _, name := range server.serviceOrder {
		instance := server.Services[name]
		startCtx, startCancel := context.WithTimeout(server.ctx, serviceStepTimeout)
		err := instance.Start(startCtx)
		startCancel()
		if err != nil {
			return pkgerrors.Wrapf(err, "failed to start service for %s", name)
		}
		log.Infof("Service %s started in server %s", name, server.Name)
	}

	if err := server.endpoint.Serve(); err != nil {
		return pkgerrors.Wrapf(err, "failed to serve server %s", server.Name)
	}
	server.Status = serverStatusRunning
	log.Infof("Server %s started successfully with %d services", server.Name, len(server.Services))
	return nil
}

// stopServer stops serving (draining in-flight requests until ctx ends),
// stops the server's services, and closes the listener. It is idempotent.
func (sm *ServiceManager) stopServer(ctx context.Context, server *ServerInstance) error {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	if server.Status == serverStatusStopped {
		return nil
	}
	server.Status = serverStatusStopping

	var stopErrors []error

	// Stop accepting and drain in-flight requests while services still run.
	if server.endpoint != nil {
		if err := server.endpoint.Shutdown(ctx); err != nil {
			log.Warnf("Server %s shutdown: %v", server.Name, err)
		} else {
			log.Infof("HTTP and gRPC servers for %s stopped", server.Name)
		}
	} else {
		if server.grpcServer != nil {
			server.grpcServer.Stop()
		}
		if server.listener != nil {
			_ = server.listener.Close()
		}
	}

	var wg sync.WaitGroup
	var errMu sync.Mutex
	for name, instance := range server.Services {
		wg.Add(1)
		go func(serviceName string, serviceInstance ServiceInstance) {
			defer wg.Done()
			log.Infof("Stopping service %s in server %s...", serviceName, server.Name)
			if err := serviceInstance.Stop(ctx); err != nil {
				log.Errorf("Service stop error in server %s: %v", server.Name, err)
				errMu.Lock()
				stopErrors = append(stopErrors, pkgerrors.Wrapf(err, "failed to stop service %s", serviceName))
				errMu.Unlock()
			} else {
				log.Infof("Service %s in server %s stopped", serviceName, server.Name)
			}
		}(name, instance)
	}
	wg.Wait()

	if server.cancelFunc != nil {
		server.cancelFunc()
	}
	server.Services = make(map[string]ServiceInstance)
	server.serviceOrder = nil
	server.Status = serverStatusStopped

	if len(stopErrors) > 0 {
		return fmt.Errorf("errors occurred while stopping services in server %s: %v", server.Name, stopErrors)
	}
	return nil
}
