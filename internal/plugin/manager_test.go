package plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/0x524A/metricsd/internal/collector"
)

func TestManager_Collect(t *testing.T) {
	t.Run("collects from multiple Go plugins in parallel", func(t *testing.T) {
		m := NewManager()
		m.AddGoPlugin("p1", &mockCollector{
			name:    "p1",
			metrics: []collector.Metric{{Name: "m1", Value: 1, Type: "gauge"}},
		})
		m.AddGoPlugin("p2", &mockCollector{
			name:    "p2",
			metrics: []collector.Metric{{Name: "m2", Value: 2, Type: "gauge"}},
		})

		metrics, err := m.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 2 {
			t.Errorf("expected 2 metrics, got %d", len(metrics))
		}
	})

	t.Run("failing plugin does not block others", func(t *testing.T) {
		m := NewManager()
		m.AddGoPlugin("good", &mockCollector{
			name:    "good",
			metrics: []collector.Metric{{Name: "m1", Value: 1, Type: "gauge"}},
		})
		m.AddGoPlugin("bad", &mockCollector{
			name: "bad",
			err:  fmt.Errorf("plugin crashed"),
		})

		metrics, err := m.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 1 {
			t.Errorf("expected 1 metric (from good), got %d", len(metrics))
		}
	})

	t.Run("circuit breaker opens after consecutive failures", func(t *testing.T) {
		m := NewManager()
		failing := &mockCollector{name: "flaky", err: fmt.Errorf("fail")}
		m.AddGoPlugin("flaky", failing)

		for i := 0; i < MaxConsecutiveFailures; i++ {
			m.Collect(context.Background())
		}

		health := m.GetHealth()
		if h, ok := health["flaky"]; !ok {
			t.Fatal("expected health entry for 'flaky'")
		} else if h.Status != "circuit_open" {
			t.Errorf("expected circuit_open, got %s", h.Status)
		}

		metrics, _ := m.Collect(context.Background())
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics (circuit open), got %d", len(metrics))
		}
	})

	t.Run("circuit breaker resets on success", func(t *testing.T) {
		m := NewManager()
		flaky := &mockCollector{name: "flaky", err: fmt.Errorf("fail")}
		m.AddGoPlugin("flaky", flaky)

		for i := 0; i < MaxConsecutiveFailures-1; i++ {
			m.Collect(context.Background())
		}

		flaky.err = nil
		flaky.metrics = []collector.Metric{{Name: "recovered", Value: 1, Type: "gauge"}}
		metrics, _ := m.Collect(context.Background())
		if len(metrics) != 1 {
			t.Errorf("expected 1 metric after recovery, got %d", len(metrics))
		}

		health := m.GetHealth()
		if h := health["flaky"]; h.ConsecutiveFails != 0 {
			t.Errorf("expected 0 consecutive fails after success, got %d", h.ConsecutiveFails)
		}
	})
}

func TestManager_Name(t *testing.T) {
	m := NewManager()
	if m.Name() != "plugins" {
		t.Errorf("expected name 'plugins', got %s", m.Name())
	}
}

func TestManager_GetHealth(t *testing.T) {
	m := NewManager()
	m.AddGoPlugin("healthy", &mockCollector{
		name:    "healthy",
		metrics: []collector.Metric{{Name: "m", Value: 1, Type: "gauge"}},
	})
	m.Collect(context.Background())

	health := m.GetHealth()
	h, ok := health["healthy"]
	if !ok {
		t.Fatal("expected health entry for 'healthy'")
	}
	if h.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", h.Status)
	}
	if h.LastMetricCount != 1 {
		t.Errorf("expected metric count 1, got %d", h.LastMetricCount)
	}
	if h.LastSuccess.IsZero() {
		t.Error("expected non-zero last success time")
	}
}

var _ collector.Collector = (*Manager)(nil)

// TestManager_AddExecPlugin verifies that AddExecPlugin registers the plugin
// and increments the count.
func TestManager_AddExecPlugin(t *testing.T) {
	m := NewManager()
	ep := NewExecPlugin(PluginConfig{Name: "my_exec_plugin", Path: "/bin/true", Timeout: 5})
	m.AddExecPlugin(ep)

	if m.PluginCount() != 1 {
		t.Errorf("expected PluginCount 1 after AddExecPlugin, got %d", m.PluginCount())
	}

	health := m.GetHealth()
	if _, ok := health["my_exec_plugin"]; !ok {
		t.Error("expected health entry for 'my_exec_plugin'")
	}
}

// TestManager_PluginCount verifies that PluginCount reflects the total number
// of registered plugins (mix of Go and exec plugins).
func TestManager_PluginCount(t *testing.T) {
	m := NewManager()

	m.AddGoPlugin("p1", &mockCollector{name: "p1"})
	m.AddGoPlugin("p2", &mockCollector{name: "p2"})
	ep := NewExecPlugin(PluginConfig{Name: "p3", Path: "/bin/true", Timeout: 5})
	m.AddExecPlugin(ep)

	if m.PluginCount() != 3 {
		t.Errorf("expected PluginCount 3, got %d", m.PluginCount())
	}
}
