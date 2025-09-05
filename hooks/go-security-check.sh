#!/bin/bash
# Go Security and Dependency Validation for Raspberry Pi
# This script validates Go dependencies and security practices

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Security and Dependency Check (Pi Optimized) ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Function to log security issue
log_issue() {
    local issue="$1"
    local severity="$2"
    local details="${3:-}"
    
    if [[ "$severity" == "ERROR" ]]; then
        echo -e "${RED}ERROR${NC}: $issue"
        [[ -n "$details" ]] && echo -e "  ${BLUE}🔒 Details${NC}: $details"
        ((ISSUES_FOUND++))
    else
        echo -e "${YELLOW}WARNING${NC}: $issue"
        [[ -n "$details" ]] && echo -e "  ${BLUE}🔒 Details${NC}: $details"
        ((WARNINGS_FOUND++))
    fi
}

# Change to backend directory
cd "/Users/gaius/Documents/WPGC web/sermon-uploader/backend" || {
    echo -e "${RED}Error: Could not change to backend directory${NC}"
    exit 1
}

# Check if go.mod exists
if [[ ! -f "go.mod" ]]; then
    echo -e "${RED}Error: go.mod file not found${NC}"
    exit 1
fi

echo "Checking Go module security and dependencies..."

# 1. Check for dependency vulnerabilities using govulncheck
echo -e "\n${BLUE}=== Vulnerability Scan ===${NC}"
if command -v govulncheck &> /dev/null; then
    echo "Running govulncheck..."
    if ! govulncheck ./...; then
        log_issue "Vulnerability scanner found security issues" "ERROR" \
            "Critical for Pi deployment security"
    else
        echo -e "${GREEN}✅ No known vulnerabilities found${NC}"
    fi
else
    echo -e "${YELLOW}govulncheck not found, installing...${NC}"
    if go install golang.org/x/vuln/cmd/govulncheck@latest; then
        if ! "$HOME/go/bin/govulncheck" ./...; then
            log_issue "Vulnerability scanner found security issues" "ERROR" \
                "Critical for Pi deployment security"
        fi
    else
        log_issue "Could not install govulncheck" "WARNING" \
            "Manual vulnerability checking required"
    fi
fi

# 2. Check dependency versions and update status
echo -e "\n${BLUE}=== Dependency Analysis ===${NC}"
outdated_deps=$(go list -u -m all 2>/dev/null | grep "\[" || echo "")
if [[ -n "$outdated_deps" ]]; then
    echo "Outdated dependencies found:"
    echo "$outdated_deps"
    log_issue "Outdated dependencies detected" "WARNING" \
        "Consider updating for security patches"
else
    echo -e "${GREEN}✅ All dependencies are up to date${NC}"
fi

# 3. Check for insecure dependencies
echo -e "\n${BLUE}=== Insecure Dependency Patterns ===${NC}"

# Known problematic or deprecated packages
insecure_patterns=(
    "github.com/pkg/errors"  # Deprecated in favor of standard errors
    "gopkg.in/yaml.v2"       # v3 is available with security fixes
    "github.com/dgrijalva/jwt-go"  # Has known vulnerabilities
    "crypto/md5"             # Cryptographically broken
    "crypto/sha1"            # Weak for cryptographic use
)

for pattern in "${insecure_patterns[@]}"; do
    if grep -r "$pattern" go.mod go.sum 2>/dev/null; then
        log_issue "Potentially insecure dependency: $pattern" "WARNING" \
            "Consider updating to more secure alternatives"
    fi
done

# 4. Check go.sum integrity
echo -e "\n${BLUE}=== Module Integrity Check ===${NC}"
if [[ -f "go.sum" ]]; then
    if go mod verify; then
        echo -e "${GREEN}✅ Module checksums verified${NC}"
    else
        log_issue "Module integrity verification failed" "ERROR" \
            "go.sum checksums do not match downloaded modules"
    fi
else
    log_issue "go.sum file missing" "ERROR" \
        "Module integrity cannot be verified"
fi

# 5. Check for direct vs indirect dependencies
echo -e "\n${BLUE}=== Dependency Tree Analysis ===${NC}"
direct_deps=$(go list -m all | grep -v "$(go list -m)" | grep -v "=>" | wc -l)
total_deps=$(go list -m all | wc -l)

echo "Direct dependencies: $direct_deps"
echo "Total dependencies: $total_deps"

if [[ $total_deps -gt 100 ]]; then
    log_issue "Large dependency tree ($total_deps dependencies)" "WARNING" \
        "Large trees increase attack surface and Pi binary size"
