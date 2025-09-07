#!/bin/bash

# Master TDD Test Runner for Large File Upload Testing (500MB+)
# Combines standard HTTP uploads and TUS resumable uploads
# Designed for Raspberry Pi at 192.168.1.195

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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
MASTER_LOG="/tmp/sermon_master_test_${TIMESTAMP}.log"
MASTER_RESULTS="/tmp/sermon_master_results_${TIMESTAMP}.json"
TEST_FILES_DIR="/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files"

# Test scripts
STANDARD_TEST_SCRIPT="$SCRIPT_DIR/test-large-files-tdd.sh"
TUS_TEST_SCRIPT="$SCRIPT_DIR/test-tus-resumable-tdd.sh"

# Results tracking
STANDARD_TESTS_RESULT=""
TUS_TESTS_RESULT=""
OVERALL_SUCCESS=false
START_TIME=$(date +%s)

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}" | tee -a "$MASTER_LOG"
}

# Function to print section headers
print_section() {
    echo | tee -a "$MASTER_LOG"
    print_color $BOLD "=================================================================="
    print_color $BOLD "$1"
    print_color $BOLD "=================================================================="
    echo | tee -a "$MASTER_LOG"
}

# Function to check prerequisites
check_master_prerequisites() {
    print_section "Master Test Suite Prerequisites Check"
    
    # Check if running on Pi
    local hostname=$(hostname 2>/dev/null || echo "unknown")
    local current_ip=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "unknown")
    
    print_color $BLUE "System Information:"
    print_color $CYAN "  Hostname: $hostname"
    print_color $CYAN "  Primary IP: $current_ip"
    print_color $CYAN "  Expected Pi IP: 192.168.1.195"
    
    # Check if test scripts exist
    if [[ ! -f "$STANDARD_TEST_SCRIPT" ]]; then
        print_color $RED "Standard test script not found: $STANDARD_TEST_SCRIPT"
        exit 1
    fi
    
    if [[ ! -f "$TUS_TEST_SCRIPT" ]]; then
        print_color $RED "TUS test script not found: $TUS_TEST_SCRIPT"
        exit 1
    fi
    
    print_color $GREEN "✓ Test scripts found"
    
    # Make scripts executable
    chmod +x "$STANDARD_TEST_SCRIPT" "$TUS_TEST_SCRIPT"
    
    # Check test files directory
    if [[ ! -d "$TEST_FILES_DIR" ]]; then
        print_color $RED "Test files directory not found: $TEST_FILES_DIR"
        print_color $YELLOW "Expected location: /home/gaius/data/sermon-test-wavs/..."
        exit 1
    fi
    
    # Count available large test files
    local large_file_count
    large_file_count=$(find "$TEST_FILES_DIR" -name "*.wav" -size +500M -type f 2>/dev/null | wc -l)
    
    if [[ $large_file_count -eq 0 ]]; then
        print_color $RED "No WAV files >= 500MB found in test directory"
        exit 1
    fi
    
    print_color $GREEN "✓ Found $large_file_count large test files (>=500MB)"
    
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
    
    print_color $GREEN "✓ All required tools available"
    
    # Test internet connectivity
    if curl -s --max-time 5 "https://sermons.wpgc.church/api/health" > /dev/null 2>&1; then
        print_color $GREEN "✓ Production API is accessible"
    else
        print_color $YELLOW "⚠ Production API health check failed (tests may still work)"
    fi
    
    print_color $GREEN "✓ Prerequisites check completed successfully"
}

