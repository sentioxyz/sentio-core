package rpc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func newTestPreparedServer(t *testing.T, handler http.Handler) (*PreparedServer, string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcServer := grpc.NewServer()
	healthpb.RegisterHealthServer(grpcServer, health.NewServer())
	if handler == nil {
		handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "ok")
		})
	}
	s := PrepareServeWithHTTP(l, handler, grpcServer)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
		_ = s.Wait()
	})
	return s, l.Addr().String()
}

func requirePortReusable(t *testing.T, addr string) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	require.NoError(t, err, "port %s must be reusable", addr)
	require.NoError(t, l.Close())
}

func httpGet(t *testing.T, addr string) string {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

func grpcHealth(t *testing.T, addr string) error {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

func TestListenTCPReturnsBindErrorSynchronously(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	l, err := ListenTCP(port)
	require.Error(t, err)
	require.Nil(t, l)
	require.Contains(t, err.Error(), fmt.Sprintf("port %d", port))
}

func TestPreparedServerAnswersNothingBeforeServe(t *testing.T) {
	s, addr := newTestPreparedServer(t, nil)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	_, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout(), "a prepared server must not answer before Serve")

	// The queued connection is answered once serving starts.
	require.NoError(t, s.Serve())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestPreparedServerServesHTTPAndGRPCOnOnePort(t *testing.T) {
	s, addr := newTestPreparedServer(t, nil)
	require.NoError(t, s.Serve())
	require.ErrorIs(t, s.Serve(), ErrPreparedServerStarted)

	require.Equal(t, "ok", httpGet(t, addr))
	require.NoError(t, grpcHealth(t, addr))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	require.NoError(t, s.Shutdown(ctx), "Shutdown is idempotent")
	require.NoError(t, s.Wait())
	select {
	case <-s.Done():
	default:
		t.Fatal("Done must be closed after Wait")
	}
	requirePortReusable(t, addr)
	require.ErrorIs(t, s.Serve(), ErrPreparedServerStopped)
}

func TestPreparedServerShutdownBeforeServeReleasesPort(t *testing.T) {
	s, addr := newTestPreparedServer(t, nil)
	require.NoError(t, s.Shutdown(context.Background()))
	require.NoError(t, s.Wait())
	requirePortReusable(t, addr)
	require.ErrorIs(t, s.Serve(), ErrPreparedServerStopped)
}

func TestPreparedServerShutdownIsBoundedByContext(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	s, addr := newTestPreparedServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer close(release)
	require.NoError(t, s.Serve())

	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + addr + "/")
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := s.Shutdown(ctx)
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second)
	require.NoError(t, s.Wait())
	requirePortReusable(t, addr)
}

func TestPreparedServerWaitReportsUnexpectedServeError(t *testing.T) {
	s, _ := newTestPreparedServer(t, nil)
	require.NoError(t, s.Serve())

	// Closing the listener behind the server's back is not a shutdown.
	require.NoError(t, s.listener.Close())
	select {
	case <-s.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("serving goroutines did not exit after the listener failed")
	}
	require.Error(t, s.Wait())
}

func TestPreparedServerWaitIsNotHeldByAnIdleConnection(t *testing.T) {
	s, addr := newTestPreparedServer(t, nil)
	require.NoError(t, s.Serve())

	// A client that connects and never sends a byte stays in cmux sniffing.
	idle, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer idle.Close()
	require.Equal(t, "ok", httpGet(t, addr))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, s.Shutdown(ctx))
	waited := make(chan error, 1)
	go func() { waited <- s.Wait() }()
	select {
	case err := <-waited:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Wait must not be held by an idle connection after Shutdown")
	}
	requirePortReusable(t, addr)
}
