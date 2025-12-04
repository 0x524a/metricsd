# Metricsd PoC Setup Guide

This guide explains how to set up metricsd as a Proof of Concept (PoC) with:
- **Cloud side**: Prometheus + Grafana for metrics storage and visualization
- **Client side**: metricsd running on edge devices, sending metrics with hostname labels

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLOUD SERVER                             │
│  ┌─────────────────┐         ┌─────────────────────────────────┐│
│  │   Prometheus    │◄────────│          Grafana                ││
│  │   (Port 9090)   │         │        (Port 3000)              ││
│  │                 │         │  - Fleet Overview Dashboard     ││
│  │ Remote Write    │         │  - Device Overview Dashboard    ││
│  │ Receiver        │         │  - Filter by hostname           ││
│  └────────▲────────┘         └─────────────────────────────────┘│
│           │                                                      │
└───────────┼──────────────────────────────────────────────────────┘
            │ Remote Write API
            │ (HTTP POST to /api/v1/write)
            │
┌───────────┴──────────────────────────────────────────────────────┐
│                        NETWORK                                    │
└───────────┬───────────────────────────────┬──────────────────────┘
            │                               │
    ┌───────▼───────┐               ┌───────▼───────┐
    │  Edge Device 1│               │  Edge Device 2│
    │  ┌──────────┐ │               │  ┌──────────┐ │
    │  │ metricsd │ │               │  │ metricsd │ │
    │  │          │ │               │  │          │ │
    │  │ hostname:│ │               │  │ hostname:│ │
    │  │ device-01│ │               │  │ device-02│ │
    │  └──────────┘ │               │  └──────────┘ │
    └───────────────┘               └───────────────┘
```

## Quick Start

### 1. Cloud Server Setup

On your cloud server (Linux machine with Docker):

```bash
# Clone the repository (or copy the cloud folder)
cd /opt
git clone https://github.com/0x524A/metricsd.git
cd metricsd/cloud

# Start the stack
docker-compose -f docker-compose.cloud.yml up -d

# Verify services are running
docker-compose -f docker-compose.cloud.yml ps

# Check Prometheus is ready
curl http://localhost:9090/-/healthy

# Check Grafana is ready
curl http://localhost:3000/api/health
```

**Access the services:**
- Prometheus: `http://YOUR_CLOUD_IP:9090`
- Grafana: `http://YOUR_CLOUD_IP:3000` (admin/admin)

### 2. Client (Edge Device) Setup

On each edge device:

```bash
# Copy the client folder to the device
scp -r client/ user@edge-device:/opt/metricsd/

# SSH to the device
ssh user@edge-device
cd /opt/metricsd

# Update the configuration with your cloud IP
nano config.client.json
# Change: "endpoint": "http://YOUR_CLOUD_IP:9090/api/v1/write"

# Build or pull the metricsd image
# Option A: Build locally
docker build -t metricsd:latest -f ../Dockerfile ..

# Option B: Pull from Docker Hub (when published)
# docker pull 0x524a/metricsd:latest

# Start metricsd
docker-compose -f docker-compose.client.yml up -d

# Check logs
docker logs -f metricsd-client
```

### 3. Verify Metrics Flow

1. **Check client is sending metrics:**
   ```bash
   docker logs metricsd-client | grep "shipped metrics"
   ```

2. **Query Prometheus:**
   Open `http://YOUR_CLOUD_IP:9090` and run:
   ```promql
   system_cpu_usage_total_percent
   ```
   You should see metrics with hostname labels.

3. **View in Grafana:**
   - Open `http://YOUR_CLOUD_IP:3000`
   - Login: admin/admin
   - Go to Dashboards → Metricsd → Fleet Overview
   - Select a device from the dropdown in "Device Overview"

## Configuration Details

