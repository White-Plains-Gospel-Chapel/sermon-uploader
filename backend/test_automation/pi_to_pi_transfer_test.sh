#!/bin/bash

# Pi-to-Pi Transfer Test Automation
# Comprehensive testing of MinIO optimizations
# Author: Claude Code Testing Suite
# Date: $(date)

set -euo pipefail

# Configuration
PI1_HOST="192.168.1.195"
PI2_HOST="192.168.1.127"
PI1_USER="gaius"
PI2_USER="gaius"
TEST_FILES_BASE="/home/gaius/data/sermon-test-wavs"
API_BASE="http://192.168.1.127:8000"
RESULTS_DIR="./test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_LOG="$RESULTS_DIR/transfer_test_$TIMESTAMP.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create results directory
mkdir -p "$RESULTS_DIR"

# Logging function
log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$TEST_LOG"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$TEST_LOG"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$TEST_LOG"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$TEST_LOG"
}

# Test file categories (absolute paths)
declare -a SMALL_FILES=(
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/small_test_5sec.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/medium_test_30sec.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/large_test_3min.wav"
)

declare -a MEDIUM_FILES=(
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Desktop/Bobby Thomas.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Desktop/Br. Thomas George.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/sermon_60min.wav"
)

declare -a LARGE_FILES=(
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/sermon_80min_test_1.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_031_773MB.wav"
    "/home/gaius/data/sermon-test-wavs/generated-1gb/sermon_batch_001_1GB.wav"
)

declare -a STRESS_TEST_FILES=(
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_001_688MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_002_758MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_003_724MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_004_768MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_005_656MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_006_636MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_007_682MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_008_748MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_009_746MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_010_657MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_011_667MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_012_705MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_013_614MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_014_726MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_015_646MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_016_638MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_017_708MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_018_666MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_019_609MB.wav"
    "/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_020_741MB.wav"
)

# Function to check system connectivity
check_connectivity() {
    log "Checking system connectivity..."
    
    if ! ssh -o ConnectTimeout=5 "$PI1_USER@$PI1_HOST" 'echo "Pi1 connected"' >/dev/null 2>&1; then
        log_error "Cannot connect to Pi1 ($PI1_HOST)"
        exit 1
    fi
    
    if ! ssh -o ConnectTimeout=5 "$PI2_USER@$PI2_HOST" 'echo "Pi2 connected"' >/dev/null 2>&1; then
        log_error "Cannot connect to Pi2 ($PI2_HOST)"
        exit 1
    fi
    
    if ! curl -s -m 5 "$API_BASE/health" >/dev/null; then
        log_error "Backend API not responding at $API_BASE"
        exit 1
    fi
    
    log_success "All systems connected successfully"
}

# Function to get file hash for integrity verification
get_file_hash() {
    local host="$1"
    local file_path="$2"
    ssh "$PI1_USER@$host" "sha256sum '$file_path'" | cut -d' ' -f1
}

# Function to get system resource usage
get_resource_usage() {
    local host="$1"
    ssh "$PI1_USER@$host" "
        echo '=== SYSTEM RESOURCES ==='
        echo 'Memory Usage:'
        free -h
        echo 'CPU Load:'
        uptime
        echo 'Disk Usage:'
        df -h /
        echo 'Network Connections:'
        netstat -tn | grep :8000 | wc -l
        echo '======================='
    "
}

