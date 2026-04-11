---
name: metricsd-ops
description: Use when investigating metricsd issues — plugin failures, collection errors, shipping problems, health degradation, or any operational troubleshooting on deployed metricsd instances. Also use when deploying, configuring, or adding new plugins/endpoints.
---

# metricsd Operations & Troubleshooting

## Overview

metricsd is a Go metrics collector at `~/devBed/rj/metricsd` (module `github.com/0x524A/metricsd`). Collects system metrics, scrapes HTTP endpoints (Prometheus format), and runs shell-script plugins. Ships to Prometheus Remote Write, HTTP JSON, Splunk HEC, or local JSON files.

## Architecture

```
Orchestrator (15s tick) ──> Registry.CollectAllParallel() ──> Shipper.Ship()
                                    |
                    ┌───────────────┼───────────────┐
                    v               v               v
              SystemCollector  HTTPCollector   plugin.Manager
              (CPU/mem/disk/   (scrapes        (parallel exec,
               net/load/GPU)    /metrics)       circuit breaker)
                                                    |
                                        ┌───────────┼───────────┐
                                        v           v           v
                                   ExecPlugin  ExecPlugin  GoPlugin
                                   (shell)     (shell)     (compiled)
```

### Key Packages

| Package | Path | Responsibility |
|---------|------|---------------|
| `collector` | `internal/collector/` | `Collector` interface, `Registry`, system/GPU/HTTP collectors |
| `plugin` | `internal/plugin/` | `Manager`, `ExecPlugin`, discovery, security, Go registry |
| `shipper` | `internal/shipper/` | Prometheus, HTTP JSON, Splunk HEC, file shippers |
| `orchestrator` | `internal/orchestrator/` | Tick loop, parallel collection, deadline warning, ship retry |
| `server` | `internal/server/` | `/health` endpoint with per-plugin status |
| `config` | `internal/config/` | JSON config + env var overrides |

### Key Interfaces

```go
// internal/collector/collector.go
type Collector interface {
    Collect(ctx context.Context) ([]Metric, error)
    Name() string
}

// internal/shipper/shipper.go
type Shipper interface {
    Ship(ctx context.Context, metrics []collector.Metric) error
    Close() error
}

// internal/server/server.go
type HealthProvider interface {
    GetHealthData() map[string]CollectorHealth
}
```

## Deployed Instances

### 192.168.1.159 (5GVI-LR202412001610)

- **OS:** Ubuntu 22.04, x86_64, Intel i7-9700TE, 32GB RAM
- **GPU:** NVIDIA A2 (15GB VRAM), driver 575.57, CUDA 12.9
- **Credentials:** vzvision/vzvision
- **Binary:** `~/metricsd/metricsd`
- **Config:** `~/metricsd/config.json`
- **Plugins:** `~/metricsd/plugins/`
- **Health:** `http://localhost:9191/health`
- **Output:** `/tmp/metricsd-output.json` (JSON lines, file shipper)
- **Logs:** `/tmp/metricsd.log`
- **Nerdctl sudo shim:** `~/metricsd/bin/nerdctl` (wraps `sudo /usr/local/bin/nerdctl`)
- **Sudoers:** `/etc/sudoers.d/metricsd-nerdctl`

**Containers running:** vision-rabbitmq, vision-adaptor, cvs-engine, vision-postgres, vision-cvs-engine

**CVS-Engine Prometheus endpoint:** `http://localhost:9090/metrics` (23 metric types: container resources, process stats, camera FPS/detection, deployment entry/exit, evidence states)

**Active plugins:** nvidia_smi, nerdctl, process_top, lm_sensors, ubuntu_pro, ufw
**Disabled:** process_monitor (JSON parse bug — newlines in /proc cmdline)

## Diagnostic Commands

### Quick Health Check

```bash
# SSH shorthand
SSH="sshpass -p 'vzvision' ssh vzvision@192.168.1.159"

# Health endpoint (per-plugin status, circuit breaker state)
$SSH 'curl -s http://localhost:9191/health | python3 -m json.tool'

# Is it running?
$SSH 'pgrep -a metricsd'

# Recent logs (last 50 lines)
$SSH 'tail -50 /tmp/metricsd.log'

# Metrics output count
$SSH 'wc -l /tmp/metricsd-output.json'

# Errors in log
$SSH 'grep -E "ERR|WRN|circuit" /tmp/metricsd.log | tail -20'
```

### Plugin Diagnostics

```bash
# Test a plugin directly
$SSH '~/metricsd/plugins/nvidia_smi | python3 -m json.tool'

# Test nerdctl with sudo shim
$SSH 'PATH=~/metricsd/bin:$PATH ~/metricsd/plugins/nerdctl | python3 -m json.tool'

# Check plugin configs
$SSH 'for f in ~/metricsd/plugins/*.json; do echo "=== $f ==="; cat "$f"; done'

# Check which plugins are discovered
$SSH 'grep "Discovered plugin\|disabled\|skipping" /tmp/metricsd.log'
```

### Metrics Inspection

