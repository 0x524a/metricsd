// internal/server/server_test.go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
