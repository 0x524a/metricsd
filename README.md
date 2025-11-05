# Metrics Collector Service

A production-ready, configurable metrics collector service written in Go that collects system and application metrics and ships them to remote endpoints.

## Features

- **System Metrics Collection**
  - CPU usage (per-core and total)
  - Memory usage (RAM and swap)
  - Disk I/O and usage statistics
  - Network I/O statistics
  - GPU metrics via NVIDIA NVML (optional)

- **Application Metrics Collection**
  - HTTP endpoint scraping for application metrics
  - Support for multiple application endpoints
  - JSON-based metrics format

- **Flexible Shipping Options**
  - Prometheus Remote Write protocol
  - HTTP JSON POST
  - TLS/SSL support for secure transmission

- **Configurable & Extensible**
  - JSON configuration with environment variable overrides
  - Adjustable collection intervals
  - Enable/disable specific metric collectors
  - Health endpoint for monitoring

- **Production-Ready**
  - Structured logging with zerolog
  - Graceful shutdown
  - Error handling and resilience
  - SOLID design principles

## Architecture

The service follows SOLID principles with a clean architecture:

```
metrics-collector/
├── cmd/
│   └── metrics-collector/     # Application entry point
│       └── main.go
├── internal/
│   ├── collector/             # Metric collectors (System, GPU, HTTP)
│   │   ├── collector.go       # Collector interface and registry
│   │   ├── system.go          # OS metrics collector
│   │   ├── gpu.go             # GPU metrics collector
│   │   └── http.go            # HTTP endpoint scraper
│   ├── config/                # Configuration management
│   │   └── config.go
│   ├── shipper/               # Metrics shipping
│   │   ├── shipper.go         # Shipper interface
│   │   ├── prometheus.go      # Prometheus remote write
│   │   └── http_json.go       # HTTP JSON shipper
│   ├── orchestrator/          # Collection orchestration
│   │   └── orchestrator.go
│   └── server/                # HTTP server for health checks
│       └── server.go
├── config.example.json        # Example configuration
├── go.mod
├── go.sum
└── README.md
```

## Installation

### Prerequisites

- Go 1.21 or later
- NVIDIA drivers and CUDA (optional, for GPU metrics)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/0x524a/metrics-collector.git
cd metrics-collector

# Download dependencies
go mod download

# Build the binary
go build -o bin/metrics-collector cmd/metrics-collector/main.go
```

## Configuration

Create a `config.json` file based on the example:

```bash
cp config.example.json config.json
```

### Configuration Options

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "collector": {
    "interval_seconds": 60,
    "enable_cpu": true,
    "enable_memory": true,
    "enable_disk": true,
    "enable_network": true,
    "enable_gpu": false
  },
  "shipper": {
    "type": "http_json",
    "endpoint": "http://localhost:9090/api/v1/metrics",
    "timeout": 30000000000,
    "tls": {
      "enabled": false,
      "cert_file": "/path/to/cert.pem",
      "key_file": "/path/to/key.pem",
      "ca_file": "/path/to/ca.pem",
      "insecure_skip_verify": false
    }
  },
  "endpoints": [
    {
      "name": "app1",
      "url": "http://localhost:3000/metrics"
    }
  ]
}
```

### Configuration Fields

| Field | Description | Default |
|-------|-------------|---------|
| `server.host` | HTTP server bind address | `0.0.0.0` |
| `server.port` | HTTP server port | `8080` |
| `collector.interval_seconds` | Collection interval in seconds | `60` |
| `collector.enable_cpu` | Enable CPU metrics collection | `true` |
| `collector.enable_memory` | Enable memory metrics collection | `true` |
| `collector.enable_disk` | Enable disk metrics collection | `true` |
| `collector.enable_network` | Enable network metrics collection | `true` |
| `collector.enable_gpu` | Enable GPU metrics collection (requires NVIDIA GPU) | `false` |
| `shipper.type` | Shipper type: `prometheus_remote_write` or `http_json` | - |
| `shipper.endpoint` | Remote endpoint URL | - |
| `shipper.timeout` | Request timeout in nanoseconds | `30000000000` (30s) |
| `shipper.tls.enabled` | Enable TLS | `false` |
| `endpoints` | Array of application HTTP endpoints to scrape | `[]` |

### Environment Variable Overrides

You can override configuration values using environment variables:

- `MC_SERVER_HOST` - Server host
- `MC_SERVER_PORT` - Server port
- `MC_COLLECTOR_INTERVAL` - Collection interval in seconds
- `MC_SHIPPER_TYPE` - Shipper type
- `MC_SHIPPER_ENDPOINT` - Shipper endpoint URL
- `MC_TLS_ENABLED` - Enable TLS (true/false)
- `MC_TLS_CERT_FILE` - TLS certificate file path
- `MC_TLS_KEY_FILE` - TLS key file path
- `MC_TLS_CA_FILE` - TLS CA certificate file path

