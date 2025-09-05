#!/bin/bash
# Go Memory Usage Pattern Validation for Raspberry Pi
# This script validates memory usage patterns critical for Pi deployment

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Memory Pattern Validation (Pi Optimized) ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Pi memory constraints (in MB)
PI_TOTAL_MEMORY=8192  # 8GB Pi 4/5
PI_AVAILABLE_MEMORY=6144  # ~75% available for applications
PI_CRITICAL_THRESHOLD=4096  # When to be very careful

# Function to log memory issue
log_issue() {
    local file="$1"
    local line="$2"
    local issue="$3"
    local severity="$4"
    local suggestion="${5:-}"
    
    if [[ "$severity" == "ERROR" ]]; then
        echo -e "${RED}ERROR${NC}: $file:$line - $issue"
        [[ -n "$suggestion" ]] && echo -e "  ${BLUE}💡 Suggestion${NC}: $suggestion"
        ((ISSUES_FOUND++))
    else
        echo -e "${YELLOW}WARNING${NC}: $file:$line - $issue"
        [[ -n "$suggestion" ]] && echo -e "  ${BLUE}💡 Suggestion${NC}: $suggestion"
        ((WARNINGS_FOUND++))
    fi
}

# Function to estimate slice memory usage
estimate_slice_memory() {
    local type="$1"
    local size="$2"
    
    case "$type" in
        "byte"|"uint8") echo $((size * 1)) ;;
        "int16"|"uint16") echo $((size * 2)) ;;
        "int32"|"uint32"|"float32") echo $((size * 4)) ;;
        "int64"|"uint64"|"float64"|"int"|"uint") echo $((size * 8)) ;;
        "string") echo $((size * 16)) ;; # Approximate string overhead
        *) echo $((size * 8)) ;; # Default to 8 bytes
    esac
}