# Function to run standard HTTP upload tests
run_standard_tests() {
    print_section "Running Standard HTTP Upload Tests (500MB+)"
    
    print_color $BLUE "Executing: $STANDARD_TEST_SCRIPT"
    print_color $CYAN "This will test:"
    print_color $CYAN "  • API health check for large upload readiness"
    print_color $CYAN "  • Single large file upload via /api/upload"
    print_color $CYAN "  • Batch upload of multiple 500MB+ files"
    print_color $CYAN "  • Timeout handling for large transfers"
    print_color $CYAN "  • Progress tracking during uploads"
    print_color $CYAN "  • Error recovery and resilience"
    echo
    
    local start_time=$(date +%s)
    
    # Run standard tests and capture exit code
    if "$STANDARD_TEST_SCRIPT" 2>&1 | tee -a "$MASTER_LOG"; then
        STANDARD_TESTS_RESULT="PASSED"
        local duration=$(($(date +%s) - start_time))
        print_color $GREEN "✓ Standard HTTP upload tests completed successfully (${duration}s)"
    else
        STANDARD_TESTS_RESULT="FAILED"
        local duration=$(($(date +%s) - start_time))
        print_color $RED "✗ Standard HTTP upload tests failed (${duration}s)"
    fi
    
    print_color $BLUE "Standard tests result: $STANDARD_TESTS_RESULT"
}

# Function to run TUS resumable upload tests
run_tus_tests() {
    print_section "Running TUS Resumable Upload Tests (500MB+)"
    
    print_color $BLUE "Executing: $TUS_TEST_SCRIPT"
    print_color $CYAN "This will test:"
    print_color $CYAN "  • TUS protocol configuration discovery"
    print_color $CYAN "  • TUS upload session creation for large files"
    print_color $CYAN "  • Chunked upload with progress tracking"
    print_color $CYAN "  • Resume capability after interruption"
    print_color $CYAN "  • Performance with different chunk sizes"
    print_color $CYAN "  • Error handling and recovery"
    echo
    
    local start_time=$(date +%s)
    
    # Run TUS tests and capture exit code
    if "$TUS_TEST_SCRIPT" 2>&1 | tee -a "$MASTER_LOG"; then
        TUS_TESTS_RESULT="PASSED"
        local duration=$(($(date +%s) - start_time))
        print_color $GREEN "✓ TUS resumable upload tests completed successfully (${duration}s)"
    else
        TUS_TESTS_RESULT="FAILED"
        local duration=$(($(date +%s) - start_time))
        print_color $RED "✗ TUS resumable upload tests failed (${duration}s)"
    fi
    
    print_color $BLUE "TUS tests result: $TUS_TESTS_RESULT"
}