## Usage

### Basic Usage

```bash
# Run with default config.json
./bin/metrics-collector

# Run with custom config file
./bin/metrics-collector -config /path/to/config.json

# Set log level
./bin/metrics-collector -log-level debug
```

### Log Levels

- `debug` - Detailed debugging information
- `info` - General informational messages (default)
- `warn` - Warning messages
- `error` - Error messages only

### Health Check

The service exposes a health endpoint:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2025-11-05T12:34:56Z",
  "uptime": "1h23m45s"
}
```

## Shipper Types

### Prometheus Remote Write

Ships metrics using the Prometheus remote write protocol with Snappy compression.

```json
{
  "shipper": {
    "type": "prometheus_remote_write",
    "endpoint": "http://prometheus:9090/api/v1/write"
  }
}
```

### HTTP JSON

Ships metrics as JSON via HTTP POST.

```json
{
  "shipper": {
    "type": "http_json",
    "endpoint": "http://collector:8080/api/v1/metrics"
  }
}
```

Payload format:
```json
{
  "timestamp": 1699185296,
  "metrics": [
    {
      "name": "system_cpu_usage_percent",
      "value": 45.2,
      "type": "gauge",
      "labels": {
        "core": "0"
      }
    }
  ]
}
```

## Collected Metrics

### System Metrics

**CPU:**
- `system_cpu_usage_percent` - Per-core CPU usage
- `system_cpu_usage_total_percent` - Overall CPU usage
- `system_cpu_count` - Number of CPU cores

**Memory:**
- `system_memory_total_bytes` - Total memory
- `system_memory_used_bytes` - Used memory
- `system_memory_available_bytes` - Available memory
- `system_memory_usage_percent` - Memory usage percentage
- `system_swap_total_bytes` - Total swap space
- `system_swap_used_bytes` - Used swap space
- `system_swap_usage_percent` - Swap usage percentage

**Disk:**
- `system_disk_total_bytes` - Total disk space
- `system_disk_used_bytes` - Used disk space
- `system_disk_free_bytes` - Free disk space
- `system_disk_usage_percent` - Disk usage percentage
- `system_disk_read_bytes_total` - Total bytes read
- `system_disk_write_bytes_total` - Total bytes written
- `system_disk_read_count_total` - Total read operations
- `system_disk_write_count_total` - Total write operations

**Network:**
- `system_network_bytes_sent_total` - Total bytes sent
- `system_network_bytes_recv_total` - Total bytes received
- `system_network_packets_sent_total` - Total packets sent
- `system_network_packets_recv_total` - Total packets received
- `system_network_errors_in_total` - Total input errors
- `system_network_errors_out_total` - Total output errors
- `system_network_drop_in_total` - Total input drops
- `system_network_drop_out_total` - Total output drops

**GPU (NVIDIA):**
- `system_gpu_count` - Number of GPUs
- `system_gpu_utilization_percent` - GPU utilization
- `system_gpu_memory_utilization_percent` - GPU memory utilization
- `system_gpu_memory_total_bytes` - Total GPU memory
- `system_gpu_memory_used_bytes` - Used GPU memory
- `system_gpu_memory_free_bytes` - Free GPU memory
- `system_gpu_temperature_celsius` - GPU temperature
- `system_gpu_power_usage_milliwatts` - GPU power usage
- `system_gpu_fan_speed_percent` - Fan speed
- `system_gpu_clock_sm_mhz` - SM clock speed
- `system_gpu_clock_memory_mhz` - Memory clock speed

### Application Metrics

Application metrics are prefixed with `app_` and include the endpoint name as a label.

## Deployment

### Systemd Service

Create `/etc/systemd/system/metrics-collector.service`:

```ini
[Unit]
Description=Metrics Collector Service
After=network.target

[Service]
Type=simple
User=metrics-collector
WorkingDirectory=/opt/metrics-collector
ExecStart=/opt/metrics-collector/bin/metrics-collector -config /opt/metrics-collector/config.json
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable metrics-collector
sudo systemctl start metrics-collector
```

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o metrics-collector cmd/metrics-collector/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/metrics-collector .
COPY config.json .
CMD ["./metrics-collector"]
```

Build and run:
```bash
docker build -t metrics-collector .
docker run -d -p 8080:8080 -v $(pwd)/config.json:/root/config.json metrics-collector
```

## Development

### Running Tests

```bash
go test ./...
```

### Code Structure

The project follows SOLID principles:

- **Single Responsibility**: Each collector, shipper, and component has one clear purpose
- **Open/Closed**: New collectors and shippers can be added without modifying existing code
- **Liskov Substitution**: All collectors implement the same interface
- **Interface Segregation**: Small, focused interfaces (Collector, Shipper)
- **Dependency Inversion**: Components depend on abstractions, not concrete implementations

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues and questions, please open an issue on GitHub.
