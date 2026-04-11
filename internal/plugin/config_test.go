package plugin

import (
	"testing"
	"time"
)

func TestPluginConfig_GetTimeout(t *testing.T) {
	fallback := 30 * time.Second

	t.Run("returns timeout when set", func(t *testing.T) {
		cfg := PluginConfig{Timeout: 10}
		got := cfg.GetTimeout(fallback)
		want := 10 * time.Second
		if got != want {
			t.Errorf("GetTimeout: got %v, want %v", got, want)
		}
	})

	t.Run("returns fallback when timeout is zero", func(t *testing.T) {
		cfg := PluginConfig{Timeout: 0}
		got := cfg.GetTimeout(fallback)
		if got != fallback {
			t.Errorf("GetTimeout: got %v, want fallback %v", got, fallback)
		}
	})

	t.Run("returns fallback when timeout is negative", func(t *testing.T) {
		cfg := PluginConfig{Timeout: -5}
		got := cfg.GetTimeout(fallback)
		if got != fallback {
			t.Errorf("GetTimeout: got %v, want fallback %v", got, fallback)
		}
	})
}

func TestPluginConfig_IsEnabled(t *testing.T) {
	t.Run("nil pointer defaults to true", func(t *testing.T) {
		cfg := PluginConfig{Enabled: nil}
		if !cfg.IsEnabled() {
			t.Error("IsEnabled: expected true when Enabled is nil")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		v := true
		cfg := PluginConfig{Enabled: &v}
		if !cfg.IsEnabled() {
			t.Error("IsEnabled: expected true when Enabled is &true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		v := false
		cfg := PluginConfig{Enabled: &v}
		if cfg.IsEnabled() {
			t.Error("IsEnabled: expected false when Enabled is &false")
		}
	})
}
