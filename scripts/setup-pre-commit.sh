#!/bin/bash

# Setup script for pre-commit hooks and development tools
# This script installs and configures all necessary tools for the sermon-uploader project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "\n${BOLD}${BLUE}=== $1 ===${NC}\n"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check version
check_version() {
    local cmd="$1"
    local min_version="$2"
    local current_version
    
    case $cmd in
        "node")
            current_version=$(node --version | sed 's/v//')
            ;;
        "go")
            current_version=$(go version | awk '{print $3}' | sed 's/go//')
            ;;
        "python")
            current_version=$(python3 --version | awk '{print $2}')
            ;;
        *)
            return 0
            ;;
    esac
    
    if command_exists sort && [ "$(printf '%s\n' "$min_version" "$current_version" | sort -V | head -n1)" = "$min_version" ]; then
        return 0
    else
        return 1
    fi
}

# Check if we're in the right directory
if [ ! -f ".pre-commit-config.yaml" ] || [ ! -f "backend/go.mod" ] || [ ! -f "frontend/package.json" ]; then
    print_error "This script must be run from the project root directory"
    print_error "Make sure you're in the sermon-uploader directory"
    exit 1
fi

print_header "Sermon Uploader Pre-commit Setup"
print_status "Setting up development environment..."

# Check system requirements
print_header "Checking System Requirements"

# Check Python
if command_exists python3; then
    if check_version python 3.8; then
        print_success "Python $(python3 --version | awk '{print $2}') found"
    else
        print_warning "Python version is older than 3.8, some tools may not work properly"
    fi
else
    print_error "Python 3 is required but not found"
    print_error "Please install Python 3.8 or later"
    exit 1
fi

# Check Node.js
if command_exists node; then
    if check_version node 18.0; then
        print_success "Node.js $(node --version) found"
    else
        print_warning "Node.js version is older than 18.0, consider upgrading"
    fi
else
    print_error "Node.js is required but not found"
    print_error "Please install Node.js 18 or later"
    exit 1
fi

# Check npm
if command_exists npm; then
    print_success "npm $(npm --version) found"
else
    print_error "npm is required but not found"
    exit 1
fi

# Check Go
if command_exists go; then
    if check_version go 1.20; then
        print_success "Go $(go version | awk '{print $3}') found"
    else
        print_warning "Go version is older than 1.20, consider upgrading"
    fi
else
    print_error "Go is required but not found"
    print_error "Please install Go 1.20 or later"
    exit 1
fi

# Check Git
if command_exists git; then
    print_success "Git $(git --version | awk '{print $3}') found"
else
    print_error "Git is required but not found"
    exit 1
fi

# Install Python tools
print_header "Installing Python Tools"

# Install pre-commit
if command_exists pre-commit; then
    print_success "pre-commit already installed"
else
    print_status "Installing pre-commit..."
    if command_exists pip3; then
        pip3 install --user pre-commit
    else
        python3 -m pip install --user pre-commit
    fi
    print_success "pre-commit installed"
fi

# Install detect-secrets
if command_exists detect-secrets; then
    print_success "detect-secrets already installed"
else
    print_status "Installing detect-secrets..."
    if command_exists pip3; then
        pip3 install --user detect-secrets
    else
        python3 -m pip install --user detect-secrets
    fi
    print_success "detect-secrets installed"
fi

# Install Go tools
print_header "Installing Go Tools"

print_status "Installing golangci-lint..."
if command_exists golangci-lint; then
    print_success "golangci-lint already installed"
else
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    print_success "golangci-lint installed"
fi

print_status "Installing gosec..."
if command_exists gosec; then
    print_success "gosec already installed"
else
    go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
    print_success "gosec installed"
fi

print_status "Installing other Go tools..."
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# Install frontend dependencies
print_header "Installing Frontend Dependencies"
print_status "Installing npm dependencies..."
cd frontend
npm ci --prefer-offline --no-audit
cd ..
print_success "Frontend dependencies installed"

