package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const fakeServiceType = "fake"

type fakeBehavior struct {
	initErr  error
	startErr error
	// handler, if set, serves GET /ping instead of the default reply.
	handler func(w http.ResponseWriter, r *http.Request)
	// grpcBlock, if set, backs a /blocked.Service/Block stream that waits on
	// it and ignores cancellation.
	grpcBlock <-chan struct{}
	entered   chan<- struct{}
}

type fakeFactory struct {
	mu        sync.Mutex
	behaviors map[string]fakeBehavior
	instances map[string]*fakeInstance
}

func newFakeFactory() *fakeFactory {
	return &fakeFactory{behaviors: map[string]fakeBehavior{}, instances: map[string]*fakeInstance{}}
}

func (f *fakeFactory) Create(name string, _ *ServiceConfig, _ *SharedConfig) (ServiceInstance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inst := &fakeInstance{name: name, behavior: f.behaviors[name], status: string(StatusStopped)}
	f.instances[name] = inst
	return inst, nil
}

func (f *fakeFactory) instance(name string) *fakeInstance {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.instances[name]
}

type fakeInstance struct {
	mu          sync.Mutex
	httpPort    int
	name        string
	behavior    fakeBehavior
	status      string
	initialized bool
	started     bool
	stops       int
}

func (i *fakeInstance) Register(grpcServer *grpc.Server, mux *runtime.ServeMux, httpPort int) error {
	i.mu.Lock()
	i.httpPort = httpPort
	i.mu.Unlock()
	healthpb.RegisterHealthServer(grpcServer, health.NewServer())
	if block := i.behavior.grpcBlock; block != nil {
		entered := i.behavior.entered
		grpcServer.RegisterService(&grpc.ServiceDesc{
			ServiceName: "blocked.Service",
			HandlerType: (*any)(nil),
			Streams: []grpc.StreamDesc{{
				StreamName:    "Block",
				ClientStreams: true,
				ServerStreams: true,
				Handler: func(any, grpc.ServerStream) error {
					close(entered)
					<-block
					return nil
				},
			}},
		}, struct{}{})
	}
	handler := i.behavior.handler
	if handler == nil {
		handler = func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "pong") }
	}
	return mux.HandlePath(http.MethodGet, "/ping", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		handler(w, r)
	})
}

func (i *fakeInstance) Initialize(context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.behavior.initErr != nil {
		return i.behavior.initErr
	}
	i.initialized = true
	return nil
}

func (i *fakeInstance) Start(context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.behavior.startErr != nil {
		return i.behavior.startErr
	}
	i.started = true
	i.status = string(StatusRunning)
	return nil
}

func (i *fakeInstance) Stop(context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stops++
	i.status = string(StatusStopped)
	return nil
}

func (i *fakeInstance) stopCount() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stops
}

func (i *fakeInstance) Status() string { i.mu.Lock(); defer i.mu.Unlock(); return i.status }
func (i *fakeInstance) Name() string   { return i.name }
func (i *fakeInstance) Type() string   { return fakeServiceType }

func serverConfig(name string, port int, services ...string) ServerConfig {
	cfg := ServerConfig{Name: name, Enabled: true, Port: port}
	for _, s := range services {
		cfg.Services = append(cfg.Services, ServiceConfig{Name: s, Type: fakeServiceType, Enabled: true})
	}
	return cfg
}

func newTestManager(t *testing.T, factory *fakeFactory, servers ...ServerConfig) *ServiceManager {
	t.Helper()
	sm := NewServiceManager(&Config{Servers: servers})
	sm.services[fakeServiceType] = factory
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sm.Stop(ctx)
		_ = sm.Wait()
	})
	return sm
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func localListener(t *testing.T) (net.Listener, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return l, l.Addr().String()
}

func requireReusable(t *testing.T, addr string) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err, "address %s must be released", addr)
	require.NoError(t, l.Close())
}

func ping(t *testing.T, addr string) string {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/ping")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func grpcCheck(t *testing.T, addr string) {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
}

func TestPrepareReturnsOccupiedPortErrorAndRollsBack(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	factory := newFakeFactory()
	firstL, firstAddr := localListener(t)
	sm := newTestManager(t, factory,
		serverConfig("first", 0, "first-svc"),
		serverConfig("second", port, "second-svc"),
	)

	err = sm.Prepare(t.Context(), WithListener("first", firstL))
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("port %d", port))

	require.Equal(t, map[string]string{"first": "stopped", "second": "stopped"}, sm.ListServers())
	requireReusable(t, firstAddr)
	require.Equal(t, 1, factory.instance("first-svc").stopCount())
	require.Nil(t, factory.instance("second-svc"), "an occupied port fails before any service is created")
	require.NoError(t, sm.Wait())
}

