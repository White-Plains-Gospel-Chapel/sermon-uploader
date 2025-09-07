#!/bin/bash
# Setup script for Go linting configuration
# Installs golangci-lint and configures git hooks

set -e

echo "🔧 Setting up Go linting configuration..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "windows"
    else
        echo "unknown"
    fi
}

OS=$(detect_os)
echo "📱 Detected OS: $OS"

# Install golangci-lint
install_golangci_lint() {
    if command_exists golangci-lint; then
        echo "✅ golangci-lint is already installed"
        golangci-lint version
        return 0
    fi

    echo "📦 Installing golangci-lint..."
    
    case $OS in
        "macos")
            if command_exists brew; then
                brew install golangci-lint
            else
                echo "⚠️  Homebrew not found. Installing using Go..."
                go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
            fi
            ;;
        "linux")
            # Install using the official installer
            curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
            ;;
        "windows")
            echo "⚠️  Please install golangci-lint manually on Windows:"
            echo "   Visit: https://golangci-lint.run/usage/install/"
            ;;
        *)
            echo "⚠️  Unknown OS. Installing using Go..."
            go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
            ;;
    esac
    
    if command_exists golangci-lint; then
        echo "✅ golangci-lint installed successfully"
        golangci-lint version
    else
        echo -e "${RED}❌ Failed to install golangci-lint${NC}"
        exit 1
    fi
}

# Install additional development tools
install_dev_tools() {
    echo "🛠️  Installing additional development tools..."
    
    tools=(
        "golang.org/x/tools/cmd/goimports@latest"
        "mvdan.cc/gofumpt@latest"
        "github.com/kisielk/errcheck@latest"
        "github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
        "golang.org/x/vuln/cmd/govulncheck@latest"
    )
    
    for tool in "${tools[@]}"; do
        echo "  📦 Installing $tool..."
        go install "$tool"
    done
    
    echo "✅ Development tools installed"
}

# Configure Git hooks
configure_git_hooks() {
    echo "🪝 Configuring Git hooks..."
    
    # Check if we're in a git repository
    if ! git rev-parse --git-dir >/dev/null 2>&1; then
        echo -e "${RED}❌ Not in a Git repository${NC}"
        return 1
    fi
    
    # Set hooks path
    git config core.hooksPath .githooks
    
    # Make hooks executable
    chmod +x .githooks/*
    
    echo "✅ Git hooks configured"
}

# Validate configuration
validate_config() {
    echo "🔍 Validating linting configuration..."
    
    if [ ! -f ".golangci.yml" ]; then
        echo -e "${RED}❌ .golangci.yml not found${NC}"
        exit 1
    fi
    
    # Test configuration with a simple check
    if command_exists golangci-lint; then
        echo "  🧪 Testing configuration..."
        if golangci-lint config verify --config .golangci.yml; then
            echo "✅ Configuration is valid"
        else
            echo -e "${RED}❌ Configuration validation failed${NC}"
            exit 1
        fi
    fi
}

# Run initial lint check
run_initial_lint() {
    echo "🔍 Running initial lint check..."
    
    if command_exists golangci-lint; then
        echo "  📊 Running golangci-lint (this may take a while)..."
        if golangci-lint run --config .golangci.yml --timeout=5m; then
            echo "✅ Linting passed!"
        else
            echo -e "${YELLOW}⚠️  Linting found issues. Run 'make lint-fix' to auto-fix some issues.${NC}"
            echo "📚 See .golangci.md for configuration details"
        fi
    fi
}

# Main installation flow
main() {
    echo "🚀 Starting Go linting setup..."
    echo ""
    
    # Check prerequisites
    if ! command_exists go; then
        echo -e "${RED}❌ Go is not installed. Please install Go first.${NC}"
        exit 1
    fi
    
    echo "✅ Go version: $(go version)"
    echo ""
    
    # Install tools
    install_golangci_lint
    echo ""
    
    install_dev_tools
    echo ""
    
    # Configure
    configure_git_hooks
    echo ""
    
    validate_config
    echo ""
    
    # Optional: Run initial check
    echo -n "🤔 Would you like to run an initial lint check? (y/N): "
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        run_initial_lint
    fi
    
    echo ""
    echo "🎉 Setup complete!"
    echo ""
    echo -e "${GREEN}Next steps:${NC}"
    echo "  • Run 'make lint' to check code quality"
    echo "  • Run 'make lint-fix' to auto-fix issues"
    echo "  • Run 'make pre-commit' for full pre-commit checks"
    echo "  • See .golangci.md for detailed configuration info"
    echo ""
    echo -e "${GREEN}Git hooks are now configured:${NC}"
    echo "  • Pre-commit: Runs linting automatically on commit"
    echo "  • Pre-push: Additional checks before push"
    echo ""
    echo "🔧 Available make targets:"
    echo "  make lint           - Run linting"
    echo "  make lint-fix       - Auto-fix issues"
    echo "  make security       - Security scan"
    echo "  make test-cover     - Test with coverage"
    echo "  make ci             - Full CI pipeline"
}

# Run main function
main "$@"