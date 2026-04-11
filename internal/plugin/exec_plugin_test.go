// internal/plugin/exec_plugin_test.go
package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/0x524A/metricsd/internal/collector"
)

func writeTestPlugin(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to write test plugin %s: %v", name, err)
	}
	return path
}

func TestExecPlugin_Collect(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exec_plugin_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("successful collection", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "good", "#!/bin/bash\necho '[{\"name\":\"cpu\",\"value\":42.5,\"type\":\"gauge\",\"labels\":{\"env\":\"test\"}}]'\n")
		ep := NewExecPlugin(PluginConfig{Name: "good", Path: path, Timeout: 5})
		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}
		if metrics[0].Name != "plugin_good_cpu" {
			t.Errorf("expected name plugin_good_cpu, got %s", metrics[0].Name)
		}
		if metrics[0].Value != 42.5 {
			t.Errorf("expected value 42.5, got %f", metrics[0].Value)
		}
		if metrics[0].Labels["plugin"] != "good" {
			t.Errorf("expected plugin label 'good', got %s", metrics[0].Labels["plugin"])
		}
	})

	t.Run("empty output returns empty slice", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "empty", "#!/bin/bash\n")
		ep := NewExecPlugin(PluginConfig{Name: "empty", Path: path, Timeout: 5})
		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(metrics))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "badjson", "#!/bin/bash\necho 'not json'\n")
		ep := NewExecPlugin(PluginConfig{Name: "badjson", Path: path, Timeout: 5})
		_, err := ep.Collect(context.Background())
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("non-zero exit returns error", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "fail", "#!/bin/bash\nexit 1\n")
		ep := NewExecPlugin(PluginConfig{Name: "fail", Path: path, Timeout: 5})
		_, err := ep.Collect(context.Background())
		if err == nil {
			t.Error("expected error for non-zero exit")
		}
	})

	t.Run("timeout returns error", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "slow", "#!/bin/bash\nsleep 30\necho '[]'\n")
		ep := NewExecPlugin(PluginConfig{Name: "slow", Path: path, Timeout: 1})
		_, err := ep.Collect(context.Background())
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("output exceeding limit returns error", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "bigoutput", "#!/bin/bash\npython3 -c \"print('[' + ','.join(['{\\\"name\\\":\\\"m\\\",\\\"value\\\":1}'] * 100000) + ']')\"\n")
		ep := NewExecPlugin(PluginConfig{Name: "big", Path: path, Timeout: 10})
		ep.maxOutputBytes = 1024 // 1KB limit for test
		_, err := ep.Collect(context.Background())
		if err == nil {
			t.Error("expected error for oversized output")
		}
	})

	t.Run("interval scheduling skips when not elapsed", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "interval", "#!/bin/bash\necho '[{\"name\":\"m\",\"value\":1}]'\n")
		ep := NewExecPlugin(PluginConfig{Name: "interval", Path: path, Timeout: 5, Interval: 3600})

		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("First collect failed: %v", err)
		}
		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric on first call, got %d", len(metrics))
		}

		metrics, err = ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Second collect failed: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics on second call (interval), got %d", len(metrics))
		}
	})

	t.Run("uses safe environment", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "envcheck", "#!/bin/bash\nif [ -z \"$USER\" ]; then\n  echo '[{\"name\":\"env_safe\",\"value\":1}]'\nelse\n  echo '[{\"name\":\"env_safe\",\"value\":0}]'\nfi\n")
		ep := NewExecPlugin(PluginConfig{Name: "envcheck", Path: path, Timeout: 5})
		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 1 || metrics[0].Value != 1 {
			t.Errorf("expected env_safe=1 (USER not inherited), got %v", metrics)
		}
	})

	t.Run("invalid metric names filtered out", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "badnames", "#!/bin/bash\necho '[{\"name\":\"valid_name\",\"value\":1},{\"name\":\"123bad\",\"value\":2}]'\n")
		ep := NewExecPlugin(PluginConfig{Name: "badnames", Path: path, Timeout: 5})
		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}
		if len(metrics) != 1 {
			t.Errorf("expected 1 valid metric, got %d", len(metrics))
		}
	})
}

func TestExecPlugin_Name(t *testing.T) {
	ep := NewExecPlugin(PluginConfig{Name: "test_plugin", Path: "/fake"})
	if ep.Name() != "plugin_test_plugin" {
		t.Errorf("expected name plugin_test_plugin, got %s", ep.Name())
	}
}

// Verify ExecPlugin satisfies collector.Collector interface
var _ collector.Collector = (*ExecPlugin)(nil)
