#!/bin/bash
# Go Resource Cleanup and Leak Detection for Raspberry Pi
# This script validates proper resource management critical for Pi deployment

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Resource Management Check (Pi Optimized) ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Function to log resource issue
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

# Function to analyze function for resource patterns
analyze_function() {
    local file="$1"
    local func_start="$2"
    local func_end="$3"
    local func_content
    
    func_content=$(sed -n "${func_start},${func_end}p" "$file")
    
    # Check for file operations without cleanup
    local file_opens=$(echo "$func_content" | grep -c "\.Open\|\.Create\|\.OpenFile" || echo 0)
    local file_closes=$(echo "$func_content" | grep -c "\.Close()\|defer.*\.Close()" || echo 0)
    
    if [[ $file_opens -gt $file_closes && $file_opens -gt 0 ]]; then
        log_issue "$file" "$func_start" "File operations ($file_opens) without matching Close() ($file_closes)" "ERROR" \
            "File descriptor leak will exhaust Pi system resources"
    fi
    
    # Check for HTTP requests without cleanup
    local http_requests=$(echo "$func_content" | grep -c "http\.Get\|http\.Post\|http\.Do" || echo 0)
    local body_closes=$(echo "$func_content" | grep -c "\.Body\.Close\|defer.*Body\.Close" || echo 0)
    
    if [[ $http_requests -gt $body_closes && $http_requests -gt 0 ]]; then
        log_issue "$file" "$func_start" "HTTP requests ($http_requests) without Body.Close() ($body_closes)" "ERROR" \
            "Connection leak will exhaust Pi network resources"
    fi
    
    # Check for database operations without cleanup
    local db_queries=$(echo "$func_content" | grep -c "\.Query\|\.Exec\|\.Begin" || echo 0)
    local db_closes=$(echo "$func_content" | grep -c "\.Close()\|defer.*\.Close()" || echo 0)
    
    if [[ $db_queries -gt 0 && $db_closes -eq 0 ]]; then
        log_issue "$file" "$func_start" "Database operations without proper cleanup" "WARNING" \
            "Database connection leak on Pi"
    fi
}