# Setup pre-commit
print_header "Setting up Pre-commit Hooks"

print_status "Installing pre-commit hooks..."
pre-commit install
pre-commit install --hook-type commit-msg
pre-commit install --hook-type pre-push
print_success "Pre-commit hooks installed"

# Generate secrets baseline
print_status "Generating secrets baseline..."
if [ ! -f ".secrets.baseline" ]; then
    detect-secrets scan --baseline .secrets.baseline
    print_success "Secrets baseline generated"
else
    print_warning "Secrets baseline already exists, skipping generation"
fi

# Configure Git settings
print_header "Configuring Git Settings"

# Set commit message template
if [ -f ".gitmessage" ]; then
    git config commit.template .gitmessage
    print_success "Git commit message template configured"
fi

# Set up Git aliases for common tasks
git config alias.precommit "!pre-commit run --all-files"
git config alias.lint "!cd frontend && npm run lint:fix && cd ../backend && golangci-lint run --fix"
git config alias.test "!cd frontend && npm test && cd ../backend && go test ./..."
git config alias.format "!cd frontend && npm run format && cd ../backend && gofmt -w ."

print_success "Git aliases configured:"
print_status "  git precommit  - Run all pre-commit hooks"
print_status "  git lint       - Run linting with auto-fix"
print_status "  git test       - Run all tests"
print_status "  git format     - Format all code"

# Make scripts executable
print_header "Setting up Scripts"
find scripts -name "*.sh" -exec chmod +x {} \;
print_success "Scripts made executable"

# Test installation
print_header "Testing Installation"

print_status "Testing pre-commit installation..."
if pre-commit --version >/dev/null 2>&1; then
    print_success "Pre-commit is working"
else
    print_error "Pre-commit test failed"
    exit 1
fi

print_status "Testing Go tools..."
if golangci-lint --version >/dev/null 2>&1; then
    print_success "golangci-lint is working"
else
    print_warning "golangci-lint test failed"
fi

print_status "Testing frontend tools..."
cd frontend
if npx eslint --version >/dev/null 2>&1; then
    print_success "ESLint is working"
else
    print_warning "ESLint test failed"
fi

if npx prettier --version >/dev/null 2>&1; then
    print_success "Prettier is working"
else
    print_warning "Prettier test failed"
fi

if npx tsc --version >/dev/null 2>&1; then
    print_success "TypeScript is working"
else
    print_warning "TypeScript test failed"
fi
cd ..

# Performance optimizations
print_header "Performance Optimizations"

print_status "Setting up caching directories..."
mkdir -p ~/.cache/pre-commit
mkdir -p ~/.cache/go-build
mkdir -p ~/.cache/golangci-lint

# Setup IDE integration hints
print_header "IDE Integration Recommendations"

print_status "For optimal development experience, configure your IDE with:"
echo "  VS Code:"
echo "    - Install ESLint extension"
echo "    - Install Prettier extension"
echo "    - Install Go extension"
echo "    - Enable format on save"
echo ""
echo "  GoLand/IntelliJ:"
echo "    - Enable golangci-lint integration"
echo "    - Configure ESLint for frontend files"
echo "    - Enable Prettier for TypeScript/JavaScript"
echo ""

print_header "Setup Complete!"
print_success "Pre-commit hooks and development tools are now configured"
print_status "You can now:"
echo "  1. Make changes to your code"
echo "  2. Stage files with 'git add'"
echo "  3. Commit with 'git commit' (hooks will run automatically)"
echo "  4. Run 'pre-commit run --all-files' to test all hooks"
echo "  5. Use the Git aliases: git precommit, git lint, git test, git format"
echo ""
print_warning "Note: First run may be slow as tools cache dependencies"
print_warning "Subsequent runs will be much faster"
echo ""
print_status "For troubleshooting, see: docs/DEVELOPMENT.md"
print_success "Happy coding! 🚀"