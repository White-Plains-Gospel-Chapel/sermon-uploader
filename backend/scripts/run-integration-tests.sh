#!/bin/bash

# Integration Test Runner Script
# Runs comprehensive integration tests for the sermon uploader system
# Supports fast pre-commit tests and full integration test suites

set -euo pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TEST_ENV_FILE="$PROJECT_DIR/.env.test"
CI_ENV_FILE="$PROJECT_DIR/.env.ci"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
FAST_TIMEOUT="30s"
INTEGRATION_TIMEOUT="600s"
PERFORMANCE_TIMEOUT="1800s"
HEALTH_TIMEOUT="120s"

# Default values
TEST_SUITE="fast"
SKIP_MINIO_CHECK=false
VERBOSE=false
CI_MODE=false
PERFORMANCE_MODE=false
CONTAINER_MODE=false
PARALLEL=true

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to print usage
print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Integration test runner for sermon uploader system.

OPTIONS:
    -s, --suite SUITE       Test suite to run (fast|integration|performance|health)
    -e, --env ENV          Environment (local|ci|performance)
    -f, --fast             Run fast integration tests (< 30s)
    -i, --integration      Run full integration test suite
    -p, --performance      Run performance tests
    -h, --health           Run health checks only
    --ci                   Run in CI mode with containerized MinIO
    --container            Use containerized MinIO for testing
    --skip-minio           Skip MinIO connectivity check
    --no-parallel          Disable parallel test execution
    -v, --verbose          Enable verbose output
    --help                 Show this help message

EXAMPLES:
    $0 --fast              # Quick pre-commit tests
    $0 --integration       # Full integration test suite
    $0 --performance       # Performance benchmarks
    $0 --health            # System health checks
    $0 --ci                # CI mode with containers
    $0 -s fast -v          # Fast tests with verbose output

ENVIRONMENT FILES:
    .env.test              # Local testing configuration
    .env.ci                # CI/CD configuration
EOF
}

# Function to check dependencies
check_dependencies() {
    print_color "$BLUE" "🔧 Checking dependencies..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_color "$RED" "❌ Go is not installed or not in PATH"
        exit 1
    fi
    
    print_color "$GREEN" "✅ Go version: $(go version)"
    
    # Check if we're in the right directory
    if [[ ! -f "$PROJECT_DIR/go.mod" ]]; then
        print_color "$RED" "❌ go.mod not found. Please run from the backend directory."
        exit 1
    fi
    
    print_color "$GREEN" "✅ Go module found: $(grep "^module" "$PROJECT_DIR/go.mod")"
    
    # Check for Docker if container mode is enabled
    if [[ "$CONTAINER_MODE" == "true" ]] || [[ "$CI_MODE" == "true" ]]; then
        if ! command -v docker &> /dev/null; then
            print_color "$RED" "❌ Docker is required for container mode"
            exit 1
        fi
        print_color "$GREEN" "✅ Docker version: $(docker --version)"
    fi
}

