# Makefile Documentation

This Makefile provides a comprehensive set of development workflows for the Sermon Uploader Go backend, with a strong emphasis on Test-Driven Development (TDD).

## Key Features

- **TDD Enforcement**: All build targets automatically run tests first
- **Multi-platform Builds**: Support for macOS, Linux, and Raspberry Pi
- **Security Scanning**: Integrated security tools (gosec, nancy)
- **Hot Reload Development**: Automatic rebuild on file changes
- **Git Hooks**: Pre-commit and pre-push automation
- **Docker Integration**: Build, run, and manage Docker containers
- **Code Quality**: Linting, formatting, and race detection

## Quick Start

```bash
# Display all available commands
make help

# Start development server with hot reload
make dev

# Run all tests (enforces TDD)
make test

# Build application (tests run first)
make build

# Install development tools
make install-deps
```

## TDD Workflow

The Makefile enforces Test-Driven Development by:

1. **Test-First Building**: All `build*` targets depend on the `test` target
2. **Pre-commit Hooks**: Automatically run tests, linting, and security scans
3. **Pre-push Hooks**: Run comprehensive test coverage and security checks
4. **CI Pipeline**: Complete validation before deployment

```bash
# This sequence enforces TDD
make test          # Write and run tests first
make build         # Build only after tests pass
make deploy        # Deploy only after build succeeds
```

## Test Targets

| Command | Description |
|---------|-------------|
| `make test` | Run all tests (unit + integration) |
| `make test-unit` | Run unit tests only (faster feedback) |
| `make test-integration` | Run integration tests |
| `make test-coverage` | Generate HTML coverage report |
| `make test-race` | Run tests with race condition detection |
| `make test-watch` | Watch files and auto-run tests |
| `make benchmark` | Run performance benchmarks |

## Build Targets

All build targets automatically run tests first to enforce TDD:

| Command | Description |
|---------|-------------|
| `make build` | Build for current platform |
| `make build-linux` | Build for Linux amd64 |
| `make build-darwin` | Build for macOS |
| `make build-pi` | Build for Raspberry Pi ARM64 |
| `make build-all` | Build for all platforms |

## Development Targets

| Command | Description |
|---------|-------------|
| `make dev` | Start development server with hot reload |
| `make watch` | Alias for `dev` |
| `make run` | Run the built application |
| `make debug` | Run with race detection and debug mode |
| `make profile` | Run with profiling enabled |

## Code Quality Targets

| Command | Description |
|---------|-------------|
| `make lint` | Run all linting checks |
| `make lint-fix` | Auto-fix linting issues |
| `make format` | Format code (gofmt + goimports) |

## Security Targets

| Command | Description |
|---------|-------------|
| `make security-scan` | Run gosec security scanner |
| `make vuln-check` | Check for vulnerabilities with nancy |
| `make security-all` | Run all security checks |

## Docker Targets

| Command | Description |
|---------|-------------|
| `make docker-build` | Build Docker image (tests run first) |
| `make docker-run` | Run Docker container |
| `make docker-push` | Push image to registry |
| `make docker-clean` | Clean Docker artifacts |

## Git Integration

### Setup Hooks
```bash
make setup-hooks
```

This installs:
- **Pre-commit hook**: Runs `format`, `lint`, `test`, `security-scan`
- **Pre-push hook**: Runs `test-coverage`, `security-all`

### Pre-commit Workflow
```bash
# Manual pre-commit check
make pre-commit
```

## Release & CI

| Command | Description |
|---------|-------------|
| `make release` | Complete release preparation |
| `make ci` | Run CI pipeline checks |

## Utility Targets

| Command | Description |
|---------|-------------|
| `make clean` | Remove all build artifacts |
| `make install-deps` | Install required development tools |
| `make deps-update` | Update Go dependencies |
| `make deps-tidy` | Clean up dependencies |
| `make version` | Show version information |
| `make env` | Show environment information |
| `make tools` | List installed tools |

## Dependencies

The Makefile automatically installs these tools via `make install-deps`:

- **golangci-lint**: Comprehensive linting
- **gosec**: Security scanning
- **nancy**: Vulnerability checking
- **goimports**: Import formatting
- **air**: Hot reload for development

## Configuration

Key variables you can override:

```bash
# Override timeout values
make test TEST_TIMEOUT=5m

# Override build configuration
make build CGO_ENABLED=1

# Override Docker settings
make docker-build DOCKER_TAG=v1.0.0
```

## Examples

### Typical Development Workflow

```bash
# 1. Install tools (first time only)
make install-deps

# 2. Setup git hooks (first time only)
make setup-hooks

# 3. Start development
make dev

# 4. Run tests continuously in another terminal
make test-watch

# 5. Before committing (automatic via hooks)
make pre-commit

# 6. Build for production
make build-all
```

### CI/CD Pipeline

```bash
# Complete CI pipeline
make ci

# Release preparation
make release
```

### Docker Development

```bash
# Build and run in Docker
make docker-build
make docker-run

# Clean up Docker resources
make docker-clean
```

## TDD Best Practices

1. **Write tests first**: Use `make test-watch` for instant feedback
2. **Commit frequently**: Pre-commit hooks ensure code quality
3. **Build only after tests pass**: The Makefile enforces this
4. **Run security scans**: Integrated into the development workflow
5. **Use coverage reports**: `make test-coverage` generates HTML reports

## Troubleshooting

### Tests Failing
```bash
# Run specific test package
go test -v ./services

# Run with detailed output
make test-race
```

### Build Issues
```bash
# Clean and rebuild
make clean
make build
```

### Missing Tools
```bash
# Reinstall development tools
make install-deps
```

### Docker Issues
```bash
# Clean Docker environment
make docker-clean
```

This Makefile enforces best practices and ensures that code quality, security, and tests are never overlooked in the development process.