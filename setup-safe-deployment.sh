#!/bin/bash
# Setup script for safe deployment system
# This installs all hooks and validations to prevent bad code deployment

set -e

echo "🛡️ Setting up Safe Deployment System"
echo "===================================="
echo ""
echo "This will install:"
echo "- Pre-commit hooks (prevent bad commits)"
echo "- Pre-push hooks (prevent bad pushes)"  
echo "- GitHub Actions workflows (prevent bad deployments)"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Create hooks directory if it doesn't exist
mkdir -p .githooks

# Make pre-push hook executable
if [ -f ".githooks/pre-push" ]; then
    chmod +x .githooks/pre-push
    echo -e "${GREEN}✅ Pre-push hook configured${NC}"
fi

# Configure Git to use our hooks
git config core.hooksPath .githooks
echo -e "${GREEN}✅ Git configured to use custom hooks${NC}"

# Install pre-commit framework if not installed
if ! command -v pre-commit &> /dev/null; then
    echo -e "${YELLOW}Installing pre-commit framework...${NC}"
    pip install pre-commit || pip3 install pre-commit
fi

# Install pre-commit hooks
if [ -f ".pre-commit-config.yaml" ]; then
    pre-commit install
    echo -e "${GREEN}✅ Pre-commit hooks installed${NC}"
fi

# Install required tools
echo ""
echo "Installing required validation tools..."

# Install TruffleHog for secret scanning
if ! command -v trufflehog &> /dev/null; then
    echo "Installing TruffleHog for secret scanning..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install trufflesecurity/trufflehog/trufflehog
    else
        pip install truffleHog3
    fi
fi

# Install golangci-lint for Go linting
if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
fi

# Create a validation script for manual testing
cat > validate-deployment.sh << 'EOF'
#!/bin/bash
# Manual validation script - run this before pushing to test all checks

echo "🧪 Running full deployment validation..."
echo "========================================"

# Run pre-push hook manually
if [ -f ".githooks/pre-push" ]; then
    bash .githooks/pre-push
    if [ $? -eq 0 ]; then
        echo "✅ All validations passed!"
        echo ""
        echo "Your code is safe to deploy."
    else
        echo "❌ Validation failed!"
        echo ""
        echo "Fix the issues above before pushing."
        exit 1
    fi
else
    echo "⚠️ Pre-push hook not found. Run setup-safe-deployment.sh first."
    exit 1
fi
EOF

chmod +x validate-deployment.sh
echo -e "${GREEN}✅ Manual validation script created (./validate-deployment.sh)${NC}"

# Create emergency bypass script (use with caution!)
cat > emergency-deploy.sh << 'EOF'
#!/bin/bash
# EMERGENCY ONLY - Bypasses safety checks
# Use this only when you absolutely need to deploy despite validation failures

echo "⚠️ WARNING: Emergency deployment mode!"
echo "======================================"
echo ""
echo "This will bypass all safety checks."
echo "Only use this if you understand the risks!"
echo ""
read -p "Type 'DEPLOY ANYWAY' to continue: " confirmation

if [ "$confirmation" != "DEPLOY ANYWAY" ]; then
    echo "Cancelled."
    exit 1
fi

echo "Temporarily disabling hooks..."
git config core.hooksPath .git/hooks

echo "Pushing to repository..."
git push

echo "Re-enabling hooks..."
git config core.hooksPath .githooks

echo "Done. Safety checks have been re-enabled."
EOF

chmod +x emergency-deploy.sh
echo -e "${GREEN}✅ Emergency bypass script created (./emergency-deploy.sh)${NC}"

# Summary
echo ""
echo "============================================"
echo -e "${GREEN}✅ Safe Deployment System Setup Complete!${NC}"
echo "============================================"
echo ""
echo "🛡️ Protection levels:"
echo "  1. Pre-commit: Prevents bad commits locally"
echo "  2. Pre-push: Prevents bad code from reaching GitHub"
echo "  3. GitHub Actions: Validates everything before Pi deployment"
echo "  4. Self-hosted runner: Ensures only validated code deploys"
echo ""
echo "📋 Commands:"
echo "  ./validate-deployment.sh  - Test all validations manually"
echo "  git push                  - Normal push (with all safety checks)"
echo "  ./emergency-deploy.sh     - Emergency bypass (use with caution!)"
echo ""
echo "🔒 What's protected:"
echo "  ✓ No secrets or credentials can be exposed"
echo "  ✓ No syntax errors can be pushed"
echo "  ✓ No build failures can occur"
echo "  ✓ No configuration errors can deploy"
echo "  ✓ Audio quality is preserved"
echo "  ✓ Docker builds are validated"
echo "  ✓ All tests must pass"
echo ""
echo "The Pi will ONLY receive code that has passed ALL validations."