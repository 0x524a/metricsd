# Sudo Configuration for Plugins

Some plugins require elevated permissions to collect metrics. This guide explains how to configure sudo access for the `metricsd` user to run specific commands without a password prompt.

## UFW Plugin

The UFW plugin needs to run `ufw status verbose` to collect firewall metrics.

### Setup

Create a sudoers file for metricsd:

```bash
sudo visudo -f /etc/sudoers.d/metricsd
```

Add the following content:

```
# Allow metricsd user to run UFW commands without password
metricsd ALL=(ALL) NOPASSWD: /usr/sbin/ufw status verbose
metricsd ALL=(ALL) NOPASSWD: /usr/sbin/ufw status
```

Set proper permissions:

```bash
sudo chmod 0440 /etc/sudoers.d/metricsd
```

Verify the configuration:

```bash
sudo -l -U metricsd
```

You should see:
```
User metricsd may run the following commands on hostname:
    (ALL) NOPASSWD: /usr/sbin/ufw status verbose
    (ALL) NOPASSWD: /usr/sbin/ufw status
```

## Nerdctl Plugin

The nerdctl plugin requires access to the containerd socket, which typically requires root or specific group membership.

### Option 1: Add metricsd user to docker/containerd group

```bash
# Check if the group exists
getent group docker || getent group containerd

# Add metricsd to the appropriate group
sudo usermod -aG docker metricsd
# OR
sudo usermod -aG containerd metricsd

# Restart metricsd service
sudo systemctl restart metricsd
```

### Option 2: Configure sudo for nerdctl

```bash
sudo visudo -f /etc/sudoers.d/metricsd
```

Add:

```
# Allow metricsd user to run nerdctl commands without password
metricsd ALL=(ALL) NOPASSWD: /usr/local/bin/nerdctl *
metricsd ALL=(ALL) NOPASSWD: /usr/bin/nerdctl *
```

Then update the plugin configuration to use sudo:

**plugins/nerdctl.json:**
```json
{
  "name": "nerdctl",
  "timeout": 60000000000,
  "enabled": true,
  "path": "/usr/lib/metricsd/plugins/nerdctl-wrapper"
}
```

**Create wrapper script** at `/usr/lib/metricsd/plugins/nerdctl-wrapper`:
```bash
#!/bin/bash
# Wrapper to run nerdctl with sudo
export PATH=/usr/local/bin:/usr/bin:$PATH
NERDCTL_CMD=$(command -v nerdctl)
exec sudo $NERDCTL_CMD "$@"
```

```bash
sudo chmod +x /usr/lib/metricsd/plugins/nerdctl-wrapper
```

### Option 3: Configure namespace via environment variable

If nerdctl is accessible but containers aren't showing, specify the namespace:

**plugins/nerdctl.json:**
```json
{
  "name": "nerdctl",
  "timeout": 60000000000,
  "enabled": true,
  "env": [
    "NERDCTL_NAMESPACE=k8s.io"
  ]
}
```

Common namespaces:
- `k8s.io` - Kubernetes containers
- `default` - Default namespace
- `moby` - Docker compatibility

The plugin will auto-detect the namespace if not specified, but manual configuration may be more reliable.

## Testing

After configuring sudo, test the plugins:

```bash
# Test as metricsd user
sudo -u metricsd /usr/lib/metricsd/plugins/ufw
sudo -u metricsd /usr/lib/metricsd/plugins/nerdctl
```

Both should output JSON metrics without errors.

## Security Considerations

1. **Least Privilege**: Only grant sudo access to specific commands needed by plugins
2. **Command Paths**: Use full paths in sudoers files to prevent PATH manipulation
3. **No Shell Escapes**: Avoid using wildcards that could allow arbitrary command execution
4. **Audit**: Regularly review sudo logs: `sudo grep metricsd /var/log/auth.log`
5. **Read-Only Commands**: Only allow commands that read data, never write/modify

## Troubleshooting

### Plugin returns only "available" metric

This usually means the plugin can detect the command is installed but can't execute it:

```bash
# Check permissions
sudo -u metricsd which ufw
sudo -u metricsd which nerdctl

# Test command execution
sudo -u metricsd ufw status verbose
sudo -u metricsd nerdctl ps
```

### Permission denied errors

```bash
# Check sudoers syntax
sudo visudo -c

# Check sudoers file permissions
ls -l /etc/sudoers.d/metricsd

# Check user exists and service is running as correct user
ps aux | grep metricsd
```

### Containers not showing (nerdctl)

```bash
# List all namespaces
sudo nerdctl namespace ls

# Check containers in each namespace
sudo nerdctl -n k8s.io ps -a
sudo nerdctl -n default ps -a

# Set namespace in config
# See "Option 3" above
```