func TestPreparedServersAnswerNothingUntilStart(t *testing.T) {
	factory := newFakeFactory()
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))

	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	status, err := sm.GetServerStatus("server")
	require.NoError(t, err)
	require.Equal(t, "prepared", status)
	require.False(t, factory.instance("svc").started, "services start only in Start")

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	_, err = io.WriteString(conn, "GET /ping HTTP/1.1\r\nHost: test\r\n\r\n")
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "a prepared server must not answer before Start")

	require.NoError(t, sm.Start(t.Context()))
	status, err = sm.GetServerStatus("server")
	require.NoError(t, err)
	require.Equal(t, "running", status)
	require.Equal(t, "pong", ping(t, addr))
	grpcCheck(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, sm.Stop(ctx))
	require.NoError(t, sm.Wait())
	requireReusable(t, addr)
}

func TestInitializeFailureRollsBackEveryPreparedServer(t *testing.T) {
	factory := newFakeFactory()
	factory.behaviors["second-svc"] = fakeBehavior{initErr: errors.New("boom")}
	firstL, firstAddr := localListener(t)
	secondL, secondAddr := localListener(t)
	sm := newTestManager(t, factory,
		serverConfig("first", 0, "first-svc"),
		serverConfig("second", 0, "second-svc"),
	)

	err := sm.Prepare(t.Context(), WithListener("first", firstL), WithListener("second", secondL))
	require.ErrorContains(t, err, "boom")
	requireReusable(t, firstAddr)
	requireReusable(t, secondAddr)
	require.Equal(t, 1, factory.instance("first-svc").stopCount())
	require.Equal(t, 1, factory.instance("second-svc").stopCount())
}

func TestStartFailureRollsBackStartedServers(t *testing.T) {
	factory := newFakeFactory()
	factory.behaviors["second-svc"] = fakeBehavior{startErr: errors.New("start boom")}
	firstL, firstAddr := localListener(t)
	secondL, secondAddr := localListener(t)
	sm := newTestManager(t, factory,
		serverConfig("first", 0, "first-svc"),
		serverConfig("second", 0, "second-svc"),
	)

	require.NoError(t, sm.Prepare(t.Context(), WithListener("first", firstL), WithListener("second", secondL)))
	err := sm.Start(t.Context())
	require.ErrorContains(t, err, "start boom")
	require.True(t, factory.instance("first-svc").started, "the first server was started before the failure")

	require.NoError(t, sm.Wait(), "a rollback is not a serve failure")
	requireReusable(t, firstAddr)
	requireReusable(t, secondAddr)
	require.Equal(t, 1, factory.instance("first-svc").stopCount())
	require.Equal(t, map[string]string{"first": "stopped", "second": "stopped"}, sm.ListServers())
}

func TestPrepareClosesInjectedListenersOnError(t *testing.T) {
	sm := newTestManager(t, newFakeFactory(), serverConfig("server", 0, "svc"))
	l, addr := localListener(t)
	err := sm.Prepare(t.Context(), WithListener("unknown", l))
	require.ErrorContains(t, err, "unknown")
	requireReusable(t, addr)
}

func TestPrepareTwiceIsRejected(t *testing.T) {
	sm := newTestManager(t, newFakeFactory(), serverConfig("server", 0, "svc"))
	l, _ := localListener(t)
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	l2, addr2 := localListener(t)
	require.Error(t, sm.Prepare(t.Context(), WithListener("server", l2)))
	requireReusable(t, addr2)
}

func TestStopIsIdempotentAndBoundedByContext(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	factory := newFakeFactory()
	factory.behaviors["svc"] = fakeBehavior{handler: func(_ http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		<-r.Context().Done()
	}}
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	require.NoError(t, sm.Start(t.Context()))

	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		if resp, err := client.Get("http://" + addr + "/ping"); err == nil {
			resp.Body.Close()
		}
	}()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	require.NoError(t, sm.Stop(ctx))
	require.Less(t, time.Since(begin), 3*time.Second)
	require.NoError(t, sm.Stop(context.Background()), "Stop is idempotent")
	require.NoError(t, sm.Wait())
	requireReusable(t, addr)
	require.Equal(t, 1, factory.instance("svc").stopCount())
}

