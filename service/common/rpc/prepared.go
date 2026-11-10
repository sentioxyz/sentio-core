package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/handlers"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"

	"sentioxyz/sentio-core/common/log"
)

// ErrPreparedServerStopped is returned by Serve when Shutdown already ran.
var ErrPreparedServerStopped = errors.New("prepared server is stopped")

// ErrPreparedServerStarted is returned by Serve when it already ran.
var ErrPreparedServerStarted = errors.New("prepared server is already serving")

// ListenTCP binds the TCP port that BindAndServeWithHTTP would bind, but
// returns a bind failure instead of exiting the process.
func ListenTCP(port int) (net.Listener, error) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("listen on port %d: %w", port, err)
	}
	return l, nil
}

// PreparedServer is the caller-owned form of BindAndServeWithHTTP: one bound
// listener multiplexed by cmux into an HTTP/1 server and a gRPC server.
//
// The lifecycle is explicit:
//   - PrepareServeWithHTTP takes an already bound listener and builds the mux
//     and servers without accepting any connection. The kernel may still
//     complete TCP handshakes into the listen backlog, but nothing reads or
//     answers them; Shutdown closes the listener and with it every connection
//     still queued there.
//   - Serve starts the cmux, HTTP and gRPC serving goroutines and returns once
//     they are established.
//   - Shutdown is idempotent and bounded by its context: it drains gracefully
//     until the context ends, then closes forcibly. It always closes the
//     listener, even if Serve never ran.
//   - Wait joins every serving goroutine and returns the serve errors that
//     were not caused by Shutdown.
//
// Unlike BindAndServeWithHTTP it installs no signal handler and never exits
// the process.
type PreparedServer struct {
	listener   net.Listener
	mux        cmux.CMux
	httpL      net.Listener
	grpcL      net.Listener
	httpServer *http.Server
	grpcServer *grpc.Server

	mu       sync.Mutex
	started  bool
	stopping atomic.Bool
	failed   atomic.Bool

	shutdownOnce sync.Once
	force        context.CancelFunc
	shutdownDone chan struct{}
	shutdownErr  error

	wg   sync.WaitGroup
	done chan struct{}

	errMu sync.Mutex
	errs  []error
}

// PrepareServeWithHTTP wraps a bound listener. The returned server owns the
// listener: Shutdown closes it.
func PrepareServeWithHTTP(listener net.Listener, mux http.Handler, grpcServer *grpc.Server) *PreparedServer {
	tracked := &trackingListener{Listener: listener, conns: make(map[*trackedConn]struct{})}
	m := cmux.New(tracked)
	s := &PreparedServer{
		listener: tracked,
		mux:      m,
		// a different listener for HTTP1
		httpL: m.Match(cmux.HTTP1Fast()),
		// a different listener for HTTP2 since gRPC uses HTTP2
		grpcL: m.Match(cmux.TLS(), cmux.HTTP2()),
		httpServer: &http.Server{
			Handler: handlers.CompressHandler(WithLogger(mux)),
		},
		grpcServer:   grpcServer,
		shutdownDone: make(chan struct{}),
		done:         make(chan struct{}),
	}
	return s
}

// Addr returns the address of the underlying listener.
func (s *PreparedServer) Addr() net.Addr {
	return s.listener.Addr()
}

// Serve starts serving on the prepared listener. It returns once the serving
// goroutines run; connections queued in the backlog are served from then on.
func (s *PreparedServer) Serve() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping.Load() {
		return ErrPreparedServerStopped
	}
	if s.started {
		return ErrPreparedServerStarted
	}
	s.started = true

	s.wg.Add(3)
	go s.run("http server", func() error { return s.httpServer.Serve(s.httpL) })
	go s.run("grpc server", func() error { return s.grpcServer.Serve(s.grpcL) })
	go s.run("mux", s.mux.Serve)
	go func() {
		s.wg.Wait()
		close(s.done)
	}()
	log.Infof("listening on %s", s.listener.Addr())
	return nil
}

func (s *PreparedServer) run(name string, serve func() error) {
	defer s.wg.Done()
	err := serve()
	// cmux errors on the matched listeners only mean the mux loop ended; that
	// loop reports its own cause.
	secondary := errors.Is(err, cmux.ErrServerClosed) || errors.Is(err, cmux.ErrListenerClosed)
	if err == nil || secondary || ((s.stopping.Load() || s.failed.Load()) && isShutdownServeError(err)) {
		log.Infof("%s on %s shut down", name, s.listener.Addr())
		return
	}
	log.Errorf("%s on %s failed: %v", name, s.listener.Addr(), err)
	s.errMu.Lock()
	s.errs = append(s.errs, fmt.Errorf("%s: %w", name, err))
	s.errMu.Unlock()
	// One failed loop takes the whole endpoint down so Wait observes it; the
	// other loops then return closed-listener errors, which are not reported.
	s.failed.Store(true)
	_ = s.listener.Close()
	s.mux.Close()
	s.listener.(*trackingListener).closeConns()
}

