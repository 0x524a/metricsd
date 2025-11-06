# metricsd - Lightweight System & GPU Metrics Collector

[![Build Status](https://github.com/0x524A/metricsd/workflows/Go/badge.svg)](https://github.com/0x524A/metricsd/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/0x524A/metricsd)](https://goreportcard.com/report/github.com/0x524A/metricsd)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Pulls](https://img.shields.io/docker/pulls/0x524a/metricsd)](https://hub.docker.com/r/0x524a/metricsd)
[![GitHub release](https://img.shields.io/github/release/0x524A/metricsd.svg)](https://github.com/0x524A/metricsd/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/0x524A/metricsd)](https://github.com/0x524A/metricsd/blob/main/go.mod)

## Repository Information for GitHub

This file contains the key information to update your GitHub repository's "About" section and metadata.

---

## Repository Description

**Short Description (160 chars max for GitHub):**
```
Lightweight, high-performance metrics collector for system, GPU, and HTTP endpoint monitoring with TLS support and flexible shipping options
```

**Detailed Description:**
```
metricsd is a lightweight, efficient metrics collection daemon written in Go that gathers system metrics (CPU, memory, disk, network), NVIDIA GPU metrics (via NVML), and HTTP endpoint data. Features enterprise-grade TLS/mTLS support, flexible shipping to Prometheus/JSON endpoints, Docker support with host metrics collection, and production-ready observability tooling.
```

---

## Repository Topics (GitHub Keywords)

Add these topics to your repository for better discoverability:

```
metrics
monitoring
observability
prometheus
nvidia
gpu-monitoring
system-metrics
golang
docker
kubernetes
tls
mtls
metrics-collection
devops
sre
performance-monitoring
nvml
telemetry
time-series
infrastructure-monitoring
```

---

## GitHub Repository Settings

### About Section

**Website:** 
```
https://github.com/0x524A/metricsd
```

**Topics:** (Copy the topics list above)

**Include in homepage:** ✅ Check this box

---

## Social Preview Image (Optional)

Consider creating a social preview image (1280x640px) showing:
- Project name "metricsd"
- Key features: System + GPU + HTTP Metrics
- Tech stack badges: Go, Docker, Prometheus, NVIDIA
- Tagline: "Lightweight Metrics Collection for Modern Infrastructure"

Upload to: Repository Settings → Social preview → Upload an image

---

## Key Features for Marketing

### 🚀 Core Capabilities
- **Multi-source metrics collection:** System, GPU (NVIDIA), HTTP endpoints
- **Production-ready:** Enterprise TLS/mTLS, secure defaults, comprehensive error handling
- **Flexible shipping:** Prometheus remote write, HTTP JSON endpoints
- **Docker-native:** Multi-stage builds, non-root containers, host metrics support
- **Low overhead:** Written in Go, minimal resource footprint
- **Easy configuration:** Simple JSON-based config with extensive examples

### 🔒 Security First
- Mutual TLS (mTLS) authentication
- Custom cipher suite configuration
- Certificate pinning and validation
- Secure defaults, no privileged requirements
- Regular security updates via Dependabot

### 📊 Observability
- Built-in HTTP API for metrics queries
- Health check endpoints
- Structured logging
- Prometheus-compatible output
- Real-time and historical metrics

### 🐳 Cloud-Native
- Official Docker images
- Kubernetes-ready
- Compose examples included
- Host metrics from containers
- Multi-architecture support (amd64, arm64)

---

## Installation Quick Start

```bash
# Binary
curl -LO https://github.com/0x524A/metricsd/releases/latest/download/metricsd
chmod +x metricsd
./metricsd -config config.json

# Docker
docker pull 0x524a/metricsd:latest
docker run -v $(pwd)/config.json:/etc/metricsd/config.json 0x524a/metricsd:latest

# Docker Compose
curl -LO https://raw.githubusercontent.com/0x524A/metricsd/main/docker-compose.yml
docker-compose up -d

# From Source
git clone https://github.com/0x524A/metricsd.git
cd metricsd
make build
./bin/metricsd -config config.json
```

---

## Use Cases

### Infrastructure Monitoring
Monitor bare-metal servers, VMs, and cloud instances with minimal overhead.

### GPU Workload Monitoring
Track NVIDIA GPU utilization, memory, temperature, and power consumption for ML/AI workloads.

### Container Observability
Collect metrics from containerized applications and underlying hosts.

### Edge Computing
Lightweight footprint perfect for edge devices and IoT gateways.

### Multi-Tenant SaaS
Ship metrics to multiple Prometheus instances or custom endpoints.

### Development & Testing
Quick setup for local development environment monitoring.

---

## Community & Support

- **Documentation:** [README.md](https://github.com/0x524A/metricsd/blob/main/README.md)
- **Contributing:** [CONTRIBUTING.md](https://github.com/0x524A/metricsd/blob/main/CONTRIBUTING.md)
- **Code of Conduct:** [CODE_OF_CONDUCT.md](https://github.com/0x524A/metricsd/blob/main/CODE_OF_CONDUCT.md)
- **Security:** [SECURITY.md](https://github.com/0x524A/metricsd/blob/main/SECURITY.md)
- **Changelog:** [CHANGELOG.md](https://github.com/0x524A/metricsd/blob/main/CHANGELOG.md)
- **Issues:** [GitHub Issues](https://github.com/0x524A/metricsd/issues)
- **Discussions:** [GitHub Discussions](https://github.com/0x524A/metricsd/discussions)

---

## Statistics & Metrics

### Project Stats
- **Language:** Go 1.24+
- **License:** MIT
- **Container Size:** ~95MB
- **Build Time:** ~90 seconds
- **Dependencies:** Minimal, well-maintained

### Repository Health
- ✅ Continuous Integration (GitHub Actions)
- ✅ Automated dependency updates (Dependabot)
- ✅ Comprehensive documentation
- ✅ Issue templates and PR templates
- ✅ Security policy
- ✅ Code of conduct
- ✅ Contributing guidelines

---

## Comparisons

### vs. node_exporter (Prometheus)
- ✅ GPU metrics built-in (no separate exporter needed)
- ✅ HTTP endpoint scraping
- ✅ Multiple shipper support
- ✅ Smaller binary and Docker image

### vs. Telegraf
- ✅ Simpler configuration
- ✅ Smaller footprint
- ✅ Better GPU support out-of-the-box
- ✅ Go-native (no plugin dependencies)

### vs. Custom Solutions
- ✅ Production-ready with TLS/mTLS
- ✅ Well-documented and maintained
- ✅ Docker and Kubernetes ready
- ✅ Active community

---

## Roadmap Ideas

Future enhancements could include:
- Additional metrics collectors (SNMP, JMX, custom scripts)
- More shipper backends (InfluxDB, Datadog, New Relic)
- Web UI for configuration and visualization
- Auto-discovery of services
- Alert manager integration
- Multi-architecture Docker builds
- Windows support
- Plugin system for extensibility

---

## Credits & Acknowledgments

### Maintainers
- [@0x524A](https://github.com/0x524A) - Creator and lead maintainer

### Technologies
- **Go** - Primary language
- **NVIDIA NVML** - GPU metrics
- **Docker** - Containerization
- **Prometheus** - Metrics format and shipping
- **GitHub Actions** - CI/CD

### Inspiration
- Prometheus ecosystem
- Cloud-native observability practices
- SRE principles

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Quick Links

- 🏠 [Home](https://github.com/0x524A/metricsd)
- 📚 [Documentation](https://github.com/0x524A/metricsd/blob/main/README.md)
- 🐛 [Report Bug](https://github.com/0x524A/metricsd/issues/new?template=bug_report.md)
- 💡 [Request Feature](https://github.com/0x524A/metricsd/issues/new?template=feature_request.md)
- ❓ [Ask Question](https://github.com/0x524A/metricsd/issues/new?template=question.md)
- 🤝 [Contribute](https://github.com/0x524A/metricsd/blob/main/CONTRIBUTING.md)
- 🔒 [Security](https://github.com/0x524A/metricsd/blob/main/SECURITY.md)

---

**⭐ If you find metricsd useful, please consider giving it a star on GitHub! ⭐**
