# Plugin System Redesign — Design Spec

**Date:** 2026-04-10
**Branch:** `feature/plugin-enhancement`
**Status:** Draft
**Scope:** Plugin architecture, security hardening, orchestrator improvements, documentation

## Context

metricsd is a Go metrics collector that ships system/application metrics to remote endpoints (Prometheus, HTTP JSON, Splunk HEC, local files). The `feature/plugin-enhancement` branch added a shell-script-based plugin system, file shipper, Splunk HEC shipper, and Debian packaging.

A code review identified bugs, security gaps, and architectural risks. This spec covers the full redesign of the plugin subsystem plus related improvements.

### Constraints

- **Deployment target:** Bare metal / VMs (container support deferred)
- **Plugin models:** Shell scripts (primary) + compile-time Go interface (extension point)
- **No dynamic loading:** No `go-plugin`, no gRPC, no `.so` files
- **Hot reload:** Not required now; design should not preclude adding it later
- **Sandboxing boundary:** systemd service unit, not Go-level seccomp/namespaces

## 1. Architecture

### Current Structure

```
internal/collector/
  collector.go          # Collector interface, Registry, CollectAll
  plugin.go             # PluginCollector (exec + parse + interval logic)
  plugin_discovery.go   # Directory scanning, config loading, validation
  system.go / gpu.go / http.go
```

Plugin logic is mixed into `collector/` — discovery, execution, security, config parsing all in two files.

### Proposed Structure

```
internal/
  collector/
    collector.go          # Collector interface + Registry (unchanged)
    system.go             # System metrics (unchanged)
    gpu.go / gpu_stub.go  # GPU metrics (unchanged)
    http.go               # HTTP scraper (unchanged)
  plugin/                 # NEW package
    manager.go            # PluginManager: lifecycle, parallel collection, circuit breaker
    exec_plugin.go        # ExecPlugin: shell script executor
    registry.go           # GoPluginFactory: compile-time Go plugin registration
    config.go             # Plugin config types, JSON parsing (seconds-based timeout)
    security.go           # Path validation, env sandboxing, output validation
  config/
    config.go             # App config (unchanged)
  shipper/                # (unchanged)
  orchestrator/
    orchestrator.go       # Enhanced: parallel collection, deadline warning, ship retry
  server/
    server.go             # Enhanced: detailed health endpoint
```

**Key principle:** `plugin.Manager` implements `collector.Collector`. The orchestrator and registry are unaware that plugins exist — they see a single collector that returns merged metrics.

## 2. Plugin Manager

### Struct

```go
type Manager struct {
    mu              sync.RWMutex
    execPlugins     []*ExecPlugin
    goPlugins       []collector.Collector
    pluginsDir      string
    defaultTimeout  time.Duration
    validateOnStart bool
}
```

### Responsibilities

- Implements `collector.Collector` (Name: `"plugins"`)
- Fans out collection to all managed plugins in parallel (goroutine per plugin)
- Enforces per-plugin timeout via `context.WithTimeout`
- **Owns all per-plugin health state:** consecutive failure count, last error, last success time. This state lives in the Manager (in a `map[string]*pluginHealth`), not in ExecPlugin or Go plugin structs — plugins are stateless executors, the Manager is the supervisor.
- Circuit breaker: after N consecutive failures (configurable, default 5), disables plugin with exponential backoff (1m → 2m → 4m → ... → 30m cap). Single success resets.
- Exposes health data for the `/health` endpoint

### Collection Flow

```
Manager.Collect(ctx)
  |-- for each ExecPlugin  --> goroutine with per-plugin timeout
  |-- for each GoPlugin    --> goroutine with per-plugin timeout
  |-- wait all (bounded by parent ctx)
  |-- merge results
  |-- update health stats (success/fail counters per plugin)
  |-- return merged []Metric
```

### Circuit Breaker Detail

| Consecutive Failures | Action |
|---------------------|--------|
| < 5 | Execute normally, log warning |
| 5 | Open circuit, skip for 1 min |
| 6 | Skip for 2 min |
| 7 | Skip for 4 min |
| ... | Double each time, cap at 30 min |
| Next success | Reset to 0, close circuit |

## 3. ExecPlugin (Shell Script Executor)

Hardened version of the current `plugin.go`. Same concept: exec a script, parse JSON stdout.

### Struct

```go
type ExecPlugin struct {
    config          PluginConfig
    mu              sync.Mutex
    lastExecution   time.Time
    lastStderr      string   // last N bytes for diagnostics
    maxOutputBytes  int64    // default 5MB
}
```

