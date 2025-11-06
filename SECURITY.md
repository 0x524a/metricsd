# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of metricsd seriously. If you believe you have found a security vulnerability, please report it to us as described below.

### Please do NOT:

* Open a public GitHub issue for security vulnerabilities
* Disclose the vulnerability publicly before it has been addressed

### Please DO:

1. **Email** your findings to: [INSERT SECURITY EMAIL]
2. **Encrypt** sensitive information using our PGP key (if available)
3. **Include** the following information:
   * Type of vulnerability
   * Full paths of source file(s) related to the vulnerability
   * Location of the affected source code (tag/branch/commit or direct URL)
   * Step-by-step instructions to reproduce the issue
   * Proof-of-concept or exploit code (if possible)
   * Impact of the vulnerability, including how an attacker might exploit it

### What to expect:

* **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours
* **Initial Assessment**: We will provide an initial assessment within 7 days
* **Updates**: We will keep you informed about our progress
* **Resolution**: We aim to resolve critical vulnerabilities within 30 days
* **Credit**: We will credit you in the security advisory (unless you prefer to remain anonymous)

## Security Best Practices

When using metricsd in production:

### TLS/SSL Configuration

* **Always enable TLS** for remote endpoint connections in production
* **Use TLS 1.2 or higher** - Set `min_version: "TLS1.2"`
* **Enable mTLS** (mutual TLS) for client certificate authentication
* **Verify certificates** - Keep `insecure_skip_verify: false`
* **Use strong cipher suites** - Configure explicit cipher suites
* **Rotate certificates regularly** - Implement a certificate rotation policy

```json
{
  "shipper": {
    "tls": {
      "enabled": true,
      "cert_file": "/etc/metricsd/certs/client.crt",
      "key_file": "/etc/metricsd/certs/client.key",
      "ca_file": "/etc/metricsd/certs/ca.crt",
      "min_version": "TLS1.2",
      "insecure_skip_verify": false
    }
  }
}
```

### File Permissions

Protect sensitive configuration and certificate files:

```bash
# Configuration file
chmod 600 /etc/metricsd/config.json
chown metricsd:metricsd /etc/metricsd/config.json

# Certificate directory
chmod 700 /etc/metricsd/certs

# Private keys
chmod 600 /etc/metricsd/certs/*.key

# Certificates
chmod 644 /etc/metricsd/certs/*.crt
```

### Running as Non-Root

Always run metricsd as a dedicated non-privileged user:

```bash
# Create dedicated user
sudo useradd -r -s /bin/false -d /opt/metricsd metricsd

# Set ownership
sudo chown -R metricsd:metricsd /opt/metricsd
```

### Network Security

* Use internal/private networks when possible
* Implement firewall rules to restrict access
* Use VPNs or secure tunnels for remote connections
* Monitor network traffic for anomalies

### Docker Security

When running in containers:

* **Use non-root user** - The Dockerfile creates user `metricsd:1000`
* **Read-only filesystems** - Mount host paths as `:ro`
* **Minimal capabilities** - Only add necessary capabilities (SYS_PTRACE, etc.)
* **Avoid privileged mode** - Use specific capabilities instead
* **Scan images regularly** - Use tools like Trivy or Snyk
* **Keep images updated** - Rebuild regularly with latest base images

### Kubernetes Security

* Use NetworkPolicies to restrict pod communication
* Enable Pod Security Standards (PSS)
* Use RBAC for fine-grained access control
* Store secrets in Kubernetes Secrets, not ConfigMaps
* Use security contexts with minimal privileges

### Configuration Security

* **Never commit secrets** to version control
* Use environment variables for sensitive values
* Implement secrets rotation policies
* Use secrets management tools (HashiCorp Vault, AWS Secrets Manager, etc.)
* Audit configuration changes

### Monitoring and Logging

* Enable detailed logging for security events
* Monitor for suspicious activity
* Set up alerts for authentication failures
* Regularly review logs
* Implement log aggregation and analysis

## Known Security Considerations

### GPU Metrics Collection

* Requires CGO and NVML libraries
* May need elevated permissions on some systems
* Only enable GPU metrics if you have NVIDIA GPUs

### Host Metrics Collection (Docker)

When collecting host metrics from containers:

* Requires mounting host filesystems (`/proc`, `/sys`, `/`)
* Needs host PID and network namespaces
* May require SYS_PTRACE and SYS_ADMIN capabilities
* Only use in trusted environments
* Always use read-only mounts (`:ro`)

### Endpoint Scraping

* metricsd makes HTTP requests to configured endpoints
* Validate endpoint URLs before adding to configuration
* Use internal networks for endpoint access
* Implement authentication for scraped endpoints

## Dependencies

We regularly update dependencies to patch security vulnerabilities:

* Run `go mod tidy` to update dependencies
* Check for vulnerabilities with `govulncheck`:
  ```bash
  go install golang.org/x/vuln/cmd/govulncheck@latest
  govulncheck ./...
  ```
* Review dependency updates in pull requests

## Security Updates

Security updates will be:

* Released as soon as possible after discovery
* Announced in GitHub Security Advisories
* Documented in release notes
* Tagged with version numbers following semver

Subscribe to GitHub notifications to stay informed about security updates.

## Bug Bounty Program

We currently do not have a bug bounty program. However, we greatly appreciate security researchers who responsibly disclose vulnerabilities.

## Hall of Fame

We thank the following security researchers for responsibly disclosing vulnerabilities:

* (None yet - be the first!)

## Questions?

For security-related questions that are not vulnerabilities, please:

* Open a GitHub Discussion
* Check existing documentation
* Review the Security section in README.md

---

**Thank you for helping keep metricsd and our users safe!** 🔒
