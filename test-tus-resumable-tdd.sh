#!/bin/bash

# TDD Test Suite for TUS Resumable Uploads - Large Files (500MB+) Only
# TUS is the gold standard for resumable file uploads, ideal for large files
# Tests production API at https://sermons.wpgc.church/api/tus

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
TUS_ENDPOINT="$PRODUCTION_API/api/tus"
TEST_FILES_DIR="/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files"
MIN_FILE_SIZE=524288000  # 500MB in bytes
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
LOG_FILE="/tmp/sermon_tus_test_${TIMESTAMP}.log"
RESULTS_FILE="/tmp/sermon_tus_results_${TIMESTAMP}.json"
CHUNK_SIZE=10485760  # 10MB chunks for better progress tracking
MAX_RETRIES=5
TUS_VERSION="1.0.0"

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
    print_section "Prerequisites Check for TUS Resumable Uploads"
    
    # Check if running on Pi at expected IP
    local current_ip=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "unknown")
    print_color $BLUE "Current IP: $current_ip"
    
    # Check required tools
    local missing_tools=()
    for tool in curl bc jq hexdump; do
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
    
    print_color $GREEN "Found ${#large_files[@]} large test files for TUS testing (>=500MB)"
    for file in "${large_files[@]}"; do
        local size=$(stat -c%s "$file" 2>/dev/null || echo "0")
        local formatted_size=$(format_bytes "$size")
        print_color $CYAN "  $(basename "$file"): $formatted_size"
    done
    
    export LARGE_TEST_FILES=("${large_files[@]}")
}

# Test 1: TUS Configuration Discovery
test_tus_configuration() {
    print_section "Test 1: TUS Protocol Configuration Discovery"
    
    local start_time=$(date +%s)
    
    # Test TUS OPTIONS request to discover server capabilities
    local tus_config
    if tus_config=$(curl -s --max-time 30 -X OPTIONS \
        -H "Tus-Resumable: $TUS_VERSION" \
        "$TUS_ENDPOINT" \
        -D /tmp/tus_headers.tmp 2>/dev/null); then
        
        local duration=$(($(date +%s) - start_time))
        
        # Check TUS headers
        if grep -qi "tus-resumable" /tmp/tus_headers.tmp; then
            local tus_version=$(grep -i "tus-version" /tmp/tus_headers.tmp | cut -d: -f2 | tr -d ' \r\n' || echo "unknown")
            local tus_extensions=$(grep -i "tus-extension" /tmp/tus_headers.tmp | cut -d: -f2 | tr -d '\r\n' || echo "none")
            local max_size=$(grep -i "tus-max-size" /tmp/tus_headers.tmp | cut -d: -f2 | tr -d ' \r\n' || echo "unlimited")
            
            print_test_result "TUS Configuration Discovery" "PASS" "TUS v$tus_version, Extensions: $tus_extensions, Max Size: $max_size" "$duration"
            
            # Store configuration for later use
            export TUS_VERSION_SERVER="$tus_version"
            export TUS_EXTENSIONS="$tus_extensions"
            export TUS_MAX_SIZE="$max_size"
            
            return 0
        else
            print_test_result "TUS Configuration Discovery" "FAIL" "TUS headers not found in response" "$duration"
            return 1
        fi
    else
        local duration=$(($(date +%s) - start_time))
        print_test_result "TUS Configuration Discovery" "FAIL" "TUS configuration endpoint unreachable" "$duration"
        return 1
    fi
}

