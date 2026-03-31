package grpcpool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ErrNoHealthyConn is returned by Get when no healthy connection is available.
var ErrNoHealthyConn = errors.New("grpcpool: no healthy connection available")

// HealthCheckFunc reports whether conn is healthy.
// A nil value makes New use the default check (gRPC connectivity state).
type HealthCheckFunc func(ctx context.Context, conn *grpc.ClientConn) bool

// entry wraps a single connection with its last-known health status.
type entry struct {
	conn    *grpc.ClientConn
	healthy atomic.Bool
}

// Pool manages a set of gRPC client connections and tracks their health.
// Connections are selected round-robin among healthy entries.
type Pool struct {
	mu          sync.RWMutex
	entries     []*entry
	healthCheck HealthCheckFunc
	interval    time.Duration
	rr          atomic.Uint64
}

// Option configures a Pool.
type Option func(*Pool)

// WithHealthCheck sets a custom health-check function and the interval at
// which it is invoked.  interval must be positive; it defaults to 30s.
func WithHealthCheck(fn HealthCheckFunc, interval time.Duration) Option {
	return func(p *Pool) {
		if fn != nil {
			p.healthCheck = fn
		}
		if interval > 0 {
			p.interval = interval
		}
	}
}

// New creates a Pool from the provided connections.
// All connections are considered healthy initially.
// Call Start to begin periodic health checking.
func New(conns []*grpc.ClientConn, opts ...Option) *Pool {
	p := &Pool{
		healthCheck: defaultHealthCheck,
		interval:    30 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.entries = make([]*entry, len(conns))
	for i, c := range conns {
		e := &entry{conn: c}
		e.healthy.Store(true)
		p.entries[i] = e
	}
	return p
}

// defaultHealthCheck considers a connection healthy when its connectivity
// state is Ready or Idle (not yet attempted).
func defaultHealthCheck(_ context.Context, conn *grpc.ClientConn) bool {
	s := conn.GetState()
	return s == connectivity.Ready || s == connectivity.Idle
}

// Start launches a background goroutine that periodically runs health checks
// on all connections.  It returns when ctx is cancelled.
// An initial check is performed immediately before the first tick.
func (p *Pool) Start(ctx context.Context) {
	p.checkAll(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.checkAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// checkAll runs the health check on every connection.
func (p *Pool) checkAll(ctx context.Context) {
	p.mu.RLock()
	entries := p.entries
	p.mu.RUnlock()
	for _, e := range entries {
		e.healthy.Store(p.healthCheck(ctx, e.conn))
	}
}

// Get returns a healthy connection using round-robin selection.
// Returns ErrNoHealthyConn if no healthy connection exists.
func (p *Pool) Get() (*grpc.ClientConn, error) {
	p.mu.RLock()
	entries := p.entries
	p.mu.RUnlock()

	n := uint64(len(entries))
	if n == 0 {
		return nil, ErrNoHealthyConn
	}
	// Try every entry once, starting from the next round-robin slot.
	start := p.rr.Add(1) - 1
	for i := uint64(0); i < n; i++ {
		e := entries[(start+i)%n]
		if e.healthy.Load() {
			return e.conn, nil
		}
	}
	return nil, ErrNoHealthyConn
}

// Len returns the total number of connections in the pool.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

// Close closes all connections in the pool.
func (p *Pool) Close() error {
	p.mu.RLock()
	entries := p.entries
	p.mu.RUnlock()
	var errs []error
	for _, e := range entries {
		if err := e.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
