package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
)

// metricNameRegex validates Prometheus metric names
var metricNameRegex = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// PluginMetric represents the JSON schema for plugin output
type PluginMetric struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
	Type   string            `json:"type,omitempty"` // "gauge" or "counter", defaults to "gauge"
}

// PluginConfig holds configuration for a single plugin
type PluginConfig struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Args       []string      `json:"args,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	Env        []string      `json:"env,omitempty"`
	WorkingDir string        `json:"working_dir,omitempty"`
	Enabled    bool          `json:"enabled"`
	Interval   int           `json:"interval_seconds,omitempty"` // Seconds between executions (0 = every collection cycle)
}

// PluginCollector implements Collector interface for external plugins
type PluginCollector struct {
	config            PluginConfig
	lastExecutionTime time.Time // Track last execution for interval-based scheduling
}

// NewPluginCollector creates a new plugin collector
func NewPluginCollector(config PluginConfig) *PluginCollector {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Name == "" {
		config.Name = filepath.Base(config.Path)
		// Remove extension if present
		if ext := filepath.Ext(config.Name); ext != "" {
			config.Name = config.Name[:len(config.Name)-len(ext)]
		}
	}
	return &PluginCollector{config: config}
}

// Name returns the collector name
func (c *PluginCollector) Name() string {
	return fmt.Sprintf("plugin_%s", c.config.Name)
}

// Validate runs the plugin once to verify it works correctly
// This is used for startup validation
func (c *PluginCollector) Validate(ctx context.Context) error {
	log.Debug().
		Str("plugin", c.config.Name).
		Msg("Validating plugin")

	// Run collection with a shorter timeout for validation
	validationTimeout := c.config.Timeout
	if validationTimeout > 10*time.Second {
		validationTimeout = 10 * time.Second
	}

	validationCtx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()

	metrics, err := c.collect(validationCtx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	log.Info().
		Str("plugin", c.config.Name).
		Int("metric_count", len(metrics)).
		Msg("Plugin validation successful")

	return nil
}

// Collect executes the plugin and parses its JSON output
// If an interval is configured, it only executes when enough time has passed
func (c *PluginCollector) Collect(ctx context.Context) ([]Metric, error) {
	// Check if interval-based scheduling applies
	if c.config.Interval > 0 && !c.lastExecutionTime.IsZero() {
		elapsed := time.Since(c.lastExecutionTime)
		intervalDuration := time.Duration(c.config.Interval) * time.Second

		if elapsed < intervalDuration {
			log.Debug().
				Str("plugin", c.config.Name).
				Dur("elapsed", elapsed).
				Dur("interval", intervalDuration).
				Msg("Skipping plugin execution (interval not elapsed)")
			return []Metric{}, nil // Return empty to reduce traffic
		}
	}

	// Execute the plugin
	metrics, err := c.collect(ctx)
	if err != nil {
		return nil, err
	}

	// Update last execution time on success
	c.lastExecutionTime = time.Now()

	return metrics, nil
}

// collect is the internal collection implementation
func (c *PluginCollector) collect(ctx context.Context) ([]Metric, error) {
	// Create timeout context for plugin execution
	execCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Build command
	cmd := exec.CommandContext(execCtx, c.config.Path, c.config.Args...)

	// Set working directory if specified
	if c.config.WorkingDir != "" {
		cmd.Dir = c.config.WorkingDir
	}

	// Set environment if specified
	if len(c.config.Env) > 0 {
		cmd.Env = append(os.Environ(), c.config.Env...)
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			log.Warn().
				Str("plugin", c.config.Name).
				Dur("timeout", c.config.Timeout).
				Msg("Plugin execution timed out")
			return nil, fmt.Errorf("plugin %s timed out after %v", c.config.Name, c.config.Timeout)
		}
		log.Warn().
			Str("plugin", c.config.Name).
			Str("stderr", stderr.String()).
			Err(err).
			Msg("Plugin execution failed")
		return nil, fmt.Errorf("plugin %s failed: %w", c.config.Name, err)
	}

	log.Debug().
		Str("plugin", c.config.Name).
		Dur("duration", duration).
		Int("output_bytes", stdout.Len()).
		Msg("Plugin executed successfully")

	// Handle empty output
	if stdout.Len() == 0 {
		log.Debug().
			Str("plugin", c.config.Name).
			Msg("Plugin returned empty output")
		return []Metric{}, nil
	}

	// Parse JSON output
	var pluginMetrics []PluginMetric
	if err := json.Unmarshal(stdout.Bytes(), &pluginMetrics); err != nil {
		log.Warn().
			Str("plugin", c.config.Name).
			Str("output", truncateString(stdout.String(), 500)).
			Err(err).
			Msg("Failed to parse plugin output as JSON")
		return nil, fmt.Errorf("failed to parse plugin %s output: %w", c.config.Name, err)
	}

	// Convert to collector.Metric with validation and prefixing
	metrics := make([]Metric, 0, len(pluginMetrics))
	prefix := fmt.Sprintf("plugin_%s_", c.config.Name)

	for i, pm := range pluginMetrics {
		// Validate metric name
		if pm.Name == "" {
			log.Warn().
				Str("plugin", c.config.Name).
				Int("index", i).
				Msg("Skipping metric with empty name")
			continue
		}

		if !validateMetricName(pm.Name) {
			log.Warn().
				Str("plugin", c.config.Name).
				Str("metric_name", pm.Name).
				Msg("Skipping metric with invalid name (must match [a-zA-Z_:][a-zA-Z0-9_:]*)")
			continue
		}

		// Determine metric type
		metricType := pm.Type
		if metricType == "" {
			metricType = "gauge"
		}
		if metricType != "gauge" && metricType != "counter" {
			log.Warn().
				Str("plugin", c.config.Name).
				Str("metric_name", pm.Name).
				Str("type", metricType).
				Msg("Invalid metric type, defaulting to gauge")
			metricType = "gauge"
		}

		// Prepare labels
		labels := pm.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		// Add plugin source label
		labels["plugin"] = c.config.Name

		// Create metric with prefixed name
		metrics = append(metrics, Metric{
			Name:   prefix + pm.Name,
			Labels: labels,
			Value:  pm.Value,
			Type:   metricType,
		})
	}

	return metrics, nil
}

// validateMetricName checks if a metric name follows Prometheus naming conventions
func validateMetricName(name string) bool {
	return metricNameRegex.MatchString(name)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