```bash
# Sample specific metric by name
$SSH "grep 'cvs_camera_fps' /tmp/metricsd-output.json | tail -1 | python3 -m json.tool"

# Count metrics per plugin
$SSH "grep 'plugin_nvidia_smi' /tmp/metricsd-output.json | wc -l"

# CVS-Engine endpoint directly
$SSH 'curl -s http://localhost:9090/metrics'

# Check for NaN/Inf (would be skipped by shippers)
$SSH "grep -i 'nan\|inf' /tmp/metricsd-output.json"
```

### Start / Stop / Restart

```bash
# Stop
$SSH 'pkill -f "metricsd -config"'

# Start (setsid keeps it alive after SSH disconnect)
$SSH 'cd ~/metricsd && setsid ./metricsd -config config.json -log-level debug > /tmp/metricsd.log 2>&1 &'

# Restart
$SSH 'pkill -f "metricsd -config"; sleep 1; cd ~/metricsd && rm -f /tmp/metricsd.log && setsid ./metricsd -config config.json -log-level debug > /tmp/metricsd.log 2>&1 &'
```

## Common Failure Modes

### Plugin "path validation failed"

**Symptom:** `Skipping plugin — path validation failed: plugin X resolves to Y which is outside plugins dir Z`

**Cause:** Relative `plugins_dir` in config (e.g. `./plugins`) and `ValidatePluginPath` comparing resolved relative path against absolute dir.

**Fix:** Fixed in commit `cedfe36` — `ValidatePluginPath` now makes plugin path absolute before `EvalSymlinks`. If you see this on an old binary, rebuild and redeploy.

### Plugin "invalid character in string literal"

**Symptom:** `failed to parse plugin process_monitor output: invalid character '\n' in string literal`

**Cause:** Shell plugin embeds process cmdline containing newlines into JSON string without escaping. Common with `process_monitor` reading `/proc/<pid>/cmdline`.

**Fix:** The plugin shell script needs to sanitize cmdline output: `tr '\n\0' ' '` before embedding in JSON. Circuit breaker handles it gracefully (skips, retries with backoff).

### Circuit Breaker Open

**Symptom:** Health endpoint shows `"status": "circuit_open"` for a plugin.

**Cause:** Plugin failed `MaxConsecutiveFailures` (5) times consecutively.

**Behavior:** Exponential backoff: 1m, 2m, 4m, 8m, ... capped at 30m. Single success resets.

**Investigate:** Check `last_error` in health response. Common causes: timeout (plugin too slow), JSON parse error (bad output), non-zero exit (plugin crashed).

### nerdctl Sees 0 Containers

**Symptom:** `plugin_nerdctl_containers_total: 0` but containers exist.

**Cause:** nerdctl needs root/sudo to access containerd socket.

**Fix:** Create sudo shim at `~/metricsd/bin/nerdctl`:
```bash
#!/bin/bash
exec sudo /usr/local/bin/nerdctl "$@"
```
Add sudoers: `echo "vzvision ALL=(ALL) NOPASSWD: /usr/local/bin/nerdctl *" > /etc/sudoers.d/metricsd-nerdctl`
Configure plugin env to prepend shim PATH:
```json
{"env": ["PATH=/home/vzvision/metricsd/bin:/usr/local/bin:/usr/bin:/bin"]}
```

### Port Already in Use

**Symptom:** `HTTP server error: listen tcp 0.0.0.0:8080: bind: address already in use`

**Fix:** Change `server.port` in config.json. On 192.168.1.159, ports 8080, 8081, 9090 are in use. Currently using 9191.

### Collection Duration Warning

**Symptom:** `Collection duration exceeds 80% of interval`

**Cause:** Plugins are slow (especially `ubuntu_pro` at ~7s). With 15s interval and ~7s collection, we're at ~47%. Safe for now.

**Fix:** Increase `interval_seconds` or optimize slow plugins. The `ubuntu_pro` plugin queries apt which is inherently slow.

### Ship Retry

**Behavior:** If `Ship()` fails, orchestrator waits 1s (context-aware — cancels on shutdown) then retries once. Second failure logs error and moves on.

## Configuration Reference

### Config File (`config.json`)

```json
{
  "server": {"host": "0.0.0.0", "port": 9191},
  "collector": {
    "interval_seconds": 15,
    "enable_cpu": true, "enable_memory": true,
    "enable_disk": true, "enable_network": true,
    "enable_gpu": false,
    "plugins": {
      "enabled": true,
      "plugins_dir": "./plugins",
      "default_timeout_seconds": 30,
      "validate_on_startup": true,
      "go_plugins": []
    }
  },
  "shipper": {
    "type": "json_file|http_json|prometheus_remote_write|splunk_hec",
    "endpoint": "http://...",
    "hec_token": "...",
    "file": {"path": "/tmp/metricsd-output.json", "max_size_mb": 10, "max_files": 3, "format": "single|multi"}
  },
  "endpoints": [
    {"name": "cvs-engine", "url": "http://localhost:9090/metrics"}
  ]
}
```

### Environment Variable Overrides