# Check for files to analyze
if [[ $# -eq 0 ]]; then
    echo "No Go files to check"
    exit 0
fi

echo "Analyzing Go files for resource management..."

for file in "$@"; do
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    echo "Checking: $file"
    
    # Find function boundaries for detailed analysis
    local func_lines=()
    while IFS= read -r line; do
        func_lines+=("$line")
    done <<< "$(grep -n "^func " "$file" || echo "")"
    
    for func_line in "${func_lines[@]}"; do
        if [[ -n "$func_line" ]]; then
            local func_start_line=$(echo "$func_line" | cut -d: -f1)
            local func_end_line
            func_end_line=$(awk -v start="$func_start_line" '
                NR >= start && /^func / && NR > start { print NR-1; exit }
                END { if (NR >= start) print NR }
            ' "$file")
            
            analyze_function "$file" "$func_start_line" "$func_end_line"
        fi
    done
    
    # Global resource leak patterns
    
    # Check for goroutine leaks
    if grep -n "go func(" "$file" > /dev/null 2>&1; then
        local goroutine_count=$(grep -c "go func(" "$file")
        local context_count=$(grep -c "context\." "$file" || echo 0)
        local channel_count=$(grep -c "chan\|<-" "$file" || echo 0)
        local waitgroup_count=$(grep -c "sync\.WaitGroup\|\.Wait()" "$file" || echo 0)
        
        if [[ $goroutine_count -gt 0 && $context_count -eq 0 && $channel_count -eq 0 && $waitgroup_count -eq 0 ]]; then
            log_issue "$file" "?" "Goroutines ($goroutine_count) without lifecycle management" "ERROR" \
                "Goroutine leak will consume Pi memory and CPU"
        fi
    fi
    
    # Check for timer/ticker leaks
    if grep -n "time\.NewTimer\|time\.NewTicker\|time\.After" "$file" > /dev/null 2>&1; then
        local timer_creates=$(grep -c "time\.NewTimer\|time\.NewTicker" "$file" || echo 0)
        local timer_stops=$(grep -c "\.Stop()\|defer.*\.Stop()" "$file" || echo 0)
        
        if [[ $timer_creates -gt $timer_stops && $timer_creates -gt 0 ]]; then
            log_issue "$file" "?" "Timers/Tickers ($timer_creates) without Stop() calls ($timer_stops)" "ERROR" \
                "Timer leak will consume Pi system resources"
        fi
        
        # Check for time.After in loops (creates timers that can't be stopped)
        if grep -n -B2 -A2 "for.*{" "$file" | grep "time\.After" > /dev/null 2>&1; then
            log_issue "$file" "?" "time.After used in loop (creates unstoppable timers)" "ERROR" \
                "Use time.NewTimer with Stop() to avoid Pi resource leak"
        fi
    fi
    
    # Check for memory leaks in slices
    if grep -n "append(" "$file" > /dev/null 2>&1; then
        local append_count=$(grep -c "append(" "$file")
        if [[ $append_count -gt 10 ]]; then
            if ! grep -n "cap(" "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "Multiple append operations ($append_count) without capacity management" "WARNING" \
                    "Consider pre-allocating slices to avoid Pi memory fragmentation"
            fi
        fi
    fi
    
    # Check for map leaks
    if grep -n "make(map\[" "$file" > /dev/null 2>&1; then
        local map_creates=$(grep -c "make(map\[" "$file")
        local map_clears=$(grep -c "delete\|clear(" "$file" || echo 0)
        
        if [[ $map_creates -gt 0 && $map_clears -eq 0 ]]; then
            local goroutine_count=$(grep -c "go func(" "$file" || echo 0)
            if [[ $goroutine_count -gt 0 ]]; then
                log_issue "$file" "?" "Maps created in concurrent code without cleanup" "WARNING" \
                    "Consider periodic cleanup to prevent Pi memory growth"
            fi
        fi
    fi
    
    # Check for channel leaks
    if grep -n "make(chan " "$file" > /dev/null 2>&1; then
        local chan_creates=$(grep -c "make(chan " "$file")
        local chan_closes=$(grep -c "close(" "$file" || echo 0)
        
        if [[ $chan_creates -gt $chan_closes && $chan_creates -gt 1 ]]; then
            log_issue "$file" "?" "Channels ($chan_creates) without close() calls ($chan_closes)" "WARNING" \
                "Unclosed channels may prevent Pi garbage collection"
        fi
    fi
    
    # Check for context leaks
    if grep -n "context\.WithCancel\|context\.WithTimeout\|context\.WithDeadline" "$file" > /dev/null 2>&1; then
        local context_creates=$(grep -c "context\.With" "$file")
        local cancel_calls=$(grep -c "cancel()\|defer.*cancel()" "$file" || echo 0)
        
        if [[ $context_creates -gt $cancel_calls && $context_creates -gt 0 ]]; then
            log_issue "$file" "?" "Context with cancel ($context_creates) without cancel() calls ($cancel_calls)" "ERROR" \
                "Context leak will consume Pi resources"
        fi
    fi
    
    # Check for finalizer usage (can delay garbage collection)
    if grep -n "runtime\.SetFinalizer" "$file" > /dev/null 2>&1; then
        log_issue "$file" "?" "Using runtime.SetFinalizer" "WARNING" \
            "Finalizers can delay GC on Pi; prefer explicit cleanup"
    fi
    
    # Check for large buffer reuse patterns
    if grep -n "bytes\.Buffer\|bytes\.NewBuffer" "$file" > /dev/null 2>&1; then
        local buffer_count=$(grep -c "bytes\.Buffer\|bytes\.NewBuffer" "$file")
        if [[ $buffer_count -gt 3 ]]; then
            if ! grep -n "sync\.Pool\|Reset()" "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "Multiple buffer allocations ($buffer_count) without pooling" "WARNING" \
                    "Use sync.Pool for buffer reuse on Pi"
            fi
        fi
    fi
    
    # Check for string builder cleanup
    if grep -n "strings\.Builder" "$file" > /dev/null 2>&1; then
        local builder_count=$(grep -c "strings\.Builder" "$file")
        local reset_count=$(grep -c "\.Reset()" "$file" || echo 0)
        
        if [[ $builder_count -gt 1 && $reset_count -eq 0 ]]; then
            log_issue "$file" "?" "String builders without Reset() calls" "WARNING" \
                "Reset builders for reuse to save Pi memory"
        fi
    fi
    
    # Check for crypto resource cleanup
    if grep -n "crypto\.\|tls\." "$file" > /dev/null 2>&1; then
        if ! grep -n "defer\|Close" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Crypto operations without explicit cleanup" "WARNING" \
                "Crypto resources should be explicitly cleaned on Pi"
        fi
    fi
    
    # Check for temporary file cleanup
    if grep -n "ioutil\.TempFile\|os\.CreateTemp" "$file" > /dev/null 2>&1; then
        local temp_creates=$(grep -c "TempFile\|CreateTemp" "$file")
        local temp_removes=$(grep -c "os\.Remove\|defer.*Remove" "$file" || echo 0)
        
        if [[ $temp_creates -gt $temp_removes && $temp_creates -gt 0 ]]; then
            log_issue "$file" "?" "Temporary files ($temp_creates) without removal ($temp_removes)" "ERROR" \
                "Temp files will fill Pi storage"
        fi
    fi
    
    # Check for signal handling cleanup
    if grep -n "signal\.Notify" "$file" > /dev/null 2>&1; then
        if ! grep -n "signal\.Stop\|signal\.Reset" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Signal handlers without cleanup" "WARNING" \
                "Signal handlers should be cleaned up on Pi"
        fi
    fi
    
done

# Resource management guidelines for Pi
echo -e "\n${BLUE}=== Pi Resource Management Guidelines ===${NC}"
echo -e "🔧 Files: Always use defer file.Close()"
echo -e "🔧 HTTP: Always close response bodies"
echo -e "🔧 Goroutines: Use context or channels for lifecycle"
echo -e "🔧 Timers: Always call Stop() on created timers"
echo -e "🔧 Channels: Close channels when done"
echo -e "🔧 Contexts: Always call cancel functions"
echo -e "🔧 Buffers: Use sync.Pool for reuse"
echo -e "🔧 Temp files: Always remove temporary files"

# Summary
echo -e "\n${BLUE}=== Resource Management Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Resource management check failed! Critical resource leaks found.${NC}"
    echo -e "${YELLOW}💡 These leaks will exhaust Pi system resources.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Resource management check passed with warnings. Monitor Pi resource usage.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ No resource management issues detected!${NC}"
    exit 0
fi