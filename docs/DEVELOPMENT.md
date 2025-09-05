# Development Guide

This guide covers the development environment setup, code quality standards, and troubleshooting for the sermon-uploader project.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Development Environment](#development-environment)
3. [Code Quality Standards](#code-quality-standards)
4. [Pre-commit Hooks](#pre-commit-hooks)
5. [IDE Setup](#ide-setup)
6. [Troubleshooting](#troubleshooting)
7. [Performance Tips](#performance-tips)

## Quick Start

1. **Clone and setup:**
   ```bash
   git clone https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git
   cd sermon-uploader
   ./scripts/setup-pre-commit.sh
   ```

2. **Start development:**
   ```bash
   # Backend
   cd backend && go run .
   
   # Frontend (in another terminal)
   cd frontend && npm run dev
   ```

3. **Make your first commit:**
   ```bash
   git add .
   git commit -m "feat: initial development setup"
   ```

## Development Environment

### System Requirements

- **Go**: 1.21 or later
- **Node.js**: 18 or later
- **Python**: 3.8 or later (for pre-commit tools)
- **Git**: Latest version
- **Docker**: Latest version (optional, for containerized development)

### Environment Setup

The `setup-pre-commit.sh` script installs all necessary tools:

- **Pre-commit framework**: Manages Git hooks
- **Go tools**: golangci-lint, gosec, goimports, gocyclo
- **Frontend tools**: ESLint, Prettier, TypeScript, Jest
- **Security tools**: detect-secrets, Trivy

## Code Quality Standards

### Go Backend Standards

- **Formatting**: All Go code must be formatted with `gofmt`
- **Linting**: Must pass `golangci-lint` with our configuration
- **Testing**: Minimum 70% code coverage
- **Security**: Must pass `gosec` security scanning
- **Documentation**: All exported functions must have comments

#### Go Style Guidelines

```go
// Good: Clear, documented function
// ProcessUpload handles file upload validation and processing
func ProcessUpload(file *multipart.FileHeader) error {
    if file.Size > MaxFileSize {
        return ErrFileTooLarge
    }
    // ... processing logic
    return nil
}

// Bad: Undocumented, unclear naming
func proc(f *multipart.FileHeader) error {
    // ... logic
}
```

### Frontend Standards

- **Framework**: Next.js 14 with App Router
- **Language**: TypeScript (strict mode)
- **Styling**: Tailwind CSS with consistent patterns
- **Testing**: Jest + Testing Library (minimum 70% coverage)
- **Accessibility**: WCAG 2.1 AA compliance

#### TypeScript Guidelines

```typescript
// Good: Proper typing and error handling
interface UploadResponse {
  success: boolean;
  fileId?: string;
  error?: string;
}

const uploadFile = async (file: File): Promise<UploadResponse> => {
  try {
    const response = await fetch('/api/upload', {
      method: 'POST',
      body: formData,
    });
    return await response.json();
  } catch (error) {
    return { success: false, error: 'Upload failed' };
  }
};

// Bad: Any types, poor error handling
const uploadFile = (file: any) => {
  fetch('/api/upload', { method: 'POST', body: file });
};
```

### Commit Standards

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `build`: Build system changes
- `ci`: CI/CD changes
- `chore`: Maintenance tasks

**Examples:**
```bash
feat(upload): add drag-and-drop file upload
fix(api): handle timeout errors properly
docs: update API documentation
test(upload): add integration tests for file validation
```

## Pre-commit Hooks

### Hook Configuration

Our pre-commit hooks are configured in `.pre-commit-config.yaml`:

1. **File validation**: Trailing whitespace, file size, merge conflicts
2. **Security**: Secret detection, private key scanning
3. **Go hooks**: fmt, vet, lint, test, build, security scan
4. **Frontend hooks**: ESLint, Prettier, TypeScript, tests
5. **Repository hooks**: Branch naming, commit messages, documentation

### Running Hooks

```bash
# Run all hooks on staged files (automatic on commit)
pre-commit run

# Run all hooks on all files
pre-commit run --all-files

# Run specific hook
pre-commit run eslint

# Skip hooks (use sparingly)
git commit --no-verify -m "hotfix: emergency fix"
```

### Hook Performance

- **First run**: 30-60 seconds (downloads and caches tools)
- **Subsequent runs**: 5-15 seconds
- **Changed files only**: Hooks run only on modified files
- **Parallel execution**: Multiple hooks run concurrently

## IDE Setup

### VS Code (Recommended)

Install extensions:
```bash
code --install-extension golang.go
code --install-extension bradlc.vscode-tailwindcss
code --install-extension esbenp.prettier-vscode
code --install-extension dbaeumer.vscode-eslint
code --install-extension ms-vscode.vscode-typescript-next
```

**Settings (`.vscode/settings.json`):**
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "source.fixAll.eslint": true,
    "source.organizeImports": true
  },
  "typescript.preferences.importModuleSpecifier": "relative"
}
```

### GoLand/IntelliJ

1. **Enable golangci-lint**: Settings > Tools > Go Linter
2. **Configure ESLint**: Settings > Languages > JavaScript > Code Quality Tools > ESLint
3. **Setup Prettier**: Settings > Languages > JavaScript > Prettier
4. **Enable format on save**: Settings > Tools > Actions on Save

## Troubleshooting

### Common Issues

#### Pre-commit Hook Failures

**Problem**: `golangci-lint: command not found`
```bash
# Solution: Install Go tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
# Add $GOPATH/bin to your PATH
```

**Problem**: `npm ci` fails in pre-commit
```bash
# Solution: Install dependencies first
cd frontend && npm ci
```

**Problem**: TypeScript errors in pre-commit
```bash
# Solution: Fix type errors or update tsconfig.json
cd frontend && npx tsc --noEmit
```

#### Git Hook Issues

**Problem**: Hooks not running
```bash
# Solution: Reinstall pre-commit hooks
pre-commit uninstall
pre-commit install
```

**Problem**: Hooks running on unchanged files
```bash
# Solution: Clear pre-commit cache
pre-commit clean
```

### Debug Mode

Enable verbose output:
```bash
# Debug pre-commit
pre-commit run --verbose --all-files

# Debug specific hook
pre-commit run --verbose eslint

# Show configuration
pre-commit --version
pre-commit sample-config
```

### Performance Issues

**Slow hook execution:**
1. Check network connection (tools download dependencies)
2. Clear caches: `pre-commit clean`
3. Update tools: `pre-commit autoupdate`
4. Use `--parallel` flag for multi-core systems

**Large repository issues:**
1. Use `.gitignore` to exclude unnecessary files
2. Run hooks only on changed files (default behavior)
3. Consider using `files` or `exclude` in hook configuration

## Performance Tips

### Development Workflow

1. **Use fast feedback loops:**
   ```bash
   # Quick type check
   cd frontend && npm run type-check
   
   # Quick lint
   cd frontend && npm run lint
   
   # Quick Go check
   cd backend && go vet ./...
   ```

2. **IDE integration:** Let your IDE run checks continuously instead of waiting for commits

3. **Partial commits:** Stage and commit related changes together
   ```bash
   git add frontend/components/Upload.tsx
   git commit -m "feat(upload): add file validation"
   ```

### CI/CD Optimization

1. **Cache dependencies:** GitHub Actions caches are configured
2. **Parallel jobs:** Tests run concurrently with linting
3. **Skip redundant checks:** Hooks skip if no relevant files changed

### Local Development

1. **Use watch mode for tests:**
   ```bash
   cd frontend && npm run test:watch
   cd backend && find . -name "*.go" | entr -r go test ./...
   ```

2. **Pre-commit on demand:** Use `pre-commit run` during development, not just on commits

## Advanced Configuration

### Custom Hook Development

Create local hooks in `.pre-commit-config.yaml`:

```yaml
- repo: local
  hooks:
    - id: custom-validation
      name: Custom validation
      entry: scripts/custom-validation.sh
      language: system
      files: \.(go|ts|tsx)$
```

### Hook Customization

Modify existing hooks:

```yaml
- repo: https://github.com/golangci/golangci-lint
  rev: v1.54.2
  hooks:
    - id: golangci-lint
      args: [--timeout=10m, --config=.golangci.yml]
      files: \.go$
```

### Environment-specific Configuration

Use different configurations for different environments:

```bash
# Development (fast)
pre-commit run --config .pre-commit-dev.yaml

# CI (comprehensive)
pre-commit run --config .pre-commit-ci.yaml
```

## Getting Help

1. **Documentation**: Check this guide and inline code comments
2. **Issues**: Search existing GitHub issues
3. **Logs**: Run commands with `--verbose` flag
4. **Community**: Ask in project Discord or create GitHub issue

Remember: The goal is to maintain high code quality while keeping the development experience smooth and efficient.