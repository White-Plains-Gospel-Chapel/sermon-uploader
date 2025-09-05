#!/bin/bash

# Pre-commit Integration Tests
# Fast integration tests designed to run before commits
# Maximum execution time: 30 seconds

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

print_color() {
    echo -e "${1}${2}${NC}"
}

main() {
    print_color "$BLUE" "🚀 Running pre-commit integration tests..."
    
    cd "$PROJECT_DIR"
    
    # Check if MinIO is available
    if ! curl -f -s --max-time 2 "http://localhost:9000/minio/health/live" > /dev/null 2>&1; then
        print_color "$BLUE" "⚠️  MinIO not available, skipping integration tests"
        print_color "$BLUE" "   (This is normal if MinIO isn't running locally)"
        exit 0
    fi
    
    # Run fast integration tests
    local start_time=$(date +%s)
    
    # Load test environment
    if [[ -f ".env.test" ]]; then
        export $(grep -v '^#' .env.test | xargs) 2>/dev/null || true
    fi
    
    # Run the fast test suite
    if go test -tags=integration -timeout=30s -run="TestEndToEndUploadOnly|TestHealthMinIOConnectivity" ./... -v; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_color "$GREEN" "✅ Pre-commit tests passed in ${duration}s"
        exit 0
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_color "$RED" "❌ Pre-commit tests failed after ${duration}s"
        exit 1
    fi
}

main "$@"