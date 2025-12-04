# Metricsd Client PoC Setup

This folder contains everything needed to run the metricsd client with test services.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CLIENT MACHINE                                 │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │              Docker Compose Services                             │   │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────────┐   │   │
│  │  │  RabbitMQ   │ │  PostgreSQL │ │    Test Generator       │   │   │
│  │  │  :15692     │ │  :5432      │ │    :8081                │   │   │
│  │  │  /metrics   │ │             │ │    /metrics             │   │   │
│  │  └──────┬──────┘ └──────┬──────┘ └───────────┬─────────────┘   │   │
│  │         │               │                     │                 │   │
│  │         │        ┌──────┴──────┐              │                 │   │
│  │         │        │  Postgres   │              │                 │   │
│  │         │        │  Exporter   │              │                 │   │
│  │         │        │  :9187      │              │                 │   │
│  │         │        │  /metrics   │              │                 │   │
│  │         │        └──────┬──────┘              │                 │   │
│  └─────────┼───────────────┼─────────────────────┼─────────────────┘   │
│            │               │                     │                      │
│            └───────────────┼─────────────────────┘                      │
│                            │                                            │
│                     ┌──────▼──────┐                                     │
│                     │   metricsd  │                                     │
│                     │   (binary)  │                                     │
│                     │             │                                     │
│                     │ + System    │                                     │
│                     │   Metrics   │                                     │
│                     └──────┬──────┘                                     │
│                            │                                            │
└────────────────────────────┼────────────────────────────────────────────┘
                             │
                             │ Remote Write
                             ▼
                    ┌─────────────────┐
                    │ Cloud Prometheus│
                    │   :9090         │
                    └─────────────────┘
```

## Quick Start

### 1. Start the Test Services

```bash
cd client
docker-compose -f docker-compose.services.yml up -d
```

This starts:
- **RabbitMQ** (port 5672, management UI on 15672, metrics on 15692)
- **PostgreSQL** (port 5432)
- **PostgreSQL Exporter** (port 9187)
- **Test Generator** (port 8081) - creates sample workload

### 2. Verify Services Are Running

```bash
# Check all services are healthy
docker-compose -f docker-compose.services.yml ps

# Test RabbitMQ metrics
curl http://localhost:15692/metrics | head -20

# Test PostgreSQL exporter metrics
curl http://localhost:9187/metrics | head -20

# Test Generator metrics
curl http://localhost:8081/metrics | head -20
```

### 3. Update metricsd Configuration

Edit `config.json` in the project root and set your cloud Prometheus IP:

```json
{
  "shipper": {
    "endpoint": "http://YOUR_CLOUD_IP:9090/api/v1/write"
  }
}
```

### 4. Run metricsd Binary

```bash
# From project root
.\bin\metricsd.exe -config config.json -log-level debug
```

## What Gets Collected

### System Metrics (from host)
- CPU usage (per core and total)
- Memory usage
- Disk usage and I/O
- Network traffic

### RabbitMQ Metrics
- Queue depths
- Message rates
- Connection counts
- Channel counts
- Memory and disk usage

### PostgreSQL Metrics
- Active connections
- Transaction rates
- Database size
- Table/index statistics
- Replication status

### Test Generator Metrics
- `test_generator_messages_published_total` - Messages sent to RabbitMQ
- `test_generator_messages_consumed_total` - Messages consumed from RabbitMQ
- `test_generator_db_queries_total` - Database queries executed
- `test_generator_db_query_duration_seconds` - Query latency histogram
- `test_generator_orders_created_total` - Orders created
- `test_generator_events_processed_total` - Events processed
- `test_generator_errors_total` - Errors by component

## Service Credentials

| Service | Username | Password | Port |
|---------|----------|----------|------|
| RabbitMQ | admin | admin123 | 5672, 15672 |
| PostgreSQL | admin | admin123 | 5432 |

## Metrics Endpoints

| Service | Metrics URL |
|---------|-------------|
| RabbitMQ | http://localhost:15692/metrics |
| PostgreSQL | http://localhost:9187/metrics |
| Test Generator | http://localhost:8081/metrics |

## Cleanup

```bash
# Stop and remove containers
docker-compose -f docker-compose.services.yml down

# Also remove volumes (data)
docker-compose -f docker-compose.services.yml down -v
```

## Troubleshooting

### Services not starting

```bash
# Check logs
docker-compose -f docker-compose.services.yml logs

# Check specific service
docker-compose -f docker-compose.services.yml logs rabbitmq
docker-compose -f docker-compose.services.yml logs postgres
docker-compose -f docker-compose.services.yml logs test-generator
```

### Metrics endpoint not responding

Ensure services are healthy:
```bash
docker-compose -f docker-compose.services.yml ps
```

Wait for health checks to pass (can take 30-60 seconds).

### metricsd can't connect to cloud

1. Check firewall allows outbound to port 9090
2. Verify cloud Prometheus is running with `--web.enable-remote-write-receiver`
3. Test connectivity: `curl http://YOUR_CLOUD_IP:9090/-/healthy`