### Changes from Current Implementation

| Area | Current | Proposed |
|------|---------|----------|
| **Config parsing** | Raw `map[string]interface{}` with manual type assertions | Proper `json.Unmarshal` into `PluginConfig` struct |
| **Timeout unit** | Ambiguous (JSON values were nanoseconds) | Seconds in JSON, converted internally. Already fixed in code. |
| **Output size** | Unlimited `bytes.Buffer` | `io.LimitReader` on stdout pipe, default 5MB, configurable |
| **Stderr** | Logged on failure only | Always capture last 4KB, expose in health endpoint |
| **Signal handling** | `exec.CommandContext` sends SIGKILL immediately | SIGTERM first, 2s grace period, then SIGKILL |
| **Environment** | Full `os.Environ()` inherited when custom env set | Minimal: `PATH=/usr/local/bin:/usr/bin:/bin`, `HOME=/nonexistent`, `LANG=C.UTF-8`, plus explicit env from config |
| **Orphan prevention** | None | `SysProcAttr.Pdeathsig = syscall.SIGTERM` (Linux) |
| **Meta-metrics** | None | Emits `plugin_execution_duration_seconds`, `plugin_execution_status`, `plugin_last_success_timestamp` per plugin |
| **Thread safety** | No mutex on `lastExecutionTime` | Mutex around interval check and timestamp update. Already fixed in code. |

## 4. Go Plugin Registry (Compile-Time Extension)

### Interface

```go
// plugin/registry.go
type GoPluginFactory func(config map[string]interface{}) (collector.Collector, error)

var registry = make(map[string]GoPluginFactory)

func Register(name string, factory GoPluginFactory) {
    registry[name] = factory
}

func GetRegistered() map[string]GoPluginFactory {
    return registry
}
```

### Usage

Third parties implement `collector.Collector`, register via `init()`:

```go
package myplugins

import "github.com/0x524A/metricsd/internal/plugin"

func init() {
    plugin.Register("redis", NewRedisCollector)
}
```

Custom binary with side-effect import:

```go
package main

import (
    _ "myorg/myplugins"  // triggers init() registration
    // ... rest identical to cmd/metricsd/main.go
)
```

### Configuration

Go plugins configured in `config.json`:

```json
{
  "collector": {
    "plugins": {
      "enabled": true,
      "plugins_dir": "plugins",
      "go_plugins": [
        {"name": "redis", "config": {"addr": "localhost:6379"}}
      ]
    }
  }
}
```

The Manager instantiates registered factories with the provided config map.

### Design Rationale

- No runtime dependencies or version coupling (unlike `go/plugin` package)
- No IPC overhead (unlike hashicorp/go-plugin)
- Testable: factories are plain functions
- The Manager treats Go plugins identically to ExecPlugins — same timeout, circuit breaker, health tracking

## 5. Security Model

All security logic centralized in `plugin/security.go`. Three layers.

### Layer 1: Plugin Path Validation

- Resolve all symlinks via `filepath.EvalSymlinks`
- Verify resolved path is within the configured plugins directory
- Reject plugins owned by non-root users when running as root (plugins dir should be root-owned in production)
- Warn if file is world-writable (`mode & 0002 != 0`)

Already partially implemented in the bug-fix pass.

### Layer 2: Execution Sandboxing