# Function to check MinIO connectivity
check_minio() {
    if [[ "$SKIP_MINIO_CHECK" == "true" ]]; then
        print_color "$YELLOW" "⚠️  Skipping MinIO connectivity check"
        return 0
    fi
    
    print_color "$BLUE" "🔌 Checking MinIO connectivity..."
    
    # Load environment variables
    if [[ "$CI_MODE" == "true" ]]; then
        if [[ -f "$CI_ENV_FILE" ]]; then
            export $(grep -v '^#' "$CI_ENV_FILE" | xargs)
        fi
        endpoint=${MINIO_ENDPOINT:-localhost:9000}
    else
        if [[ -f "$TEST_ENV_FILE" ]]; then
            export $(grep -v '^#' "$TEST_ENV_FILE" | xargs)
        fi
        endpoint=${MINIO_ENDPOINT:-localhost:9000}
    fi
    
    # Parse endpoint
    if [[ $endpoint =~ ^https?:// ]]; then
        endpoint=${endpoint#*//}
    fi
    
    # Try to connect to MinIO
    local protocol="http"
    if [[ "${MINIO_SECURE:-false}" == "true" ]]; then
        protocol="https"
    fi
    
    if curl -f -s --max-time 5 "${protocol}://${endpoint}/minio/health/live" > /dev/null 2>&1; then
        print_color "$GREEN" "✅ MinIO is accessible at ${protocol}://${endpoint}"
    else
        print_color "$YELLOW" "⚠️  MinIO not accessible at ${protocol}://${endpoint}"
        print_color "$YELLOW" "    Tests will use containerized MinIO or skip MinIO-dependent tests"
    fi
}

# Function to setup test environment
setup_test_env() {
    print_color "$BLUE" "🏗️  Setting up test environment..."
    
    # Create temp directory for tests
    local temp_dir="${TEMP_DIR:-/tmp/sermon-test}"
    mkdir -p "$temp_dir"
    export TEMP_DIR="$temp_dir"
    
    # Set environment file
    if [[ "$CI_MODE" == "true" ]]; then
        export ENV_FILE="$CI_ENV_FILE"
        print_color "$GREEN" "✅ Using CI environment configuration"
    elif [[ "$PERFORMANCE_MODE" == "true" ]]; then
        export ENV_FILE="$TEST_ENV_FILE"
        export TEST_PERFORMANCE_MODE=true
        print_color "$GREEN" "✅ Using performance test configuration"
    else
        export ENV_FILE="$TEST_ENV_FILE"
        print_color "$GREEN" "✅ Using test environment configuration"
    fi
    
    # Set container mode
    if [[ "$CONTAINER_MODE" == "true" ]] || [[ "$CI_MODE" == "true" ]]; then
        export TEST_CONTAINER_MODE=true
        print_color "$GREEN" "✅ Container mode enabled"
    fi
    
    # Set build tags
    export BUILD_TAGS="integration"
}

# Function to run tests
run_tests() {
    local suite=$1
    local timeout=$2
    local test_pattern=${3:-""}
    
    print_color "$BLUE" "🧪 Running $suite tests..."
    
    cd "$PROJECT_DIR"
    
    # Build test flags
    local test_flags=""
    test_flags+="-tags=integration"
    
    if [[ "$VERBOSE" == "true" ]]; then
        test_flags+=" -v"
    fi
    
    if [[ "$PARALLEL" == "true" ]]; then
        test_flags+=" -parallel=8"
    fi
    
    test_flags+=" -timeout=$timeout"
    
    # Add test pattern if specified
    if [[ -n "$test_pattern" ]]; then
        test_flags+=" -run=$test_pattern"
    fi
    
    # Run the tests
    local start_time=$(date +%s)
    
    print_color "$BLUE" "Running: go test $test_flags ./..."
    
    if go test $test_flags ./...; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_color "$GREEN" "✅ $suite tests passed in ${duration}s"
        return 0
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_color "$RED" "❌ $suite tests failed after ${duration}s"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    print_color "$BLUE" "📊 Running performance benchmarks..."
    
    cd "$PROJECT_DIR"
    
    local bench_flags=""
    bench_flags+="-tags=integration"
    bench_flags+=" -bench=."
    bench_flags+=" -benchmem"
    bench_flags+=" -timeout=$PERFORMANCE_TIMEOUT"
    
    if [[ "$VERBOSE" == "true" ]]; then
        bench_flags+=" -v"
    fi
    
    print_color "$BLUE" "Running: go test $bench_flags ./..."
    
    if go test $bench_flags ./...; then
        print_color "$GREEN" "✅ Performance benchmarks completed"
        return 0
    else
        print_color "$RED" "❌ Performance benchmarks failed"
        return 1
    fi
}

# Function to cleanup
cleanup() {
    print_color "$BLUE" "🧹 Cleaning up test environment..."
    
    # Remove temporary files
    local temp_dir="${TEMP_DIR:-/tmp/sermon-test}"
    if [[ -d "$temp_dir" ]]; then
        rm -rf "$temp_dir" || true
    fi
    
    # Stop any test containers if running
    if [[ "$CONTAINER_MODE" == "true" ]] || [[ "$CI_MODE" == "true" ]]; then
        docker ps -q --filter "ancestor=minio/minio" | xargs -r docker stop || true
        docker ps -aq --filter "ancestor=minio/minio" | xargs -r docker rm || true
    fi
    
    print_color "$GREEN" "✅ Cleanup completed"
}

# Function to run test suite based on selection
run_test_suite() {
    case $TEST_SUITE in
        "fast")
            print_color "$BLUE" "🚀 Running fast integration tests (pre-commit)"
            run_tests "fast" "$FAST_TIMEOUT" "TestEndToEndUploadOnly|TestHealthMinIOConnectivity|TestHealthMemoryUsage"
            ;;
        "integration")
            print_color "$BLUE" "🔄 Running full integration test suite"
            run_tests "integration" "$INTEGRATION_TIMEOUT"
            ;;
        "performance")
            print_color "$BLUE" "⚡ Running performance tests"
            export TEST_PERFORMANCE_MODE=true
            run_tests "performance" "$PERFORMANCE_TIMEOUT" "TestPerformance"
            if [[ $? -eq 0 ]]; then
                run_benchmarks
            fi
            ;;
        "health")
            print_color "$BLUE" "🏥 Running health checks"
            run_tests "health" "$HEALTH_TIMEOUT" "TestHealth|TestSystemHealth"
            ;;
        *)
            print_color "$RED" "❌ Unknown test suite: $TEST_SUITE"
            print_usage
            exit 1
            ;;
    esac
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -s|--suite)
                TEST_SUITE="$2"
                shift 2
                ;;
            -e|--env)
                case $2 in
                    "ci")
                        CI_MODE=true
                        ;;
                    "performance")
                        PERFORMANCE_MODE=true
                        ;;
                    "local")
                        # Default
                        ;;
                    *)
                        print_color "$RED" "❌ Unknown environment: $2"
                        exit 1
                        ;;
                esac
                shift 2
                ;;
            -f|--fast)
                TEST_SUITE="fast"
                shift
                ;;
            -i|--integration)
                TEST_SUITE="integration"
                shift
                ;;
            -p|--performance)
                TEST_SUITE="performance"
                PERFORMANCE_MODE=true
                shift
                ;;
            -h|--health)
                TEST_SUITE="health"
                shift
                ;;
            --ci)
                CI_MODE=true
                CONTAINER_MODE=true
                shift
                ;;
            --container)
                CONTAINER_MODE=true
                shift
                ;;
            --skip-minio)
                SKIP_MINIO_CHECK=true
                shift
                ;;
            --no-parallel)
                PARALLEL=false
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            --help)
                print_usage
                exit 0
                ;;
            *)
                print_color "$RED" "❌ Unknown option: $1"
                print_usage
                exit 1
                ;;
        esac
    done
}

