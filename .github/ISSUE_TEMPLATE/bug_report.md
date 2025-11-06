---
name: Bug Report
about: Create a report to help us improve metricsd
title: '[BUG] '
labels: 'bug'
assignees: ''

---

## Bug Description
<!-- A clear and concise description of what the bug is -->

## To Reproduce
Steps to reproduce the behavior:
1. Configure metricsd with '...'
2. Run command '...'
3. Observe error '...'
4. See error

## Expected Behavior
<!-- A clear and concise description of what you expected to happen -->

## Actual Behavior
<!-- What actually happened -->

## Environment
<!-- Please complete the following information -->

**metricsd version:**
<!-- Output of: ./metricsd -version -->

**Operating System:**
<!-- e.g., Ubuntu 22.04, CentOS 7, macOS 13, Windows 11 -->

**Architecture:**
<!-- e.g., amd64, arm64 -->

**Go version (if building from source):**
<!-- Output of: go version -->

**Deployment method:**
- [ ] Binary
- [ ] Docker
- [ ] Kubernetes
- [ ] Built from source

**GPU present:**
- [ ] Yes (NVIDIA)
- [ ] No

## Configuration
<!-- Please provide relevant parts of your config.json (sanitize sensitive data) -->

```json
{
  "collector": {
    ...
  }
}
```

## Logs
<!-- Please provide relevant log output. Use code blocks for better readability -->

```
[paste logs here]
```

## Additional Context
<!-- Add any other context about the problem here -->

<!-- Screenshots, if applicable -->

## Possible Solution
<!-- Optional: Suggest a fix or reason for the bug -->

## Checklist
- [ ] I have searched existing issues to ensure this is not a duplicate
- [ ] I have provided all requested information
- [ ] I have sanitized any sensitive data from logs and configuration
- [ ] I am using a supported version of metricsd
