#!/bin/bash
# Cloud server setup script for metricsd PoC
# Run this on your cloud server to start the metrics receiver stack

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "================================================"
echo "  Metricsd Cloud Setup"
echo "================================================"

# Check Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    exit 1
fi

# Check Docker Compose is installed
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "Error: Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Create required directories
echo "Creating directories..."
mkdir -p prometheus grafana/provisioning/datasources grafana/provisioning/dashboards grafana/dashboards

# Create Prometheus configuration file
echo "Creating Prometheus configuration..."
cat > prometheus/prometheus.yml << 'EOF'
# Prometheus configuration for receiving metrics from remote metricsd clients
global:
  scrape_interval: 60s
  evaluation_interval: 60s
  external_labels:
    monitor: 'metricsd-cloud'

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
        labels:
          instance: 'prometheus-server'
EOF

# Create Grafana datasource configuration
echo "Creating Grafana datasource configuration..."
cat > grafana/provisioning/datasources/datasources.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
    jsonData:
      httpMethod: POST
      manageAlerts: true
      prometheusType: Prometheus
      prometheusVersion: 2.48.0
EOF

# Create Grafana dashboard provisioning configuration
echo "Creating Grafana dashboard provisioning..."
cat > grafana/provisioning/dashboards/dashboards.yml << 'EOF'
apiVersion: 1

providers:
  - name: 'Metricsd Dashboards'
    orgId: 1
    folder: 'Metricsd'
    folderUid: 'metricsd'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards
EOF

echo "Configuration files created successfully!"

echo "Starting services..."
if docker compose version &> /dev/null; then
    docker compose -f docker-compose.cloud.yml up -d
else
    docker-compose -f docker-compose.cloud.yml up -d
fi

echo ""
echo "Waiting for services to be ready..."
sleep 10

# Check Prometheus
echo -n "Checking Prometheus... "
if curl -s http://localhost:9090/-/healthy | grep -q "Healthy"; then
    echo "✓ Ready"
else
    echo "✗ Not ready (may need more time)"
fi

# Check Grafana
echo -n "Checking Grafana... "
if curl -s http://localhost:3000/api/health | grep -q "ok"; then
    echo "✓ Ready"
else
    echo "✗ Not ready (may need more time)"
fi

# Get server IP
SERVER_IP=$(hostname -I | awk '{print $1}')

echo ""
echo "================================================"
echo "  Setup Complete!"
echo "================================================"
echo ""
echo "Services:"
echo "  - Prometheus: http://${SERVER_IP}:9090"
echo "  - Grafana:    http://${SERVER_IP}:3000 (admin/admin)"
echo ""
echo "Remote Write Endpoint for clients:"
echo "  http://${SERVER_IP}:9090/api/v1/write"
echo ""
echo "Update your client config.json with:"
echo "  \"endpoint\": \"http://${SERVER_IP}:9090/api/v1/write\""
echo ""
echo "View logs:"
echo "  docker-compose -f docker-compose.cloud.yml logs -f"
echo ""