- **Minimal environment:** `PATH=/usr/local/bin:/usr/bin:/bin`, `HOME=/nonexistent`, `LANG=C.UTF-8`, plus explicitly configured env vars from plugin JSON config
- **No inherited secrets:** Do not pass `os.Environ()`. Only the minimal set above.
- **WorkingDir:** Defaults to `/tmp` (not metricsd's CWD) unless explicitly configured in plugin JSON
- **Orphan prevention:** `SysProcAttr{Pdeathsig: syscall.SIGTERM}` on Linux. If metricsd crashes, spawned plugins receive SIGTERM.

### Layer 3: Output Validation

- **Max output size:** 5MB default, enforced via `io.LimitReader` on stdout pipe
- **JSON parsing:** `json.NewDecoder` with `DisallowUnknownFields` on plugin *output* (the `[]PluginMetric` array) — reject unexpected fields in metric output. Plugin *config* files are parsed permissively (they may contain arbitrary plugin-specific keys like `processes`).
- **Metric name validation:** Existing regex `^[a-zA-Z_:][a-zA-Z0-9_:]*$` (keep as-is)
- **Label key validation:** Reject labels starting with `__` (reserved in Prometheus)
- **Label value length:** Cap at 1024 characters (prevents cardinality explosion)

### What This Does NOT Do

- **No seccomp/namespace sandboxing** — this is systemd's job. The `.service` file should use `ProtectSystem=strict`, `ProtectHome=yes`, `NoNewPrivileges=yes`, `PrivateTmp=yes`. Documented in packaging section.
- **No chroot** — plugins need system tools (`pgrep`, `nerdctl`, `nvidia-smi`).

## 6. Orchestrator & Observability

### Orchestrator Changes

| Area | Current | Proposed |
|------|---------|----------|
| **Collection** | Sequential `for` loop over collectors | Parallel via `errgroup`, concurrency capped at collector count |
| **Deadline warning** | None | Log warning if collection exceeds 80% of configured interval |
| **Ship retry** | Fail → log → wait for next cycle | One immediate retry with 1s backoff before giving up |
| **Error tracking** | Log only | Per-collector error count exposed to health endpoint |

### Health Endpoint (`/health`)

Current: returns `200 OK` with no body. Proposed:

```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "collectors": {
    "system": {"status": "ok", "last_collect": "2026-04-10T19:00:00Z", "metric_count": 42},
    "plugin_nvidia_smi": {"status": "circuit_open", "consecutive_failures": 5, "last_error": "timeout"}
  },
  "shipper": {
    "type": "splunk_hec",
    "status": "ok",
    "last_ship": "2026-04-10T19:00:00Z"
  }
}
```

**Status codes:** 200 if core collectors work, 503 if shipper failing or all collectors down. Individual plugin failures do not degrade overall health.

### Internal Metrics

metricsd emits metrics about itself, shipped alongside collected metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `metricsd_collection_duration_seconds` | gauge | Time for last collection cycle |
| `metricsd_ship_duration_seconds` | gauge | Time for last ship operation |
| `metricsd_ship_errors_total` | counter | Cumulative ship failures |
| `metricsd_collectors_active` | gauge | Number of active collectors |
| `metricsd_plugins_healthy` | gauge | Plugins with closed circuit |
| `metricsd_plugins_circuit_open` | gauge | Plugins with open circuit |

## 7. Documentation & Packaging

### Documentation Fixes

| Document | Issue | Fix |
|----------|-------|-----|
| `README.md` | No mention of plugins, file shipper, Splunk HEC, Debian packaging | Add sections for each |
| `README.md` | Architecture diagram outdated | Update with `plugin/`, `file.go`, `splunk_hec.go` |
| `README.md` | Config example shows nanosecond timeout | Update to seconds |
| `SUDO_SETUP.md` | Example JSON uses nanosecond timeout | Update to seconds |
| `process_monitor.md` | Table says "timeout in nanoseconds" | Correct to seconds |
| **New:** `docs/plugin-authoring.md` | Does not exist | Write guide: expected JSON schema, exit codes, error handling, testing |

### Packaging Fixes

**`packaging/debian/metricsd.service`** — add systemd hardening:

```ini
[Service]
ProtectSystem=strict
ProtectHome=yes
NoNewPrivileges=yes
PrivateTmp=yes
ReadWritePaths=/var/lib/metricsd
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
```

**`config.example.json`** — add plugin section:

```json
{
  "collector": {
    "plugins": {
      "enabled": false,
      "plugins_dir": "/usr/lib/metricsd/plugins",
      "default_timeout_seconds": 30,
      "validate_on_startup": true
    }
  }
}
```

## 8. Out of Scope (YAGNI)

| Item | Reason |
|------|--------|
| Hot reload | Nice-to-have. Design supports adding `Manager.Rescan()` later. |
| Container/K8s deployment | Deferred to later phase per project decision. |
| Dynamic Go plugin loading | Unnecessary complexity. Compile-time registration sufficient. |
| seccomp/namespace sandboxing | systemd handles this at the service level. |
| Plugin marketplace / registry | No current need. |
| Windows support | Bare metal Linux only for now. |

## 9. Implementation Order

1. Extract `plugin/` package (manager, exec_plugin, config, security)
2. Implement circuit breaker in Manager
3. Harden ExecPlugin (output limit, signal handling, minimal env, stderr capture)
4. Go plugin registry
5. Orchestrator parallel collection + deadline warning + ship retry
6. Health endpoint enhancement
7. Internal metrics
8. Documentation updates (README, SUDO_SETUP, process_monitor, new plugin-authoring guide)
9. Packaging fixes (systemd hardening, config.example.json)