fi

# 6. Check for replace directives in go.mod
if grep -q "replace " go.mod; then
    echo -e "\n${BLUE}=== Replace Directives Found ===${NC}"
    replace_count=$(grep -c "replace " go.mod)
    grep "replace " go.mod
    log_issue "$replace_count replace directives found" "WARNING" \
        "Verify security of replaced modules"
fi

# 7. Check for retract directives
if grep -q "retract " go.mod; then
    echo -e "\n${BLUE}=== Retract Directives Found ===${NC}"
    retract_count=$(grep -c "retract " go.mod)
    grep "retract " go.mod
    echo -e "${GREEN}Found $retract_count retract directives (good practice)${NC}"
fi

# 8. Analyze dependency licenses (Pi deployment consideration)
echo -e "\n${BLUE}=== License Analysis ===${NC}"
go list -m all | while read -r module version; do
    if [[ "$module" != "$(go list -m)" ]]; then
        # This is a simplified check - in production, use a proper license scanner
        if [[ "$module" =~ gopkg.in|github.com ]]; then
            echo "Dependency: $module $version"
        fi
    fi
done | head -10

# 9. Check for minimum Go version requirements
echo -e "\n${BLUE}=== Go Version Requirements ===${NC}"
go_version=$(grep "^go " go.mod | awk '{print $2}')
echo "Required Go version: $go_version"

# Check if it's compatible with common Pi Go installations
case "$go_version" in
    "1.19"|"1.20"|"1.21"|"1.22"|"1.23")
        echo -e "${GREEN}✅ Go version compatible with Pi deployment${NC}"
        ;;
    *)
        log_issue "Go version $go_version compatibility unknown for Pi" "WARNING" \
            "Verify Pi Go version compatibility"
        ;;
esac

# 10. Check for CGO usage (can complicate Pi cross-compilation)
echo -e "\n${BLUE}=== CGO Usage Analysis ===${NC}"
if go list -deps ./... | xargs go list -f '{{if .CgoFiles}}{{.ImportPath}}{{end}}' | grep -v "^$"; then
    log_issue "CGO usage detected in dependencies" "WARNING" \
        "CGO can complicate Pi cross-compilation"
else
    echo -e "${GREEN}✅ No CGO usage detected${NC}"
fi

# 11. Check build tags for security-sensitive code
echo -e "\n${BLUE}=== Build Tags Security Check ===${NC}"
security_tags=()
while IFS= read -r -d '' file; do
    if head -5 "$file" | grep -q "//.*build.*"; then
        build_tags=$(head -5 "$file" | grep "//.*build.*" | head -1)
        if echo "$build_tags" | grep -q "debug\|test\|unsafe"; then
            security_tags+=("$file: $build_tags")
        fi
    fi
done < <(find . -name "*.go" -print0)

if [[ ${#security_tags[@]} -gt 0 ]]; then
    echo "Security-sensitive build tags found:"
    for tag in "${security_tags[@]}"; do
        echo "  $tag"
    done
    log_issue "Security-sensitive build tags detected" "WARNING" \
        "Ensure proper build configuration for Pi production"
fi

# 12. Module download and verification test
echo -e "\n${BLUE}=== Module Download Test ===${NC}"
if go mod download -x; then
    echo -e "${GREEN}✅ All modules downloaded successfully${NC}"
else
    log_issue "Module download failed" "ERROR" \
        "Network or repository access issues"
fi

# 13. Check for private repository usage
if grep -q "gopkg.in\|github.com.*/" go.mod && grep -q "replace.*=>" go.mod; then
    log_issue "Potential private repository usage" "WARNING" \
        "Ensure Pi build environment has access to private repos"
fi

# Pi-specific security recommendations
echo -e "\n${BLUE}=== Pi Security Guidelines ===${NC}"
echo -e "🔒 Keep dependencies minimal for smaller attack surface"
echo -e "🔒 Regularly update dependencies for security patches"
echo -e "🔒 Avoid CGO when possible for easier cross-compilation"
echo -e "🔒 Use govulncheck in CI/CD pipeline"
echo -e "🔒 Verify module integrity with go.sum"
echo -e "🔒 Monitor dependency licenses for compliance"

# Summary
echo -e "\n${BLUE}=== Security Check Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"
echo -e "Total dependencies: $total_deps"
echo -e "Go version: $go_version"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Security check failed! Critical security issues found.${NC}"
    echo -e "${YELLOW}💡 These issues pose security risks for Pi deployment.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Security check passed with warnings. Review security practices.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ No security issues detected!${NC}"
    exit 0
fi