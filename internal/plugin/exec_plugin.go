// internal/plugin/exec_plugin.go
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

const defaultMaxOutputBytes = 5 * 1024 * 1024 // 5MB
const maxStderrCapture = 4096                  // 4KB

// ExecPlugin executes a shell script and parses its JSON output.
type ExecPlugin struct {
	config         PluginConfig
	mu             sync.Mutex
	lastExecution  time.Time
	lastStderr     string
	maxOutputBytes int64
}

// NewExecPlugin creates a new shell script plugin executor.
func NewExecPlugin(config PluginConfig) *ExecPlugin {
	return &ExecPlugin{
		config:         config,
		maxOutputBytes: defaultMaxOutputBytes,
	}
}

// Name returns the collector name with plugin_ prefix.
func (e *ExecPlugin) Name() string {
	return fmt.Sprintf("plugin_%s", e.config.Name)
}

// Collect executes the plugin and returns parsed metrics.
// Respects interval scheduling — returns empty if interval not elapsed.
func (e *ExecPlugin) Collect(ctx context.Context) ([]collector.Metric, error) {
	e.mu.Lock()
	if e.config.Interval > 0 && !e.lastExecution.IsZero() {
		elapsed := time.Since(e.lastExecution)
		interval := time.Duration(e.config.Interval) * time.Second
		if elapsed < interval {
			e.mu.Unlock()
			return []collector.Metric{}, nil
		}
	}
	e.mu.Unlock()

	metrics, err := e.execute(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.lastExecution = time.Now()
	e.mu.Unlock()

	return metrics, nil
}

// LastStderr returns the last captured stderr output for diagnostics.
func (e *ExecPlugin) LastStderr() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastStderr
}

func (e *ExecPlugin) execute(ctx context.Context) ([]collector.Metric, error) {
	timeout := e.config.GetTimeout(DefaultTimeout)
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.config.Path, e.config.Args...)

	// Set working directory (default /tmp)
	cmd.Dir = e.config.WorkingDir
	if cmd.Dir == "" {
		cmd.Dir = "/tmp"
	}

	// Safe environment — no os.Environ() inheritance
	cmd.Env = BuildSafeEnv(e.config.Env)

	// Orphan prevention on Linux
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}

	// Capture stdout with size limit
	var stdout bytes.Buffer
	stdoutLW := &limitedWriter{w: &stdout, limit: e.maxOutputBytes}
	cmd.Stdout = stdoutLW

	// Capture stderr (last N bytes for diagnostics)
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, limit: maxStderrCapture}

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	// Always capture stderr for diagnostics
	e.mu.Lock()
	e.lastStderr = stderr.String()
	e.mu.Unlock()

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("plugin %s timed out after %v", e.config.Name, timeout)
		}
		return nil, fmt.Errorf("plugin %s failed: %w (stderr: %s)", e.config.Name, err, truncate(stderr.String(), 200))
	}

	log.Debug().
		Str("plugin", e.config.Name).
		Dur("duration", duration).
		Int("output_bytes", stdout.Len()).
		Msg("Plugin executed")

	if stdout.Len() == 0 {
		return []collector.Metric{}, nil
	}

	// Check if output was truncated
	if stdoutLW.truncated {
		return nil, fmt.Errorf("plugin %s output exceeded %d bytes limit", e.config.Name, e.maxOutputBytes)
	}

	// Parse JSON output
	var pluginMetrics []PluginMetric
	if err := json.Unmarshal(stdout.Bytes(), &pluginMetrics); err != nil {
		return nil, fmt.Errorf("failed to parse plugin %s output: %w", e.config.Name, err)
	}

	// Validate and sanitize
	validated := ValidateMetricOutput(pluginMetrics, e.config.Name)

	// Convert to collector.Metric with prefixing
	prefix := fmt.Sprintf("plugin_%s_", e.config.Name)
	metrics := make([]collector.Metric, 0, len(validated))

	for _, pm := range validated {
		metricType := pm.Type
		if metricType == "" {
			metricType = "gauge"
		}
		if metricType != "gauge" && metricType != "counter" {
			metricType = "gauge"
		}

		labels := pm.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["plugin"] = e.config.Name

		metrics = append(metrics, collector.Metric{
			Name:   prefix + pm.Name,
			Labels: labels,
			Value:  pm.Value,
			Type:   metricType,
		})
	}

	return metrics, nil
}

// limitedWriter wraps a writer with a byte limit.
type limitedWriter struct {
	w         io.Writer
	limit     int64
	written   int64
	truncated bool
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written >= lw.limit {
		lw.truncated = true
		return len(p), nil // Discard but don't error — let the process finish
	}
	remaining := lw.limit - lw.written
	if int64(len(p)) > remaining {
		p = p[:remaining]
		lw.truncated = true
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
