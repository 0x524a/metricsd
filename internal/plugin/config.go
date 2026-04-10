// internal/plugin/config.go
package plugin

import (
	"time"
)

// PluginConfig holds configuration for a single shell plugin.
// Timeout is specified in seconds in JSON, converted to time.Duration internally.
type PluginConfig struct {
	Name       string   `json:"name"`
	Path       string   `json:"-"` // Set by discovery, not from JSON
	Args       []string `json:"args,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`    // Seconds
	Env        []string `json:"env,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Enabled    *bool    `json:"enabled,omitempty"`     // Pointer to distinguish unset from false
	Interval   int      `json:"interval_seconds,omitempty"`
}

// GetTimeout returns the timeout as a Duration, defaulting to fallback if unset.
func (c PluginConfig) GetTimeout(fallback time.Duration) time.Duration {
	if c.Timeout > 0 {
		return time.Duration(c.Timeout) * time.Second
	}
	return fallback
}

// IsEnabled returns whether the plugin is enabled, defaulting to true if unset.
func (c PluginConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// PluginMetric represents the JSON schema for plugin output.
type PluginMetric struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
	Type   string            `json:"type,omitempty"` // "gauge" or "counter", defaults to "gauge"
}

// PluginHealth tracks the runtime health state of a single plugin.
// Owned by the Manager, not the plugin itself.
type PluginHealth struct {
	Name              string
	Status            string    // "ok", "failing", "circuit_open"
	ConsecutiveFails  int
	LastError         string
	LastSuccess       time.Time
	LastCollect       time.Time
	LastMetricCount   int
	CircuitOpenUntil  time.Time // Zero means circuit closed
}

// DefaultTimeout is the fallback plugin timeout.
const DefaultTimeout = 30 * time.Second

// MaxConsecutiveFailures before opening circuit breaker.
const MaxConsecutiveFailures = 5

// MaxCircuitOpenDuration caps the exponential backoff.
const MaxCircuitOpenDuration = 30 * time.Minute
