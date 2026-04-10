package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateMetricName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple", "my_metric", true},
		{"valid with colon", "my:metric", true},
		{"valid starts with underscore", "_metric", true},
		{"valid starts with colon", ":metric", true},
		{"valid uppercase", "MY_METRIC", true},
		{"valid mixed case", "MyMetric_123", true},
		{"invalid starts with number", "123metric", false},
		{"invalid contains hyphen", "my-metric", false},
		{"invalid contains space", "my metric", false},
		{"invalid contains dot", "my.metric", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateMetricName(tt.input)
			if result != tt.expected {
				t.Errorf("validateMetricName(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"equal to max", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestNewPluginCollector(t *testing.T) {
	t.Run("sets defaults", func(t *testing.T) {
		config := PluginConfig{
			Path:    "/path/to/plugin.sh",
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		if collector.config.Timeout != 30*time.Second {
			t.Errorf("expected default timeout of 30s, got %v", collector.config.Timeout)
		}
		if collector.config.Name != "plugin" {
			t.Errorf("expected name 'plugin', got %q", collector.config.Name)
		}
	})

	t.Run("preserves custom values", func(t *testing.T) {
		config := PluginConfig{
			Name:    "custom",
			Path:    "/path/to/plugin",
			Timeout: 60 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		if collector.config.Timeout != 60*time.Second {
			t.Errorf("expected timeout of 60s, got %v", collector.config.Timeout)
		}
		if collector.config.Name != "custom" {
			t.Errorf("expected name 'custom', got %q", collector.config.Name)
		}
	})
}

func TestPluginCollector_Name(t *testing.T) {
	config := PluginConfig{
		Name:    "test_plugin",
		Path:    "/path/to/plugin",
		Enabled: true,
	}
	collector := NewPluginCollector(config)

	expected := "plugin_test_plugin"
	if collector.Name() != expected {
		t.Errorf("expected name %q, got %q", expected, collector.Name())
	}
}

func TestPluginCollector_Collect(t *testing.T) {
	// Create a temporary directory for test plugins
	tempDir, err := os.MkdirTemp("", "plugin_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("successful collection", func(t *testing.T) {
		// Create a simple test plugin that outputs JSON
		pluginPath := filepath.Join(tempDir, "test_plugin")
		pluginContent := `#!/bin/bash
echo '[{"name":"test_metric","value":42.5,"type":"gauge","labels":{"env":"test"}}]'
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "test",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		metrics, err := collector.Collect(ctx)
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}

		metric := metrics[0]
		expectedName := "plugin_test_test_metric"
		if metric.Name != expectedName {
			t.Errorf("expected metric name %q, got %q", expectedName, metric.Name)
		}
		if metric.Value != 42.5 {
			t.Errorf("expected value 42.5, got %f", metric.Value)
		}
		if metric.Type != "gauge" {
			t.Errorf("expected type 'gauge', got %q", metric.Type)
		}
		if metric.Labels["env"] != "test" {
			t.Errorf("expected label env=test, got %v", metric.Labels)
		}
		if metric.Labels["plugin"] != "test" {
			t.Errorf("expected label plugin=test, got %v", metric.Labels)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "empty_plugin")
		pluginContent := `#!/bin/bash
# Outputs nothing
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "empty",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		metrics, err := collector.Collect(ctx)
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics, got %d", len(metrics))
		}
	})

	t.Run("invalid JSON output", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "invalid_json_plugin")
		pluginContent := `#!/bin/bash
echo 'not valid json'
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "invalid",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		_, err := collector.Collect(ctx)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("plugin timeout", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "slow_plugin")
		pluginContent := `#!/bin/bash
sleep 10
echo '[]'
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "slow",
			Path:    pluginPath,
			Timeout: 100 * time.Millisecond,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		_, err := collector.Collect(ctx)
		if err == nil {
			t.Error("expected timeout error, got nil")
		}
	})

	t.Run("plugin non-zero exit", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "failing_plugin")
		pluginContent := `#!/bin/bash
exit 1
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "failing",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		_, err := collector.Collect(ctx)
		if err == nil {
			t.Error("expected error for non-zero exit, got nil")
		}
	})

	t.Run("invalid metric names are skipped", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "invalid_names_plugin")
		pluginContent := `#!/bin/bash
echo '[{"name":"valid_metric","value":1},{"name":"123invalid","value":2},{"name":"also-invalid","value":3}]'
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "names",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		metrics, err := collector.Collect(ctx)
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		// Only the valid metric should be returned
		if len(metrics) != 1 {
			t.Errorf("expected 1 valid metric, got %d", len(metrics))
		}
		if len(metrics) > 0 && metrics[0].Name != "plugin_names_valid_metric" {
			t.Errorf("expected metric name 'plugin_names_valid_metric', got %q", metrics[0].Name)
		}
	})

	t.Run("working directory", func(t *testing.T) {
		workDir := filepath.Join(tempDir, "workdir")
		if err := os.MkdirAll(workDir, 0755); err != nil {
			t.Fatalf("Failed to create work dir: %v", err)
		}

		// Create a file in the work directory
		testFile := filepath.Join(workDir, "testfile.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		pluginPath := filepath.Join(tempDir, "workdir_plugin")
		pluginContent := `#!/bin/bash
if [ -f testfile.txt ]; then
    echo '[{"name":"file_found","value":1}]'
else
    echo '[{"name":"file_found","value":0}]'
fi
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:       "workdir",
			Path:       pluginPath,
			WorkingDir: workDir,
			Timeout:    5 * time.Second,
			Enabled:    true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		metrics, err := collector.Collect(ctx)
		if err != nil {
			t.Fatalf("Collect failed: %v", err)
		}

		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(metrics))
		}
		if metrics[0].Value != 1 {
			t.Errorf("expected value 1 (file found), got %f", metrics[0].Value)
		}
	})
}

func TestPluginCollector_Validate(t *testing.T) {
	// Create a temporary directory for test plugins
	tempDir, err := os.MkdirTemp("", "plugin_validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("valid plugin passes validation", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "valid_plugin")
		pluginContent := `#!/bin/bash
echo '[{"name":"test","value":1}]'
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "valid",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		err := collector.Validate(ctx)
		if err != nil {
			t.Errorf("expected validation to pass, got error: %v", err)
		}
	})

	t.Run("failing plugin fails validation", func(t *testing.T) {
		pluginPath := filepath.Join(tempDir, "failing_plugin")
		pluginContent := `#!/bin/bash
exit 1
`
		if err := os.WriteFile(pluginPath, []byte(pluginContent), 0755); err != nil {
			t.Fatalf("Failed to write test plugin: %v", err)
		}

		config := PluginConfig{
			Name:    "failing",
			Path:    pluginPath,
			Timeout: 5 * time.Second,
			Enabled: true,
		}
		collector := NewPluginCollector(config)

		ctx := context.Background()
		err := collector.Validate(ctx)
		if err == nil {
			t.Error("expected validation to fail, got nil")
		}
	})
}