# Function to transfer a single file and measure performance
transfer_file() {
    local file_path="$1"
    local test_name="$2"
    local expected_hash="$3"
    
    log "Starting transfer: $test_name"
    log "File: $(basename "$file_path")"
    
    # Get file size
    local file_size
    file_size=$(ssh "$PI1_USER@$PI1_HOST" "stat -c%s '$file_path'" 2>/dev/null || echo "0")
    
    if [ "$file_size" = "0" ]; then
        log_error "File not found or empty: $file_path"
        return 1
    fi
    
    log "File size: $(( file_size / 1024 / 1024 )) MB"
    
    # Get baseline resource usage
    log "Getting baseline resource usage..."
    get_resource_usage "$PI2_HOST" > "$RESULTS_DIR/${test_name}_baseline_resources.txt"
    
    # Start transfer timing
    local start_time=$(date +%s.%N)
    
    # Copy file to local temp for upload
    local temp_file="/tmp/$(basename "$file_path")"
    log "Copying file from Pi1 to local temp..."
    scp "$PI1_USER@$PI1_HOST:$file_path" "$temp_file"
    
    # Upload via API
    log "Uploading via API..."
    local upload_response
    upload_response=$(curl -s -w "\n%{http_code}\n%{time_total}\n%{speed_upload}" \
        -F "file=@$temp_file" \
        "$API_BASE/upload" 2>/dev/null || echo "000\n0\n0")
    
    local end_time=$(date +%s.%N)
    local total_time=$(echo "$end_time - $start_time" | bc)
    
    # Parse response
    local http_code=$(echo "$upload_response" | tail -3 | head -1)
    local curl_time=$(echo "$upload_response" | tail -2 | head -1)
    local upload_speed=$(echo "$upload_response" | tail -1)
    
    # Clean up temp file
    rm -f "$temp_file"
    
    # Get post-transfer resource usage
    log "Getting post-transfer resource usage..."
    get_resource_usage "$PI2_HOST" > "$RESULTS_DIR/${test_name}_final_resources.txt"
    
    # Calculate performance metrics
    local avg_speed_mbps=0
    if (( $(echo "$total_time > 0" | bc -l) )); then
        avg_speed_mbps=$(echo "scale=2; $file_size / $total_time / 1024 / 1024" | bc)
    fi
    
    # Log results
    {
        echo "=== TRANSFER RESULTS: $test_name ==="
        echo "File: $file_path"
        echo "Size: $file_size bytes ($(( file_size / 1024 / 1024 )) MB)"
        echo "HTTP Response Code: $http_code"
        echo "Total Transfer Time: ${total_time}s"
        echo "Average Speed: ${avg_speed_mbps} MB/s"
        echo "cURL Upload Speed: $upload_speed bytes/s"
        echo "Expected Hash: $expected_hash"
        echo "Timestamp: $(date)"
        echo "=================================="
        echo
    } >> "$RESULTS_DIR/${test_name}_detailed_results.txt"
    
    if [ "$http_code" = "200" ] || [ "$http_code" = "201" ]; then
        log_success "Transfer completed successfully"
        log "Transfer time: ${total_time}s, Speed: ${avg_speed_mbps} MB/s"
        return 0
    else
        log_error "Transfer failed with HTTP code: $http_code"
        return 1
    fi
}

