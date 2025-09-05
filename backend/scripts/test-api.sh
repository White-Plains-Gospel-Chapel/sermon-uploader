#!/bin/bash

# Comprehensive API Testing Script for Sermon Uploader Backend
# Supports multiple test environments and generates detailed reports

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
ENVIRONMENT=${ENVIRONMENT:-"local"}
VERBOSE=${VERBOSE:-"false"}
GENERATE_REPORT=${GENERATE_REPORT:-"true"}
BENCHMARK=${BENCHMARK:-"false"}
PARALLEL=${PARALLEL:-"false"}
OUTPUT_DIR="test-results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="$OUTPUT_DIR/api_test_report_$TIMESTAMP.json"

# Test configuration
TEST_TIMEOUT=${TEST_TIMEOUT:-"30s"}
BENCH_TIME=${BENCH_TIME:-"10s"}
BENCH_CPU=${BENCH_CPU:-"2"}

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to print section headers
print_section() {
    echo
    print_color $BLUE "=================================="
    print_color $BLUE "$1"
    print_color $BLUE "=================================="
    echo
}

# Function to print usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

API Testing Script for Sermon Uploader Backend

OPTIONS:
    -e, --environment ENV    Set test environment (local, ci, production) [default: local]
    -v, --verbose           Enable verbose output
    -r, --no-report         Skip generating test report
    -b, --benchmark         Run benchmark tests
    -p, --parallel          Run tests in parallel
    -t, --timeout DURATION  Set test timeout [default: 30s]
    --bench-time DURATION   Set benchmark duration [default: 10s]
    --bench-cpu COUNT       Set benchmark CPU count [default: 2]
    -o, --output-dir DIR    Set output directory [default: test-results]
    -h, --help             Show this help message

EXAMPLES:
    $0                      # Run basic tests
    $0 -v -b               # Run with verbose output and benchmarks
    $0 -e ci -p            # Run in CI environment with parallel execution
    $0 --benchmark --bench-time 30s  # Run 30-second benchmarks

ENVIRONMENT VARIABLES:
    ENVIRONMENT    Test environment (local, ci, production)
    VERBOSE        Enable verbose output (true, false)
    GENERATE_REPORT Generate test report (true, false)
    BENCHMARK      Run benchmark tests (true, false)
    PARALLEL       Run tests in parallel (true, false)
    TEST_TIMEOUT   Test timeout duration
    BENCH_TIME     Benchmark duration
    BENCH_CPU      Benchmark CPU count

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="true"
            shift
            ;;
        -r|--no-report)
            GENERATE_REPORT="false"
            shift
            ;;
        -b|--benchmark)
            BENCHMARK="true"
            shift
            ;;
        -p|--parallel)
            PARALLEL="true"
            shift
            ;;
        -t|--timeout)
            TEST_TIMEOUT="$2"
            shift 2
            ;;
        --bench-time)
            BENCH_TIME="$2"
            shift 2
            ;;
        --bench-cpu)
            BENCH_CPU="$2"
            shift 2
            ;;
        -o|--output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option $1"
            usage
            exit 1
            ;;
    esac
done

# Validate environment
case $ENVIRONMENT in
    local|ci|production)
        ;;
    *)
        print_color $RED "Invalid environment: $ENVIRONMENT"
        print_color $RED "Valid environments: local, ci, production"
        exit 1
        ;;
esac

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Set environment-specific configuration
case $ENVIRONMENT in
    local)
        export ENV="development"
        export PORT="8000"
        export MINIO_ENDPOINT="localhost:9000"
        ;;
    ci)
        export ENV="test"
        export PORT="0" # Random port
        export MINIO_ENDPOINT="localhost:9000"
        ;;
    production)
        export ENV="production"
        # Production settings would be loaded from environment
        ;;
esac

print_section "API Testing Suite - Sermon Uploader Backend"

