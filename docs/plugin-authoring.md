# Plugin Authoring Guide

## Overview

metricsd discovers executable shell scripts (and binaries) from the configured `plugins_dir`
(default: `./plugins`). Each executable is run on a configurable interval; its stdout is parsed
as a JSON array of metric objects and exposed via the Prometheus endpoint.

---

## JSON Output Schema

Plugins must write a JSON array to **stdout**:

```json
[
  {"name": "metric_name", "value": 42.5, "type": "gauge", "labels": {"key": "val"}}
]
```

| Field    | Required | Type              | Description |
|----------|----------|-------------------|-------------|
| `name`   | yes      | string            | Prometheus-compatible metric name matching `^[a-zA-Z_:][a-zA-Z0-9_:]*$` |
| `value`  | yes      | float64           | Numeric metric value |
| `type`   | no       | `"gauge"` \| `"counter"` | Metric type; defaults to `"gauge"` |
| `labels` | no       | map[string]string | Key-value label pairs attached to the metric |

---

## Exit Codes

| Code     | Meaning |
|----------|---------|
| `0`      | Success — stdout is parsed as metrics |
| non-zero | Failure — stderr is captured for diagnostics; no metrics are recorded |

---

## Timeout

Each plugin has a configurable execution timeout (see Sidecar Config below). When the timeout
expires, metricsd sends **SIGTERM** to the process. If the process has not exited after 2 seconds,
**SIGKILL** is sent.

---

## Environment

Plugins run with a **minimal, controlled environment**. The parent process environment is
**not** inherited. The base environment is:

```
PATH=/usr/local/bin:/usr/bin:/bin
HOME=/nonexistent
LANG=C.UTF-8
```

Additional variables can be injected via the `env` field in the sidecar config.

---

## Sidecar Config

Place a JSON file named `<plugin-name>.json` next to the plugin executable to configure it.

```json
{
  "name": "my_plugin",
  "enabled": true,
  "timeout": 30,
  "interval_seconds": 60,
  "args": ["--verbose"],
  "env": ["MY_VAR=hello"],
  "working_dir": "/tmp"
}
```

| Field              | Type    | Description |
|--------------------|---------|-------------|
| `name`             | string  | Plugin identifier (used in metric labels) |
| `timeout`          | integer | Execution timeout in **seconds** |
| `args`             | array   | Extra command-line arguments passed to the plugin |
| `env`              | array   | Additional environment variables (`"KEY=VALUE"` strings) |
| `working_dir`      | string  | Working directory for the plugin process |
| `enabled`          | boolean | Set to `false` to disable without removing the file |
| `interval_seconds` | integer | How often to run the plugin (overrides global default) |

---

## Label Restrictions

- Label names must not start with `__` (reserved by Prometheus).
- Label values are truncated / rejected if they exceed **1024 characters**.

---

## Testing

Run the plugin directly and verify the JSON output is valid:

```bash
# Pretty-print with Python (no extra dependencies)
./plugins/my_plugin | python3 -m json.tool

# Pretty-print with jq
./plugins/my_plugin | jq .
```

Check the exit code:

```bash
./plugins/my_plugin; echo "exit: $?"
```

---

## Example Plugin

```bash
#!/bin/bash
# my_plugin — reports a single gauge metric

set -euo pipefail

value=$(cat /proc/loadavg | awk '{print $1}')

printf '[{"name":"system_load_1m","value":%s,"type":"gauge","labels":{"host":"%s"}}]\n' \
  "$value" "$(hostname)"
```

Install and enable:

```bash
cp my_plugin ./plugins/
chmod +x ./plugins/my_plugin

cat > ./plugins/my_plugin.json <<'EOF'
{
  "name": "my_plugin",
  "enabled": true,
  "timeout": 10,
  "interval_seconds": 60
}
EOF
```

---

## See Also

- `config.example.json` — top-level plugin configuration (`plugins_dir`, `default_timeout_seconds`)
- `plugins/process_monitor.md` — example of a more complex plugin with its own config schema
- `plugins/SUDO_SETUP.md` — configuring sudo for plugins that need elevated privileges
