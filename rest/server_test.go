//go:build unit
// +build unit

package rest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	return addr
}

func TestNewServer_defaults(t *testing.T) {
	s := NewServer("test")
	if s.Name != "test" {
		t.Errorf("Name: want %q, got %q", "test", s.Name)
	}
	if s.shutdownTimeout != defaultShutdownTimeout {
		t.Errorf("shutdownTimeout: want %v, got %v", defaultShutdownTimeout, s.shutdownTimeout)
	}
	if s.onShutdown != nil {
		t.Error("onShutdown: want nil, got non-nil")
	}
	if s.root == nil {
		t.Error("root router: want non-nil, got nil")
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	s := NewServer("test", WithShutdownTimeout(5*time.Second))
	if s.shutdownTimeout != 5*time.Second {
		t.Errorf("shutdownTimeout: want 5s, got %v", s.shutdownTimeout)
	}
}

func TestWithOnShutdown(t *testing.T) {
	called := false
	hook := func(ctx context.Context) { called = true }

	s := NewServer("test", WithOnShutdown(hook))
	if s.onShutdown == nil {
		t.Fatal("onShutdown: want non-nil, got nil")
	}
	s.onShutdown(context.Background())
	if !called {
		t.Error("onShutdown hook was not called")
	}
}

func TestGetServer_multiton(t *testing.T) {
	servers = nil

	s1 := GetServer("a")
	s2 := GetServer("a")
	s3 := GetServer("b")

	if s1 != s2 {
		t.Error("GetServer should return the same instance for the same name")
	}
	if s1 == s3 {
		t.Error("GetServer should return different instances for different names")
	}
}

func TestGetServer_appliesOptions(t *testing.T) {
	servers = nil

	s := GetServer("opts", WithShutdownTimeout(10*time.Second))
	if s.shutdownTimeout != 10*time.Second {
		t.Errorf("shutdownTimeout: want 10s, got %v", s.shutdownTimeout)
	}
}

func TestGetServer_ignoresOptionsOnExisting(t *testing.T) {
	servers = nil

	s1 := GetServer("reuse", WithShutdownTimeout(10*time.Second))
	_ = GetServer("reuse", WithShutdownTimeout(99*time.Second))

	if s1.shutdownTimeout != 10*time.Second {
		t.Errorf("shutdownTimeout should not change: want 10s, got %v", s1.shutdownTimeout)
	}
}

func TestListen_gracefulShutdown(t *testing.T) {
	bind := freePort(t)
	s := NewServer("graceful")
	s.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := s.Listen(ctx, bind)

	// Wait for the server to be ready.
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/ping", bind))
		if err == nil {
			_ = resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server did not become ready")
	}

	cancel()

	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}
}

func TestListen_callsOnShutdownHook(t *testing.T) {
	bind := freePort(t)
	var hookCalled atomic.Int32

	s := NewServer("hook",
		WithOnShutdown(func(ctx context.Context) {
			hookCalled.Store(1)
		}),
	)
	s.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := s.Listen(ctx, bind)

	// Wait for the server to be ready.
	for i := 0; i < 50; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/ping", bind))
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}

	if hookCalled.Load() != 1 {
		t.Error("onShutdown hook was not called during shutdown")
	}
}

func TestListen_onShutdownReceivesDeadlineContext(t *testing.T) {
	bind := freePort(t)
	timeout := 3 * time.Second
	var hasDeadline atomic.Int32

	s := NewServer("deadline",
		WithShutdownTimeout(timeout),
		WithOnShutdown(func(ctx context.Context) {
			if _, ok := ctx.Deadline(); ok {
				hasDeadline.Store(1)
			}
		}),
	)
	s.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := s.Listen(ctx, bind)

	for i := 0; i < 50; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/ping", bind))
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}

	if hasDeadline.Load() != 1 {
		t.Error("shutdown context should carry a deadline")
	}
}

func TestListen_drainsInflightRequests(t *testing.T) {
	bind := freePort(t)
	var requestCompleted atomic.Int32

	s := NewServer("drain", WithShutdownTimeout(5*time.Second))
	s.Router().Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		requestCompleted.Store(1)
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := s.Listen(ctx, bind)

	// Wait for the server to be ready.
	for i := 0; i < 50; i++ {
		resp, err := http.Get(fmt.Sprintf("http://%s/slow", bind))
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Start an in-flight request, then cancel the context while it's running.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		resp, err := http.Get(fmt.Sprintf("http://%s/slow", bind))
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	// Give the request time to reach the handler.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("server did not shut down")
	}

	<-doneCh

	if requestCompleted.Load() != 1 {
		t.Error("in-flight request was not drained before shutdown")
	}
}

func TestListen_invalidBind_reportsError(t *testing.T) {
	s := NewServer("bad-bind")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Bind to an invalid address to trigger a ListenAndServe error.
	errCh := s.Listen(ctx, "127.0.0.1:-1")

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected an error for invalid bind address")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive error within 5s")
	}
}