| Variable | Config Path |
|----------|------------|
| `MC_SERVER_HOST` / `MC_SERVER_PORT` | server.host / server.port |
| `MC_COLLECTOR_INTERVAL` | collector.interval_seconds |
| `MC_SHIPPER_TYPE` / `MC_SHIPPER_ENDPOINT` | shipper.type / shipper.endpoint |
| `MC_HEC_TOKEN` | shipper.hec_token |
| `MC_PLUGINS_ENABLED` / `MC_PLUGINS_DIR` | collector.plugins.enabled / plugins_dir |
| `MC_PLUGINS_DEFAULT_TIMEOUT` / `MC_PLUGINS_VALIDATE` | default_timeout / validate_on_startup |

### Plugin Sidecar Config (`<plugin>.json`)

```json
{
  "name": "custom_name",
  "timeout": 30,
  "enabled": true,
  "interval_seconds": 3600,
  "env": ["KEY=value"],
  "args": ["--flag"],
  "working_dir": "/path"
}
```

Timeout is in **seconds** (not nanoseconds). `enabled: false` disables without removing.

## Security Model

- **Path validation:** Symlinks resolved, must stay within plugins dir
- **Safe env:** Plugins get `PATH=/usr/local/bin:/usr/bin:/bin`, `HOME=/nonexistent`, `LANG=C.UTF-8` only. No parent env inherited.
- **Output limit:** 5MB max stdout, truncated beyond that
- **Label validation:** `__` prefix rejected, values capped at 1024 chars
- **Orphan prevention:** `Pdeathsig: SIGTERM` on Linux
- **systemd hardening:** `ProtectSystem=strict`, `NoNewPrivileges=true`, `PrivateTmp=true` (in packaged service)

## CVS-Engine Metrics Reference

Scraped from `http://localhost:9090/metrics` (Prometheus text format), labeled `endpoint: "cvs-engine"`.

| Metric | Type | Description |
|--------|------|-------------|
| `cvs_container_cpu_percent` | gauge | Container CPU usage |
| `cvs_container_memory_used_bytes` | gauge | Container memory |
| `cvs_container_memory_limit_bytes` | gauge | Container memory limit |
| `cvs_container_memory_percent` | gauge | Memory % of limit |
| `cvs_process_cpu_percent` | gauge | CVS process CPU (multi-core, can exceed 100%) |
| `cvs_process_rss_bytes` | gauge | Resident set size |
| `cvs_process_vms_bytes` | gauge | Virtual memory |
| `cvs_process_threads` | gauge | Thread count |
| `cvs_cameras_active` | gauge | Active camera count |
| `cvs_deployments_active` | gauge | Active deployment count |
| `cvs_uptime_seconds` | gauge | Engine uptime |
| `cvs_evidence_total{state=...}` | gauge | Evidence by state (ready/processing/pending/failed) |
| `cvs_camera_connected{camera=...}` | gauge | Camera stream connected (0/1) |
| `cvs_camera_fps{camera=...}` | gauge | Measured FPS |
| `cvs_camera_frame_buffer_size` | gauge | Frames in buffer |
| `cvs_camera_frame_buffer_bytes` | gauge | Buffer memory |
| `cvs_camera_detection_count` | gauge | Current detection count |
| `cvs_camera_detection_age_seconds` | gauge | Seconds since last detection |
| `cvs_deployment_entry_exit_in` | counter | People entered |
| `cvs_deployment_entry_exit_out` | counter | People exited |
| `cvs_deployment_entry_exit_active_trajectories` | gauge | Active tracked trajectories |
| `cvs_deployment_last_alarm_seconds` | gauge | Seconds since last alarm |

## Build & Deploy

```bash
# Build
cd ~/devBed/rj/metricsd
make build-linux-amd64

# Deploy binary
sshpass -p 'vzvision' scp bin/metricsd-linux-amd64 vzvision@192.168.1.159:~/metricsd/metricsd

# Deploy plugins
sshpass -p 'vzvision' scp plugins/<name> plugins/<name>.json vzvision@192.168.1.159:~/metricsd/plugins/

# Set permissions + restart
sshpass -p 'vzvision' ssh vzvision@192.168.1.159 'chmod +x ~/metricsd/metricsd ~/metricsd/plugins/*; pkill -f "metricsd -config"; sleep 1; cd ~/metricsd && setsid ./metricsd -config config.json -log-level debug > /tmp/metricsd.log 2>&1 &'
```

## Internal Metrics

metricsd emits self-monitoring metrics each cycle:

| Metric | Description |
|--------|-------------|
| `metricsd_collection_duration_seconds` | Collection cycle duration |
| `metricsd_ship_duration_seconds` | Ship duration (from previous cycle) |

## Adding a New HTTP Endpoint

Add to `endpoints` array in config.json and restart:

```json
{"name": "my-app", "url": "http://localhost:PORT/metrics"}
```

Auto-detects Prometheus text or JSON format.

## Adding a New Shell Plugin

1. Create executable script in plugins dir, output JSON: `[{"name":"metric","value":1,"type":"gauge"}]`
2. Optional: create `<name>.json` sidecar for config
3. Restart metricsd — plugin is auto-discovered
4. See `docs/plugin-authoring.md` for full guide
