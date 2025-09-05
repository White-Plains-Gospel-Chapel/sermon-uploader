#!/bin/bash
# Raspberry Pi Specific Go Optimization Validation
# This script validates Pi-specific optimizations and best practices

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Raspberry Pi Go Optimization Validation ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Pi-specific constraints
PI_MAX_GOROUTINES=100
PI_MAX_FILE_HANDLES=1024
PI_MAX_MEMORY_PER_ALLOCATION=100 # MB

# Function to log Pi optimization issue
log_issue() {
    local file="$1"
    local line="$2"
    local issue="$3"
    local severity="$4"
    local pi_impact="${5:-}"
    
    if [[ "$severity" == "ERROR" ]]; then
        echo -e "${RED}ERROR${NC}: $file:$line - $issue"
        [[ -n "$pi_impact" ]] && echo -e "  ${BLUE}🔧 Pi Impact${NC}: $pi_impact"
        ((ISSUES_FOUND++))
    else
        echo -e "${YELLOW}WARNING${NC}: $file:$line - $issue"
        [[ -n "$pi_impact" ]] && echo -e "  ${BLUE}🔧 Pi Impact${NC}: $pi_impact"
        ((WARNINGS_FOUND++))
    fi
}

# Check for files to analyze
if [[ $# -eq 0 ]]; then
    echo "No Go files to check"
    exit 0
fi

echo "Analyzing Go files for Pi-specific optimizations..."

for file in "$@"; do
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    echo "Checking: $file"
    
    # Check for GOMAXPROCS consideration
    if grep -n "runtime\.GOMAXPROCS\|GOMAXPROCS" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            if [[ "$match" =~ GOMAXPROCS\(([0-9]+)\) ]]; then
                local procs="${BASH_REMATCH[1]}"
                if [[ $procs -gt 8 ]]; then
                    log_issue "$file" "$line_num" "GOMAXPROCS set to $procs (Pi has max 8 cores)" "WARNING" \
                        "Pi 4/5 has 4-8 cores, setting higher may reduce performance"
                fi
            fi
        done <<< "$(grep -n "runtime\.GOMAXPROCS\|GOMAXPROCS" "$file")"
    fi
    
    # Check for CPU-intensive operations without context
    if grep -n "for.*{" "$file" | grep -v "range" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            # Look for CPU-intensive loops
            local loop_content=$(sed -n "${line_num},$((line_num + 10))p" "$file")
            if echo "$loop_content" | grep -E "(math\.|crypto\.|regexp\.|json\.)" > /dev/null 2>&1; then
                if ! echo "$loop_content" | grep "context\|time\.After\|select" > /dev/null 2>&1; then
                    log_issue "$file" "$line_num" "CPU-intensive loop without context or yield" "WARNING" \
                        "Pi CPU may be overwhelmed without cooperative yielding"
                fi
            fi
        done <<< "$(grep -n "for.*{" "$file" | grep -v "range")"
    fi
    
    # Check for ARM64-specific optimizations
    if grep -n "unsafe\." "$file" > /dev/null 2>&1; then
        log_issue "$file" "?" "Using unsafe package" "WARNING" \
            "Verify ARM64 compatibility and memory alignment on Pi"
    fi
    
    # Check for thermal throttling considerations
    if grep -n "time\.Sleep\|time\.Tick" "$file" > /dev/null 2>&1; then
        local sleep_count=$(grep -c "time\.Sleep" "$file" || echo 0)
        if [[ $sleep_count -eq 0 ]]; then
            local intensive_ops=$(grep -c "crypto\.\|compress\.\|image\." "$file" || echo 0)
            if [[ $intensive_ops -gt 5 ]]; then
                log_issue "$file" "?" "CPU-intensive operations without thermal breaks" "WARNING" \
                    "Pi may thermal throttle without periodic breaks"
            fi
        fi
    fi
    
    # Check for memory-mapped file usage
    if grep -n "mmap\|syscall\.Mmap" "$file" > /dev/null 2>&1; then
        log_issue "$file" "?" "Memory-mapped files detected" "WARNING" \
            "Monitor Pi memory usage with mmap operations"
    fi
    
    # Check for network buffer optimization
    if grep -n "net\.Conn\|http\." "$file" > /dev/null 2>&1; then
        if ! grep -n "SetReadBuffer\|SetWriteBuffer\|bufio\." "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Network operations without buffer optimization" "WARNING" \
                "Pi networking benefits from properly sized buffers"
        fi
    fi
    
    # Check for file I/O optimization
    if grep -n "os\.Open\|ioutil\." "$file" > /dev/null 2>&1; then
        if ! grep -n "bufio\.\|ReadAll\|WriteAll" "$file" > /dev/null 2>&1; then
            local file_ops=$(grep -c "os\.Open\|os\.Create" "$file" || echo 0)
            if [[ $file_ops -gt 3 ]]; then
                log_issue "$file" "?" "Multiple file operations without buffering" "WARNING" \
                    "Pi SD card benefits from buffered I/O operations"
            fi
        fi
    fi
    
    # Check for goroutine pool implementation
    if grep -n "go func(" "$file" > /dev/null 2>&1; then
        local goroutine_count=$(grep -c "go func(" "$file")
        if [[ $goroutine_count -gt 20 ]]; then
            if ! grep -n "pool\|worker\|semaphore" "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "High goroutine count ($goroutine_count) without pooling" "ERROR" \
                    "Pi should limit concurrent goroutines to avoid resource exhaustion"
            fi
        fi
    fi
    
    # Check for caching implementation
    if grep -n "http\.Get\|http\.Post\|database\." "$file" > /dev/null 2>&1; then
        if ! grep -n "cache\|Cache\|lru\|sync\.Map" "$file" > /dev/null 2>&1; then
            local external_calls=$(grep -c "http\.\|database\." "$file" || echo 0)
            if [[ $external_calls -gt 5 ]]; then
                log_issue "$file" "?" "Multiple external calls without caching" "WARNING" \
                    "Pi benefits from caching to reduce network/disk I/O"
            fi
        fi
    fi
    
    # Check for JSON streaming with large files
    if grep -n "json\.Unmarshal\|json\.Marshal" "$file" > /dev/null 2>&1; then
        if grep -n "[\[\]]byte" "$file" > /dev/null 2>&1; then
            if ! grep -n "json\.Decoder\|json\.Encoder" "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "Large JSON operations without streaming" "WARNING" \
                    "Pi should stream large JSON to avoid memory pressure"
            fi
        fi
    fi
    
    # Check for compression usage
    if grep -n "gzip\|compress\|deflate" "$file" > /dev/null 2>&1; then
        if ! grep -n "BestSpeed\|level.*1" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Compression without Pi-optimized settings" "WARNING" \
                "Use fast compression levels to avoid Pi CPU bottleneck"
        fi
    fi
    
    # Check for database connection pooling
    if grep -n "sql\.Open\|database/sql" "$file" > /dev/null 2>&1; then
        if ! grep -n "SetMaxOpenConns\|SetMaxIdleConns" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Database connections without Pi-appropriate limits" "WARNING" \
                "Pi should limit database connections (suggest 5-10 max)"
        fi
    fi
    
    # Check for image processing optimization
    if grep -n "image\.\|draw\." "$file" > /dev/null 2>&1; then
        if ! grep -n "resize\|thumbnail" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Image processing without size optimization" "WARNING" \
                "Pi should process smaller images to avoid memory/CPU pressure"
        fi
    fi
    
    # Check for logging optimization
    if grep -n "log\.\|fmt\.Print" "$file" > /dev/null 2>&1; then
        local log_count=$(grep -c "log\.\|fmt\.Print" "$file" || echo 0)
        if [[ $log_count -gt 10 ]]; then
            if ! grep -n "sync\.Once\|buffer\|async" "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "High-frequency logging without optimization" "WARNING" \
                    "Pi I/O can be bottleneck; use async logging or buffering"
            fi
        fi
    fi
    
    # Check for time zone handling (Pi often has limited TZ data)
    if grep -n "time\.LoadLocation\|time\.Parse.*Z" "$file" > /dev/null 2>&1; then
        if ! grep -n "UTC\|time\.UTC" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Complex timezone handling" "WARNING" \
                "Pi may have limited timezone data; prefer UTC when possible"
        fi
    fi
    
    # Check for reflection usage (slower on ARM)
    if grep -n "reflect\." "$file" > /dev/null 2>&1; then
        local reflect_count=$(grep -c "reflect\." "$file")
        if [[ $reflect_count -gt 3 ]]; then
            log_issue "$file" "?" "Heavy reflection usage ($reflect_count calls)" "WARNING" \
                "Reflection is slower on Pi ARM; consider code generation"
        fi
    fi
    
    # Check for crypto operations (Pi has hardware acceleration)
    if grep -n "crypto/" "$file" > /dev/null 2>&1; then
        if ! grep -n "AES\|SHA" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Crypto operations that may not use Pi hardware acceleration" "WARNING" \
                "Pi 4/5 has hardware acceleration for AES and SHA"
        fi
    fi
    
    # Check for build constraints for Pi
    if ! grep -n "//.*build.*linux\|//.*build.*arm" "$file" > /dev/null 2>&1; then
        local platform_specific=$(grep -c "syscall\.\|unsafe\.\|runtime\." "$file" || echo 0)
        if [[ $platform_specific -gt 0 ]]; then
            log_issue "$file" "?" "Platform-specific code without build constraints" "WARNING" \
                "Add build constraints for Pi-specific optimizations"
        fi
    fi
    
done

# Pi-specific recommendations
echo -e "\n${BLUE}=== Pi Optimization Guidelines ===${NC}"
echo -e "🔧 CPU: Use GOMAXPROCS(4-8), avoid CPU-intensive loops"
echo -e "🔧 Memory: Limit allocations, use streaming for large data"
echo -e "🔧 I/O: Buffer file/network operations, use async when possible"
echo -e "🔧 Thermal: Add breaks in intensive operations"
echo -e "🔧 Network: Optimize buffer sizes for Pi networking"
echo -e "🔧 Storage: Minimize SD card writes, use compression"

# Summary
echo -e "\n${BLUE}=== Pi Optimization Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Pi optimization check failed! Critical Pi-specific issues found.${NC}"
    echo -e "${YELLOW}💡 These issues can significantly impact Pi performance.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Pi optimization check passed with warnings. Consider Pi-specific optimizations.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ Pi optimization patterns look good!${NC}"
    exit 0
fi