# Check for files to analyze
if [[ $# -eq 0 ]]; then
    echo "No Go files to check"
    exit 0
fi

echo "Analyzing Go files for Pi memory patterns..."

for file in "$@"; do
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    echo "Checking: $file"
    
    # Check for large slice allocations
    if grep -n "make(\[\]" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            # Extract type and size
            if [[ "$match" =~ make\(\[\]([^,\)]+)[^0-9]*([0-9]+) ]]; then
                local slice_type="${BASH_REMATCH[1]}"
                local slice_size="${BASH_REMATCH[2]}"
                local memory_mb=$(($(estimate_slice_memory "$slice_type" "$slice_size") / 1024 / 1024))
                
                if [[ $memory_mb -gt 100 ]]; then
                    log_issue "$file" "$line_num" "Large slice allocation: ${memory_mb}MB (${slice_size} ${slice_type})" "ERROR" \
                        "Consider streaming processing or sync.Pool for large data"
                elif [[ $memory_mb -gt 50 ]]; then
                    log_issue "$file" "$line_num" "Medium slice allocation: ${memory_mb}MB (${slice_size} ${slice_type})" "WARNING" \
                        "Monitor memory usage on Pi"
                fi
            fi
        done <<< "$(grep -n "make(\[\]" "$file")"
    fi
    
    # Check for map allocations without size hint
    if grep -n "make(map\[" "$file" | grep -v ", [0-9]" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Map allocation without size hint" "WARNING" \
                "Use make(map[K]V, expectedSize) for better Pi memory allocation"
        done <<< "$(grep -n "make(map\[" "$file" | grep -v ", [0-9]")"
    fi
    
    # Check for potential memory leaks - missing Close() calls
    if grep -n "\\.Open\|\\.Create\|\\.OpenFile" "$file" > /dev/null 2>&1; then
        local open_calls=$(grep -c "\\.Open\|\\.Create\|\\.OpenFile" "$file")
        local close_calls=$(grep -c "\\.Close()" "$file")
        local defer_close_calls=$(grep -c "defer.*\\.Close()" "$file")
        
        if [[ $((close_calls + defer_close_calls)) -lt $open_calls ]]; then
            log_issue "$file" "?" "Potential resource leak: $open_calls Open calls vs $((close_calls + defer_close_calls)) Close calls" "ERROR" \
                "Ensure all opened resources are closed with defer"
        fi
    fi
    
    # Check for goroutine memory leaks
    if grep -n "go func(" "$file" > /dev/null 2>&1; then
        local goroutine_count=$(grep -c "go func(" "$file")
        local context_count=$(grep -c "context\." "$file")
        local channel_count=$(grep -c "chan\|<-" "$file")
        
        if [[ $context_count -eq 0 && $channel_count -eq 0 && $goroutine_count -gt 0 ]]; then
            log_issue "$file" "?" "$goroutine_count goroutines without context or channels (potential leak)" "ERROR" \
                "Use context.Context for goroutine lifecycle management on Pi"
        fi
    fi
    
    # Check for string building inefficiency
    if grep -n "+=" "$file" | grep -B2 -A2 "string\|String" > /dev/null 2>&1; then
        local string_concat_count=$(grep -c "+=" "$file" | head -1)
        if [[ $string_concat_count -gt 3 ]]; then
            log_issue "$file" "?" "Multiple string concatenations detected ($string_concat_count)" "WARNING" \
                "Use strings.Builder for efficient string building on Pi"
        fi
    fi
    
    # Check for inefficient buffer usage
    if grep -n "bytes\.Buffer" "$file" > /dev/null 2>&1; then
        if ! grep -n "sync\.Pool\|bufferPool" "$file" > /dev/null 2>&1; then
            local buffer_count=$(grep -c "bytes\.Buffer" "$file")
            if [[ $buffer_count -gt 2 ]]; then
                log_issue "$file" "?" "Multiple buffer allocations ($buffer_count) without pooling" "WARNING" \
                    "Consider sync.Pool for buffer reuse on Pi"
            fi
        fi
    fi
    
    # Check for large constant arrays/slices
    if grep -n "var.*=.*\[\].*{" "$file" | grep -A1 -B1 "," > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            local element_count=$(grep -A20 "$match" "$file" | grep -o "," | wc -l)
            if [[ $element_count -gt 1000 ]]; then
                log_issue "$file" "$line_num" "Large constant array/slice ($element_count elements)" "WARNING" \
                    "Consider loading from external file or using embedded resources"
            fi
        done <<< "$(grep -n "var.*=.*\[\].*{" "$file")"
    fi
    
    # Check for inefficient JSON handling
    if grep -n "json\.Marshal\|json\.Unmarshal" "$file" > /dev/null 2>&1; then
        if ! grep -n "json\.Decoder\|json\.Encoder" "$file" > /dev/null 2>&1; then
            local json_calls=$(grep -c "json\.Marshal\|json\.Unmarshal" "$file")
            if [[ $json_calls -gt 5 ]]; then
                log_issue "$file" "?" "Multiple JSON marshal/unmarshal calls ($json_calls) without streaming" "WARNING" \
                    "Consider json.Decoder/Encoder for streaming on Pi"
            fi
        fi
    fi
    
    # Check for memory-intensive regex compilation
    if grep -n "regexp\.Compile\|regexp\.MustCompile" "$file" | grep -v "var\|const" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Regex compiled in function (memory overhead)" "WARNING" \
                "Move regex compilation to package-level variable"
        done <<< "$(grep -n "regexp\.Compile\|regexp\.MustCompile" "$file" | grep -v "var\|const")"
    fi
    
    # Check for potential slice leaks
    if grep -n "\[:.*\]" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            if [[ "$match" =~ \[:[0-9]+\] ]]; then
                log_issue "$file" "$line_num" "Slice operation may retain large underlying array" "WARNING" \
                    "Consider copy() for small slices from large arrays on Pi"
            fi
        done <<< "$(grep -n "\[:.*\]" "$file")"
    fi
    
    # Check for HTTP client without timeout (can cause memory buildup)
    if grep -n "http\.Client\|http\.Get\|http\.Post" "$file" > /dev/null 2>&1; then
        if ! grep -n "Timeout\|context\.With" "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "HTTP operations without timeout (can cause memory buildup)" "ERROR" \
                "Always set timeouts for HTTP operations on Pi"
        fi
    fi
    
    # Check for channels without buffer size consideration
    if grep -n "make(chan " "$file" | grep -v ", [0-9]" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Unbuffered channel may cause goroutine blocking" "WARNING" \
                "Consider buffered channels for better Pi performance"
        done <<< "$(grep -n "make(chan " "$file" | grep -v ", [0-9]")"
    fi
    
    # Check for interface{} usage (boxing overhead)
    if grep -n "interface{}" "$file" | grep -v "test\|Test" > /dev/null 2>&1; then
        local interface_count=$(grep -c "interface{}" "$file")
        if [[ $interface_count -gt 3 ]]; then
            log_issue "$file" "?" "Multiple interface{} usage ($interface_count) causes boxing overhead" "WARNING" \
                "Use specific types when possible for Pi efficiency"
        fi
    fi
    
done

# Memory usage estimation
echo -e "\n${BLUE}=== Memory Usage Guidelines for Pi ===${NC}"
echo -e "Pi Total Memory: ${PI_TOTAL_MEMORY}MB"
echo -e "Available for Apps: ${PI_AVAILABLE_MEMORY}MB" 
echo -e "Critical Threshold: ${PI_CRITICAL_THRESHOLD}MB"

# Summary
echo -e "\n${BLUE}=== Memory Check Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Memory check failed! Critical Pi memory issues found.${NC}"
    echo -e "${YELLOW}💡 These issues can cause OOM conditions on Raspberry Pi.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Memory check passed with warnings. Monitor memory usage on Pi.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ No memory pattern issues detected!${NC}"
    exit 0
fi