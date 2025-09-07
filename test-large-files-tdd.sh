#!/bin/bash

# TDD Test Suite for Sermon Uploader API - Large Files (500MB+) Only
# Designed to run from Raspberry Pi at 192.168.1.195
# Tests production API at https://sermons.wpgc.church

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
PRODUCTION_API="https://sermons.wpgc.church"
TEST_FILES_DIR="/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files"
MIN_FILE_SIZE=524288000  # 500MB in bytes
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
LOG_FILE="/tmp/sermon_upload_test_${TIMESTAMP}.log"
RESULTS_FILE="/tmp/sermon_upload_results_${TIMESTAMP}.json"
CURL_TIMEOUT=3600  # 1 hour timeout for large files
MAX_RETRIES=3
BATCH_SIZE=3

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0
START_TIME=$(date +%s)

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}" | tee -a "$LOG_FILE"
}

# Function to print section headers
print_section() {
    echo | tee -a "$LOG_FILE"
    print_color $BOLD "=================================================================="
    print_color $BOLD "$1"
    print_color $BOLD "=================================================================="
    echo | tee -a "$LOG_FILE"
}

# Function to print test results
print_test_result() {
    local test_name=$1
    local status=$2
    local details=$3
    local duration=$4
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    if [[ "$status" == "PASS" ]]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        print_color $GREEN "✓ TEST PASSED: $test_name"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        print_color $RED "✗ TEST FAILED: $test_name"
    fi
    
    if [[ -n "$details" ]]; then
        print_color $CYAN "  Details: $details"
    fi
    
    if [[ -n "$duration" ]]; then
        print_color $BLUE "  Duration: ${duration}s"
    fi
    echo | tee -a "$LOG_FILE"
}

# Function to format bytes
format_bytes() {
    local bytes=$1
    if (( bytes >= 1073741824 )); then
        echo "$(( bytes / 1073741824 ))GB"
    elif (( bytes >= 1048576 )); then
        echo "$(( bytes / 1048576 ))MB"
    elif (( bytes >= 1024 )); then
        echo "$(( bytes / 1024 ))KB"
    else
        echo "${bytes}B"
    fi
}

