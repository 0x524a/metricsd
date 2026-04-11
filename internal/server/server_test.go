// internal/server/server_test.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockHealthProvider struct {
	data map[string]CollectorHealth
}

func (m *mockHealthProvider) GetHealthData() map[string]CollectorHealth {
	return m.data
}

func TestHealthEndpoint(t *testing.T) {
	t.Run("returns healthy without provider", func(t *testing.T) {
		srv := NewServer("localhost", 0, nil)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		srv.handleHealth(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var status DetailedHealthStatus
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if status.Status != "healthy" {
			t.Errorf("expected status 'healthy', got %q", status.Status)
		}
		if status.Uptime <= 0 {
			t.Error("expected positive uptime")
		}
		if status.Collectors != nil {
			t.Error("expected nil collectors with no provider")
		}
	})

	t.Run("returns collectors from provider", func(t *testing.T) {
		provider := &mockHealthProvider{
			data: map[string]CollectorHealth{
				"plugin_test": {
					Status:      "ok",
					MetricCount: 5,
					LastCollect: "2026-04-10T19:00:00Z",
				},
				"plugin_flaky": {
					Status:           "circuit_open",
					ConsecutiveFails: 5,
					LastError:        "timeout",
				},
			},
		}
		srv := NewServer("localhost", 0, provider)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		srv.handleHealth(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var status DetailedHealthStatus
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if len(status.Collectors) != 2 {
			t.Errorf("expected 2 collectors, got %d", len(status.Collectors))
		}
		if c, ok := status.Collectors["plugin_test"]; !ok {
			t.Error("expected plugin_test in collectors")
		} else if c.MetricCount != 5 {
			t.Errorf("expected metric count 5, got %d", c.MetricCount)
		}
		if c, ok := status.Collectors["plugin_flaky"]; !ok {
			t.Error("expected plugin_flaky in collectors")
		} else if c.Status != "circuit_open" {
			t.Errorf("expected status circuit_open, got %q", c.Status)
		}
	})

	t.Run("rejects non-GET method", func(t *testing.T) {
		srv := NewServer("localhost", 0, nil)
		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		w := httptest.NewRecorder()

		srv.handleHealth(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})

	t.Run("content type is json", func(t *testing.T) {
		srv := NewServer("localhost", 0, nil)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		srv.handleHealth(w, req)

		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
	})
}

// freePort asks the OS for an available TCP port and returns it.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestServer_StartAndShutdown(t *testing.T) {
	port := freePort(t)

	srv := NewServer("127.0.0.1", port, nil)

	ctx, cancel := context.WithCancel(context.Background())

	startErr := make(chan error, 1)
	go func() {
		startErr <- srv.Start(ctx)
	}()

	// Poll until the server is accepting connections (up to 2 s).
	addr := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	var resp *http.Response
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get(addr) //nolint:noctx
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("server never became ready: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("health endpoint returned %d, want 200", resp.StatusCode)
	}

	// Cancel the context — Start should return nil (graceful shutdown).
	cancel()

	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("Start() returned non-nil error after context cancel: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Start() did not return within 3 s after context cancel")
	}
}

func TestNewServer_NilProvider(t *testing.T) {
	// NewServer with nil provider must not panic.
	srv := NewServer("localhost", 0, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	// Health handler must still respond correctly.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Ensure no panic occurs.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleHealth panicked with nil provider: %v", r)
			}
		}()
		srv.handleHealth(w, req)
	}()

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var status DetailedHealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if status.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", status.Status)
	}
	if status.Collectors != nil {
		t.Error("expected nil collectors when provider is nil")
	}
}
