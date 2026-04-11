# Plugin System Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract plugin logic into a dedicated `internal/plugin/` package with a Manager (parallel collection, circuit breaker, health tracking), hardened ExecPlugin (output limits, signal handling, sandboxed env), compile-time Go plugin registry, enhanced orchestrator, and updated documentation.

**Architecture:** The `plugin.Manager` implements `collector.Collector`, wrapping N ExecPlugins and Go plugins behind a single interface. The orchestrator and registry remain unaware of plugins. Circuit breaker logic and health state live in the Manager, not in individual plugins.

**Tech Stack:** Go 1.24+, zerolog, gopsutil/v3, standard library (`os/exec`, `sync`, `context`, `encoding/json`), `golang.org/x/sync/errgroup`

**Spec:** `docs/superpowers/specs/2026-04-10-plugin-system-redesign.md`

---

## File Structure

### New files to create

| File | Responsibility |
|------|---------------|
| `internal/plugin/config.go` | `PluginConfig`, `PluginMetric` types, JSON parsing with seconds-based timeout |
| `internal/plugin/security.go` | `ValidatePluginPath()`, `BuildSafeEnv()`, `ValidateMetricOutput()` |
| `internal/plugin/exec_plugin.go` | `ExecPlugin` struct — execute shell script, parse JSON, SIGTERM→SIGKILL, output limit |
| `internal/plugin/exec_plugin_test.go` | Tests for ExecPlugin (success, timeout, bad JSON, output limit, signal handling) |
| `internal/plugin/security_test.go` | Tests for path validation, env sandboxing, output validation |
| `internal/plugin/manager.go` | `Manager` struct — parallel collection, circuit breaker, health state |
| `internal/plugin/manager_test.go` | Tests for Manager (parallel collection, circuit breaker, health) |
| `internal/plugin/discovery.go` | `DiscoverPlugins()` — scan dir, load configs, validate paths |
| `internal/plugin/discovery_test.go` | Tests for discovery (executable detection, config loading, symlink rejection) |
| `internal/plugin/registry.go` | `GoPluginFactory`, `Register()`, `GetRegistered()` |
| `internal/plugin/registry_test.go` | Tests for Go plugin registry |
| `docs/plugin-authoring.md` | Guide for writing shell plugins |

### Files to modify

| File | Change |
|------|--------|
| `cmd/metricsd/main.go` | Replace `collector.DiscoverPlugins` with `plugin.NewManager`, pass health provider to server |
| `internal/config/config.go` | Add `GoPluginConfig` type, add `GoPlugins` field to `PluginSystemConfig` |
| `internal/orchestrator/orchestrator.go` | Parallel `CollectAll` via errgroup, deadline warning, ship retry |
| `internal/server/server.go` | Accept `HealthProvider` interface, expose detailed `/health` |
| `internal/collector/collector.go` | Add `CollectAllParallel()` method using errgroup |
| `plugins/SUDO_SETUP.md` | Fix nanosecond timeout references |
| `plugins/process_monitor.md` | Fix nanosecond timeout references |
| `README.md` | Add plugin system, file shipper, Splunk HEC sections; update architecture diagram |
| `config.example.json` | Add `go_plugins` field example |

### Files to delete

| File | Reason |
|------|--------|
| `internal/collector/plugin.go` | Logic moves to `internal/plugin/exec_plugin.go` |
| `internal/collector/plugin_discovery.go` | Logic moves to `internal/plugin/discovery.go` |
| `internal/collector/plugin_test.go` | Tests move to `internal/plugin/` |

---

## Task 1: Create `internal/plugin/config.go` — Plugin Types

**Files:**
- Create: `internal/plugin/config.go`

- [ ] **Step 1: Create the plugin package and config types**

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /home/ritwik/devBed/rj/metricsd && go build ./internal/plugin/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/plugin/config.go
git commit -m "feat(plugin): add plugin config types and health tracking structs"
```

---

## Task 2: Create `internal/plugin/security.go` — Security Layer

**Files:**
- Create: `internal/plugin/security.go`
- Create: `internal/plugin/security_test.go`

- [ ] **Step 1: Write security tests**

```go
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
		// Check we're NOT inheriting random env vars from the process
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestValidate`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement security functions**

```go
// internal/plugin/security.go
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

var metricNameRegex = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// ValidatePluginPath resolves symlinks and verifies the path stays within pluginsDir.
// Returns the resolved absolute path, or an error if the path escapes or is invalid.
func ValidatePluginPath(pluginPath, pluginsDir string) (string, error) {
	absPluginsDir, err := filepath.Abs(pluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve plugins dir: %w", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(absPluginsDir)
	if err != nil {
		return "", fmt.Errorf("failed to eval plugins dir symlinks: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(pluginPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve plugin path %s: %w", pluginPath, err)
	}

	// Check the resolved path is inside the resolved plugins dir
	if !strings.HasPrefix(resolvedPath, resolvedDir+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin %s resolves to %s which is outside plugins dir %s", pluginPath, resolvedPath, resolvedDir)
	}

	// Warn if world-writable
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat plugin: %w", err)
	}
	if info.Mode()&0002 != 0 {
		log.Warn().
			Str("plugin", resolvedPath).
			Msg("Plugin is world-writable — this is a security risk")
	}

	return resolvedPath, nil
}