func isShutdownServeError(err error) bool {
	return errors.Is(err, http.ErrServerClosed) ||
		errors.Is(err, grpc.ErrServerStopped) ||
		errors.Is(err, cmux.ErrServerClosed) ||
		errors.Is(err, cmux.ErrListenerClosed) ||
		errors.Is(err, net.ErrClosed)
}

// Shutdown stops serving and closes the listener. It drains gracefully until
// ctx ends and then closes forcibly. It is idempotent and safe to call
// concurrently: every caller waits for the one shutdown, and any caller whose
// ctx ends forces it, so each call is bounded by its own ctx.
func (s *PreparedServer) Shutdown(ctx context.Context) error {
	s.shutdownOnce.Do(func() {
		force, cancel := context.WithCancel(context.Background())
		s.force = cancel
		go s.shutdown(force)
	})
	select {
	case <-s.shutdownDone:
	case <-ctx.Done():
		s.force()
		<-s.shutdownDone
	}
	return s.shutdownErr
}

// shutdown runs once; force ends when some Shutdown caller's ctx ends.
func (s *PreparedServer) shutdown(force context.Context) {
	defer close(s.shutdownDone)
	defer s.force()
	s.mu.Lock()
	s.stopping.Store(true)
	s.listener.(*trackingListener).draining.Store(true)
	started := s.started
	s.mu.Unlock()

	if !started {
		s.shutdownErr = s.closeListener()
		s.mux.Close()
		return
	}

	var wg sync.WaitGroup
	var httpErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		// The matched listeners share the root listener, so the gRPC side
		// may already have closed it.
		if err := s.httpServer.Shutdown(force); err != nil && !errors.Is(err, net.ErrClosed) {
			httpErr = err
			_ = s.httpServer.Close()
		}
	}()
	go func() {
		defer wg.Done()
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-force.Done():
			// Both Stop and GracefulStop wait for handlers to return, even
			// ones that ignore cancellation. Stop is still started to cancel
			// every RPC, but neither is awaited: closing the listener and
			// every accepted connection below is what bounds the forced path.
			go s.grpcServer.Stop()
		}
	}()
	wg.Wait()
	forced := force.Err() != nil
	_ = s.closeListener()
	s.mux.Close()
	// A connection still being sniffed by cmux (for example one that
	// connected and sent nothing) would keep the mux loop from returning;
	// the HTTP and gRPC servers are down, so close them all.
	s.listener.(*trackingListener).closeConns()
	if httpErr != nil {
		s.shutdownErr = fmt.Errorf("http server shutdown: %w", httpErr)
	} else if forced {
		s.shutdownErr = errors.New("shutdown forced before draining completed")
	}
}

func (s *PreparedServer) closeListener() error {
	if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

// Done is closed once every serving goroutine has returned. It is never
// closed if Serve never ran.
func (s *PreparedServer) Done() <-chan struct{} {
	return s.done
}

// Wait joins the serving goroutines and returns the errors that were not
// caused by Shutdown. It returns immediately if Serve never ran. grpc-go keeps
// its Serve loop alive until every RPC handler has returned, so after a forced
// Shutdown a handler that ignores cancellation delays Wait (never Shutdown)
// until it returns; callers that need a bound select on their own deadline.
func (s *PreparedServer) Wait() error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if started {
		<-s.done
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return errors.Join(s.errs...)
}

// trackingListener remembers every accepted connection until it is closed, so
// Shutdown can close connections that no server has taken over yet.
type trackingListener struct {
	net.Listener
	mu     sync.Mutex
	conns  map[*trackedConn]struct{}
	closed bool
	// draining is set by Shutdown, which closes the remaining connections
	// itself once the servers have drained.
	draining atomic.Bool
}

func (l *trackingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		var netErr net.Error
		temporary := errors.As(err, &netErr) && netErr.Temporary() //nolint:staticcheck // cmux retries exactly these
		if !temporary && !l.draining.Load() {
			// The root listener failed outside Shutdown. cmux waits for
			// its sniffing goroutines before returning this error, and an
			// idle connection would block that forever; the endpoint is
			// going down, so close every connection now.
			l.closeConns()
		}
		return nil, err
	}
	tc := &trackedConn{Conn: c, owner: l}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		_ = c.Close()
		return nil, net.ErrClosed
	}
	l.conns[tc] = struct{}{}
	return tc, nil
}

func (l *trackingListener) closeConns() {
	l.mu.Lock()
	l.closed = true
	conns := make([]*trackedConn, 0, len(l.conns))
	for c := range l.conns {
		conns = append(conns, c)
	}
	l.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

type trackedConn struct {
	net.Conn
	owner *trackingListener
	once  sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		c.owner.mu.Lock()
		delete(c.owner.conns, c)
		c.owner.mu.Unlock()
	})
	return c.Conn.Close()
}