print_color $BLUE "Configuration:"
echo "  Environment: $ENVIRONMENT"
echo "  Verbose: $VERBOSE"
echo "  Generate Report: $GENERATE_REPORT"
echo "  Benchmark: $BENCHMARK"
echo "  Parallel: $PARALLEL"
echo "  Test Timeout: $TEST_TIMEOUT"
echo "  Output Directory: $OUTPUT_DIR"
echo

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_color $RED "Go is not installed or not in PATH"
    exit 1
fi

# Check if we're in the backend directory
if [[ ! -f "go.mod" ]]; then
    print_color $RED "go.mod not found. Please run this script from the backend directory."
    exit 1
fi

# Install test dependencies
print_section "Installing Test Dependencies"
print_color $YELLOW "Installing required Go modules..."

go mod download
if ! go list -m github.com/stretchr/testify &> /dev/null; then
    go get github.com/stretchr/testify/assert
    go get github.com/stretchr/testify/require
fi

# Check if services are available (for integration tests)
check_services() {
    print_section "Service Health Checks"
    
    # Check if MinIO is accessible (optional for unit tests)
    if command -v curl &> /dev/null; then
        if curl -s --max-time 3 "http://$MINIO_ENDPOINT/minio/health/live" > /dev/null 2>&1; then
            print_color $GREEN "✓ MinIO is accessible"
            export MINIO_AVAILABLE="true"
        else
            print_color $YELLOW "⚠ MinIO is not accessible (unit tests will still run)"
            export MINIO_AVAILABLE="false"
        fi
    else
        print_color $YELLOW "⚠ curl not available, skipping service checks"
        export MINIO_AVAILABLE="false"
    fi
}

# Run Go tests
run_tests() {
    print_section "Running API Tests"
    
    local test_flags="-v"
    local test_pattern="./..."
    
    if [[ "$PARALLEL" == "true" ]]; then
        test_flags="$test_flags -parallel=4"
        print_color $BLUE "Running tests in parallel (4 workers)"
    fi
    
    if [[ "$VERBOSE" == "true" ]]; then
        test_flags="$test_flags -v"
    fi
    
    # Add timeout
    test_flags="$test_flags -timeout=$TEST_TIMEOUT"
    
    # Create test output file
    local test_output="$OUTPUT_DIR/test_output_$TIMESTAMP.txt"
    local test_json="$OUTPUT_DIR/test_results_$TIMESTAMP.json"
    
    print_color $BLUE "Running unit tests..."
    print_color $YELLOW "Output will be saved to: $test_output"
    
    # Run tests and capture output
    if go test $test_flags -json $test_pattern > "$test_json" 2>&1; then
        print_color $GREEN "✓ All tests passed"
        TEST_EXIT_CODE=0
    else
        print_color $RED "✗ Some tests failed"
        TEST_EXIT_CODE=1
    fi
    
    # Also save human-readable output
    go test $test_flags $test_pattern > "$test_output" 2>&1 || true
    
    # Parse test results
    if [[ -f "$test_json" ]]; then
        # Count test results using basic text processing since jq might not be available
        local pass_count=$(grep '"Action":"pass"' "$test_json" | grep '"Test":' | wc -l)
        local fail_count=$(grep '"Action":"fail"' "$test_json" | grep '"Test":' | wc -l)
        local skip_count=$(grep '"Action":"skip"' "$test_json" | grep '"Test":' | wc -l)
        
        print_color $BLUE "Test Results Summary:"
        print_color $GREEN "  Passed: $pass_count"
        print_color $RED "  Failed: $fail_count"
        print_color $YELLOW "  Skipped: $skip_count"
        
        # Store results for report
        export TEST_PASS_COUNT=$pass_count
        export TEST_FAIL_COUNT=$fail_count
        export TEST_SKIP_COUNT=$skip_count
    fi
}