// BuildSafeEnv constructs a minimal environment for plugin execution.
// Does NOT inherit os.Environ(). Only includes safe defaults + explicit extras.
func BuildSafeEnv(extraEnv []string) []string {
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/nonexistent",
		"LANG=C.UTF-8",
	}
	env = append(env, extraEnv...)
	return env
}

// ValidateMetricOutput filters and sanitizes plugin output metrics.
// Rejects: empty names, invalid names, labels starting with __.
// Truncates: label values over 1024 chars.
func ValidateMetricOutput(metrics []PluginMetric, pluginName string) []PluginMetric {
	valid := make([]PluginMetric, 0, len(metrics))

	for _, pm := range metrics {
		if pm.Name == "" {
			log.Warn().Str("plugin", pluginName).Msg("Skipping metric with empty name")
			continue
		}
		if !metricNameRegex.MatchString(pm.Name) {
			log.Warn().Str("plugin", pluginName).Str("name", pm.Name).Msg("Skipping metric with invalid name")
			continue
		}

		// Check for reserved label prefixes and truncate long values
		hasReserved := false
		sanitizedLabels := make(map[string]string, len(pm.Labels))
		for k, v := range pm.Labels {
			if strings.HasPrefix(k, "__") {
				log.Warn().Str("plugin", pluginName).Str("label", k).Msg("Rejecting metric with reserved label prefix __")
				hasReserved = true
				break
			}
			if len(v) > 1024 {
				v = v[:1024]
			}
			sanitizedLabels[k] = v
		}
		if hasReserved {
			continue
		}

		pm.Labels = sanitizedLabels
		valid = append(valid, pm)
	}

	return valid
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/security.go internal/plugin/security_test.go
git commit -m "feat(plugin): add security layer — path validation, safe env, output validation"
```

---

## Task 3: Create `internal/plugin/exec_plugin.go` — Shell Script Executor

**Files:**
- Create: `internal/plugin/exec_plugin.go`
- Create: `internal/plugin/exec_plugin_test.go`

- [ ] **Step 1: Write ExecPlugin tests**

```go
// internal/plugin/exec_plugin_test.go
package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		path := writeTestPlugin(t, tmpDir, "good", `#!/bin/bash
echo '[{"name":"cpu","value":42.5,"type":"gauge","labels":{"env":"test"}}]'
`)
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
		// Generate a script that outputs more than MaxOutputBytes
		path := writeTestPlugin(t, tmpDir, "bigoutput", `#!/bin/bash
python3 -c "print('[' + ','.join(['{\"name\":\"m\",\"value\":1}'] * 100000) + ']')"
`)
		ep := NewExecPlugin(PluginConfig{Name: "big", Path: path, Timeout: 10})
		ep.maxOutputBytes = 1024 // 1KB limit for test
		_, err := ep.Collect(context.Background())
		if err == nil {
			t.Error("expected error for oversized output")
		}
	})

	t.Run("interval scheduling skips when not elapsed", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "interval", `#!/bin/bash
echo '[{"name":"m","value":1}]'
`)
		ep := NewExecPlugin(PluginConfig{Name: "interval", Path: path, Timeout: 5, Interval: 3600})

		// First call should succeed
		metrics, err := ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("First collect failed: %v", err)
		}
		if len(metrics) != 1 {
			t.Fatalf("expected 1 metric on first call, got %d", len(metrics))
		}

		// Second call should return empty (interval not elapsed)
		metrics, err = ep.Collect(context.Background())
		if err != nil {
			t.Fatalf("Second collect failed: %v", err)
		}
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics on second call (interval), got %d", len(metrics))
		}
	})

	t.Run("uses safe environment", func(t *testing.T) {
		path := writeTestPlugin(t, tmpDir, "envcheck", `#!/bin/bash
# Should NOT see USER from parent env
if [ -z "$USER" ]; then
  echo '[{"name":"env_safe","value":1}]'
else
  echo '[{"name":"env_safe","value":0}]'
fi
`)
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
		path := writeTestPlugin(t, tmpDir, "badnames", `#!/bin/bash
echo '[{"name":"valid_name","value":1},{"name":"123bad","value":2}]'
`)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestExecPlugin`
Expected: FAIL — `NewExecPlugin` not defined

- [ ] **Step 3: Implement ExecPlugin**

```go
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
	cmd.Stdout = &limitedWriter{w: &stdout, limit: e.maxOutputBytes}

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
	if lw, ok := cmd.Stdout.(*limitedWriter); ok && lw.truncated {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/exec_plugin.go internal/plugin/exec_plugin_test.go
git commit -m "feat(plugin): add ExecPlugin with output limits, safe env, signal handling"
```

---

## Task 4: Create `internal/plugin/discovery.go` — Plugin Discovery

**Files:**
- Create: `internal/plugin/discovery.go`
- Create: `internal/plugin/discovery_test.go`

- [ ] **Step 1: Write discovery tests**

```go
// internal/plugin/discovery_test.go
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

		// Non-executable — should be skipped
		os.WriteFile(filepath.Join(tmpDir, "notexec"), []byte("data"), 0644)
		// Config file — should be skipped
		os.WriteFile(filepath.Join(tmpDir, "plugin_a.json"), []byte(`{"name":"custom_a"}`), 0644)
		// Markdown — should be skipped
		os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("docs"), 0644)

		plugins, err := DiscoverPlugins(tmpDir, 30*time.Second, false)
		if err != nil {
			t.Fatalf("DiscoverPlugins failed: %v", err)
		}
		if len(plugins) != 2 {
			t.Errorf("expected 2 plugins, got %d", len(plugins))
		}

		// Check custom name from config
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
			t.Fatalf("expected nil error for non-existent dir, got: %v", err)
		}
		if len(plugins) != 0 {
			t.Errorf("expected 0 plugins, got %d", len(plugins))
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
			t.Errorf("expected 0 plugins (disabled), got %d", len(plugins))
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
			t.Errorf("expected 0 plugins (symlink escape), got %d", len(plugins))
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
			t.Fatalf("expected 1 plugin, got %d", len(plugins))
		}
		expected := 45 * time.Second
		actual := plugins[0].config.GetTimeout(DefaultTimeout)
		if actual != expected {
			t.Errorf("expected timeout %v, got %v", expected, actual)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestDiscoverPlugins`
Expected: FAIL — `DiscoverPlugins` not defined

- [ ] **Step 3: Implement DiscoverPlugins**

```go
// internal/plugin/discovery.go
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// skipExtensions lists file extensions that are not plugins.
var skipExtensions = []string{".json", ".md", ".txt", ".example", ".bak", ".log", ".old", ".swp", ".tmp"}

// DiscoverPlugins scans pluginsDir for executable files and returns ExecPlugin instances.
// Config is loaded from <plugin>.json sidecar files.
// If validate is true, each plugin is executed once at startup to verify it works.
func DiscoverPlugins(pluginsDir string, defaultTimeout time.Duration, validate bool) ([]*ExecPlugin, error) {
	info, err := os.Stat(pluginsDir)
	if os.IsNotExist(err) {
		log.Info().Str("dir", pluginsDir).Msg("Plugins directory does not exist, skipping discovery")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, err
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	var plugins []*ExecPlugin

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		rawPath := filepath.Join(pluginsDir, name)

		// Skip non-plugin extensions
		skip := false
		for _, ext := range skipExtensions {
			if strings.HasSuffix(name, ext) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Security: validate path (symlink check)
		resolvedPath, err := ValidatePluginPath(rawPath, pluginsDir)
		if err != nil {
			log.Warn().Str("file", name).Err(err).Msg("Skipping plugin — path validation failed")
			continue
		}

		// Check executable bit
		fileInfo, err := os.Stat(resolvedPath)
		if err != nil {
			log.Warn().Str("file", name).Err(err).Msg("Skipping plugin — stat failed")
			continue
		}
		if fileInfo.Mode()&0111 == 0 {
			continue
		}

		// Build config from sidecar JSON
		config := PluginConfig{
			Name: strings.TrimSuffix(name, filepath.Ext(name)),
			Path: resolvedPath,
		}

		configPath := rawPath + ".json"
		if data, err := os.ReadFile(configPath); err == nil {
			var fileCfg PluginConfig
			if err := json.Unmarshal(data, &fileCfg); err != nil {
				log.Warn().Str("config", configPath).Err(err).Msg("Failed to parse plugin config")
			} else {
				if fileCfg.Name != "" {
					config.Name = fileCfg.Name
				}
				if fileCfg.Timeout > 0 {
					config.Timeout = fileCfg.Timeout
				}
				if len(fileCfg.Args) > 0 {
					config.Args = fileCfg.Args
				}
				if len(fileCfg.Env) > 0 {
					config.Env = fileCfg.Env
				}
				if fileCfg.WorkingDir != "" {
					config.WorkingDir = fileCfg.WorkingDir
				}
				if fileCfg.Interval > 0 {
					config.Interval = fileCfg.Interval
				}
				if fileCfg.Enabled != nil {
					config.Enabled = fileCfg.Enabled
				}
			}
		}

		if !config.IsEnabled() {
			log.Info().Str("plugin", config.Name).Msg("Plugin disabled, skipping")
			continue
		}

		ep := NewExecPlugin(config)

		if validate {
			if _, err := ep.Collect(nil); err != nil {
				log.Warn().Str("plugin", config.Name).Err(err).Msg("Plugin failed validation, skipping")
				continue
			}
		}

		plugins = append(plugins, ep)
		log.Info().
			Str("plugin", config.Name).
			Str("path", resolvedPath).
			Dur("timeout", config.GetTimeout(defaultTimeout)).
			Msg("Discovered plugin")
	}

	return plugins, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/discovery.go internal/plugin/discovery_test.go
git commit -m "feat(plugin): add plugin discovery with sidecar config and security validation"
```

---

## Task 5: Create `internal/plugin/registry.go` — Go Plugin Registry

**Files:**
- Create: `internal/plugin/registry.go`
- Create: `internal/plugin/registry_test.go`

- [ ] **Step 1: Write registry tests**

```go
// internal/plugin/registry_test.go
package plugin

import (
	"context"
	"testing"

	"github.com/0x524A/metricsd/internal/collector"
)

// mockCollector is a test double for collector.Collector
type mockCollector struct {
	name    string
	metrics []collector.Metric
	err     error
}

func (m *mockCollector) Name() string { return m.name }
func (m *mockCollector) Collect(ctx context.Context) ([]collector.Metric, error) {
	return m.metrics, m.err
}

func TestGoPluginRegistry(t *testing.T) {
	// Reset registry for test isolation
	goPluginRegistry = make(map[string]GoPluginFactory)

	t.Run("register and retrieve", func(t *testing.T) {
		factory := func(cfg map[string]interface{}) (collector.Collector, error) {
			return &mockCollector{name: "test_go_plugin"}, nil
		}
		RegisterGoPlugin("test", factory)

		reg := GetRegisteredGoPlugins()
		if _, ok := reg["test"]; !ok {
			t.Error("expected 'test' factory in registry")
		}
	})

	t.Run("factory creates collector", func(t *testing.T) {
		factory := func(cfg map[string]interface{}) (collector.Collector, error) {
			addr := "default"
			if v, ok := cfg["addr"].(string); ok {
				addr = v
			}
			return &mockCollector{
				name:    "redis",
				metrics: []collector.Metric{{Name: "redis_up", Value: 1, Labels: map[string]string{"addr": addr}}},
			}, nil
		}
		RegisterGoPlugin("redis", factory)

		reg := GetRegisteredGoPlugins()
		c, err := reg["redis"](map[string]interface{}{"addr": "localhost:6379"})
		if err != nil {
			t.Fatalf("factory failed: %v", err)
		}
		if c.Name() != "redis" {
			t.Errorf("expected name 'redis', got %s", c.Name())
		}
		metrics, _ := c.Collect(context.Background())
		if len(metrics) != 1 || metrics[0].Labels["addr"] != "localhost:6379" {
			t.Errorf("unexpected metrics: %v", metrics)
		}
	})

	// Clean up
	goPluginRegistry = make(map[string]GoPluginFactory)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestGoPlugin`
Expected: FAIL — `GoPluginFactory` not defined

- [ ] **Step 3: Implement registry**

```go
// internal/plugin/registry.go
package plugin

import (
	"github.com/0x524A/metricsd/internal/collector"
)

// GoPluginFactory creates a collector.Collector from a config map.
// Used for compile-time Go plugin registration.
type GoPluginFactory func(config map[string]interface{}) (collector.Collector, error)

// goPluginRegistry holds all registered Go plugin factories.
var goPluginRegistry = make(map[string]GoPluginFactory)

// RegisterGoPlugin registers a Go plugin factory by name.
// Call this from init() in your plugin package.
func RegisterGoPlugin(name string, factory GoPluginFactory) {
	goPluginRegistry[name] = factory
}

// GetRegisteredGoPlugins returns all registered Go plugin factories.
func GetRegisteredGoPlugins() map[string]GoPluginFactory {
	return goPluginRegistry
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestGoPlugin`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/registry.go internal/plugin/registry_test.go
git commit -m "feat(plugin): add compile-time Go plugin registry"
```

---

## Task 6: Create `internal/plugin/manager.go` — Plugin Manager with Circuit Breaker

**Files:**
- Create: `internal/plugin/manager.go`
- Create: `internal/plugin/manager_test.go`

- [ ] **Step 1: Write Manager tests**

```go
// internal/plugin/manager_test.go
package plugin

import (
	"context"
	"fmt"
	"testing"
	"time"

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
			t.Errorf("expected 1 metric (from good plugin), got %d", len(metrics))
		}
	})

	t.Run("circuit breaker opens after consecutive failures", func(t *testing.T) {
		m := NewManager()
		failing := &mockCollector{name: "flaky", err: fmt.Errorf("fail")}
		m.AddGoPlugin("flaky", failing)

		// Fail MaxConsecutiveFailures times
		for i := 0; i < MaxConsecutiveFailures; i++ {
			m.Collect(context.Background())
		}

		// Check health — should be circuit_open
		health := m.GetHealth()
		if h, ok := health["flaky"]; !ok {
			t.Fatal("expected health entry for 'flaky'")
		} else if h.Status != "circuit_open" {
			t.Errorf("expected circuit_open, got %s", h.Status)
		}

		// Next collect should skip the flaky plugin (circuit open)
		metrics, _ := m.Collect(context.Background())
		if len(metrics) != 0 {
			t.Errorf("expected 0 metrics (circuit open), got %d", len(metrics))
		}
	})

	t.Run("circuit breaker resets on success", func(t *testing.T) {
		m := NewManager()
		flaky := &mockCollector{name: "flaky", err: fmt.Errorf("fail")}
		m.AddGoPlugin("flaky", flaky)

		// Fail a few times (but less than threshold)
		for i := 0; i < MaxConsecutiveFailures-1; i++ {
			m.Collect(context.Background())
		}

		// Now succeed
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

// Verify Manager satisfies collector.Collector
var _ collector.Collector = (*Manager)(nil)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -run TestManager`
Expected: FAIL — `NewManager` not defined

- [ ] **Step 3: Implement Manager**

```go
// internal/plugin/manager.go
package plugin

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x524A/metricsd/internal/collector"
)

// pluginEntry wraps a collector with its name for health tracking.
type pluginEntry struct {
	name      string
	collector collector.Collector
}

// Manager coordinates all plugin collectors with parallel execution,
// circuit breaker, and health tracking.
// Implements collector.Collector.
type Manager struct {
	mu      sync.RWMutex
	plugins []pluginEntry
	health  map[string]*PluginHealth
}

// NewManager creates a new plugin Manager.
func NewManager() *Manager {
	return &Manager{
		health: make(map[string]*PluginHealth),
	}
}

// Name returns "plugins".
func (m *Manager) Name() string {
	return "plugins"
}

// AddExecPlugin adds a shell script plugin to the manager.
func (m *Manager) AddExecPlugin(ep *ExecPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := ep.config.Name
	m.plugins = append(m.plugins, pluginEntry{name: name, collector: ep})
	m.health[name] = &PluginHealth{Name: name, Status: "ok"}
}

// AddGoPlugin adds a Go plugin collector to the manager.
func (m *Manager) AddGoPlugin(name string, c collector.Collector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins = append(m.plugins, pluginEntry{name: name, collector: c})
	m.health[name] = &PluginHealth{Name: name, Status: "ok"}
}

// Collect fans out to all plugins in parallel, applying circuit breaker logic.
func (m *Manager) Collect(ctx context.Context) ([]collector.Metric, error) {
	m.mu.RLock()
	entries := make([]pluginEntry, len(m.plugins))
	copy(entries, m.plugins)
	m.mu.RUnlock()

	type result struct {
		name    string
		metrics []collector.Metric
		err     error
	}

	results := make(chan result, len(entries))
	var wg sync.WaitGroup

	for _, entry := range entries {
		// Check circuit breaker
		m.mu.RLock()
		h := m.health[entry.name]
		m.mu.RUnlock()

		if h != nil && !h.CircuitOpenUntil.IsZero() && time.Now().Before(h.CircuitOpenUntil) {
			log.Debug().
				Str("plugin", entry.name).
				Time("circuit_open_until", h.CircuitOpenUntil).
				Msg("Skipping plugin — circuit open")
			continue
		}

		wg.Add(1)
		go func(e pluginEntry) {
			defer wg.Done()
			metrics, err := e.collector.Collect(ctx)
			results <- result{name: e.name, metrics: metrics, err: err}
		}(entry)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	var allMetrics []collector.Metric
	for r := range results {
		m.mu.Lock()
		h := m.health[r.name]
		h.LastCollect = time.Now()

		if r.err != nil {
			h.ConsecutiveFails++
			h.LastError = r.err.Error()
			log.Warn().
				Str("plugin", r.name).
				Int("consecutive_fails", h.ConsecutiveFails).
				Err(r.err).
				Msg("Plugin collection failed")

			if h.ConsecutiveFails >= MaxConsecutiveFailures {
				backoff := time.Duration(1<<uint(h.ConsecutiveFails-MaxConsecutiveFailures)) * time.Minute
				if backoff > MaxCircuitOpenDuration {
					backoff = MaxCircuitOpenDuration
				}
				h.CircuitOpenUntil = time.Now().Add(backoff)
				h.Status = "circuit_open"
				log.Warn().
					Str("plugin", r.name).
					Dur("backoff", backoff).
					Msg("Circuit breaker opened")
			} else {
				h.Status = "failing"
			}
		} else {
			h.ConsecutiveFails = 0
			h.CircuitOpenUntil = time.Time{}
			h.Status = "ok"
			h.LastSuccess = time.Now()
			h.LastMetricCount = len(r.metrics)
			h.LastError = ""
			allMetrics = append(allMetrics, r.metrics...)
		}
		m.mu.Unlock()
	}

	return allMetrics, nil
}

// GetHealth returns a snapshot of health state for all managed plugins.
func (m *Manager) GetHealth() map[string]PluginHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := make(map[string]PluginHealth, len(m.health))
	for k, v := range m.health {
		snapshot[k] = *v
	}
	return snapshot
}

// PluginCount returns the number of managed plugins.
func (m *Manager) PluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/ritwik/devBed/rj/metricsd && go test ./internal/plugin/... -v -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/manager.go internal/plugin/manager_test.go
git commit -m "feat(plugin): add Manager with parallel collection and circuit breaker"
```

---

## Task 7: Wire Plugin Manager into main.go and Remove Old Plugin Code

**Files:**
- Modify: `cmd/metricsd/main.go`
- Modify: `internal/config/config.go`
- Delete: `internal/collector/plugin.go`
- Delete: `internal/collector/plugin_discovery.go`
- Delete: `internal/collector/plugin_test.go`

- [ ] **Step 1: Add GoPluginConfig to config.go**

Add after the `PluginSystemConfig` struct in `internal/config/config.go`:

```go
// GoPluginEntry configures a compile-time registered Go plugin.
type GoPluginEntry struct {
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config,omitempty"`
}
```

Add the `GoPlugins` field to `PluginSystemConfig`:

```go
type PluginSystemConfig struct {
	Enabled               bool             `json:"enabled"`
	PluginsDir            string           `json:"plugins_dir"`
	DefaultTimeoutSeconds int              `json:"default_timeout_seconds,omitempty"`
	ValidateOnStartup     bool             `json:"validate_on_startup,omitempty"`
	GoPlugins             []GoPluginEntry  `json:"go_plugins,omitempty"`
}
```

- [ ] **Step 2: Update main.go to use plugin.Manager**

Replace the plugin registration block in `setupCollectors()` (lines 176-198) with:

```go
	// Register plugin manager
	if cfg.Collector.Plugins.Enabled {
		pluginMgr := plugin.NewManager()

		// Discover shell plugins
		defaultTimeout := time.Duration(cfg.Collector.Plugins.DefaultTimeoutSeconds) * time.Second
		if defaultTimeout == 0 {
			defaultTimeout = plugin.DefaultTimeout
		}
		execPlugins, err := plugin.DiscoverPlugins(
			cfg.Collector.Plugins.PluginsDir,
			defaultTimeout,
			cfg.Collector.Plugins.ValidateOnStartup,
		)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover plugins")
		}
		for _, ep := range execPlugins {
			pluginMgr.AddExecPlugin(ep)
		}

		// Instantiate registered Go plugins
		for _, gpCfg := range cfg.Collector.Plugins.GoPlugins {
			factories := plugin.GetRegisteredGoPlugins()
			factory, ok := factories[gpCfg.Name]
			if !ok {
				log.Warn().Str("name", gpCfg.Name).Msg("No registered Go plugin factory found, skipping")
				continue
			}
			c, err := factory(gpCfg.Config)
			if err != nil {
				log.Warn().Str("name", gpCfg.Name).Err(err).Msg("Go plugin factory failed, skipping")
				continue
			}
			pluginMgr.AddGoPlugin(gpCfg.Name, c)
		}

		if pluginMgr.PluginCount() > 0 {
			registry.Register(pluginMgr)
			log.Info().Int("plugin_count", pluginMgr.PluginCount()).Msg("Plugin manager registered")
		}
	}
```

Update the import block in `main.go` — add:

```go
	"github.com/0x524A/metricsd/internal/plugin"
```

Remove the unused import if `collector.DiscoverPlugins` / `collector.PluginDiscoveryConfig` are no longer referenced:

```go
	// These types no longer exist in collector package — remove references
```

- [ ] **Step 3: Delete old plugin files from collector package**

```bash
git rm internal/collector/plugin.go internal/collector/plugin_discovery.go internal/collector/plugin_test.go
```

- [ ] **Step 4: Verify compilation and tests**

Run: `cd /home/ritwik/devBed/rj/metricsd && go build ./... && go test ./... -count=1`
Expected: builds and all tests pass

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: wire plugin.Manager into main, remove old collector/plugin code"
```

---

## Task 8: Enhance Orchestrator — Parallel Collection, Deadline Warning, Ship Retry

**Files:**
- Modify: `internal/collector/collector.go`
- Modify: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Add errgroup dependency**

Run: `cd /home/ritwik/devBed/rj/metricsd && go get golang.org/x/sync/errgroup`

- [ ] **Step 2: Add CollectAllParallel to collector.Registry**

Add to `internal/collector/collector.go`:

```go
import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// CollectAllParallel collects from all registered collectors in parallel.
func (r *Registry) CollectAllParallel(ctx context.Context) ([]Metric, error) {
	type result struct {
		metrics []Metric
	}

	var mu sync.Mutex
	var allMetrics []Metric
	var wg sync.WaitGroup

	for _, c := range r.collectors {
		wg.Add(1)
		go func(col Collector) {
			defer wg.Done()
			metrics, err := col.Collect(ctx)
			if err != nil {
				return
			}
			mu.Lock()
			allMetrics = append(allMetrics, metrics...)
			mu.Unlock()
		}(c)
	}

	wg.Wait()
	return allMetrics, nil
}
```

- [ ] **Step 3: Update orchestrator with parallel collection, deadline warning, ship retry**

Replace the `collectAndShip` method in `internal/orchestrator/orchestrator.go`:

```go
func (o *Orchestrator) collectAndShip(ctx context.Context) {
	startTime := time.Now()

	log.Debug().Msg("Starting metrics collection")

	// Collect metrics from all collectors in parallel
	metrics, err := o.registry.CollectAllParallel(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to collect metrics")
		return
	}

	collectDuration := time.Since(startTime)

	// Deadline warning: if collection took >80% of interval, warn
	threshold := time.Duration(float64(o.interval) * 0.8)
	if collectDuration > threshold {
		log.Warn().
			Dur("collection_duration", collectDuration).
			Dur("interval", o.interval).
			Msg("Collection duration exceeds 80% of interval — consider increasing interval or reducing collectors")
	}

	log.Debug().
		Int("metric_count", len(metrics)).
		Dur("duration", collectDuration).
		Msg("Metrics collected")

	// Ship metrics with one retry on failure
	if err := o.shipper.Ship(ctx, metrics); err != nil {
		log.Warn().Err(err).Msg("Ship failed, retrying in 1s")
		time.Sleep(1 * time.Second)

		if err := o.shipper.Ship(ctx, metrics); err != nil {
			log.Error().Err(err).Msg("Ship retry failed")
			return
		}
	}

	log.Info().
		Int("metric_count", len(metrics)).
		Dur("total_duration", time.Since(startTime)).
		Msg("Collection and shipping cycle completed successfully")
}
```

- [ ] **Step 4: Verify compilation and tests**

Run: `cd /home/ritwik/devBed/rj/metricsd && go build ./... && go test ./... -count=1`
Expected: builds and all tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/collector/collector.go internal/orchestrator/orchestrator.go go.mod go.sum
git commit -m "feat(orchestrator): parallel collection, deadline warning, ship retry"
```

---

## Task 9: Enhance Health Endpoint

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/metricsd/main.go`

- [ ] **Step 1: Define HealthProvider interface and update Server**

Replace `internal/server/server.go` with:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// CollectorHealth is the health info for a single collector.
type CollectorHealth struct {
	Status            string `json:"status"`
	LastCollect       string `json:"last_collect,omitempty"`
	MetricCount       int    `json:"metric_count,omitempty"`
	ConsecutiveFails  int    `json:"consecutive_failures,omitempty"`
	LastError         string `json:"last_error,omitempty"`
}

// DetailedHealthStatus is the full health response.
type DetailedHealthStatus struct {
	Status     string                       `json:"status"`
	Uptime     float64                      `json:"uptime_seconds"`
	Collectors map[string]CollectorHealth   `json:"collectors,omitempty"`
}

// HealthProvider supplies plugin health data to the server.
type HealthProvider interface {
	GetHealthData() map[string]CollectorHealth
}

// Server provides HTTP endpoints for health checks.
type Server struct {
	host           string
	port           int
	server         *http.Server
	startTime      time.Time
	healthProvider HealthProvider
}

// NewServer creates a new HTTP server.
// healthProvider may be nil if no detailed health is available.
func NewServer(host string, port int, healthProvider HealthProvider) *Server {
	return &Server{
		host:           host,
		port:           port,
		startTime:      time.Now(),
		healthProvider: healthProvider,
	}
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: mux,
	}

	log.Info().Str("host", s.host).Int("port", s.port).Msg("Starting HTTP server")

	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		log.Info().Msg("Shutting down HTTP server")
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startTime).Seconds()
	status := DetailedHealthStatus{
		Status: "healthy",
		Uptime: uptime,
	}

	if s.healthProvider != nil {
		status.Collectors = s.healthProvider.GetHealthData()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode health status")
	}
}
```

- [ ] **Step 2: Update main.go to pass health provider to server**

In `cmd/metricsd/main.go`, the `NewServer` call changes to accept a third argument. Create a simple adapter struct in main.go:

```go
// pluginHealthAdapter adapts plugin.Manager to server.HealthProvider
type pluginHealthAdapter struct {
	mgr *plugin.Manager
}

func (a *pluginHealthAdapter) GetHealthData() map[string]server.CollectorHealth {
	if a.mgr == nil {
		return nil
	}
	health := a.mgr.GetHealth()
	result := make(map[string]server.CollectorHealth, len(health))
	for name, h := range health {
		result[name] = server.CollectorHealth{
			Status:           h.Status,
			LastCollect:      formatTime(h.LastCollect),
			MetricCount:      h.LastMetricCount,
			ConsecutiveFails: h.ConsecutiveFails,
			LastError:        h.LastError,
		}
	}
	return result
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
```

Update the `main()` function — store the plugin manager reference and pass to server:

```go
	var pluginMgr *plugin.Manager
	// ... (in setupCollectors, store pluginMgr)

	healthAdapter := &pluginHealthAdapter{mgr: pluginMgr}
	httpServer := server.NewServer(cfg.Server.Host, cfg.Server.Port, healthAdapter)
```

- [ ] **Step 3: Verify compilation and tests**

Run: `cd /home/ritwik/devBed/rj/metricsd && go build ./... && go test ./... -count=1`
Expected: builds and all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/server/server.go cmd/metricsd/main.go
git commit -m "feat(server): enhanced health endpoint with per-plugin status"
```

---

## Task 10: Documentation Updates

**Files:**
- Modify: `plugins/SUDO_SETUP.md`
- Modify: `plugins/process_monitor.md`
- Create: `docs/plugin-authoring.md`
- Modify: `config.example.json`

- [ ] **Step 1: Fix nanosecond references in SUDO_SETUP.md**

Search and replace all `"timeout": 60000000000` → `"timeout": 60` and `"timeout": 30000000000` → `"timeout": 30` in `plugins/SUDO_SETUP.md`.

- [ ] **Step 2: Fix nanosecond references in process_monitor.md**

In `plugins/process_monitor.md`:
- Change `"timeout": 30000000000` to `"timeout": 30`
- Change the table row from `timeout | integer | Plugin execution timeout in nanoseconds` to `timeout | integer | Plugin execution timeout in seconds`

- [ ] **Step 3: Update config.example.json**

Add `go_plugins` to the plugins section:

```json
{
  "collector": {
    "plugins": {
      "enabled": true,
      "plugins_dir": "./plugins",
      "default_timeout_seconds": 30,
      "validate_on_startup": true,
      "go_plugins": []
    }
  }
}
```

- [ ] **Step 4: Create plugin authoring guide**

Write `docs/plugin-authoring.md` covering:
- Expected JSON output schema (`[{"name":"...", "value":N, "type":"gauge|counter", "labels":{...}}]`)
- Exit codes (0=success, non-zero=failure)
- Timeout behavior (SIGTERM then SIGKILL after 2s)
- Environment (minimal PATH, no inherited secrets)
- Sidecar config file format (`.json` next to executable)
- Timeout field is in seconds
- Metric naming conventions (Prometheus-compatible: `^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
- Label restrictions (no `__` prefix, max 1024 chars per value)
- Testing: `./plugins/my_plugin | python3 -m json.tool`

- [ ] **Step 5: Commit**

```bash
git add plugins/SUDO_SETUP.md plugins/process_monitor.md config.example.json docs/plugin-authoring.md
git commit -m "docs: fix timeout units, add plugin authoring guide, update config example"
```

---

## Task 11: Update README.md

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add plugin system to features list**

Add under Features section:

```markdown
- **Plugin System**
  - Shell script plugins with JSON output
  - Automatic plugin discovery from directory
  - Per-plugin timeout and interval scheduling
  - Circuit breaker for failing plugins
  - Compile-time Go plugin extension point
  - Security: path validation, sandboxed execution environment

- **Splunk Integration**
  - Splunk HEC (HTTP Event Collector) shipper
  - JSON file shipper for Splunk Universal Forwarder
  - Single-metric and multi-metric JSON formats

- **Debian Packaging**
  - `.deb` packages for amd64 and arm64
  - systemd service with security hardening
  - Automatic user/group creation
```

- [ ] **Step 2: Update architecture diagram**

Replace the architecture diagram with:

```
metricsd/
├── cmd/metricsd/           # Application entry point
├── internal/
│   ├── collector/          # Collector interface, registry, system/GPU/HTTP collectors
│   ├── plugin/             # Plugin manager, exec plugin, discovery, security, Go registry
│   ├── config/             # Configuration management
│   ├── shipper/            # Prometheus, HTTP JSON, Splunk HEC, file shippers
│   ├── orchestrator/       # Collection orchestration (parallel, retry)
│   └── server/             # HTTP health endpoint
├── plugins/                # Shell script plugins + sidecar configs
├── packaging/debian/       # Debian package scripts + systemd service
└── docs/                   # Plugin authoring guide, specs
```

- [ ] **Step 3: Update config example to use seconds for timeout**

Change the example JSON in README from `"timeout": 30000000000` to standard seconds-based values, and add the plugins section.

- [ ] **Step 4: Add Plugin System section**

Add a new section after Configuration Options explaining plugin usage, discovery, writing custom plugins, and the Go plugin interface.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: update README with plugin system, Splunk, packaging, architecture"
```

---

## Task 12: Internal Metrics — metricsd Self-Monitoring

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Add internal metrics to orchestrator**

The orchestrator already knows collection duration and ship success/failure. Emit these as `collector.Metric` values appended to each collection cycle. Add to `collectAndShip` in `internal/orchestrator/orchestrator.go`, after the `CollectAllParallel` call:

```go
	// Append internal metrics about metricsd itself
	internalMetrics := []collector.Metric{
		{
			Name:   "metricsd_collection_duration_seconds",
			Value:  collectDuration.Seconds(),
			Type:   "gauge",
			Labels: map[string]string{},
		},
		{
			Name:   "metricsd_collectors_active",
			Value:  float64(len(metrics)), // rough proxy; exact count requires registry method
			Type:   "gauge",
			Labels: map[string]string{},
		},
	}
	metrics = append(metrics, internalMetrics...)
```

After the ship call (success path), append ship duration:

```go
	shipDuration := time.Since(shipStart)
	// These will be included in the *next* cycle's metrics
	o.lastShipDuration = shipDuration
	o.lastShipSuccess = true
```

Store `lastShipDuration` and `lastShipSuccess` as fields on `Orchestrator` and emit them at the start of the next cycle. This avoids chicken-and-egg (can't include ship duration in the metrics being shipped).

- [ ] **Step 2: Verify compilation and tests**

Run: `cd /home/ritwik/devBed/rj/metricsd && go build ./... && go test ./... -count=1`
Expected: builds and all tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrator/orchestrator.go
git commit -m "feat(orchestrator): emit internal metricsd self-monitoring metrics"
```