func TestWaitReportsUnexpectedServeError(t *testing.T) {
	l, _ := localListener(t)
	sm := newTestManager(t, newFakeFactory(), serverConfig("server", 0, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	require.NoError(t, sm.Start(t.Context()))

	// Closing the listener behind the manager's back is not a Stop.
	require.NoError(t, l.Close())
	waited := make(chan error, 1)
	go func() { waited <- sm.Wait() }()
	select {
	case err := <-waited:
		require.Error(t, err)
		require.Contains(t, err.Error(), "server server")
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not observe the failed serving loop")
	}
}

// startAllOnFreePort runs StartAll on a configured port. Another process may
// take a probed free port before StartAll binds it, so retry on that error.
func startAllOnFreePort(t *testing.T, factory *fakeFactory) (*ServiceManager, string, error) {
	t.Helper()
	for attempt := 0; ; attempt++ {
		port := freePort(t)
		sm := newTestManager(t, factory, serverConfig("server", port, "svc"))
		err := sm.StartAll(t.Context())
		if err != nil && strings.Contains(err.Error(), "address already in use") && attempt < 5 {
			continue
		}
		return sm, fmt.Sprintf("127.0.0.1:%d", port), err
	}
}

func TestStartAllStopAllAndRestartServerWrappers(t *testing.T) {
	factory := newFakeFactory()
	sm, addr, err := startAllOnFreePort(t, factory)
	require.NoError(t, err)
	require.Equal(t, "pong", ping(t, addr))
	grpcCheck(t, addr)

	require.NoError(t, sm.RestartServer(t.Context(), "server"))
	require.Equal(t, "pong", ping(t, addr))

	require.NoError(t, sm.StopAll(t.Context()))
	requireReusable(t, addr)
	require.Equal(t, map[string]string{"server": "stopped"}, sm.ListServers())

	// The manager can be started again after StopAll.
	require.NoError(t, sm.StartAll(t.Context()))
	require.Equal(t, "pong", ping(t, addr))
	require.NoError(t, sm.StopAll(t.Context()))
	requireReusable(t, addr)
}

func TestStartAllFailureLeavesNoListener(t *testing.T) {
	factory := newFakeFactory()
	factory.behaviors["svc"] = fakeBehavior{startErr: errors.New("start boom")}
	_, addr, err := startAllOnFreePort(t, factory)
	require.ErrorContains(t, err, "start boom")
	requireReusable(t, addr)
}

func TestServicesRegisterAgainstTheAdoptedListenerPort(t *testing.T) {
	factory := newFakeFactory()
	l, _ := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 9, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	factory.instance("svc").mu.Lock()
	defer factory.instance("svc").mu.Unlock()
	require.Equal(t, l.Addr().(*net.TCPAddr).Port, factory.instance("svc").httpPort,
		"gateways must dial the port actually listened on, not the configured one")
}

func TestStartHonorsItsOwnCancellation(t *testing.T) {
	factory := newFakeFactory()
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, sm.Start(ctx), context.Canceled)
	requireReusable(t, addr)
	require.Equal(t, map[string]string{"server": "stopped"}, sm.ListServers())
}

func TestPrepareContextDoesNotOutliveItsStep(t *testing.T) {
	factory := newFakeFactory()
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))

	prepareCtx, cancel := context.WithCancel(t.Context())
	require.NoError(t, sm.Prepare(prepareCtx, WithListener("server", l)))
	cancel()
	require.NoError(t, sm.Start(t.Context()), "a finished Prepare's context must not cancel Start")
	require.Equal(t, "pong", ping(t, addr))
}

func TestStopAllIsBoundedByAGRPCHandlerThatIgnoresCancellation(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseHandler()
	entered := make(chan struct{})
	factory := newFakeFactory()
	factory.behaviors["svc"] = fakeBehavior{grpcBlock: release, entered: entered}
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	require.NoError(t, sm.Start(t.Context()))

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	stream, err := conn.NewStream(context.Background(),
		&grpc.StreamDesc{ClientStreams: true, ServerStreams: true}, "/blocked.Service/Block")
	require.NoError(t, err)
	require.NoError(t, stream.SendMsg(&healthpb.HealthCheckRequest{}))
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	begin := time.Now()
	require.NoError(t, sm.StopAll(ctx))
	require.Less(t, time.Since(begin), 3*time.Second, "StopAll must honor its deadline")
	requireReusable(t, addr)
	require.Equal(t, map[string]string{"server": "stopped"}, sm.ListServers())

	releaseHandler()
	require.NoError(t, sm.Wait())
}

func TestConcurrentStopHonorsEachCallersDeadline(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	factory := newFakeFactory()
	factory.behaviors["svc"] = fakeBehavior{handler: func(_ http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		<-r.Context().Done() // drains only when the server is forced
	}}
	l, addr := localListener(t)
	sm := newTestManager(t, factory, serverConfig("server", 0, "svc"))
	require.NoError(t, sm.Prepare(t.Context(), WithListener("server", l)))
	require.NoError(t, sm.Start(t.Context()))
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		if resp, err := client.Get("http://" + addr + "/ping"); err == nil {
			resp.Body.Close()
		}
	}()
	<-entered

	// A graceful Stop with no deadline is stuck on the in-flight request.
	first := make(chan error, 1)
	go func() { first <- sm.Stop(context.Background()) }()
	select {
	case <-first:
		t.Fatal("a graceful Stop must wait for the in-flight request")
	case <-time.After(100 * time.Millisecond):
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	second := make(chan error, 1)
	go func() { second <- sm.Stop(ctx) }()
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("a second Stop must force the drain with its own deadline")
	}
	select {
	case err := <-first:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("the forced drain must release the first Stop")
	}
	require.NoError(t, sm.Wait())
	requireReusable(t, addr)
	require.Equal(t, 1, factory.instance("svc").stopCount())
}
