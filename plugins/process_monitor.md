# Process Monitor Plugin

## Overview

The `process_monitor` plugin monitors specific processes/daemons with detailed resource usage statistics. It allows you to specify an array of process names to monitor and collects comprehensive metrics for each matching process.

## Requirements

- Linux with `/proc` filesystem
- `jq` (optional, for robust JSON parsing)
- `pgrep` or `ps` command

## Configuration

The plugin reads its configuration from `process_monitor.json` in the same directory.

### Configuration Format

```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "nginx",
    "postgres",
    "redis-server",
    "mysql",
    "docker"
  ]
}
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Plugin name (must be "process_monitor") |
| `timeout` | integer | Plugin execution timeout in nanoseconds |
| `enabled` | boolean | Enable/disable the plugin |
| `processes` | array | List of process names to monitor |

## Process Matching

The plugin matches processes by name using:
1. Exact match on process name (`pgrep -x`)
2. Fallback to pattern match (`pgrep -f`)
3. Multiple PIDs can match a single process name

## Metrics Collected

All metrics are prefixed with `plugin_process_monitor_` by metricsd.

### Per-Process Metrics

| Metric Name | Type | Unit | Description |
|-------------|------|------|-------------|
| `process_cpu_percent` | gauge | percentage | CPU usage percentage (from `ps`) |
| `process_memory_rss_bytes` | gauge | bytes | Resident Set Size (physical memory) |
| `process_memory_vms_bytes` | gauge | bytes | Virtual Memory Size |
| `process_memory_percent` | gauge | percentage | Memory usage as % of system total |
| `process_threads` | gauge | count | Number of threads |
| `process_fds` | gauge | count | Open file descriptors |
| `process_io_read_bytes` | counter | bytes | Cumulative bytes read from disk |
| `process_io_write_bytes` | counter | bytes | Cumulative bytes written to disk |
| `process_uptime_seconds` | gauge | seconds | Process uptime since start |
| `process_status` | gauge | enum | Process state (0-7, see below) |

### Process Status Values

| Value | State | Description |
|-------|-------|-------------|
| 0 | NotFound | Process not running |
| 1 | Running | Currently executing |
| 2 | Sleeping | Interruptible sleep (waiting for event) |
| 3 | Waiting | Uninterruptible sleep (usually I/O) |
| 4 | Zombie | Terminated but not reaped |
| 5 | Stopped | Stopped by signal or tracing |
| 6 | Dead | Dead process |
| 7 | Unknown | Unknown state |

## Labels

Each metric includes the following labels:

| Label | Description | Example |
|-------|-------------|---------|
| `process_name` | Name from configuration | `nginx` |
| `pid` | Process ID | `1234` |
| `cmdline` | Command line (truncated to 100 chars) | `/usr/sbin/nginx -g daemon off;` |
| `user` | Process owner | `www-data` |
| `state` | Human-readable state | `Sleeping`, `Running` |
| `plugin` | Plugin identifier (auto-added) | `process_monitor` |

## Example Output

```json
[
  {
    "name": "process_cpu_percent",
    "value": 2.3,
    "type": "gauge",
    "labels": {
      "process_name": "nginx",
      "pid": "1234",
      "cmdline": "/usr/sbin/nginx -g daemon off;",
      "user": "www-data",
      "state": "Sleeping"
    }
  },
  {
    "name": "process_memory_rss_bytes",
    "value": 12582912,
    "type": "gauge",
    "labels": {
      "process_name": "nginx",
      "pid": "1234",
      "cmdline": "/usr/sbin/nginx -g daemon off;",
      "user": "www-data",
      "state": "Sleeping"
    }
  },
  {
    "name": "process_threads",
    "value": 4,
    "type": "gauge",
    "labels": {
      "process_name": "nginx",
      "pid": "1234",
      "cmdline": "/usr/sbin/nginx -g daemon off;",
      "user": "www-data",
      "state": "Sleeping"
    }
  }
]
```

## Usage Examples

### Monitor Web Servers

```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "nginx",
    "apache2",
    "httpd"
  ]
}
```

### Monitor Databases

```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "postgres",
    "mysqld",
    "redis-server",
    "mongod"
  ]
}
```

### Monitor System Services

```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "systemd",
    "sshd",
    "cron",
    "rsyslogd"
  ]
}
```

### Monitor Container Runtimes

```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "dockerd",
    "containerd",
    "kubelet"
  ]
}
```

## Installation

### Package Installation

When installing via DEB package, the plugin is included as an example:

```bash
# List available plugins
ls /usr/lib/metricsd/plugins/*.json.example

# Enable the plugin
cd /usr/lib/metricsd/plugins
sudo cp process_monitor.json.example process_monitor.json

# Edit to add your processes
sudo nano process_monitor.json
```

Example configuration:
```json
{
  "name": "process_monitor",
  "timeout": 30000000000,
  "enabled": true,
  "processes": [
    "nginx",
    "postgres"
  ]
}
```

### Manual Installation

```bash
# Copy plugin script
sudo cp process_monitor /usr/lib/metricsd/plugins/
sudo chmod +x /usr/lib/metricsd/plugins/process_monitor

# Create configuration
sudo nano /usr/lib/metricsd/plugins/process_monitor.json
```

## Troubleshooting

### No Metrics Returned

1. **Check if process is running:**
   ```bash
   pgrep -x nginx  # Should return PIDs
   ```

2. **Check config file location:**
   ```bash
   ls -la /usr/lib/metricsd/plugins/process_monitor.json
   ```

3. **Test plugin manually:**
   ```bash
   /usr/lib/metricsd/plugins/process_monitor | jq '.'
   ```

4. **Check permissions:**
   ```bash
   # Plugin needs read access to /proc/<pid>/
   # Run as metricsd user or root
   sudo -u metricsd /usr/lib/metricsd/plugins/process_monitor
   ```

### Process Name Not Matching

The plugin uses `pgrep` which matches:
- Exact process name: `nginx`
- Not full path: Not `/usr/sbin/nginx`

If exact match fails, it falls back to pattern match (`pgrep -f`) which matches the full command line.

### I/O Stats Not Available

Some processes may not have I/O stats available in `/proc/<pid>/io` due to kernel configuration or permissions. This is normal and the plugin will skip those metrics.

## Performance

- **Overhead:** Low - reads from `/proc` filesystem
- **Execution time:** ~10-50ms for 5 processes
- **Recommended interval:** 60 seconds or higher
- **CPU impact:** Negligible (<0.1% CPU per execution)

## Limitations

1. **Linux only:** Requires `/proc` filesystem
2. **Per-process network I/O:** Not currently collected (requires netlink or complex parsing)
3. **Process name matching:** May match multiple processes with similar names
4. **Historical data:** CPU percentage is instantaneous snapshot, not averaged over interval

## Tips

- Use specific process names to avoid matching unintended processes
- For multi-instance processes (e.g., multiple nginx workers), all instances will be monitored separately
- Use `interval_seconds` in the config to reduce collection frequency for stable processes
- Monitor parent processes (e.g., `dockerd`) rather than container processes

## See Also

- [process_top plugin](./process_top) - Top N processes by resource usage
- [Metricsd Plugin Documentation](../README.md#plugins)