# Main execution function
main() {
    # Set trap for cleanup
    trap cleanup EXIT
    
    print_color "$BLUE" "🎯 Sermon Uploader Integration Test Runner"
    print_color "$BLUE" "============================================"
    
    # Parse arguments
    parse_args "$@"
    
    # Display configuration
    print_color "$BLUE" "Configuration:"
    print_color "$BLUE" "  Test Suite: $TEST_SUITE"
    print_color "$BLUE" "  CI Mode: $CI_MODE"
    print_color "$BLUE" "  Performance Mode: $PERFORMANCE_MODE"
    print_color "$BLUE" "  Container Mode: $CONTAINER_MODE"
    print_color "$BLUE" "  Verbose: $VERBOSE"
    print_color "$BLUE" "  Parallel: $PARALLEL"
    echo
    
    # Check dependencies
    check_dependencies
    
    # Check MinIO connectivity
    check_minio
    
    # Setup test environment
    setup_test_env
    
    # Run the selected test suite
    local start_time=$(date +%s)
    
    if run_test_suite; then
        local end_time=$(date +%s)
        local total_duration=$((end_time - start_time))
        echo
        print_color "$GREEN" "🎉 All tests completed successfully in ${total_duration}s!"
        exit 0
    else
        local end_time=$(date +%s)
        local total_duration=$((end_time - start_time))
        echo
        print_color "$RED" "💥 Tests failed after ${total_duration}s"
        exit 1
    fi
}

# Run main function with all arguments
main "$@"