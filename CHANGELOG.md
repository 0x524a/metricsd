# Changelog

All notable changes to metricsd will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Docker support with multi-stage builds
- Docker Compose configurations for container and host metrics collection
- Comprehensive TLS/SSL support for remote endpoints
  - Mutual TLS (mTLS) authentication
  - Custom cipher suite configuration
  - TLS version pinning (1.0, 1.1, 1.2, 1.3)
  - Server Name Indication (SNI) support
  - Session ticket control
  - Certificate validation options
- Host metrics collection from Docker containers
- GitHub issue templates (bug report, feature request, question)
- Pull request template
- Contributing guidelines (CONTRIBUTING.md)
- Code of Conduct (CODE_OF_CONDUCT.md)
- Security policy (SECURITY.md)
- Enhanced README with comprehensive documentation

### Changed
- Updated GitHub Actions workflow to Go 1.24
- Migrated Docker images from Alpine to Debian (bookworm) for NVML compatibility
- Improved documentation with detailed examples and troubleshooting guides

### Fixed
- Module import paths corrected to github.com/0x524A/metricsd
- CGO compatibility issues with NVIDIA NVML libraries
- GitHub CI/CD pipeline build failures

## [1.0.0] - YYYY-MM-DD

### Added
- Initial release
- System metrics collection (CPU, memory, disk, network)
- GPU metrics collection (NVIDIA NVML)
- HTTP endpoint metrics collection
- JSON and Prometheus shipper support
- HTTP API server for metrics querying
- Configurable collection intervals
- JSON-based configuration

### Features
- **Collectors:**
  - System metrics: CPU usage, memory, disk I/O, network stats
  - GPU metrics: utilization, memory, temperature, power (NVIDIA)
  - HTTP endpoint scraping
- **Shippers:**
  - HTTP JSON shipper
  - Prometheus remote write support
- **Server:**
  - HTTP API for querying current metrics
  - Health check endpoints
- **Configuration:**
  - Flexible JSON configuration
  - Support for multiple collectors and shippers
  - Configurable collection intervals

---

## Versioning Guidelines

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

## Release Types

### Added
New features or functionality added to the project.

### Changed
Changes in existing functionality.

### Deprecated
Features that will be removed in upcoming releases.

### Removed
Features that have been removed.

### Fixed
Bug fixes.

### Security
Security vulnerability fixes and improvements.

---

## Links

- [Unreleased]: https://github.com/0x524A/metricsd/compare/v1.0.0...HEAD
- [1.0.0]: https://github.com/0x524A/metricsd/releases/tag/v1.0.0

---

**Note:** This changelog is updated with each release. For the full commit history, see the [GitHub repository](https://github.com/0x524A/metricsd/commits/main).