### Client Configuration (`config.client.json`)

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "collector": {
    "interval_seconds": 60,        // How often to collect metrics
    "enable_cpu": true,
    "enable_memory": true,
    "enable_disk": true,
    "enable_network": true,
    "enable_gpu": false            // Set true for NVIDIA GPU monitoring
  },
  "shipper": {
    "type": "prometheus_remote_write",
    "endpoint": "http://YOUR_CLOUD_IP:9090/api/v1/write",
    "timeout": 30000000000         // 30 seconds in nanoseconds
  },
  "global_labels": {
    "environment": "production",   // Custom labels added to all metrics
    "region": "us-east"
  }
}
```

### Hostname Label

The hostname is **automatically detected** from the system and added to all metrics.
You can override it by setting a custom hostname in `global_labels`:

```json
{
  "global_labels": {
    "hostname": "custom-device-name"
  }
}
```

### Environment Variable Overrides

Override config values without editing the file:

```bash
# Override endpoint
export MC_SHIPPER_ENDPOINT=http://new-cloud-ip:9090/api/v1/write

# Override collection interval
export MC_COLLECTOR_INTERVAL=30

# Enable TLS
export MC_TLS_ENABLED=true
export MC_TLS_CERT_FILE=/etc/metricsd/certs/client.crt
export MC_TLS_KEY_FILE=/etc/metricsd/certs/client.key
export MC_TLS_CA_FILE=/etc/metricsd/certs/ca.crt
```

## Grafana Dashboards

### Fleet Overview
Shows all devices at a glance:
- Total device count
- Devices with high CPU/Memory/Disk usage
- Device status table with links to individual device dashboards
- CPU and Memory trends by device

### Device Overview
Detailed view of a single device:
- CPU, Memory, Disk usage stats
- CPU & Memory over time
- Network traffic graphs
- Disk usage by mount point

## Firewall Configuration

### Cloud Server
Open these ports:
- `9090/tcp` - Prometheus (remote write receiver)
- `3000/tcp` - Grafana web interface

```bash
# UFW example
sudo ufw allow 9090/tcp
sudo ufw allow 3000/tcp
```

### Edge Devices
- Outbound to cloud server port `9090/tcp`
- Optionally expose `8080/tcp` for local health checks

## TLS/Security Setup (Production)

For production deployments, enable TLS:

### 1. Generate Certificates

```bash
# On cloud server, generate CA and server certs
cd /opt/metricsd/certs

# Generate CA
openssl req -x509 -new -nodes -keyout ca.key -sha256 -days 1825 -out ca.crt \
  -subj "/CN=Metricsd CA"

# Generate client certificate for each device
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr \
  -subj "/CN=metricsd-client"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 825 -sha256
```

### 2. Update Client Configuration

```json
{
  "shipper": {
    "type": "prometheus_remote_write",
    "endpoint": "https://YOUR_CLOUD_IP:9090/api/v1/write",
    "tls": {
      "enabled": true,
      "cert_file": "/etc/metricsd/certs/client.crt",
      "key_file": "/etc/metricsd/certs/client.key",
      "ca_file": "/etc/metricsd/certs/ca.crt"
    }
  }
}
```

## Troubleshooting

### Metrics not appearing in Prometheus

1. Check client logs:
   ```bash
   docker logs metricsd-client
   ```

2. Test connectivity from client:
   ```bash
   curl -X POST http://YOUR_CLOUD_IP:9090/api/v1/write
   # Should return 400 (Bad Request) - means endpoint is reachable
   ```

3. Verify Prometheus remote write is enabled:
   ```bash
   docker logs prometheus-receiver | grep "remote write"
   ```

### High CPU/Memory on Client

Increase collection interval:
```json
{
  "collector": {
    "interval_seconds": 120
  }
}
```

Or disable unneeded collectors:
```json
{
  "collector": {
    "enable_disk": false,
    "enable_network": false
  }
}
```

### Grafana shows "No data"

1. Check Prometheus has data:
   ```promql
   {hostname=~".+"}
   ```

2. Verify time range includes data collection period

3. Check datasource configuration in Grafana

## Scaling Considerations

- **Prometheus retention**: Default is 30 days, adjust in docker-compose
- **High device count**: Consider VictoriaMetrics for better performance
- **Network bandwidth**: Each device sends ~1-5KB per collection interval

## Next Steps

1. Set up alerting rules in Prometheus
2. Add more dashboards for specific use cases
3. Implement TLS for production security
4. Set up backup for Prometheus data

