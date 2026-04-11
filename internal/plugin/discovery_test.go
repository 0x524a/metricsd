package plugin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverPlugins(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "discovery_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("discovers executable files", func(t *testing.T) {
		writeTestPlugin(t, tmpDir, "plugin_a", "#!/bin/bash\necho '[]'")
		writeTestPlugin(t, tmpDir, "plugin_b", "#!/bin/bash\necho '[]'")
		os.WriteFile(filepath.Join(tmpDir, "notexec"), []byte("data"), 0644)
		os.WriteFile(filepath.Join(tmpDir, "plugin_a.json"), []byte(`{"name":"custom_a"}`), 0644)
		os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("docs"), 0644)

		plugins, err := DiscoverPlugins(tmpDir, 30*time.Second, false)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if len(plugins) != 2 {
			t.Errorf("expected 2 plugins, got %d", len(plugins))
		}
		found := false
		for _, p := range plugins {
			if p.config.Name == "custom_a" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected plugin_a to have custom_a name from config")
		}
	})

	t.Run("non-existent dir returns empty", func(t *testing.T) {
		plugins, err := DiscoverPlugins("/nonexistent", 30*time.Second, false)
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("expected 0, got %d", len(plugins))
		}
	})

	t.Run("disabled plugin skipped", func(t *testing.T) {
		disDir, _ := os.MkdirTemp("", "disabled_test")
		defer os.RemoveAll(disDir)
		writeTestPlugin(t, disDir, "disabled_p", "#!/bin/bash\necho '[]'")
		os.WriteFile(filepath.Join(disDir, "disabled_p.json"), []byte(`{"enabled":false}`), 0644)

		plugins, err := DiscoverPlugins(disDir, 30*time.Second, false)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("expected 0 (disabled), got %d", len(plugins))
		}
	})

	t.Run("symlink escaping dir rejected", func(t *testing.T) {
		symDir, _ := os.MkdirTemp("", "symlink_test")
		defer os.RemoveAll(symDir)
		os.Symlink("/usr/bin/env", filepath.Join(symDir, "escape"))

		plugins, err := DiscoverPlugins(symDir, 30*time.Second, false)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("expected 0 (escape), got %d", len(plugins))
		}
	})

	t.Run("config timeout parsed as seconds", func(t *testing.T) {
		cfgDir, _ := os.MkdirTemp("", "timeout_test")
		defer os.RemoveAll(cfgDir)
		writeTestPlugin(t, cfgDir, "timed", "#!/bin/bash\necho '[]'")
		os.WriteFile(filepath.Join(cfgDir, "timed.json"), []byte(`{"timeout":45}`), 0644)

		plugins, err := DiscoverPlugins(cfgDir, 30*time.Second, false)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if len(plugins) != 1 {
			t.Fatalf("expected 1, got %d", len(plugins))
		}
		expected := 45 * time.Second
		actual := plugins[0].config.GetTimeout(DefaultTimeout)
		if actual != expected {
			t.Errorf("expected %v, got %v", expected, actual)
		}
	})
}