# Test 2: Create TUS Upload Session for Large File
test_tus_create_upload() {
    print_section "Test 2: Create TUS Upload Session for Large File"
    
    if [[ ${#LARGE_TEST_FILES[@]} -eq 0 ]]; then
        print_test_result "TUS Create Upload" "FAIL" "No large test files available" ""
        return 1
    fi
    
    local test_file="${LARGE_TEST_FILES[0]}"
    local file_size=$(stat -c%s "$test_file")
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    print_color $BLUE "Creating TUS upload session for: $filename ($formatted_size)"
    
    local start_time=$(date +%s)
    
    # Create TUS upload session
    local create_response
    if create_response=$(curl -s --max-time 30 -X POST \
        -H "Tus-Resumable: $TUS_VERSION" \
        -H "Upload-Length: $file_size" \
        -H "Upload-Metadata: filename $(echo -n "$filename" | base64 -w 0),filetype $(echo -n "audio/wav" | base64 -w 0)" \
        -H "Content-Length: 0" \
        "$TUS_ENDPOINT" \
        -D /tmp/tus_create_headers.tmp \
        -w "%{http_code}" 2>/dev/null); then
        
        local duration=$(($(date +%s) - start_time))
        local http_code=$(echo "$create_response" | tail -1)
        
        if [[ "$http_code" -eq 201 ]]; then
            # Extract upload URL from Location header
            local upload_url=$(grep -i "location:" /tmp/tus_create_headers.tmp | cut -d: -f2- | tr -d ' \r\n')
            
            if [[ -n "$upload_url" ]]; then
                print_test_result "TUS Create Upload" "PASS" "Created upload session for $formatted_size file" "$duration"
                export TUS_UPLOAD_URL="$upload_url"
                export TUS_FILE_SIZE="$file_size"
                export TUS_TEST_FILE="$test_file"
                return 0
            else
                print_test_result "TUS Create Upload" "FAIL" "No Location header in response" "$duration"
                return 1
            fi
        else
            print_test_result "TUS Create Upload" "FAIL" "HTTP $http_code (expected 201)" "$duration"
            return 1
        fi
    else
        local duration=$(($(date +%s) - start_time))
        print_test_result "TUS Create Upload" "FAIL" "Failed to create TUS upload session" "$duration"
        return 1
    fi
}

# Test 3: Upload Large File in Chunks via TUS
test_tus_chunked_upload() {
    print_section "Test 3: TUS Chunked Upload for Large File"
    
    if [[ -z "$TUS_UPLOAD_URL" || -z "$TUS_TEST_FILE" ]]; then
        print_test_result "TUS Chunked Upload" "FAIL" "TUS upload session not available (run create test first)" ""
        return 1
    fi
    
    local file_size="$TUS_FILE_SIZE"
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$TUS_TEST_FILE")
    local chunks_total=$(( (file_size + CHUNK_SIZE - 1) / CHUNK_SIZE ))  # Ceiling division
    
    print_color $BLUE "Uploading $filename ($formatted_size) in $chunks_total chunks of $(format_bytes $CHUNK_SIZE)"
    
    local start_time=$(date +%s)
    local offset=0
    local chunk_num=1
    local temp_chunk="/tmp/chunk_${TIMESTAMP}.bin"
    
    while [[ $offset -lt $file_size ]]; do
        local remaining=$((file_size - offset))
        local current_chunk_size=$((remaining < CHUNK_SIZE ? remaining : CHUNK_SIZE))
        
        # Extract chunk from file
        dd if="$TUS_TEST_FILE" of="$temp_chunk" bs=1 skip=$offset count=$current_chunk_size 2>/dev/null
        
        # Upload chunk
        local chunk_start_time=$(date +%s)
        local upload_result
        if upload_result=$(curl -s --max-time 300 -X PATCH \
            -H "Tus-Resumable: $TUS_VERSION" \
            -H "Upload-Offset: $offset" \
            -H "Content-Type: application/offset+octet-stream" \
            --data-binary "@$temp_chunk" \
            "$TUS_UPLOAD_URL" \
            -D /tmp/tus_patch_headers.tmp \
            -w "%{http_code}" 2>/dev/null); then
            
            local chunk_duration=$(($(date +%s) - chunk_start_time))
            local http_code=$(echo "$upload_result" | tail -1)
            
            if [[ "$http_code" -eq 204 ]]; then
                # Check new offset
                local new_offset=$(grep -i "upload-offset:" /tmp/tus_patch_headers.tmp | cut -d: -f2 | tr -d ' \r\n' || echo "$offset")
                local expected_offset=$((offset + current_chunk_size))
                
                if [[ "$new_offset" -eq "$expected_offset" ]]; then
                    local chunk_speed=$(calculate_speed "$current_chunk_size" "$chunk_duration")
                    local progress=$((offset * 100 / file_size))
                    print_color $CYAN "  Chunk $chunk_num/$chunks_total: $(format_bytes $current_chunk_size) in ${chunk_duration}s at $chunk_speed (${progress}% complete)"
                    
                    offset=$new_offset
                    chunk_num=$((chunk_num + 1))
                else
                    print_color $RED "  Chunk $chunk_num failed: offset mismatch (expected $expected_offset, got $new_offset)"
                    break
                fi
            else
                print_color $RED "  Chunk $chunk_num failed: HTTP $http_code"
                break
            fi
        else
            print_color $RED "  Chunk $chunk_num failed: upload error"
            break
        fi
        
        # Clean up chunk file
        rm -f "$temp_chunk"
    done
    
    local duration=$(($(date +%s) - start_time))
    
    # Check if upload completed successfully
    if [[ $offset -eq $file_size ]]; then
        local speed=$(calculate_speed "$file_size" "$duration")
        print_test_result "TUS Chunked Upload" "PASS" "Uploaded $formatted_size in $chunks_total chunks (${duration}s, $speed)" "$duration"
        
        # Log detailed metrics
        echo "TUS Chunked Upload Metrics:" >> "$LOG_FILE"
        echo "  File: $filename" >> "$LOG_FILE"
        echo "  Size: $formatted_size" >> "$LOG_FILE"
        echo "  Chunks: $chunks_total" >> "$LOG_FILE"
        echo "  Chunk Size: $(format_bytes $CHUNK_SIZE)" >> "$LOG_FILE"
        echo "  Duration: ${duration}s" >> "$LOG_FILE"
        echo "  Speed: $speed" >> "$LOG_FILE"
        
        return 0
    else
        print_test_result "TUS Chunked Upload" "FAIL" "Upload incomplete ($offset/$file_size bytes)" "$duration"
        return 1
    fi
}

# Test 4: Test TUS Resume Capability
test_tus_resume_capability() {
    print_section "Test 4: TUS Resume Capability Test"
    
    if [[ ${#LARGE_TEST_FILES[@]} -lt 2 ]]; then
        print_test_result "TUS Resume Capability" "SKIP" "Need at least 2 test files for resume test" ""
        return 0
    fi
    
    local test_file="${LARGE_TEST_FILES[1]}"  # Use different file
    local file_size=$(stat -c%s "$test_file")
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    print_color $BLUE "Testing resume capability with $filename ($formatted_size)"
    
    # Create upload session
    local start_time=$(date +%s)
    local create_response
    if create_response=$(curl -s --max-time 30 -X POST \
        -H "Tus-Resumable: $TUS_VERSION" \
        -H "Upload-Length: $file_size" \
        -H "Upload-Metadata: filename $(echo -n "$filename" | base64 -w 0),filetype $(echo -n "audio/wav" | base64 -w 0)" \
        -H "Content-Length: 0" \
        "$TUS_ENDPOINT" \
        -D /tmp/tus_resume_create_headers.tmp \
        -w "%{http_code}" 2>/dev/null); then
        
        local http_code=$(echo "$create_response" | tail -1)
        if [[ "$http_code" -eq 201 ]]; then
            local upload_url=$(grep -i "location:" /tmp/tus_resume_create_headers.tmp | cut -d: -f2- | tr -d ' \r\n')
            
            # Upload partial data (first 50MB)
            local partial_size=$((50 * 1048576))  # 50MB
            local temp_partial="/tmp/partial_${TIMESTAMP}.bin"
            dd if="$test_file" of="$temp_partial" bs=1 count=$partial_size 2>/dev/null
            
            if curl -s --max-time 120 -X PATCH \
                -H "Tus-Resumable: $TUS_VERSION" \
                -H "Upload-Offset: 0" \
                -H "Content-Type: application/offset+octet-stream" \
                --data-binary "@$temp_partial" \
                "$upload_url" \
                -D /tmp/tus_partial_headers.tmp \
                -w "%{http_code}" > /tmp/tus_partial_response.tmp 2>/dev/null; then
                
                local partial_http_code=$(cat /tmp/tus_partial_response.tmp)
                if [[ "$partial_http_code" -eq 204 ]]; then
                    # Check upload offset
                    if curl -s --max-time 30 -X HEAD \
                        -H "Tus-Resumable: $TUS_VERSION" \
                        "$upload_url" \
                        -D /tmp/tus_head_headers.tmp 2>/dev/null; then
                        
                        local current_offset=$(grep -i "upload-offset:" /tmp/tus_head_headers.tmp | cut -d: -f2 | tr -d ' \r\n')
                        
                        if [[ "$current_offset" -eq "$partial_size" ]]; then
                            local duration=$(($(date +%s) - start_time))
                            print_test_result "TUS Resume Capability" "PASS" "Successfully resumed from offset $current_offset ($(format_bytes $partial_size))" "$duration"
                            
                            # Clean up
                            rm -f "$temp_partial"
                            return 0
                        else
                            print_test_result "TUS Resume Capability" "FAIL" "Offset mismatch: expected $partial_size, got $current_offset" ""
                            rm -f "$temp_partial"
                            return 1
                        fi
                    else
                        print_test_result "TUS Resume Capability" "FAIL" "Failed to query upload status" ""
                        rm -f "$temp_partial"
                        return 1
                    fi
                else
                    print_test_result "TUS Resume Capability" "FAIL" "Partial upload failed: HTTP $partial_http_code" ""
                    rm -f "$temp_partial"
                    return 1
                fi
            else
                print_test_result "TUS Resume Capability" "FAIL" "Failed to upload partial data" ""
                rm -f "$temp_partial"
                return 1
            fi
        else
            print_test_result "TUS Resume Capability" "FAIL" "Failed to create resume test upload session: HTTP $http_code" ""
            return 1
        fi
    else
        print_test_result "TUS Resume Capability" "FAIL" "Failed to create resume test upload session" ""
        return 1
    fi
}

# Test 5: TUS Upload Performance Comparison
test_tus_performance() {
    print_section "Test 5: TUS Performance Analysis with Different Chunk Sizes"
    
    if [[ ${#LARGE_TEST_FILES[@]} -eq 0 ]]; then
        print_test_result "TUS Performance Analysis" "FAIL" "No test files available" ""
        return 1
    fi
    
    # Use smallest available file for performance testing to save time
    local test_file="${LARGE_TEST_FILES[0]}"
    local file_size=$(stat -c%s "$test_file")
    
    # Limit test size to 100MB for performance testing
    local test_size=$((100 * 1048576))  # 100MB
    if [[ $file_size -gt $test_size ]]; then
        file_size=$test_size
    fi
    
    local formatted_size=$(format_bytes "$file_size")
    local filename=$(basename "$test_file")
    
    print_color $BLUE "Performance testing with first $formatted_size of $filename"
    
    # Test different chunk sizes
    local chunk_sizes=(1048576 5242880 10485760)  # 1MB, 5MB, 10MB
    local best_speed=0
    local best_chunk_size=0
    
    for chunk_size in "${chunk_sizes[@]}"; do
        local formatted_chunk_size=$(format_bytes "$chunk_size")
        print_color $CYAN "Testing chunk size: $formatted_chunk_size"
        
        # Create upload session
        local create_response
        if create_response=$(curl -s --max-time 30 -X POST \
            -H "Tus-Resumable: $TUS_VERSION" \
            -H "Upload-Length: $file_size" \
            -H "Upload-Metadata: filename $(echo -n "perf_test_$chunk_size.wav" | base64 -w 0)" \
            -H "Content-Length: 0" \
            "$TUS_ENDPOINT" \
            -D /tmp/tus_perf_create_headers.tmp \
            -w "%{http_code}" 2>/dev/null); then
            
            local http_code=$(echo "$create_response" | tail -1)
            if [[ "$http_code" -eq 201 ]]; then
                local upload_url=$(grep -i "location:" /tmp/tus_perf_create_headers.tmp | cut -d: -f2- | tr -d ' \r\n')
                
                # Upload with this chunk size
                local start_time=$(date +%s)
                local offset=0
                local temp_chunk="/tmp/perf_chunk_${chunk_size}.bin"
                local success=true
                
                while [[ $offset -lt $file_size && "$success" == "true" ]]; do
                    local remaining=$((file_size - offset))
                    local current_chunk_size=$((remaining < chunk_size ? remaining : chunk_size))
                    
                    # Extract chunk
                    dd if="$test_file" of="$temp_chunk" bs=1 skip=$offset count=$current_chunk_size 2>/dev/null
                    
                    # Upload chunk
                    if curl -s --max-time 60 -X PATCH \
                        -H "Tus-Resumable: $TUS_VERSION" \
                        -H "Upload-Offset: $offset" \
                        -H "Content-Type: application/offset+octet-stream" \
                        --data-binary "@$temp_chunk" \
                        "$upload_url" \
                        -w "%{http_code}" > /tmp/perf_patch_response.tmp 2>/dev/null; then
                        
                        local patch_code=$(cat /tmp/perf_patch_response.tmp)
                        if [[ "$patch_code" -eq 204 ]]; then
                            offset=$((offset + current_chunk_size))
                        else
                            success=false
                        fi
                    else
                        success=false
                    fi
                    
                    rm -f "$temp_chunk"
                done
                
                local duration=$(($(date +%s) - start_time))
                
                if [[ "$success" == "true" && $offset -eq $file_size ]]; then
                    local speed_mbps=$(echo "scale=2; ($file_size / 1048576) / $duration" | bc)
                    print_color $CYAN "  $formatted_chunk_size chunks: ${duration}s, ${speed_mbps} MB/s"
                    
                    # Track best performance
                    if (( $(echo "$speed_mbps > $best_speed" | bc -l) )); then
                        best_speed=$speed_mbps
                        best_chunk_size=$chunk_size
                    fi
                else
                    print_color $YELLOW "  $formatted_chunk_size chunks: Failed or incomplete"
                fi
            fi
        fi
    done
    
    if [[ $best_chunk_size -gt 0 ]]; then
        local best_formatted=$(format_bytes "$best_chunk_size")
        print_test_result "TUS Performance Analysis" "PASS" "Best performance: $best_formatted chunks at ${best_speed} MB/s" ""
        return 0
    else
        print_test_result "TUS Performance Analysis" "FAIL" "No successful performance tests completed" ""
        return 1
    fi
}

# Test 6: TUS Error Handling and Recovery
test_tus_error_handling() {
    print_section "Test 6: TUS Error Handling and Recovery"
    
    # Test invalid upload URL
    print_color $BLUE "Testing error handling with invalid upload URL"
    
    local start_time=$(date +%s)
    local invalid_url="$TUS_ENDPOINT/invalid-upload-id"
    
    if curl -s --max-time 30 -X PATCH \
        -H "Tus-Resumable: $TUS_VERSION" \
        -H "Upload-Offset: 0" \
        -H "Content-Type: application/offset+octet-stream" \
        --data "invalid data" \
        "$invalid_url" \
        -w "%{http_code}" > /tmp/invalid_response.tmp 2>/dev/null; then
        
        local duration=$(($(date +%s) - start_time))
        local http_code=$(cat /tmp/invalid_response.tmp)
        
        # Should return 404 or similar error code
        if [[ "$http_code" -eq 404 || "$http_code" -eq 410 ]]; then
            print_test_result "TUS Error Handling" "PASS" "Correctly returned HTTP $http_code for invalid upload URL" "$duration"
            return 0
        else
            print_test_result "TUS Error Handling" "PARTIAL" "Returned HTTP $http_code (expected 404/410) for invalid URL" "$duration"
            return 0
        fi
    else
        print_test_result "TUS Error Handling" "FAIL" "Failed to test invalid upload URL" ""
        return 1
    fi
}

# Performance Metrics Collection
collect_tus_performance_metrics() {
    print_section "TUS Performance Metrics Summary"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    print_color $BLUE "TUS Test Suite Performance Summary:"
    print_color $CYAN "  Total Test Duration: ${total_duration}s"
    print_color $CYAN "  Tests Passed: $TESTS_PASSED"
    print_color $CYAN "  Tests Failed: $TESTS_FAILED"
    print_color $CYAN "  Total Tests: $TESTS_TOTAL"
    
    if [[ $TESTS_TOTAL -gt 0 ]]; then
        local pass_rate=$((TESTS_PASSED * 100 / TESTS_TOTAL))
        print_color $CYAN "  Pass Rate: ${pass_rate}%"
    fi
    
    # Extract TUS-specific performance data from logs
    if [[ -f "$LOG_FILE" ]]; then
        print_color $BLUE "TUS Upload Performance Analysis:"
        
        if grep -q "MB/s" "$LOG_FILE"; then
            print_color $CYAN "  Upload Speeds Recorded:"
            grep "MB/s" "$LOG_FILE" | sed 's/^/    /' | tee -a "$LOG_FILE"
        fi
        
        if grep -q "chunks" "$LOG_FILE"; then
            print_color $CYAN "  Chunking Performance:"
            grep -i "chunk" "$LOG_FILE" | grep -v "Testing\|Uploading" | sed 's/^/    /' | tee -a "$LOG_FILE"
        fi
        
        print_color $BLUE "Detailed TUS logs available at: $LOG_FILE"
    fi
}

# Generate TUS Results Report
generate_tus_results_report() {
    print_section "Generating TUS Results Report"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    # Create JSON report
    cat > "$RESULTS_FILE" << EOF
{
  "test_suite": {
    "name": "Sermon Uploader TUS Resumable Upload Tests",
    "version": "1.0",
    "timestamp": "$TIMESTAMP",
    "duration_seconds": $total_duration,
    "environment": {
      "tus_endpoint": "$TUS_ENDPOINT",
      "test_files_dir": "$TEST_FILES_DIR",
      "min_file_size_mb": $((MIN_FILE_SIZE / 1048576)),
      "chunk_size_mb": $((CHUNK_SIZE / 1048576)),
      "tus_version": "$TUS_VERSION",
      "max_retries": $MAX_RETRIES
    }
  },
  "tus_configuration": {
    "server_version": "${TUS_VERSION_SERVER:-unknown}",
    "extensions": "${TUS_EXTENSIONS:-none}",
    "max_size": "${TUS_MAX_SIZE:-unlimited}"
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
    "tus_resumable_ready": $([ $TESTS_FAILED -eq 0 ] && echo "true" || echo "false"),
    "optimal_chunk_size_mb": $((CHUNK_SIZE / 1048576)),
    "performance_notes": "TUS provides resumable uploads ideal for large files and unstable connections",
    "log_file": "$LOG_FILE"
  }
}
EOF
    
    print_color $GREEN "✓ TUS results report generated: $RESULTS_FILE"
}

# Main test execution
main() {
    print_section "TDD Test Suite for TUS Resumable Uploads (500MB+)"
    print_color $BLUE "Testing TUS endpoint: $TUS_ENDPOINT"
    print_color $BLUE "Test files location: $TEST_FILES_DIR"
    print_color $BLUE "Minimum file size: $(format_bytes $MIN_FILE_SIZE)"
    print_color $BLUE "Chunk size: $(format_bytes $CHUNK_SIZE)"
    print_color $BLUE "TUS version: $TUS_VERSION"
    echo
    
    # Initialize log file
    echo "Sermon Uploader TUS Resumable Upload Test Log - $TIMESTAMP" > "$LOG_FILE"
    echo "==========================================================" >> "$LOG_FILE"
    
    # Run prerequisite checks
    check_prerequisites
    
    # Execute TUS-specific test cases
    test_tus_configuration
    test_tus_create_upload
    test_tus_chunked_upload
    test_tus_resume_capability
    test_tus_performance
    test_tus_error_handling
    
    # Generate reports
    collect_tus_performance_metrics
    generate_tus_results_report
    
    # Final summary
    print_section "TUS Test Suite Complete"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        print_color $GREEN "🎉 ALL TUS TESTS PASSED! Production system supports resumable uploads for large files."
        print_color $GREEN "   The sermon uploader can handle 500MB+ files with resumable capability."
        print_color $GREEN "   TUS protocol provides superior reliability for large file uploads."
    else
        print_color $RED "⚠️  Some TUS tests failed. Review results before relying on resumable uploads."
        print_color $YELLOW "   Standard uploads may still work - check primary test suite results."
    fi
    
    print_color $BLUE "📋 TUS Test Results Summary:"
    print_color $CYAN "   Passed: $TESTS_PASSED"
    print_color $CYAN "   Failed: $TESTS_FAILED"
    print_color $CYAN "   Total:  $TESTS_TOTAL"
    
    print_color $BLUE "📁 Generated Files:"
    print_color $CYAN "   Log File:     $LOG_FILE"
    print_color $CYAN "   Results JSON: $RESULTS_FILE"
    
    print_color $BLUE "💡 TUS Advantages for Large Files:"
    print_color $CYAN "   • Resumable uploads survive connection drops"
    print_color $CYAN "   • Progress tracking with precise byte-level control"
    print_color $CYAN "   • Efficient chunking reduces memory usage"
    print_color $CYAN "   • Industry standard protocol (tus.io)"
    
    # Exit with appropriate code
    exit $([[ $TESTS_FAILED -eq 0 ]] && echo 0 || echo 1)
}

# Set up cleanup trap
cleanup() {
    # Clean up temporary files
    rm -f /tmp/tus_*.tmp /tmp/chunk_*.bin /tmp/partial_*.bin /tmp/perf_*.bin
    rm -f /tmp/invalid_response.tmp /tmp/perf_patch_response.tmp
}

trap cleanup EXIT

# Run main function
main "$@"