# Function to analyze performance across both test suites
analyze_combined_performance() {
    print_section "Combined Performance Analysis"
    
    # Extract performance metrics from individual test logs
    local standard_log_pattern="/tmp/sermon_upload_test_*"
    local tus_log_pattern="/tmp/sermon_tus_test_*"
    
    print_color $BLUE "Performance Comparison Summary:"
    
    # Find and analyze standard test logs
    if ls $standard_log_pattern 1> /dev/null 2>&1; then
        local latest_standard_log
        latest_standard_log=$(ls -t $standard_log_pattern | head -1)
        
        if [[ -f "$latest_standard_log" ]]; then
            print_color $CYAN "Standard HTTP Upload Performance:"
            if grep -q "MB/s" "$latest_standard_log"; then
                grep "MB/s" "$latest_standard_log" | sed 's/^/  /' | head -5
            else
                print_color $CYAN "  No performance data available"
            fi
        fi
    fi
    
    # Find and analyze TUS test logs
    if ls $tus_log_pattern 1> /dev/null 2>&1; then
        local latest_tus_log
        latest_tus_log=$(ls -t $tus_log_pattern | head -1)
        
        if [[ -f "$latest_tus_log" ]]; then
            print_color $CYAN "TUS Resumable Upload Performance:"
            if grep -q "MB/s" "$latest_tus_log"; then
                grep "MB/s" "$latest_tus_log" | sed 's/^/  /' | head -5
            else
                print_color $CYAN "  No performance data available"
            fi
        fi
    fi
    
    # Provide recommendations
    print_color $BLUE "Upload Method Recommendations:"
    
    if [[ "$STANDARD_TESTS_RESULT" == "PASSED" && "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        print_color $GREEN "✓ Both upload methods are working correctly"
        print_color $CYAN "  Recommendations:"
        print_color $CYAN "  • Use TUS resumable uploads for files >1GB or unreliable connections"
        print_color $CYAN "  • Use standard HTTP uploads for smaller files or stable connections"
        print_color $CYAN "  • Implement automatic fallback: TUS first, then standard HTTP"
    elif [[ "$STANDARD_TESTS_RESULT" == "PASSED" && "$TUS_TESTS_RESULT" == "FAILED" ]]; then
        print_color $YELLOW "⚠ Standard uploads work, but TUS resumable uploads have issues"
        print_color $CYAN "  Recommendations:"
        print_color $CYAN "  • Use standard HTTP uploads for production"
        print_color $CYAN "  • Investigate TUS configuration issues"
        print_color $CYAN "  • Consider implementing retry logic for standard uploads"
    elif [[ "$STANDARD_TESTS_RESULT" == "FAILED" && "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        print_color $YELLOW "⚠ TUS resumable uploads work, but standard uploads have issues"
        print_color $CYAN "  Recommendations:"
        print_color $CYAN "  • Use TUS resumable uploads for production"
        print_color $CYAN "  • Investigate standard upload configuration"
        print_color $CYAN "  • TUS provides better reliability for large files anyway"
    else
        print_color $RED "✗ Both upload methods have issues"
        print_color $CYAN "  Recommendations:"
        print_color $CYAN "  • Check server configuration and connectivity"
        print_color $CYAN "  • Verify MinIO backend is accessible"
        print_color $CYAN "  • Review server logs for detailed error information"
    fi
}

# Function to generate master results report
generate_master_report() {
    print_section "Generating Master Test Results Report"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    # Determine overall success
    if [[ "$STANDARD_TESTS_RESULT" == "PASSED" || "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        OVERALL_SUCCESS=true
    fi
    
    # Find individual result files
    local standard_results=""
    local tus_results=""
    
    if ls /tmp/sermon_upload_results_* 1> /dev/null 2>&1; then
        standard_results=$(ls -t /tmp/sermon_upload_results_* | head -1)
    fi
    
    if ls /tmp/sermon_tus_results_* 1> /dev/null 2>&1; then
        tus_results=$(ls -t /tmp/sermon_tus_results_* | head -1)
    fi
    
    # Create comprehensive master report
    cat > "$MASTER_RESULTS" << EOF
{
  "master_test_suite": {
    "name": "Sermon Uploader Large File Upload TDD Test Suite",
    "version": "1.0",
    "timestamp": "$TIMESTAMP",
    "total_duration_seconds": $total_duration,
    "environment": {
      "test_files_dir": "$TEST_FILES_DIR",
      "production_api": "https://sermons.wpgc.church",
      "min_file_size_mb": 500,
      "test_types": ["standard_http", "tus_resumable"]
    }
  },
  "overall_results": {
    "success": $OVERALL_SUCCESS,
    "standard_http_tests": "$STANDARD_TESTS_RESULT",
    "tus_resumable_tests": "$TUS_TESTS_RESULT",
    "recommendation": "$(
      if [[ "$STANDARD_TESTS_RESULT" == "PASSED" && "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        echo "Both upload methods working - use TUS for large files, HTTP for smaller ones"
      elif [[ "$STANDARD_TESTS_RESULT" == "PASSED" ]]; then
        echo "Use standard HTTP uploads - TUS needs investigation"
      elif [[ "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        echo "Use TUS resumable uploads - more reliable for large files"
      else
        echo "Both methods have issues - investigate server configuration"
      fi
    )"
  },
  "production_readiness": {
    "ready_for_500mb_files": $OVERALL_SUCCESS,
    "recommended_method": "$(
      if [[ "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        echo "TUS resumable uploads"
      elif [[ "$STANDARD_TESTS_RESULT" == "PASSED" ]]; then
        echo "Standard HTTP uploads"
      else
        echo "None - configuration needed"
      fi
    )",
    "sunday_batch_upload_ready": $([[ "$STANDARD_TESTS_RESULT" == "PASSED" || "$TUS_TESTS_RESULT" == "PASSED" ]] && echo "true" || echo "false")
  },
  "individual_reports": {
    "standard_http_results_file": "${standard_results:-null}",
    "tus_resumable_results_file": "${tus_results:-null}",
    "master_log_file": "$MASTER_LOG"
  },
  "test_execution_summary": {
    "standard_tests_executed": $([[ -n "$STANDARD_TESTS_RESULT" ]] && echo "true" || echo "false"),
    "tus_tests_executed": $([[ -n "$TUS_TESTS_RESULT" ]] && echo "true" || echo "false"),
    "performance_analysis_completed": true,
    "recommendations_generated": true
  }
}
EOF
    
    print_color $GREEN "✓ Master results report generated: $MASTER_RESULTS"
}

# Function to display final summary
display_final_summary() {
    print_section "FINAL TEST SUITE SUMMARY"
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    print_color $BOLD "🎯 LARGE FILE UPLOAD TDD TEST RESULTS"
    echo | tee -a "$MASTER_LOG"
    
    # Overall status
    if [[ "$OVERALL_SUCCESS" == "true" ]]; then
        print_color $GREEN "🎉 OVERALL STATUS: READY FOR PRODUCTION"
        print_color $GREEN "   The sermon uploader can handle 500MB+ files reliably"
    else
        print_color $RED "⚠️  OVERALL STATUS: NEEDS CONFIGURATION"
        print_color $RED "   Large file uploads require attention before production use"
    fi
    
    echo | tee -a "$MASTER_LOG"
    
    # Individual test results
    print_color $BLUE "📊 Test Results Breakdown:"
    
    if [[ "$STANDARD_TESTS_RESULT" == "PASSED" ]]; then
        print_color $GREEN "   ✓ Standard HTTP Uploads: PASSED"
        print_color $CYAN "     Good for stable connections and smaller large files"
    else
        print_color $RED "   ✗ Standard HTTP Uploads: FAILED"
        print_color $YELLOW "     May need timeout or configuration adjustments"
    fi
    
    if [[ "$TUS_TESTS_RESULT" == "PASSED" ]]; then
        print_color $GREEN "   ✓ TUS Resumable Uploads: PASSED"
        print_color $CYAN "     Best for unstable connections and very large files"
    else
        print_color $RED "   ✗ TUS Resumable Uploads: FAILED"
        print_color $YELLOW "     May need TUS protocol configuration"
    fi
    
    echo | tee -a "$MASTER_LOG"
    
    # Sunday batch upload readiness
    print_color $BLUE "🎪 Sunday Batch Upload Readiness:"
    if [[ "$OVERALL_SUCCESS" == "true" ]]; then
        print_color $GREEN "   ✓ Ready for multiple 500MB+ sermon recordings"
        print_color $CYAN "     System can handle typical Sunday upload scenarios"
    else
        print_color $RED "   ✗ Not ready for Sunday batch uploads"
        print_color $YELLOW "     Address upload issues before Sunday use"
    fi
    
    echo | tee -a "$MASTER_LOG"
    
    # Performance insights
    print_color $BLUE "⚡ Performance Insights:"
    print_color $CYAN "   • Total test duration: ${total_duration}s"
    print_color $CYAN "   • Tested with files from: $TEST_FILES_DIR"
    print_color $CYAN "   • Production API: https://sermons.wpgc.church"
    
    echo | tee -a "$MASTER_LOG"
    
    # Next steps
    print_color $BLUE "🚀 Next Steps for Production:"
    
    if [[ "$OVERALL_SUCCESS" == "true" ]]; then
        print_color $CYAN "   1. System is ready for 500MB+ file uploads"
        print_color $CYAN "   2. Monitor first few production uploads for performance"
        print_color $CYAN "   3. Consider implementing progress indicators for users"
        print_color $CYAN "   4. Set up monitoring alerts for upload failures"
    else
        print_color $CYAN "   1. Review individual test logs for specific error details"
        print_color $CYAN "   2. Check server configuration and MinIO connectivity"
        print_color $CYAN "   3. Verify network stability and timeout settings"
        print_color $CYAN "   4. Re-run tests after addressing configuration issues"
    fi
    
    echo | tee -a "$MASTER_LOG"
    
    # Generated files
    print_color $BLUE "📁 Generated Test Files:"
    print_color $CYAN "   Master Log:     $MASTER_LOG"
    print_color $CYAN "   Master Report:  $MASTER_RESULTS"
    
    # List individual report files
    if ls /tmp/sermon_upload_results_* 1> /dev/null 2>&1; then
        local std_results
        std_results=$(ls -t /tmp/sermon_upload_results_* | head -1)
        print_color $CYAN "   Standard Tests: $std_results"
    fi
    
    if ls /tmp/sermon_tus_results_* 1> /dev/null 2>&1; then
        local tus_results
        tus_results=$(ls -t /tmp/sermon_tus_results_* | head -1)
        print_color $CYAN "   TUS Tests:      $tus_results"
    fi
    
    print_color $BLUE "=================================================================="
}

# Main execution function
main() {
    print_section "TDD Master Test Suite for Large File Uploads (500MB+)"
    print_color $BLUE "Testing both standard HTTP and TUS resumable upload methods"
    print_color $BLUE "Production API: https://sermons.wpgc.church"
    print_color $BLUE "Test scope: Files 500MB and larger only"
    echo
    
    # Initialize master log
    echo "Sermon Uploader Large File TDD Master Test Suite - $TIMESTAMP" > "$MASTER_LOG"
    echo "===============================================================" >> "$MASTER_LOG"
    echo "Testing both standard HTTP uploads and TUS resumable uploads" >> "$MASTER_LOG"
    echo "Focus: Files 500MB+ for Sunday sermon batch uploads" >> "$MASTER_LOG"
    echo "===============================================================" >> "$MASTER_LOG"
    
    # Execute test sequence
    check_master_prerequisites
    
    print_color $YELLOW "Starting comprehensive large file upload testing..."
    print_color $YELLOW "This may take significant time due to 500MB+ file transfers"
    echo
    
    # Run both test suites
    run_standard_tests
    echo | tee -a "$MASTER_LOG"
    
    run_tus_tests
    echo | tee -a "$MASTER_LOG"
    
    # Analyze and report
    analyze_combined_performance
    generate_master_report
    display_final_summary
    
    # Exit with appropriate code
    exit $([[ "$OVERALL_SUCCESS" == "true" ]] && echo 0 || echo 1)
}

# Set up cleanup
cleanup() {
    print_color $BLUE "Cleaning up temporary test files..."
    # Note: We keep the result files for analysis, just clean up intermediate temp files
    rm -f /tmp/temp_chunk* /tmp/perf_chunk* /tmp/partial_*
    rm -f /tmp/tus_*.tmp /tmp/curl_*.tmp /tmp/*_response.tmp
}

trap cleanup EXIT

# Show usage if requested
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    cat << EOF
TDD Master Test Suite for Large File Uploads (500MB+)

This script runs comprehensive tests for both standard HTTP uploads and 
TUS resumable uploads using files 500MB and larger.

REQUIREMENTS:
  • Run from Raspberry Pi at 192.168.1.195
  • Test files must be in: $TEST_FILES_DIR
  • Files must be WAV format and >= 500MB
  • Production API must be accessible: https://sermons.wpgc.church

TESTS PERFORMED:
  Standard HTTP Upload Tests:
    • API health check for large upload readiness
    • Single large file upload via /api/upload
    • Batch upload of multiple 500MB+ files  
    • Timeout handling for large transfers
    • Progress tracking during uploads
    • Error recovery and resilience

  TUS Resumable Upload Tests:
    • TUS protocol configuration discovery
    • TUS upload session creation for large files
    • Chunked upload with progress tracking
    • Resume capability after interruption
    • Performance with different chunk sizes
    • Error handling and recovery

OUTPUT:
  The script generates detailed logs and JSON reports for analysis.
  Exit code 0 = at least one upload method works for production
  Exit code 1 = both upload methods have issues

USAGE:
  $0 [--help]

EOF
    exit 0
fi

# Run main function
main "$@"