# Function to calculate upload speed
calculate_speed() {
    local bytes=$1
    local seconds=$2
    if (( seconds > 0 )); then
        local mbps=$(echo "scale=2; ($bytes / 1048576) / $seconds" | bc)
        echo "${mbps} MB/s"
    else
        echo "N/A"
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_section "Prerequisites Check"
    
    # Check if running on Pi at expected IP
    local current_ip=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "unknown")
    print_color $BLUE "Current IP: $current_ip"
    
    # Check required tools
    local missing_tools=()
    for tool in curl bc jq; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        print_color $RED "Missing required tools: ${missing_tools[*]}"
        print_color $YELLOW "Install with: sudo apt-get install ${missing_tools[*]}"
        exit 1
    fi
    
    # Check test files directory
    if [[ ! -d "$TEST_FILES_DIR" ]]; then
        print_color $RED "Test files directory not found: $TEST_FILES_DIR"
        exit 1
    fi
    
    # Find 500MB+ test files
    mapfile -t large_files < <(find "$TEST_FILES_DIR" -name "*.wav" -size +500M -type f 2>/dev/null)
    
    if [[ ${#large_files[@]} -eq 0 ]]; then
        print_color $RED "No WAV files >= 500MB found in $TEST_FILES_DIR"
        exit 1
    fi
    
    print_color $GREEN "Found ${#large_files[@]} large test files (>=500MB)"
    for file in "${large_files[@]}"; do
        local size=$(stat -c%s "$file" 2>/dev/null || echo "0")
        local formatted_size=$(format_bytes "$size")
        print_color $CYAN "  $(basename "$file"): $formatted_size"
    done
    
    export LARGE_TEST_FILES=("${large_files[@]}")
}

# Test 1: API Health Check - Verify system ready for large uploads
test_api_health() {
    print_section "Test 1: API Health Check for Large Upload Readiness"
    
    local start_time=$(date +%s)
    
    # Test basic health endpoint
    local health_response
    if health_response=$(curl -s --max-time 30 --fail "$PRODUCTION_API/api/health" 2>/dev/null); then
        local status_code=$?
        
        # Check if response contains expected fields
        if echo "$health_response" | jq -e '.status' > /dev/null 2>&1; then
            local api_status=$(echo "$health_response" | jq -r '.status')
            
            if [[ "$api_status" == "ok" || "$api_status" == "healthy" ]]; then
                local duration=$(($(date +%s) - start_time))
                print_test_result "API Health Check" "PASS" "API is healthy and ready" "$duration"
                return 0
            else
                print_test_result "API Health Check" "FAIL" "API status: $api_status" ""
                return 1
            fi
        else
            print_test_result "API Health Check" "FAIL" "Invalid health response format" ""
            return 1
        fi
    else
        print_test_result "API Health Check" "FAIL" "Health endpoint unreachable or returned error" ""
        return 1
    fi
}

# Test 2: Single Large File Upload
test_single_large_upload() {
    print_section "Test 2: Single Large File Upload (500MB+)"
    
    if [[ ${#LARGE_TEST_FILES[@]} -eq 0 ]]; then
        print_test_result "Single Large File Upload" "FAIL" "No large test files available" ""
        return 1
    fi
    
    local test_file="${LARGE_TEST_FILES[0]}"
    local file_size=$(stat -c%s "$test_file")
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    print_color $BLUE "Uploading: $filename ($formatted_size)"
    
    local start_time=$(date +%s)
    local temp_log="/tmp/curl_upload_${TIMESTAMP}.log"
    
    # Upload with progress tracking
    local upload_result
    if curl -X POST \
        --max-time $CURL_TIMEOUT \
        --retry $MAX_RETRIES \
        --retry-delay 5 \
        --progress-bar \
        --write-out "%{http_code}:%{time_total}:%{speed_upload}" \
        -F "files=@$test_file" \
        "$PRODUCTION_API/api/upload" \
        -o "$temp_log" 2>&1 > /tmp/upload_response.tmp; then
        
        upload_result=$(cat /tmp/upload_response.tmp)
        local http_code=$(echo "$upload_result" | cut -d: -f1)
        local time_total=$(echo "$upload_result" | cut -d: -f2)
        local speed_upload=$(echo "$upload_result" | cut -d: -f3)
        
        local duration=$(($(date +%s) - start_time))
        local speed_mbps=$(calculate_speed "$file_size" "$duration")
        
        if [[ "$http_code" -eq 200 || "$http_code" -eq 201 ]]; then
            print_test_result "Single Large File Upload" "PASS" "Uploaded $formatted_size in ${duration}s at $speed_mbps" "$duration"
            
            # Log detailed metrics
            echo "Single Upload Metrics:" >> "$LOG_FILE"
            echo "  File: $filename" >> "$LOG_FILE"
            echo "  Size: $formatted_size" >> "$LOG_FILE"
            echo "  Duration: ${duration}s" >> "$LOG_FILE"
            echo "  Speed: $speed_mbps" >> "$LOG_FILE"
            echo "  HTTP Code: $http_code" >> "$LOG_FILE"
            
            return 0
        else
            print_test_result "Single Large File Upload" "FAIL" "HTTP $http_code after ${duration}s" "$duration"
            return 1
        fi
    else
        local duration=$(($(date +%s) - start_time))
        print_test_result "Single Large File Upload" "FAIL" "Upload failed or timed out" "$duration"
        return 1
    fi
}

# Test 3: Batch Upload of Large Files
test_batch_upload() {
    print_section "Test 3: Batch Upload of Large Files (3-5 files, 500MB+ each)"
    
    # Select up to BATCH_SIZE files for batch testing
    local batch_files=()
    local total_batch_size=0
    
    for ((i=0; i<${#LARGE_TEST_FILES[@]} && i<$BATCH_SIZE; i++)); do
        batch_files+=("${LARGE_TEST_FILES[i]}")
        local file_size=$(stat -c%s "${LARGE_TEST_FILES[i]}")
        total_batch_size=$((total_batch_size + file_size))
    done
    
    if [[ ${#batch_files[@]} -lt 2 ]]; then
        print_test_result "Batch Upload" "FAIL" "Insufficient files for batch test (need at least 2)" ""
        return 1
    fi
    
    local formatted_total_size=$(format_bytes "$total_batch_size")
    print_color $BLUE "Batch uploading ${#batch_files[@]} files (total: $formatted_total_size)"
    
    for file in "${batch_files[@]}"; do
        local size=$(stat -c%s "$file")
        local formatted_size=$(format_bytes "$size")
        print_color $CYAN "  $(basename "$file"): $formatted_size"
    done
    
    local start_time=$(date +%s)
    
    # Build curl command for batch upload
    local curl_cmd="curl -X POST --max-time $CURL_TIMEOUT --retry $MAX_RETRIES --retry-delay 10 --progress-bar --write-out '%{http_code}:%{time_total}:%{speed_upload}'"
    
    # Add each file to the form data
    for file in "${batch_files[@]}"; do
        curl_cmd="$curl_cmd -F 'files=@$file'"
    done
    
    curl_cmd="$curl_cmd '$PRODUCTION_API/api/upload' -o /tmp/batch_response.tmp"
    
    local upload_result
    if eval "$curl_cmd" 2>/tmp/batch_curl.log; then
        upload_result=$(cat /tmp/batch_response.tmp)
        local http_code=$(echo "$upload_result" | cut -d: -f1)
        local time_total=$(echo "$upload_result" | cut -d: -f2)
        
        local duration=$(($(date +%s) - start_time))
        local speed_mbps=$(calculate_speed "$total_batch_size" "$duration")
        
        if [[ "$http_code" -eq 200 || "$http_code" -eq 201 ]]; then
            print_test_result "Batch Upload" "PASS" "Uploaded ${#batch_files[@]} files ($formatted_total_size) in ${duration}s at $speed_mbps" "$duration"
            
            # Log detailed metrics
            echo "Batch Upload Metrics:" >> "$LOG_FILE"
            echo "  Files: ${#batch_files[@]}" >> "$LOG_FILE"
            echo "  Total Size: $formatted_total_size" >> "$LOG_FILE"
            echo "  Duration: ${duration}s" >> "$LOG_FILE"
            echo "  Speed: $speed_mbps" >> "$LOG_FILE"
            echo "  HTTP Code: $http_code" >> "$LOG_FILE"
            
            return 0
        else
            print_test_result "Batch Upload" "FAIL" "HTTP $http_code after ${duration}s" "$duration"
            return 1
        fi
    else
        local duration=$(($(date +%s) - start_time))
        print_test_result "Batch Upload" "FAIL" "Batch upload failed or timed out" "$duration"
        return 1
    fi
}

# Test 4: Timeout Handling for Large Files
test_timeout_handling() {
    print_section "Test 4: Timeout Handling for Large Files"
    
    if [[ ${#LARGE_TEST_FILES[@]} -eq 0 ]]; then
        print_test_result "Timeout Handling" "FAIL" "No large test files available" ""
        return 1
    fi
    
    local test_file="${LARGE_TEST_FILES[0]}"
    local file_size=$(stat -c%s "$test_file")
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    # Test with very short timeout to simulate timeout scenario
    local short_timeout=5
    print_color $BLUE "Testing timeout handling with ${short_timeout}s timeout on $filename ($formatted_size)"
    
    local start_time=$(date +%s)
    
    # This should timeout for large files
    if curl -X POST \
        --max-time $short_timeout \
        --connect-timeout 5 \
        -F "files=@$test_file" \
        "$PRODUCTION_API/api/upload" \
        -o /tmp/timeout_test.tmp 2>/dev/null; then
        
        # If it succeeds, it means the connection is very fast
        local duration=$(($(date +%s) - start_time))
        print_test_result "Timeout Handling" "PASS" "Upload completed faster than expected timeout ($duration s < $short_timeout s)" "$duration"
        return 0
    else
        local curl_exit_code=$?
        local duration=$(($(date +%s) - start_time))
        
        # Check if it was a timeout (exit code 28)
        if [[ $curl_exit_code -eq 28 ]]; then
            print_test_result "Timeout Handling" "PASS" "Correctly handled timeout after ${duration}s" "$duration"
            return 0
        else
            print_test_result "Timeout Handling" "FAIL" "Failed with exit code $curl_exit_code, not timeout" "$duration"
            return 1
        fi
    fi
}

# Test 5: Progress Tracking for Large Uploads
test_progress_tracking() {
    print_section "Test 5: Progress Tracking for Large Uploads"
    
    if [[ ${#LARGE_TEST_FILES[@]} -eq 0 ]]; then
        print_test_result "Progress Tracking" "FAIL" "No large test files available" ""
        return 1
    fi
    
    local test_file="${LARGE_TEST_FILES[0]}"
    local file_size=$(stat -c%s "$test_file")
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    print_color $BLUE "Testing progress tracking for $filename ($formatted_size)"
    
    local start_time=$(date +%s)
    local progress_log="/tmp/progress_${TIMESTAMP}.log"
    
    # Upload with detailed progress tracking
    if curl -X POST \
        --max-time $CURL_TIMEOUT \
        --retry $MAX_RETRIES \
        --progress-bar \
        -F "files=@$test_file" \
        "$PRODUCTION_API/api/upload" \
        -o /tmp/progress_response.tmp 2>"$progress_log"; then
        
        local duration=$(($(date +%s) - start_time))
        local speed_mbps=$(calculate_speed "$file_size" "$duration")
        
        # Check if progress was tracked
        if [[ -f "$progress_log" && -s "$progress_log" ]]; then
            local progress_lines=$(wc -l < "$progress_log")
            print_test_result "Progress Tracking" "PASS" "Upload completed with progress tracking ($progress_lines progress updates, $speed_mbps)" "$duration"
            
            # Show sample progress output
            print_color $CYAN "Sample progress output:"
            tail -5 "$progress_log" | while read -r line; do
                print_color $CYAN "  $line"
            done
            
            return 0
        else
            print_test_result "Progress Tracking" "PARTIAL" "Upload succeeded but no progress data captured" "$duration"
            return 0
        fi
    else
        local duration=$(($(date +%s) - start_time))
        print_test_result "Progress Tracking" "FAIL" "Upload failed during progress tracking test" "$duration"
        return 1
    fi
}

# Test 6: Error Recovery and Resilience
test_error_recovery() {
    print_section "Test 6: Error Recovery and Upload Resilience"
    
    # Test 6a: Retry mechanism
    print_color $BLUE "Testing retry mechanism with connection interruption simulation"
    
    local start_time=$(date +%s)
    
    # Test with retries on a deliberately problematic URL (non-existent subdomain)
    local fake_endpoint="https://nonexistent-subdomain.sermons.wpgc.church/api/upload"
    
    if curl -X POST \
        --max-time 10 \
        --retry 2 \
        --retry-delay 1 \
        --retry-connrefused \
        -F "files=@${LARGE_TEST_FILES[0]}" \
        "$fake_endpoint" \
        -o /tmp/retry_test.tmp 2>/tmp/retry_log.tmp; then
        
        # This should not succeed
        print_test_result "Error Recovery - Retry Mechanism" "FAIL" "Unexpectedly succeeded on fake endpoint" ""
        return 1
    else
        local duration=$(($(date +%s) - start_time))
        
        # Check if retries were attempted
        if grep -q "retry\|Trying\|Failed" /tmp/retry_log.tmp 2>/dev/null; then
            print_test_result "Error Recovery - Retry Mechanism" "PASS" "Correctly attempted retries before failing (${duration}s)" "$duration"
        else
            print_test_result "Error Recovery - Retry Mechanism" "PARTIAL" "Failed quickly without visible retry attempts" "$duration"
        fi
    fi
    
    # Test 6b: Real endpoint resilience
    print_color $BLUE "Testing real endpoint resilience with aggressive retry settings"
    
    if [[ ${#LARGE_TEST_FILES[@]} -gt 1 ]]; then
        local test_file="${LARGE_TEST_FILES[1]}"  # Use second file to avoid cache
        local file_size=$(stat -c%s "$test_file")
        local formatted_size=$(format_bytes "$file_size")
        local filename=$(basename "$test_file")
        
        start_time=$(date +%s)
        
        if curl -X POST \
            --max-time $CURL_TIMEOUT \
            --retry 5 \
            --retry-delay 10 \
            --retry-max-time 300 \
            -F "files=@$test_file" \
            "$PRODUCTION_API/api/upload" \
            -o /tmp/resilience_test.tmp 2>/tmp/resilience_log.tmp; then
            
            local duration=$(($(date +%s) - start_time))
            local speed_mbps=$(calculate_speed "$file_size" "$duration")
            print_test_result "Error Recovery - Endpoint Resilience" "PASS" "Upload succeeded with resilient settings ($speed_mbps)" "$duration"
            return 0
        else
            local duration=$(($(date +%s) - start_time))
            print_test_result "Error Recovery - Endpoint Resilience" "FAIL" "Upload failed even with aggressive retry settings" "$duration"
            return 1
        fi
    else
        print_test_result "Error Recovery - Endpoint Resilience" "SKIP" "Insufficient test files for resilience test" ""
        return 0
    fi
}

# Performance Metrics Collection
collect_performance_metrics() {
    print_section "Performance Metrics Summary"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    print_color $BLUE "Test Suite Performance Summary:"
    print_color $CYAN "  Total Test Duration: ${total_duration}s"
    print_color $CYAN "  Tests Passed: $TESTS_PASSED"
    print_color $CYAN "  Tests Failed: $TESTS_FAILED"
    print_color $CYAN "  Total Tests: $TESTS_TOTAL"
    
    if [[ $TESTS_TOTAL -gt 0 ]]; then
        local pass_rate=$((TESTS_PASSED * 100 / TESTS_TOTAL))
        print_color $CYAN "  Pass Rate: ${pass_rate}%"
    fi
    
    # Extract performance data from logs
    if [[ -f "$LOG_FILE" ]]; then
        print_color $BLUE "Upload Performance Analysis:"
        
        # Extract upload speeds from logs
        if grep -q "MB/s" "$LOG_FILE"; then
            print_color $CYAN "  Upload Speeds Recorded:"
            grep "MB/s" "$LOG_FILE" | sed 's/^/    /' | tee -a "$LOG_FILE"
        fi
        
        # Show log file location
        print_color $BLUE "Detailed logs available at: $LOG_FILE"
    fi
}

# Generate JSON Results Report
generate_results_report() {
    print_section "Generating Results Report"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    # Create JSON report
    cat > "$RESULTS_FILE" << EOF
{
  "test_suite": {
    "name": "Sermon Uploader Large File TDD Tests",
    "version": "1.0",
    "timestamp": "$TIMESTAMP",
    "duration_seconds": $total_duration,
    "environment": {
      "api_endpoint": "$PRODUCTION_API",
      "test_files_dir": "$TEST_FILES_DIR",
      "min_file_size_mb": $((MIN_FILE_SIZE / 1048576)),
      "curl_timeout": $CURL_TIMEOUT,
      "max_retries": $MAX_RETRIES,
      "batch_size": $BATCH_SIZE
    }
  },
  "results": {
    "total_tests": $TESTS_TOTAL,
    "passed": $TESTS_PASSED,
    "failed": $TESTS_FAILED,
    "pass_rate_percent": $((TESTS_TOTAL > 0 ? TESTS_PASSED * 100 / TESTS_TOTAL : 0))
  },
  "test_files": [
EOF
    
    # Add test files info
    local first=true
    for file in "${LARGE_TEST_FILES[@]}"; do
        if [[ "$first" == "true" ]]; then
            first=false
        else
            echo "," >> "$RESULTS_FILE"
        fi
        
        local size=$(stat -c%s "$file")
        local name=$(basename "$file")
        echo "    {\"filename\": \"$name\", \"size_bytes\": $size, \"size_mb\": $((size / 1048576))}" >> "$RESULTS_FILE"
    done
    
    cat >> "$RESULTS_FILE" << EOF
  ],
  "recommendations": {
    "production_readiness": $([ $TESTS_FAILED -eq 0 ] && echo "true" || echo "false"),
    "performance_notes": "See detailed logs for upload speeds and timing analysis",
    "log_file": "$LOG_FILE"
  }
}
EOF
    
    print_color $GREEN "✓ Results report generated: $RESULTS_FILE"
}

# Main test execution
main() {
    print_section "TDD Test Suite for Large File Uploads (500MB+)"
    print_color $BLUE "Testing production API: $PRODUCTION_API"
    print_color $BLUE "Test files location: $TEST_FILES_DIR"
    print_color $BLUE "Minimum file size: $(format_bytes $MIN_FILE_SIZE)"
    echo
    
    # Initialize log file
    echo "Sermon Uploader Large File TDD Test Log - $TIMESTAMP" > "$LOG_FILE"
    echo "=========================================================" >> "$LOG_FILE"
    
    # Run prerequisite checks
    check_prerequisites
    
    # Execute test cases
    test_api_health
    test_single_large_upload
    test_batch_upload
    test_timeout_handling
    test_progress_tracking
    test_error_recovery
    
    # Generate reports
    collect_performance_metrics
    generate_results_report
    
    # Final summary
    print_section "TDD Test Suite Complete"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        print_color $GREEN "🎉 ALL TESTS PASSED! Production system is ready for large file uploads."
        print_color $GREEN "   The sermon uploader can handle 500MB+ files reliably."
    else
        print_color $RED "⚠️  Some tests failed. Review results before production use."
        print_color $YELLOW "   Check logs for detailed failure analysis."
    fi
    
    print_color $BLUE "📋 Test Results Summary:"
    print_color $CYAN "   Passed: $TESTS_PASSED"
    print_color $CYAN "   Failed: $TESTS_FAILED"
    print_color $CYAN "   Total:  $TESTS_TOTAL"
    
    print_color $BLUE "📁 Generated Files:"
    print_color $CYAN "   Log File:     $LOG_FILE"
    print_color $CYAN "   Results JSON: $RESULTS_FILE"
    
    # Exit with appropriate code
    exit $([[ $TESTS_FAILED -eq 0 ]] && echo 0 || echo 1)
}

# Set up cleanup trap
cleanup() {
    # Clean up temporary files
    rm -f /tmp/curl_upload_*.log /tmp/upload_response.tmp /tmp/batch_response.tmp
    rm -f /tmp/batch_curl.log /tmp/timeout_test.tmp /tmp/progress_response.tmp
    rm -f /tmp/progress_*.log /tmp/retry_test.tmp /tmp/retry_log.tmp
    rm -f /tmp/resilience_test.tmp /tmp/resilience_log.tmp
}

trap cleanup EXIT

# Run main function
main "$@"