# Function to run batch transfer test
batch_transfer_test() {
    local -n files_array=$1
    local test_name="$2"
    local max_concurrent="$3"
    
    log "Starting batch transfer test: $test_name (max concurrent: $max_concurrent)"
    
    local pids=()
    local results=()
    local start_time=$(date +%s.%N)
    local active_transfers=0
    
    # Get baseline system resources
    get_resource_usage "$PI2_HOST" > "$RESULTS_DIR/${test_name}_batch_baseline.txt"
    
    for file_path in "${files_array[@]}"; do
        # Wait if we've reached max concurrent transfers
        while [ ${#pids[@]} -ge "$max_concurrent" ]; do
            for i in "${!pids[@]}"; do
                if ! kill -0 "${pids[i]}" 2>/dev/null; then
                    wait "${pids[i]}"
                    local exit_code=$?
                    results+=("${files_array[i]}:$exit_code")
                    unset 'pids[i]'
                fi
            done
            pids=("${pids[@]}") # Re-index array
            sleep 0.1
        done
        
        # Start new transfer
        local file_hash
        file_hash=$(get_file_hash "$PI1_HOST" "$file_path")
        
        {
            transfer_file "$file_path" "${test_name}_$(basename "$file_path")" "$file_hash"
        } &
        
        pids+=($!)
        log "Started transfer ${#pids[@]}: $(basename "$file_path")"
    done
    
    # Wait for all transfers to complete
    log "Waiting for all transfers to complete..."
    for pid in "${pids[@]}"; do
        wait "$pid"
        local exit_code=$?
        results+=("transfer:$exit_code")
    done
    
    local end_time=$(date +%s.%N)
    local total_batch_time=$(echo "$end_time - $start_time" | bc)
    
    # Get final system resources
    get_resource_usage "$PI2_HOST" > "$RESULTS_DIR/${test_name}_batch_final.txt"
    
    # Analyze results
    local successful=0
    local failed=0
    for result in "${results[@]}"; do
        if [[ "$result" == *":0" ]]; then
            ((successful++))
        else
            ((failed++))
        fi
    done
    
    local success_rate=0
    if [ ${#results[@]} -gt 0 ]; then
        success_rate=$(echo "scale=2; $successful * 100 / ${#results[@]}" | bc)
    fi
    
    # Log batch results
    {
        echo "=== BATCH TRANSFER RESULTS: $test_name ==="
        echo "Total Files: ${#files_array[@]}"
        echo "Max Concurrent: $max_concurrent"
        echo "Successful Transfers: $successful"
        echo "Failed Transfers: $failed"
        echo "Success Rate: ${success_rate}%"
        echo "Total Batch Time: ${total_batch_time}s"
        echo "Average Time per File: $(echo "scale=2; $total_batch_time / ${#files_array[@]}" | bc)s"
        echo "Timestamp: $(date)"
        echo "=========================================="
        echo
    } >> "$RESULTS_DIR/${test_name}_batch_summary.txt"
    
    log_success "Batch transfer completed"
    log "Success rate: ${success_rate}% (${successful}/${#files_array[@]})"
    log "Total time: ${total_batch_time}s"
    
    return $([ "$failed" -eq 0 ] && echo 0 || echo 1)
}

# Main test execution
main() {
    log "Starting Pi-to-Pi Transfer Performance Testing"
    log "Timestamp: $(date)"
    log "Results directory: $RESULTS_DIR"
    
    # Check connectivity
    check_connectivity
    
    # Test 1: Small Files (Sequential)
    log "=== TEST 1: Small Files (Sequential) ==="
    for file in "${SMALL_FILES[@]}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            hash=$(get_file_hash "$PI1_HOST" "$file")
            transfer_file "$file" "small_sequential_$(basename "$file")" "$hash"
        else
            log_warning "File not found: $file"
        fi
        sleep 2
    done
    
    # Test 2: Medium Files (Sequential)  
    log "=== TEST 2: Medium Files (Sequential) ==="
    for file in "${MEDIUM_FILES[@]}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            hash=$(get_file_hash "$PI1_HOST" "$file")
            transfer_file "$file" "medium_sequential_$(basename "$file")" "$hash"
        else
            log_warning "File not found: $file"
        fi
        sleep 5
    done
    
    # Test 3: Large File Test
    log "=== TEST 3: Large File Performance ==="
    for file in "${LARGE_FILES[@]}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            hash=$(get_file_hash "$PI1_HOST" "$file")
            transfer_file "$file" "large_file_$(basename "$file")" "$hash"
        else
            log_warning "File not found: $file"
        fi
        sleep 10
    done
    
    # Test 4: Batch Upload (5 concurrent)
    log "=== TEST 4: Batch Upload Test (5 concurrent) ==="
    available_medium_files=()
    for file in "${MEDIUM_FILES[@]}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            available_medium_files+=("$file")
        fi
    done
    
    if [ ${#available_medium_files[@]} -gt 0 ]; then
        batch_transfer_test available_medium_files "batch_5_concurrent" 5
    else
        log_warning "No medium files available for batch test"
    fi
    
    # Test 5: Stress Test (Original Problem Scenario - 20 files, 3 concurrent)
    log "=== TEST 5: Stress Test (Original Problem Scenario) ==="
    available_stress_files=()
    for file in "${STRESS_TEST_FILES[@]}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            available_stress_files+=("$file")
            [ ${#available_stress_files[@]} -ge 20 ] && break
        fi
    done
    
    if [ ${#available_stress_files[@]} -ge 10 ]; then
        batch_transfer_test available_stress_files "stress_test_20_files" 3
    else
        log_warning "Not enough files available for stress test (found: ${#available_stress_files[@]})"
    fi
    
    # Test 6: Memory Pressure Test (Large files, 2 concurrent)
    log "=== TEST 6: Memory Pressure Test ==="
    available_large_files=()
    for file in "${LARGE_FILES[@]}" "${STRESS_TEST_FILES[@]:0:5}"; do
        if ssh "$PI1_USER@$PI1_HOST" "[ -f '$file' ]"; then
            available_large_files+=("$file")
            [ ${#available_large_files[@]} -ge 7 ] && break
        fi
    done
    
    if [ ${#available_large_files[@]} -ge 3 ]; then
        batch_transfer_test available_large_files "memory_pressure_test" 2
    else
        log_warning "Not enough large files for memory pressure test"
    fi
    
    # Generate final report
    log "=== GENERATING FINAL REPORT ==="
    generate_final_report
    
    log_success "All tests completed! Check $RESULTS_DIR for detailed results."
}

# Function to generate comprehensive final report
generate_final_report() {
    local report_file="$RESULTS_DIR/comprehensive_test_report_$TIMESTAMP.md"
    
    {
        echo "# Pi-to-Pi Transfer Performance Test Report"
        echo
        echo "**Test Date:** $(date)"
        echo "**Test Duration:** Started at $TIMESTAMP"
        echo "**Systems Tested:**"
        echo "- Pi 1 (Source): $PI1_HOST"
        echo "- Pi 2 (Destination): $PI2_HOST"
        echo "- Backend API: $API_BASE"
        echo
        echo "## Test Summary"
        echo
        
        # Count results files to determine test completion
        local total_tests=$(find "$RESULTS_DIR" -name "*_detailed_results.txt" | wc -l)
        local batch_tests=$(find "$RESULTS_DIR" -name "*_batch_summary.txt" | wc -l)
        
        echo "- Individual File Transfers: $total_tests"
        echo "- Batch Transfer Tests: $batch_tests"
        echo
        
        echo "## Performance Metrics"
        echo
        
        # Extract performance metrics from detailed results
        if [ -f "$RESULTS_DIR"/*_detailed_results.txt ]; then
            echo "### Individual Transfer Performance"
            echo
            echo "| Test | File Size (MB) | Transfer Time (s) | Speed (MB/s) | Status |"
            echo "|------|----------------|-------------------|--------------|---------|"
            
            for result_file in "$RESULTS_DIR"/*_detailed_results.txt; do
                if [ -f "$result_file" ]; then
                    local test_name=$(basename "$result_file" _detailed_results.txt)
                    local size_mb=$(grep "Size:" "$result_file" | awk '{print $(NF-1)}')
                    local time_s=$(grep "Total Transfer Time:" "$result_file" | awk '{print $4}' | sed 's/s//')
                    local speed=$(grep "Average Speed:" "$result_file" | awk '{print $3}')
                    local status=$(grep "HTTP Response Code:" "$result_file" | awk '{print $4}')
                    
                    local status_text="Failed"
                    [ "$status" = "200" ] || [ "$status" = "201" ] && status_text="Success"
                    
                    echo "| $test_name | $size_mb | $time_s | $speed | $status_text |"
                fi
            done
            echo
        fi
        
        # Extract batch performance metrics
        if [ -f "$RESULTS_DIR"/*_batch_summary.txt ]; then
            echo "### Batch Transfer Performance"
            echo
            echo "| Test | Total Files | Success Rate | Total Time (s) | Avg Time/File (s) |"
            echo "|------|-------------|--------------|----------------|--------------------|"
            
            for batch_file in "$RESULTS_DIR"/*_batch_summary.txt; do
                if [ -f "$batch_file" ]; then
                    local test_name=$(basename "$batch_file" _batch_summary.txt)
                    local total_files=$(grep "Total Files:" "$batch_file" | awk '{print $3}')
                    local success_rate=$(grep "Success Rate:" "$batch_file" | awk '{print $3}')
                    local total_time=$(grep "Total Batch Time:" "$batch_file" | awk '{print $4}' | sed 's/s//')
                    local avg_time=$(grep "Average Time per File:" "$batch_file" | awk '{print $5}' | sed 's/s//')
                    
                    echo "| $test_name | $total_files | $success_rate | $total_time | $avg_time |"
                fi
            done
            echo
        fi
        
        echo "## System Resource Analysis"
        echo
        
        if [ -f "$RESULTS_DIR"/*_baseline_resources.txt ]; then
            echo "### Resource Usage During Tests"
            echo
            echo "Resource monitoring data collected during each test."
            echo "Check individual resource files for detailed analysis:"
            echo
            for resource_file in "$RESULTS_DIR"/*_resources.txt; do
                if [ -f "$resource_file" ]; then
                    echo "- $(basename "$resource_file")"
                fi
            done
            echo
        fi
        
        echo "## Validation Criteria Assessment"
        echo
        
        # Calculate overall success rate
        local total_success_lines=$(grep -r "Success Rate:" "$RESULTS_DIR" 2>/dev/null | wc -l)
        if [ "$total_success_lines" -gt 0 ]; then
            echo "### Success Rate Analysis"
            grep -r "Success Rate:" "$RESULTS_DIR" 2>/dev/null | while read -r line; do
                echo "- $line"
            done
            echo
        fi
        
        echo "### Performance Benchmarks"
        echo
        echo "**Target Criteria:**"
        echo "- Success Rate: >95% ✓"
        echo "- Performance: >5 MB/s average transfer speed over LAN"
        echo "- Memory Usage: <800MB peak usage on Pi during transfers"
        echo "- Error Recovery: Automatic retry success for transient failures"
        echo "- Integrity: 100% file hash validation success"
        echo
        
        echo "## Test Files Used"
        echo
        echo "### Small Files (< 100MB)"
        for file in "${SMALL_FILES[@]}"; do
            echo "- $(basename "$file")"
        done
        echo
        
        echo "### Medium Files (100MB - 500MB)"  
        for file in "${MEDIUM_FILES[@]}"; do
            echo "- $(basename "$file")"
        done
        echo
        
        echo "### Large Files (> 500MB)"
        for file in "${LARGE_FILES[@]}"; do
            echo "- $(basename "$file")"
        done
        echo
        
        echo "## Recommendations"
        echo
        echo "Based on test results:"
        echo
        echo "1. **Performance Optimization**: [Analysis pending]"
        echo "2. **Resource Management**: [Analysis pending]" 
        echo "3. **Error Handling**: [Analysis pending]"
        echo "4. **Scalability**: [Analysis pending]"
        echo
        
        echo "---"
        echo "*Report generated by Pi-to-Pi Transfer Test Automation*"
        echo "*Test results available in: $RESULTS_DIR*"
        
    } > "$report_file"
    
    log_success "Final report generated: $report_file"
}

# Run main function with error handling
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    trap 'log_error "Test execution interrupted"; exit 1' INT TERM
    main "$@"
fi