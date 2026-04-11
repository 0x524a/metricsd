// internal/plugin/security_test.go
package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePluginPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "plugin_security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a valid plugin inside the dir
	validPlugin := filepath.Join(tmpDir, "good_plugin")
	if err := os.WriteFile(validPlugin, []byte("#!/bin/bash\necho '[]'"), 0755); err != nil {
		t.Fatalf("Failed to write plugin: %v", err)
	}

	// Create a symlink that escapes the dir
	escapePath := filepath.Join(tmpDir, "escape_plugin")
	os.Symlink("/usr/bin/env", escapePath)

	t.Run("valid plugin passes", func(t *testing.T) {
		resolved, err := ValidatePluginPath(validPlugin, tmpDir)
		if err != nil {
			t.Errorf("expected valid path, got error: %v", err)
		}
		if resolved == "" {
			t.Error("expected non-empty resolved path")
		}
	})

	t.Run("symlink escaping dir rejected", func(t *testing.T) {
		_, err := ValidatePluginPath(escapePath, tmpDir)
		if err == nil {
			t.Error("expected error for escaping symlink")
		}
	})

	t.Run("non-existent path rejected", func(t *testing.T) {
		_, err := ValidatePluginPath(filepath.Join(tmpDir, "nonexistent"), tmpDir)
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("world-writable warns but succeeds", func(t *testing.T) {
		wwPlugin := filepath.Join(tmpDir, "ww_plugin")
		if err := os.WriteFile(wwPlugin, []byte("#!/bin/bash\necho '[]'"), 0757); err != nil {
			t.Fatalf("Failed to write plugin: %v", err)
		}
		resolved, err := ValidatePluginPath(wwPlugin, tmpDir)
		if err != nil {
			t.Errorf("expected success with warning, got error: %v", err)
		}
		if resolved == "" {
			t.Error("expected non-empty resolved path")
		}
	})
}

func TestBuildSafeEnv(t *testing.T) {
	env := BuildSafeEnv([]string{"CUSTOM_VAR=hello", "ANOTHER=world"})

	hasPath := false
	hasCustom := false
	hasHome := false
	inheritedSecrets := false

	for _, e := range env {
		if len(e) >= 5 && e[:5] == "PATH=" {
			hasPath = true
		}
		if e == "CUSTOM_VAR=hello" {
			hasCustom = true
		}
		if len(e) >= 5 && e[:5] == "HOME=" {
			hasHome = true
		}
		if len(e) >= 5 && e[:5] == "USER=" {
			inheritedSecrets = true
		}
	}

	if !hasPath {
		t.Error("expected PATH in safe env")
	}
	if !hasCustom {
		t.Error("expected CUSTOM_VAR in safe env")
	}
	if !hasHome {
		t.Error("expected HOME in safe env")
	}
	if inheritedSecrets {
		t.Error("safe env should NOT inherit USER from parent process")
	}
}

func TestValidateMetricOutput(t *testing.T) {
	t.Run("valid metrics pass", func(t *testing.T) {
		metrics := []PluginMetric{
			{Name: "cpu_usage", Value: 42.5, Type: "gauge", Labels: map[string]string{"host": "a"}},
		}
		result := ValidateMetricOutput(metrics, "test_plugin")
		if len(result) != 1 {
			t.Errorf("expected 1 metric, got %d", len(result))
		}
	})

	t.Run("reserved label prefix rejected", func(t *testing.T) {
		metrics := []PluginMetric{
			{Name: "cpu_usage", Value: 1, Labels: map[string]string{"__internal": "bad"}},
		}
		result := ValidateMetricOutput(metrics, "test_plugin")
		if len(result) != 0 {
			t.Errorf("expected 0 metrics (reserved label), got %d", len(result))
		}
	})

	t.Run("label value over 1024 chars truncated", func(t *testing.T) {
		longVal := make([]byte, 2000)
		for i := range longVal {
			longVal[i] = 'a'
		}
		metrics := []PluginMetric{
			{Name: "cpu_usage", Value: 1, Labels: map[string]string{"tag": string(longVal)}},
		}
		result := ValidateMetricOutput(metrics, "test_plugin")
		if len(result) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(result))
		}
		if len(result[0].Labels["tag"]) > 1024 {
			t.Error("expected label value to be truncated to 1024 chars")
		}
	})

	t.Run("empty metric name rejected", func(t *testing.T) {
		metrics := []PluginMetric{{Name: "", Value: 1}}
		result := ValidateMetricOutput(metrics, "test_plugin")
		if len(result) != 0 {
			t.Errorf("expected 0 metrics (empty name), got %d", len(result))
		}
	})

	t.Run("invalid metric name rejected", func(t *testing.T) {
		metrics := []PluginMetric{{Name: "123invalid", Value: 1}}
		result := ValidateMetricOutput(metrics, "test_plugin")
		if len(result) != 0 {
			t.Errorf("expected 0 metrics (invalid name), got %d", len(result))
		}
	})
}

func TestValidatePluginPath_BadPluginsDir(t *testing.T) {
	// Plugins dir that doesn't exist — EvalSymlinks fails
	_, err := ValidatePluginPath("/tmp/nonexistent/plugin", "/tmp/nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugins dir")
	}
}

func TestValidateMetricOutput_MultipleLabels(t *testing.T) {
	// Test with valid metric having multiple labels
	metrics := []PluginMetric{
		{Name: "test_metric", Value: 1, Labels: map[string]string{"a": "1", "b": "2", "c": "3"}},
	}
	result := ValidateMetricOutput(metrics, "test")
	if len(result) != 1 {
		t.Errorf("expected 1 metric, got %d", len(result))
	}
	if len(result[0].Labels) != 3 {
		t.Errorf("expected 3 labels, got %d", len(result[0].Labels))
	}
}

func TestValidateMetricOutput_NilLabels(t *testing.T) {
	metrics := []PluginMetric{
		{Name: "test_metric", Value: 1, Labels: nil},
	}
	result := ValidateMetricOutput(metrics, "test")
	if len(result) != 1 {
		t.Errorf("expected 1 metric, got %d", len(result))
	}
}
