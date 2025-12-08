# Custom Metric Plugins

Define plugins as JSON files inside this directory (one object or an array per file). The service loads `*.json` on startup; missing directories are ignored.

## Global settings
- Directory: `plugins/` (override via `plugins.directory` in config or `MC_PLUGINS_DIR`)
- Metric prefix: default `metricsd_plugin_` (override via `plugins.prefix` or `MC_PLUGIN_PREFIX`)

## Plugin schema
```json
{
  "name": "bandwidth_rx",
  "metric": "bandwidth_rx_bytes_total",
  "metric_type": "counter",           // "gauge" | "counter"
  "interval_seconds": 15,             // per-plugin interval
  "timeout_seconds": 10,              // optional, default 10s
  "labels": { "interface": "eth0" },  // optional
  "parser": { "mode": "number" },     // or { "mode": "regex", "regex": "(\\d+)" }
  "file": { "path": "/sys/class/net/eth0/statistics/rx_bytes" }
}
```

Exactly one source is allowed per plugin:
- `command`: `{"command": ["sh", "-c", "cat /sys/.../rx_bytes"]}` (blacklist enforced: rm, sudo, mv, cp, shutdown, reboot, halt, init)
- `http`: `{"url": "http://localhost:9000/metric"}` (body parsed with `parser`)
- `file`: `{"path": "/sys/.../rx_bytes"}`

Parser modes:
- `number` (default): trims and parses a single numeric value
- `regex`: first capture group must be numeric

Metric name will be `<prefix><metric>` and a `plugin=<name>` label is added automatically.

## Examples
- File-based RX bytes: `example_bandwidth_rx.json`
- Command-based TX bytes (uses `cat /proc/net/dev | grep eth0 | awk '{print $10}'`): `example_bandwidth_cmd.json`
- Container stats (docker/nerdctl CPU/MEM % via `stats --no-stream --format`): `example_container_stats.json`
- System health + NetPulse + USG audit samples: `example_system_health.json`

