# Metrics Collector Service (metricsd)

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/0x524A/metricsd)](https://goreportcard.com/report/github.com/0x524A/metricsd)
[![GitHub Release](https://img.shields.io/github/v/release/0x524A/metricsd)](https://github.com/0x524A/metricsd/releases)
[![Docker Image Size](https://img.shields.io/docker/image-size/0x524a/metricsd/latest)](https://hub.docker.com/r/0x524a/metricsd)
[![GitHub Issues](https://img.shields.io/github/issues/0x524A/metricsd)](https://github.com/0x524A/metricsd/issues)
[![GitHub Stars](https://img.shields.io/github/stars/0x524A/metricsd?style=social)](https://github.com/0x524A/metricsd/stargazers)

A production-ready, high-performance metrics collector service written in Go that collects system and application metrics and ships them to remote endpoints with enterprise-grade security.

> **🚀 Features**: System metrics (CPU, Memory, Disk, Network) • GPU monitoring (NVIDIA) • Application endpoint scraping • TLS/mTLS support • Prometheus & HTTP JSON shipping • Docker & Kubernetes ready

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Shipper Types](#shipper-types)
- [TLS Configuration](#tls-configuration)
- [Collected Metrics](#collected-metrics)
- [Security Considerations](#security-considerations)
- [Deployment](#deployment)
- [Performance Tuning](#performance-tuning)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

Get metricsd up and running in 5 minutes:

```bash
# Clone and build
git clone https://github.com/0x524A/metricsd.git
cd metricsd
go build -o bin/metricsd cmd/metricsd/main.go

# Create configuration
cp config.example.json config.json

# Edit config.json to set your endpoint
# For example, change endpoint to your Prometheus or metrics collector URL

# Run the service
./bin/metricsd -config config.json

# Check health
curl http://localhost:8080/health
```

**With TLS:**
```bash
# Generate self-signed certificates (for testing)
mkdir -p certs && cd certs
openssl req -x509 -newkey rsa:4096 -keyout client.key -out client.crt -days 365 -nodes \
  -subj "/CN=metricsd-client"
cd ..

# Update config.json to enable TLS
# Set shipper.tls.enabled to true
# Set certificate paths in shipper.tls section

# Run with TLS
./bin/metricsd -config config.json
```

**With Docker:**
```bash
docker build -t metricsd:latest .
docker run -d -p 8080:8080 -v $(pwd)/config.json:/etc/metricsd/config.json:ro metricsd:latest
```

## Features

- **Comprehensive Metrics Collection**
  - CPU usage (per-core and total utilization)
  - Memory usage (RAM and swap statistics)
  - Disk I/O and usage statistics
  - Network I/O statistics
  - GPU metrics via NVIDIA NVML (optional)
  - Custom application endpoint scraping

- **Application Metrics Collection**
  - HTTP endpoint scraping for application metrics
  - Support for multiple application endpoints
  - JSON-based metrics format
  - Configurable timeout and retry logic

- **Flexible Shipping Options**
  - Prometheus Remote Write protocol with Snappy compression
  - HTTP JSON POST
  - Advanced TLS/SSL support for secure transmission
  - Configurable request timeouts

- **Enterprise-Grade Security**
  - Full TLS 1.2/1.3 support with custom configuration
  - Client certificate authentication (mTLS)
  - Custom CA certificate support
  - Configurable cipher suites
  - SNI (Server Name Indication) support
  - TLS version pinning (min/max)
  - Session ticket management
  - Optional certificate verification bypass for testing

- **Configurable & Extensible**
  - JSON configuration with environment variable overrides
  - Adjustable collection intervals
  - Enable/disable specific metric collectors
  - Health endpoint for monitoring
  - Flexible shipper interface for custom backends

- **Production-Ready**
  - Structured logging with zerolog
  - Graceful shutdown with cleanup
  - Error handling and resilience
  - SOLID design principles
  - Resource cleanup and leak prevention

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

- Go 1.24 or later
- NVIDIA drivers and CUDA (optional, for GPU metrics)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/jainri3/metrics-collector.git
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
    "endpoint": "https://collector.example.com:9090/api/v1/metrics",
    "timeout": 30000000000,
    "tls": {
      "enabled": true,
      "cert_file": "/path/to/client-cert.pem",
      "key_file": "/path/to/client-key.pem",
      "ca_file": "/path/to/ca.pem",
      "insecure_skip_verify": false,
      "server_name": "collector.example.com",
      "min_version": "TLS1.2",
      "max_version": "TLS1.3",
      "cipher_suites": [
        "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
      ],
      "session_tickets": true
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
| `shipper.tls.enabled` | Enable TLS/SSL | `false` |
| `shipper.tls.cert_file` | Path to client certificate file (PEM) | - |
| `shipper.tls.key_file` | Path to client private key file (PEM) | - |
| `shipper.tls.ca_file` | Path to CA certificate file for server verification | - |
| `shipper.tls.insecure_skip_verify` | Skip server certificate verification (not recommended) | `false` |
| `shipper.tls.server_name` | Server name for SNI (overrides hostname from endpoint) | - |
| `shipper.tls.min_version` | Minimum TLS version: `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` | `TLS1.2` |
| `shipper.tls.max_version` | Maximum TLS version: `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` | `TLS1.3` |
| `shipper.tls.cipher_suites` | Array of allowed cipher suites (see Cipher Suites section) | System defaults |
| `shipper.tls.session_tickets` | Enable TLS session ticket resumption | `true` |
| `endpoints` | Array of application HTTP endpoints to scrape | `[]` |

### Environment Variable Overrides

You can override configuration values using environment variables:

| Environment Variable | Description | Example |
|---------------------|-------------|---------|
| `MC_SERVER_HOST` | Server bind address | `0.0.0.0` |
| `MC_SERVER_PORT` | Server port number | `8080` |
| `MC_COLLECTOR_INTERVAL` | Collection interval in seconds | `60` |
| `MC_SHIPPER_TYPE` | Shipper type | `prometheus_remote_write` |
| `MC_SHIPPER_ENDPOINT` | Shipper endpoint URL | `https://metrics.example.com/write` |
| `MC_TLS_ENABLED` | Enable TLS | `true` |
| `MC_TLS_CERT_FILE` | Client certificate file path | `/etc/metricsd/certs/client.crt` |
| `MC_TLS_KEY_FILE` | Client private key file path | `/etc/metricsd/certs/client.key` |
| `MC_TLS_CA_FILE` | CA certificate file path | `/etc/metricsd/certs/ca.crt` |
| `MC_TLS_SERVER_NAME` | SNI server name | `collector.example.com` |
| `MC_TLS_MIN_VERSION` | Minimum TLS version | `TLS1.2` |
| `MC_TLS_INSECURE_SKIP_VERIFY` | Skip certificate verification | `false` |

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

## TLS Configuration

The service supports advanced TLS configuration for secure communication with remote endpoints. This includes mutual TLS (mTLS), custom cipher suites, and version pinning.

### Basic TLS Setup

For simple TLS with server certificate verification:

```json
{
  "shipper": {
    "type": "prometheus_remote_write",
    "endpoint": "https://metrics.example.com/api/v1/write",
    "tls": {
      "enabled": true,
      "ca_file": "/etc/metricsd/certs/ca.pem"
    }
  }
}
```

### Mutual TLS (mTLS)

For client certificate authentication:

```json
{
  "shipper": {
    "type": "http_json",
    "endpoint": "https://secure-collector.example.com/metrics",
    "tls": {
      "enabled": true,
      "cert_file": "/etc/metricsd/certs/client.crt",
      "key_file": "/etc/metricsd/certs/client.key",
      "ca_file": "/etc/metricsd/certs/ca.crt",
      "server_name": "secure-collector.example.com"
    }
  }
}
```

### Advanced TLS Configuration

Full control over TLS parameters:

```json
{
  "shipper": {
    "tls": {
      "enabled": true,
      "cert_file": "/etc/metricsd/certs/client.crt",
      "key_file": "/etc/metricsd/certs/client.key",
      "ca_file": "/etc/metricsd/certs/ca.crt",
      "server_name": "metrics.internal.example.com",
      "min_version": "TLS1.2",
      "max_version": "TLS1.3",
      "cipher_suites": [
        "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
        "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
      ],
      "session_tickets": true,
      "insecure_skip_verify": false
    }
  }
}
```

### TLS Configuration Options

| Option | Description | Values |
|--------|-------------|--------|
| `enabled` | Enable/disable TLS | `true`, `false` |
| `cert_file` | Client certificate for mTLS | Path to PEM file |
| `key_file` | Client private key for mTLS | Path to PEM file |
| `ca_file` | CA certificate for server verification | Path to PEM file |
| `server_name` | SNI hostname override | Domain name |
| `min_version` | Minimum TLS version | `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` |
| `max_version` | Maximum TLS version | `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` |
| `cipher_suites` | Allowed cipher suites | Array of suite names |
| `session_tickets` | Enable session resumption | `true`, `false` |
| `insecure_skip_verify` | Skip certificate verification | `true`, `false` (not recommended for production) |

### Supported Cipher Suites

**TLS 1.3 Cipher Suites:**
- `TLS_AES_128_GCM_SHA256`
- `TLS_AES_256_GCM_SHA384`
- `TLS_CHACHA20_POLY1305_SHA256`

**TLS 1.2 Cipher Suites (Recommended):**
- `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`
- `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
- `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
- `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
- `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`
- `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`

**Additional TLS 1.2 Cipher Suites:**
- `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256`
- `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256`
- `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`
- `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`
- `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`
- `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`
- `TLS_RSA_WITH_AES_128_GCM_SHA256`
- `TLS_RSA_WITH_AES_256_GCM_SHA384`
- `TLS_RSA_WITH_AES_128_CBC_SHA256`
- `TLS_RSA_WITH_AES_128_CBC_SHA`
- `TLS_RSA_WITH_AES_256_CBC_SHA`

> **Note:** If cipher suites are not specified, Go's default secure cipher suite list will be used. TLS 1.3 cipher suites cannot be configured in Go and use the protocol's default settings.

### TLS Best Practices

1. **Use TLS 1.2 or higher** - Set `min_version` to `TLS1.2` minimum
2. **Enable mTLS** - Use client certificates for mutual authentication
3. **Verify certificates** - Keep `insecure_skip_verify` as `false` in production
4. **Use strong cipher suites** - Prefer ECDHE and AEAD ciphers
5. **Configure SNI** - Set `server_name` when using name-based virtual hosting
6. **Rotate certificates** - Implement a certificate rotation strategy
7. **Secure key storage** - Protect private keys with appropriate file permissions

### Certificate Generation Examples

**Generate self-signed CA:**
```bash
openssl req -x509 -new -nodes -keyout ca.key -sha256 -days 1825 -out ca.crt \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=CA"
```

**Generate client certificate:**
```bash
# Generate private key
openssl genrsa -out client.key 2048

# Generate certificate signing request
openssl req -new -key client.key -out client.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=metricsd-client"

# Sign with CA
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 825 -sha256
```

**Set secure file permissions:**
```bash
chmod 600 /etc/metricsd/certs/*.key
chmod 644 /etc/metricsd/certs/*.crt
chown metricsd:metricsd /etc/metricsd/certs/*
```

### Troubleshooting TLS

**Certificate verification failed:**
- Ensure CA certificate includes the full chain
- Verify `server_name` matches the certificate CN or SAN
- Check certificate expiration dates

**Handshake failure:**
- Verify cipher suites are compatible with server
- Check TLS version compatibility (min/max versions)
- Ensure client certificate is valid and trusted by server

**Enable debug logging:**
```bash
./bin/metricsd -log-level debug
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

## Security Considerations

### File Permissions

Protect sensitive configuration and certificate files:

```bash
# Configuration file
chmod 600 /opt/metricsd/config.json
chown metricsd:metricsd /opt/metricsd/config.json

# Certificate directory
chmod 700 /etc/metricsd/certs
chown -R metricsd:metricsd /etc/metricsd/certs

# Private keys
chmod 600 /etc/metricsd/certs/*.key

# Certificates
chmod 644 /etc/metricsd/certs/*.crt
```

### Running as Non-Root User

Always run the service as a dedicated non-privileged user:

```bash
# Create dedicated user
sudo useradd -r -s /bin/false -d /opt/metricsd metricsd

# Set ownership
sudo chown -R metricsd:metricsd /opt/metricsd
```

### Network Security

- Use TLS for all remote communications
- Enable mTLS when possible for mutual authentication
- Restrict network access using firewalls
- Use internal/private networks when available
- Regularly update certificates before expiration

### Configuration Security

- Store sensitive values in environment variables
- Use secrets management tools (HashiCorp Vault, AWS Secrets Manager, etc.)
- Rotate credentials regularly
- Audit configuration changes
- Enable detailed logging for security monitoring

## Deployment

### Systemd Service

Create `/etc/systemd/system/metricsd.service`:

```ini
[Unit]
Description=Metrics Collector Service (metricsd)
Documentation=https://github.com/0x524A/metricsd
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=metricsd
Group=metricsd
WorkingDirectory=/opt/metricsd
ExecStart=/opt/metricsd/bin/metricsd -config /opt/metricsd/config.json -log-level info
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=10
KillMode=process
TimeoutStopSec=30

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/metricsd
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Resource limits
LimitNOFILE=65536
LimitNPROC=512

[Install]
WantedBy=multi-user.target
```

Install and enable:
```bash
# Copy binary and config
sudo mkdir -p /opt/metricsd/{bin,certs}
sudo cp bin/metricsd /opt/metricsd/bin/
sudo cp config.json /opt/metricsd/

# Create user
sudo useradd -r -s /bin/false -d /opt/metricsd metricsd

# Set permissions
sudo chown -R metricsd:metricsd /opt/metricsd
sudo chmod 600 /opt/metricsd/config.json
sudo chmod 755 /opt/metricsd/bin/metricsd

# Install and start service
sudo cp metricsd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable metricsd
sudo systemctl start metricsd

# Check status
sudo systemctl status metricsd
sudo journalctl -u metricsd -f
```

### Docker

#### Building the Container Image

**Prerequisites:**
- Docker installed (version 20.10+ recommended)
- Docker Compose (optional, for easier deployment)
- At least 500MB free disk space for the image

**Step 1: Create the Dockerfile**

Create a file named `Dockerfile` in the project root:

```dockerfile
FROM golang:1.24-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build with all features including GPU support (NVML)
RUN go build -ldflags '-w -s' -o metricsd cmd/metricsd/main.go

FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 metricsd && \
    useradd -r -u 1000 -g metricsd -s /bin/false metricsd

# Create directories
RUN mkdir -p /etc/metricsd/certs /var/lib/metricsd
RUN chown -R metricsd:metricsd /etc/metricsd /var/lib/metricsd

WORKDIR /home/metricsd

# Copy binary
COPY --from=builder /app/metricsd /usr/local/bin/metricsd
RUN chmod +x /usr/local/bin/metricsd

# Switch to non-root user
USER metricsd

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/metricsd"]
CMD ["-config", "/etc/metricsd/config.json"]
```

**Step 2: Build the Image**

```bash
# Basic build
docker build -t metricsd:latest .

# Build with custom tag
docker build -t metricsd:v1.0.0 .

# Build with specific platform (for cross-platform)
docker build --platform linux/amd64 -t metricsd:latest .

# Build with build arguments (if needed)
docker build --build-arg GO_VERSION=1.21 -t metricsd:latest .

# Build with no cache (clean build)
docker build --no-cache -t metricsd:latest .

# Build and show build progress
docker build --progress=plain -t metricsd:latest .
```

**Step 3: Verify the Build**

```bash
# List the image
docker images | grep metricsd

# Check image size (should be around 20-30MB)
docker images metricsd:latest --format "{{.Size}}"

# Inspect the image
docker inspect metricsd:latest

# Test run (quick check)
docker run --rm metricsd:latest -help
```

**Step 4: Tag for Registry (Optional)**

```bash
# Tag for Docker Hub
docker tag metricsd:latest 0x524A/metricsd:latest
docker tag metricsd:latest 0x524A/metricsd:v1.0.0

# Tag for private registry
docker tag metricsd:latest registry.example.com/metricsd:latest

# Push to registry
docker push 0x524A/metricsd:latest
```

**Optimizing the Build**

Create a `.dockerignore` file to exclude unnecessary files:

```
# .dockerignore
.git
.gitignore
.github
README.md
LICENSE
*.md
.vscode
.idea
bin/
*.log
*.tmp
.env
.DS_Store
Makefile
docker-compose.yml
```

**Build Troubleshooting**

Common build issues:

```bash
# Issue: "cannot find package"
# Solution: Ensure go.mod and go.sum are present
go mod tidy
docker build -t metricsd:latest .

# Issue: "no space left on device"
# Solution: Clean up Docker
docker system prune -a --volumes

# Issue: Build is slow
# Solution: Use BuildKit (faster builds)
DOCKER_BUILDKIT=1 docker build -t metricsd:latest .

# Issue: Platform mismatch (M1 Mac, ARM)
# Solution: Build for specific platform
docker build --platform linux/amd64 -t metricsd:latest .

# Issue: Can't connect to Docker daemon
# Solution: Start Docker or check permissions
sudo systemctl start docker  # Linux
sudo usermod -aG docker $USER  # Add user to docker group
```

#### Docker Compose Files

**docker-compose.yml** (for container metrics):
```yaml
version: '3.8'

services:
  metricsd:
    build: .
    image: metricsd:latest
    container_name: metricsd
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config.json:/etc/metricsd/config.json:ro
      - ./certs:/etc/metricsd/certs:ro
    environment:
      - MC_LOG_LEVEL=info
      - MC_SHIPPER_ENDPOINT=https://prometheus:9090/api/v1/write
      - MC_TLS_ENABLED=true
      - MC_TLS_CERT_FILE=/etc/metricsd/certs/client.crt
      - MC_TLS_KEY_FILE=/etc/metricsd/certs/client.key
      - MC_TLS_CA_FILE=/etc/metricsd/certs/ca.crt
    networks:
      - metrics
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

networks:
  metrics:
    driver: bridge
```

**docker-compose.yml** (for HOST metrics - recommended for production):
```yaml
version: '3.8'

services:
  metricsd:
    build: .
    image: metricsd:latest
    container_name: metricsd
    restart: unless-stopped
    # Use host network to access host metrics
    network_mode: host
    # Use host PID namespace to see host processes
    pid: host
    volumes:
      # Mount host filesystems for accurate host metrics
      - /:/rootfs:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./config.json:/etc/metricsd/config.json:ro
      - ./certs:/etc/metricsd/certs:ro
    environment:
      # Tell gopsutil to use host filesystems
      - HOST_PROC=/host/proc
      - HOST_SYS=/host/sys
      - HOST_ROOT=/rootfs
      - MC_LOG_LEVEL=info
      - MC_SHIPPER_ENDPOINT=https://prometheus:9090/api/v1/write
      - MC_TLS_ENABLED=true
      - MC_TLS_CERT_FILE=/etc/metricsd/certs/client.crt
      - MC_TLS_KEY_FILE=/etc/metricsd/certs/client.key
      - MC_TLS_CA_FILE=/etc/metricsd/certs/ca.crt
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    # Privileged mode may be needed for full system access
    # privileged: true
    # Or use specific capabilities
    cap_add:
      - SYS_PTRACE
      - SYS_ADMIN
```

#### Running the Container

**Prerequisites:**
- Built Docker image (see steps above)
- `config.json` file prepared
- TLS certificates (optional, if using TLS)

**Option 1: Quick Start (Container Metrics)**

```bash
# Prepare configuration
cp config.example.json config.json
# Edit config.json with your settings

# Run container
docker run -d \
  --name metricsd \
  -p 8080:8080 \
  -v $(pwd)/config.json:/etc/metricsd/config.json:ro \
  -e MC_LOG_LEVEL=info \
  metricsd:latest

# Check if it's running
docker ps | grep metricsd

# View logs
docker logs -f metricsd

# Check health
curl http://localhost:8080/health
```

**Option 2: With TLS (Secure)**

```bash
# Ensure you have certificates
ls -la certs/
# Should have: client.crt, client.key, ca.crt

# Run with TLS
docker run -d \
  --name metricsd \
  -p 8080:8080 \
  -v $(pwd)/config.json:/etc/metricsd/config.json:ro \
  -v $(pwd)/certs:/etc/metricsd/certs:ro \
  -e MC_LOG_LEVEL=info \
  -e MC_TLS_ENABLED=true \
  -e MC_TLS_CERT_FILE=/etc/metricsd/certs/client.crt \
  -e MC_TLS_KEY_FILE=/etc/metricsd/certs/client.key \
  -e MC_TLS_CA_FILE=/etc/metricsd/certs/ca.crt \
  metricsd:latest
```

**Option 3: Host Metrics Collection (Recommended for Production)**

This mounts host filesystems to collect actual host metrics instead of container metrics:

```bash
docker run -d \
  --name metricsd-host \
  --pid=host \
  --network=host \
  --restart=unless-stopped \
  -v /:/rootfs:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/config.json:/etc/metricsd/config.json:ro \
  -v $(pwd)/certs:/etc/metricsd/certs:ro \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_ROOT=/rootfs \
  -e MC_LOG_LEVEL=info \
  metricsd:latest
```

**Option 4: Using Docker Compose (Easiest)**

```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f metricsd

# Stop
docker-compose down

# Rebuild and restart
docker-compose up -d --build

# View service status
docker-compose ps
```

**Container Management:**

```bash
# Stop container
docker stop metricsd

# Start container
docker start metricsd

# Restart container
docker restart metricsd

# Remove container
docker rm -f metricsd

# View logs (last 100 lines)
docker logs --tail 100 metricsd

# Follow logs in real-time
docker logs -f metricsd

# Check container health status
docker inspect --format='{{.State.Health.Status}}' metricsd

# Execute command in container
docker exec -it metricsd sh

# View container resource usage
docker stats metricsd

# Export container logs to file
docker logs metricsd > metricsd.log 2>&1
```

### Kubernetes

> **Note:** The Deployment below collects **pod/container** metrics. To collect **node/host** metrics in Kubernetes, use a **DaemonSet** instead. See the "Collecting Host Metrics from Docker Container" section for a DaemonSet example.

**deployment.yaml** (for pod metrics):
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: metricsd-config
  namespace: monitoring
data:
  config.json: |
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
        "type": "prometheus_remote_write",
        "endpoint": "https://prometheus.monitoring.svc.cluster.local:9090/api/v1/write",
        "timeout": 30000000000,
        "tls": {
          "enabled": true,
          "cert_file": "/etc/metricsd/certs/tls.crt",
          "key_file": "/etc/metricsd/certs/tls.key",
          "ca_file": "/etc/metricsd/certs/ca.crt",
          "server_name": "prometheus.monitoring.svc.cluster.local",
          "min_version": "TLS1.2"
        }
      },
      "endpoints": []
    }

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metricsd
  namespace: monitoring
  labels:
    app: metricsd
spec:
  replicas: 1
  selector:
    matchLabels:
      app: metricsd
  template:
    metadata:
      labels:
        app: metricsd
    spec:
      serviceAccountName: metricsd
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: metricsd
        image: metricsd:latest
        imagePullPolicy: IfNotPresent
        args:
          - "-config"
          - "/etc/metricsd/config.json"
          - "-log-level"
          - "info"
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        volumeMounts:
        - name: config
          mountPath: /etc/metricsd
          readOnly: true
        - name: certs
          mountPath: /etc/metricsd/certs
          readOnly: true
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
      volumes:
      - name: config
        configMap:
          name: metricsd-config
      - name: certs
        secret:
          secretName: metricsd-tls

---
apiVersion: v1
kind: Service
metadata:
  name: metricsd
  namespace: monitoring
  labels:
    app: metricsd
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app: metricsd

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metricsd
  namespace: monitoring
```

**Create TLS secret:**
```bash
kubectl create secret generic metricsd-tls \
  --from-file=tls.crt=certs/client.crt \
  --from-file=tls.key=certs/client.key \
  --from-file=ca.crt=certs/ca.crt \
  -n monitoring
```

**Deploy:**
```bash
kubectl apply -f deployment.yaml
kubectl get pods -n monitoring
kubectl logs -f -n monitoring deployment/metricsd
```

### Collecting Host Metrics from Docker Container

By default, a containerized application collects metrics from **inside the container** (container CPU, container memory, etc.). To collect metrics from the **host system** instead, you need to mount host filesystems into the container.

#### Why This Matters

- **Container metrics**: Shows resource usage of the container itself (limited by cgroups)
- **Host metrics**: Shows actual host machine CPU, memory, disk, and network usage
- **Use case**: Monitoring the physical/virtual machine where Docker is running

#### Required Mounts

Mount these host paths into your container:

| Host Path | Container Mount | Purpose |
|-----------|----------------|---------|
| `/proc` | `/host/proc:ro` | Process information, CPU stats |
| `/sys` | `/host/sys:ro` | System information, block devices |
| `/` | `/rootfs:ro` | Root filesystem for disk metrics |
| `/var/run/docker.sock` | `/var/run/docker.sock:ro` | Docker socket (optional) |

#### Environment Variables

Set these environment variables to tell the `gopsutil` library to use host paths:

```bash
HOST_PROC=/host/proc
HOST_SYS=/host/sys
HOST_ROOT=/rootfs
```

#### Complete Example

```bash
docker run -d \
  --name metricsd-host-metrics \
  --pid=host \
  --network=host \
  --restart=unless-stopped \
  -v /:/rootfs:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/config.json:/etc/metricsd/config.json:ro \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  -e HOST_ROOT=/rootfs \
  -e MC_LOG_LEVEL=info \
  metricsd:latest
```

#### Docker Compose Example

```yaml
version: '3.8'

services:
  metricsd-host:
    image: metricsd:latest
    container_name: metricsd-host-metrics
    restart: unless-stopped
    network_mode: host  # Access host network interfaces
    pid: host           # Access host processes
    volumes:
      - /:/rootfs:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./config.json:/etc/metricsd/config.json:ro
      - ./certs:/etc/metricsd/certs:ro
    environment:
      - HOST_PROC=/host/proc
      - HOST_SYS=/host/sys
      - HOST_ROOT=/rootfs
    cap_add:
      - SYS_PTRACE  # For process monitoring
```

#### Security Considerations

When collecting host metrics:

- ✅ **Use read-only mounts** (`:ro`) for host filesystems
- ✅ **Minimize capabilities** - only add what's needed (SYS_PTRACE, SYS_ADMIN)
- ⚠️ **Avoid `privileged: true`** unless absolutely necessary
- ✅ **Run as non-root user** when possible
- ✅ **Review mounted paths** - only mount what you need

#### Kubernetes DaemonSet for Host Metrics

For Kubernetes, use a DaemonSet to run one pod per node:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: metricsd-host
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: metricsd-host
  template:
    metadata:
      labels:
        app: metricsd-host
    spec:
      hostNetwork: true
      hostPID: true
      containers:
      - name: metricsd
        image: metricsd:latest
        env:
        - name: HOST_PROC
          value: /host/proc
        - name: HOST_SYS
          value: /host/sys
        - name: HOST_ROOT
          value: /rootfs
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: proc
          mountPath: /host/proc
          readOnly: true
        - name: sys
          mountPath: /host/sys
          readOnly: true
        - name: root
          mountPath: /rootfs
          readOnly: true
        - name: config
          mountPath: /etc/metricsd
        - name: certs
          mountPath: /etc/metricsd/certs
        securityContext:
          capabilities:
            add:
            - SYS_PTRACE
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: root
        hostPath:
          path: /
      - name: config
        configMap:
          name: metricsd-config
      - name: certs
        secret:
          secretName: metricsd-tls
```

#### Verifying Host Metrics Collection

Check the logs to ensure host metrics are being collected:

```bash
# Check logs
docker logs metricsd-host-metrics

# You should see metrics for ALL host CPUs, not just container limits
# Example: If host has 16 cores, you should see metrics for all 16

# Test with debug logging
docker run --rm -it \
  --pid=host \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v $(pwd)/config.json:/etc/metricsd/config.json:ro \
  -e HOST_PROC=/host/proc \
  -e HOST_SYS=/host/sys \
  metricsd:latest -config /etc/metricsd/config.json -log-level debug
```

## Performance Tuning

### Collection Interval

Adjust based on your needs:
- **High-frequency monitoring**: 10-30 seconds
- **Standard monitoring**: 60 seconds (recommended)
- **Low-frequency monitoring**: 300+ seconds

### TLS Performance

- **Enable session tickets** - Reduces TLS handshake overhead
- **Use TLS 1.3** - Faster handshake and better performance
- **Connection pooling** - Automatically handled by the HTTP client
- **Keep-alive** - Connections are reused between shipments

### Resource Usage

Typical resource usage:
- **CPU**: 50-200m (minimal overhead)
- **Memory**: 50-150 MB RSS
- **Network**: Depends on metric volume and shipping frequency

Optimize with:
```json
{
  "collector": {
    "interval_seconds": 60,
    "enable_cpu": true,
    "enable_memory": true,
    "enable_disk": false,
    "enable_network": false,
    "enable_gpu": false
  }
}
```

### Monitoring metricsd

The service exposes its own health endpoint:
- Monitor HTTP response time at `/health`
- Check logs for shipping errors
- Monitor system resource usage
- Set up alerts for service failures

## Development

### Getting Started

```bash
# Clone repository
git clone https://github.com/0x524A/metricsd.git
cd metricsd

# Install dependencies
go mod download

# Build
make build

# Run with development config
./bin/metricsd -config config.json -log-level debug
```

### Project Structure

```
metricsd/
├── cmd/
│   └── metricsd/              # Main application entry point
│       └── main.go
├── internal/                  # Internal packages
│   ├── collector/             # Metric collectors
│   │   ├── collector.go       # Collector interface & registry
│   │   ├── system.go          # System metrics (CPU, memory, disk, network)
│   │   ├── gpu.go             # GPU metrics (NVIDIA NVML)
│   │   └── http.go            # HTTP endpoint scraper
│   ├── config/                # Configuration management
│   │   └── config.go          # Config structs & validation
│   ├── shipper/               # Metric shipping backends
│   │   ├── shipper.go         # Shipper interface
│   │   ├── prometheus.go      # Prometheus remote write protocol
│   │   └── http_json.go       # HTTP JSON POST
│   ├── orchestrator/          # Collection & shipping coordination
│   │   └── orchestrator.go
│   └── server/                # HTTP server (health checks)
│       └── server.go
├── bin/                       # Compiled binaries
├── config.json                # Runtime configuration
├── config.example.json        # Example configuration
├── Makefile                   # Build automation
├── go.mod                     # Go module definition
└── README.md                  # This file
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/collector/...

# Run with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./...
```

### Building

```bash
# Build for current platform
go build -o bin/metricsd cmd/metricsd/main.go

# Build with optimizations
go build -ldflags="-s -w" -o bin/metricsd cmd/metricsd/main.go

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o bin/metricsd-linux-amd64 cmd/metricsd/main.go
GOOS=darwin GOARCH=amd64 go build -o bin/metricsd-darwin-amd64 cmd/metricsd/main.go
GOOS=windows GOARCH=amd64 go build -o bin/metricsd-windows-amd64.exe cmd/metricsd/main.go

# Using Makefile (if available)
make build
make test
make clean
```

### Code Style

Follow standard Go conventions:
- Use `gofmt` for formatting
- Use `golint` for linting
- Use `go vet` for static analysis

```bash
# Format code
gofmt -w .

# Run linter
golangci-lint run

# Static analysis
go vet ./...
```

### Adding a New Collector

1. Create a new collector in `internal/collector/`:
```go
package collector

type MyCollector struct {
    // fields
}

func NewMyCollector() *MyCollector {
    return &MyCollector{}
}

func (c *MyCollector) Collect(ctx context.Context) ([]Metric, error) {
    // Implementation
    return metrics, nil
}

func (c *MyCollector) Name() string {
    return "my_collector"
}
```

2. Register in `cmd/metricsd/main.go`:
```go
myCollector := collector.NewMyCollector()
registry.Register(myCollector)
```

### Adding a New Shipper

1. Create a new shipper in `internal/shipper/`:
```go
package shipper

type MyShipper struct {
    endpoint string
    client   *http.Client
}

func NewMyShipper(endpoint string, tlsConfig *tls.Config) (*MyShipper, error) {
    // Implementation
    return &MyShipper{...}, nil
}

func (s *MyShipper) Ship(ctx context.Context, metrics []collector.Metric) error {
    // Implementation
    return nil
}

func (s *MyShipper) Close() error {
    // Cleanup
    return nil
}
```

2. Add shipper type to config validation in `internal/config/config.go`

3. Add initialization in `cmd/metricsd/main.go`

### SOLID Design Principles

The project adheres to SOLID principles:

- **Single Responsibility Principle (SRP)**
  - Each collector focuses on one metric source
  - Each shipper handles one protocol
  - Orchestrator only coordinates collection and shipping

- **Open/Closed Principle (OCP)**
  - New collectors can be added without modifying existing code
  - New shippers can be plugged in via the interface
  - Configuration is extensible

- **Liskov Substitution Principle (LSP)**
  - All collectors implement the `Collector` interface
  - All shippers implement the `Shipper` interface
  - Components are interchangeable

- **Interface Segregation Principle (ISP)**
  - Small, focused interfaces (`Collector`, `Shipper`)
  - Clients depend only on methods they use
  - No fat interfaces

- **Dependency Inversion Principle (DIP)**
  - High-level modules depend on abstractions (interfaces)
  - Concrete implementations are injected
  - Loose coupling throughout the codebase

## Troubleshooting

### Common Issues

**Service won't start:**
```bash
# Check logs
sudo journalctl -u metricsd -n 50

# Verify configuration
./bin/metricsd -config config.json # Should show validation errors

# Check file permissions
ls -la /opt/metricsd/config.json
ls -la /etc/metricsd/certs/
```

**TLS handshake errors:**
```bash
# Test TLS connection
openssl s_client -connect metrics.example.com:443 \
  -cert /etc/metricsd/certs/client.crt \
  -key /etc/metricsd/certs/client.key \
  -CAfile /etc/metricsd/certs/ca.crt

# Verify certificate
openssl x509 -in /etc/metricsd/certs/client.crt -text -noout

# Check certificate expiration
openssl x509 -in /etc/metricsd/certs/client.crt -checkend 0
```

**Metrics not shipping:**
- Check network connectivity to endpoint
- Verify TLS configuration
- Check endpoint authentication requirements
- Review logs for error messages
- Test endpoint manually with curl

**High memory usage:**
- Reduce collection frequency
- Disable unused collectors
- Check for memory leaks in logs
- Monitor with pprof if needed

**Permission denied errors:**
```bash
# Fix ownership
sudo chown -R metricsd:metricsd /opt/metricsd
sudo chown -R metricsd:metricsd /etc/metricsd

# Fix permissions
sudo chmod 600 /opt/metricsd/config.json
sudo chmod 600 /etc/metricsd/certs/*.key
sudo chmod 644 /etc/metricsd/certs/*.crt
```

## FAQ

**Q: Can I use metricsd without TLS?**
A: Yes, set `shipper.tls.enabled` to `false`. However, TLS is strongly recommended for production.

**Q: Does metricsd support custom metrics?**
A: Yes, add application endpoints to the `endpoints` array in the configuration. The HTTP collector will scrape them.

**Q: How do I rotate TLS certificates?**
A: Update the certificate files, then restart the service. Consider implementing a certificate rotation process with minimal downtime.

**Q: Can I ship to multiple endpoints?**
A: Currently, one shipper endpoint is supported per instance. Run multiple instances for multiple destinations.

**Q: What's the performance impact?**
A: Minimal. Typical CPU usage is <1% and memory usage is around 50-150MB depending on enabled collectors.

**Q: How do I monitor metricsd itself?**
A: Use the `/health` endpoint and monitor the service logs. You can also use process monitoring tools.

**Q: Does it work on Windows?**
A: Yes, but some system metrics may have limited support. GPU metrics require NVIDIA drivers.

**Q: Can I use this with Grafana?**
A: Yes, ship metrics to Prometheus (using remote write) and configure Grafana to query Prometheus.

**Q: How do I debug TLS issues?**
A: Enable debug logging with `-log-level debug` and review the detailed TLS handshake logs.

**Q: Is IPv6 supported?**
A: Yes, both IPv4 and IPv6 are supported for all network operations.

**Q: How do I collect host metrics when running in Docker?**
A: Mount the host's `/proc`, `/sys`, and `/` into the container and set environment variables. See the "Collecting Host Metrics from Docker Container" section for complete instructions.

**Q: Why are my CPU/memory metrics showing container limits instead of host resources?**
A: Without host filesystem mounts, the container only sees its own cgroup limits. Mount host paths and set `HOST_PROC=/host/proc` and `HOST_SYS=/host/sys` to collect host metrics.

## Roadmap

- [ ] Add support for multiple shipper endpoints
- [ ] Implement metric aggregation and buffering
- [ ] Add support for metric filtering and transformation
- [ ] Implement retry logic with exponential backoff
- [ ] Add support for custom labels on system metrics
- [ ] Implement metric caching for offline scenarios
- [ ] Add Datadog, InfluxDB, and other shipper backends
- [ ] Add web UI for configuration and monitoring
- [ ] Implement metric sampling for high-volume scenarios
- [ ] Add support for Windows-specific metrics
- [ ] Implement health check with detailed status information

## License

MIT License - see [LICENSE](LICENSE) file for details

## Contributing

Contributions are welcome! Here's how you can help:

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Make your changes**
4. **Add tests** for new functionality
5. **Ensure tests pass** (`go test ./...`)
6. **Format your code** (`gofmt -w .`)
7. **Commit your changes** (`git commit -m 'Add amazing feature'`)
8. **Push to the branch** (`git push origin feature/amazing-feature`)
9. **Open a Pull Request**

### Contribution Guidelines

- Follow Go best practices and idioms
- Maintain SOLID design principles
- Add tests for new functionality
- Update documentation as needed
- Keep commits atomic and well-described
- Ensure backward compatibility when possible

## Support

### Getting Help

- **Issues**: [GitHub Issues](https://github.com/0x524A/metricsd/issues)
- **Discussions**: [GitHub Discussions](https://github.com/0x524A/metricsd/discussions)
- **Documentation**: This README and inline code comments

### Reporting Bugs

When reporting bugs, please include:
- metricsd version
- Operating system and version
- Go version
- Configuration file (sanitized)
- Relevant log output
- Steps to reproduce

### Feature Requests

Feature requests are welcome! Please:
- Check existing issues first
- Provide detailed use case
- Explain expected behavior
- Consider contributing the feature

## Acknowledgments

Built with:
- [zerolog](https://github.com/rs/zerolog) - Fast structured logging
- [gopsutil](https://github.com/shirou/gopsutil) - System metrics collection
- [prometheus/client_golang](https://github.com/prometheus/client_golang) - Prometheus integration
- [NVML](https://developer.nvidia.com/nvidia-management-library-nvml) - GPU metrics

## Authors

- **Your Name** - *Initial work*

See also the list of [contributors](https://github.com/0x524A/metricsd/contributors) who participated in this project.

---

**Made with ❤️ by the metricsd team**