func TestDiscoverPlugins(t *testing.T) {
	// Create a temporary directory for test plugins
	tempDir, err := os.MkdirTemp("", "plugin_discovery_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("discovers executable files", func(t *testing.T) {
		// Create test plugins
		plugin1 := filepath.Join(tempDir, "plugin1")
		if err := os.WriteFile(plugin1, []byte("#!/bin/bash\necho '[]'"), 0755); err != nil {
			t.Fatalf("Failed to write plugin1: %v", err)
		}

		plugin2 := filepath.Join(tempDir, "plugin2")
		if err := os.WriteFile(plugin2, []byte("#!/bin/bash\necho '[]'"), 0755); err != nil {
			t.Fatalf("Failed to write plugin2: %v", err)
		}

		// Create a non-executable file (should be skipped)
		nonExec := filepath.Join(tempDir, "notexec")
		if err := os.WriteFile(nonExec, []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to write non-executable: %v", err)
		}

		// Create a config file (should be skipped)
		configFile := filepath.Join(tempDir, "plugin1.json")
		if err := os.WriteFile(configFile, []byte(`{"name":"custom_name"}`), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg := PluginDiscoveryConfig{
			PluginsDir:        tempDir,
			Enabled:           true,
			DefaultTimeout:    5 * time.Second,
			ValidateOnStartup: false,
		}

		plugins, err := DiscoverPlugins(cfg)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}

		// Should find 2 plugins (plugin1 with custom name, plugin2)
		if len(plugins) != 2 {
			t.Errorf("expected 2 plugins, got %d", len(plugins))
		}

		// Check that custom name from config was applied
		found := false
		for _, p := range plugins {
			if p.config.Name == "custom_name" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected plugin1 to have custom_name from config file")
		}
	})

	t.Run("disabled discovery returns nil", func(t *testing.T) {
		cfg := PluginDiscoveryConfig{
			PluginsDir: tempDir,
			Enabled:    false,
		}

		plugins, err := DiscoverPlugins(cfg)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if plugins != nil {
			t.Errorf("expected nil, got %v", plugins)
		}
	})

	t.Run("non-existent directory returns nil", func(t *testing.T) {
		cfg := PluginDiscoveryConfig{
			PluginsDir:     "/nonexistent/path",
			Enabled:        true,
			DefaultTimeout: 5 * time.Second,
		}

		plugins, err := DiscoverPlugins(cfg)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if plugins != nil {
			t.Errorf("expected nil, got %v", plugins)
		}
	})

	t.Run("disabled plugin in config is skipped", func(t *testing.T) {
		disabledDir, err := os.MkdirTemp("", "disabled_plugin_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(disabledDir)

		plugin := filepath.Join(disabledDir, "disabled_plugin")
		if err := os.WriteFile(plugin, []byte("#!/bin/bash\necho '[]'"), 0755); err != nil {
			t.Fatalf("Failed to write plugin: %v", err)
		}

		configFile := filepath.Join(disabledDir, "disabled_plugin.json")
		if err := os.WriteFile(configFile, []byte(`{"enabled":false}`), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg := PluginDiscoveryConfig{
			PluginsDir:        disabledDir,
			Enabled:           true,
			DefaultTimeout:    5 * time.Second,
			ValidateOnStartup: false,
		}

		plugins, err := DiscoverPlugins(cfg)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}

		if len(plugins) != 0 {
			t.Errorf("expected 0 plugins (disabled), got %d", len(plugins))
		}
	})
}