# Run benchmark tests
run_benchmarks() {
    if [[ "$BENCHMARK" != "true" ]]; then
        return 0
    fi
    
    print_section "Running Benchmark Tests"
    
    local bench_output="$OUTPUT_DIR/benchmark_results_$TIMESTAMP.txt"
    local bench_flags="-bench=. -benchtime=$BENCH_TIME -benchmem"
    
    if [[ "$BENCH_CPU" -gt 1 ]]; then
        bench_flags="$bench_flags -cpu=$BENCH_CPU"
    fi
    
    print_color $BLUE "Running benchmarks for $BENCH_TIME with $BENCH_CPU CPU(s)..."
    print_color $YELLOW "Output will be saved to: $bench_output"
    
    if go test $bench_flags ./... > "$bench_output" 2>&1; then
        print_color $GREEN "✓ Benchmarks completed successfully"
        
        # Show key benchmark results
        if command -v grep &> /dev/null; then
            print_color $BLUE "Key Benchmark Results:"
            grep "^Benchmark" "$bench_output" | head -10 || true
        fi
    else
        print_color $RED "✗ Benchmark tests failed"
    fi
}

# Generate test report
generate_report() {
    if [[ "$GENERATE_REPORT" != "true" ]]; then
        return 0
    fi
    
    print_section "Generating Test Report"
    
    cat > "$REPORT_FILE" << EOF
{
  "test_run": {
    "timestamp": "$TIMESTAMP",
    "environment": "$ENVIRONMENT",
    "configuration": {
      "verbose": $VERBOSE,
      "parallel": $PARALLEL,
      "benchmark": $BENCHMARK,
      "test_timeout": "$TEST_TIMEOUT",
      "bench_time": "$BENCH_TIME",
      "bench_cpu": $BENCH_CPU
    }
  },
  "test_results": {
    "exit_code": ${TEST_EXIT_CODE:-1},
    "passed": ${TEST_PASS_COUNT:-0},
    "failed": ${TEST_FAIL_COUNT:-0},
    "skipped": ${TEST_SKIP_COUNT:-0},
    "total": $((${TEST_PASS_COUNT:-0} + ${TEST_FAIL_COUNT:-0} + ${TEST_SKIP_COUNT:-0}))
  },
  "coverage": {
    "note": "Coverage data would be available with -cover flag"
  },
  "performance": {
    "benchmarks_run": $BENCHMARK,
    "benchmark_results_file": "${BENCHMARK:+benchmark_results_$TIMESTAMP.txt}"
  },
  "files": {
    "test_output": "test_output_$TIMESTAMP.txt",
    "test_json": "test_results_$TIMESTAMP.json",
    "benchmark_results": "${BENCHMARK:+benchmark_results_$TIMESTAMP.txt}"
  }
}
EOF
    
    print_color $GREEN "✓ Test report generated: $REPORT_FILE"
}

# Cleanup function
cleanup() {
    print_section "Cleanup"
    # Clean up any temporary files or processes if needed
    print_color $BLUE "Cleaning up temporary files..."
}

# Set up trap for cleanup
trap cleanup EXIT

# Main execution
main() {
    check_services
    run_tests
    run_benchmarks
    generate_report
    
    print_section "Test Execution Complete"
    
    if [[ -f "$REPORT_FILE" ]]; then
        print_color $BLUE "Test report: $REPORT_FILE"
    fi
    
    print_color $BLUE "Test artifacts in: $OUTPUT_DIR"
    
    # List generated files
    print_color $BLUE "Generated files:"
    ls -la "$OUTPUT_DIR"/*"$TIMESTAMP"* 2>/dev/null || print_color $YELLOW "No test files generated"
    
    # Exit with appropriate code
    exit ${TEST_EXIT_CODE:-1}
}

# Performance monitoring
if [[ "$VERBOSE" == "true" ]]; then
    print_section "System Information"
    echo "Go version: $(go version)"
    echo "OS: $(uname -s)"
    echo "Architecture: $(uname -m)"
    if command -v nproc &> /dev/null; then
        echo "CPU cores: $(nproc)"
    fi
    if command -v free &> /dev/null; then
        echo "Memory: $(free -h | grep Mem | awk '{print $2}')"
    fi
fi

# Run main function
main