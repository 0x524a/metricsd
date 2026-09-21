# Contributing to metricsd

Contributions are welcome! We appreciate your help in making metricsd better. This document outlines the process and guidelines for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it to understand our community standards and expectations.

## Security Vulnerabilities

**Do not open public issues for security vulnerabilities.** Instead, please refer to our [Security Policy](SECURITY.md) for responsible disclosure procedures.

## Getting Started

### Prerequisites

- Go 1.25 or higher
- Make (for running build targets)
- golangci-lint (for linting)
- Docker (optional, for container builds)

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/metricsd.git
   cd metricsd
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/0x524A/metricsd.git
   ```

## Development Workflow

Follow this process for contributing:

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** and commit them with clear, descriptive messages:
   ```bash
   git commit -m 'Add your feature description'
   ```

3. **Add tests** for new functionality to maintain code coverage

4. **Run tests locally**:
   ```bash
   make test
   ```

5. **Format your code**:
   ```bash
   make fmt
   ```

6. **Run the linter**:
   ```bash
   make lint
   ```

7. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

8. **Open a Pull Request** on GitHub with a clear title and description

## Available Make Targets

The following commands are available via the Makefile:

| Target | Description |
|--------|-------------|
| `make build` | Build the application binary |
| `make run` | Run the application with default config |
| `make clean` | Remove build artifacts |
| `make test` | Run tests with verbose output |
| `make deps` | Download and tidy dependencies |
| `make fmt` | Format code using go fmt |
| `make vet` | Run go vet static analysis |
| `make lint` | Run golangci-lint (must be installed) |
| `make install` | Install the binary to GOPATH/bin |
| `make docker-build` | Build Docker image |
| `make build-linux-amd64` | Build binary for Linux amd64 |
| `make build-linux-arm64` | Build binary for Linux arm64 |
| `make build-all-linux` | Build for all supported Linux architectures |
| `make package-deb` | Build Debian packages for all architectures |

Run `make help` to see all available targets.

## Contribution Guidelines

- **Follow Go best practices** and idioms as outlined in [Effective Go](https://golang.org/doc/effective_go)
- **Maintain SOLID design principles** for clean, maintainable code
- **Add tests** for all new functionality to keep coverage high
- **Update documentation** as needed (README, inline comments, etc.)
- **Keep commits atomic** with clear, descriptive commit messages
- **Ensure backward compatibility** when possible
- **Run tests and linters** before submitting a pull request

### Code Style

- Code is formatted using `gofmt` (run `make fmt`)
- Linting is performed using golangci-lint (run `make lint`)
- Follow the conventions used in the existing codebase
- Write clear variable and function names
- Add comments for non-obvious logic

### Testing

- Write unit tests for new features
- Aim for high test coverage (currently maintained at 80%+)
- Run `make test` to execute the full test suite
- Include integration tests when appropriate

### Commit Messages

- Use clear, descriptive commit messages
- Start with a verb (Add, Fix, Update, etc.)
- Keep the first line under 50 characters
- Add a longer description if needed, separated by a blank line
- Reference related issues when applicable (e.g., "Fixes #123")

## Pull Request Process

1. Ensure your code passes all tests and linting checks
2. Update relevant documentation
3. Add an entry to the CHANGELOG if appropriate
4. Provide a clear description of the changes in your PR
5. Link related issues using GitHub's issue linking syntax
6. Be responsive to review feedback and questions

## Questions or Need Help?

- **Report Issues**: [GitHub Issues](https://github.com/0x524A/metricsd/issues)
- **Discussions**: [GitHub Discussions](https://github.com/0x524A/metricsd/discussions)
- **Documentation**: See the main [README](README.md) and inline code comments

## Recognition

All contributions are appreciated and acknowledged. We maintain a list of [contributors](https://github.com/0x524A/metricsd/contributors) on GitHub.

Thank you for helping make metricsd better!
