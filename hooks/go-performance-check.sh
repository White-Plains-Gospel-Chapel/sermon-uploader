#!/bin/bash
# Go Performance Anti-Pattern Detection for Raspberry Pi
# This script detects performance anti-patterns that are critical on Pi hardware

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Performance Anti-Pattern Detection (Pi Optimized) ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Function to log performance issue
log_issue() {
    local file="$1"
    local line="$2"
    local issue="$3"
    local severity="$4"
    
    if [[ "$severity" == "ERROR" ]]; then
        echo -e "${RED}ERROR${NC}: $file:$line - $issue"
        ((ISSUES_FOUND++))
    else
        echo -e "${YELLOW}WARNING${NC}: $file:$line - $issue"
        ((WARNINGS_FOUND++))
    fi
}

# Check for files to analyze
if [[ $# -eq 0 ]]; then
    echo "No Go files to check"
    exit 0
fi

echo "Analyzing Go files for Pi performance anti-patterns..."

for file in "$@"; do
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    echo "Checking: $file"
    
    # Check for large slice allocations without capacity
    if grep -n "make(\[\].*)" "$file" | grep -v ", [0-9]" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Large slice allocation without capacity specification (Pi memory constraint)" "ERROR"
        done <<< "$(grep -n "make(\[\].*)" "$file" | grep -v ", [0-9]")"
    fi
    
    # Check for string concatenation in loops
    if grep -n -A5 -B5 "for.*range\|for.*;" "$file" | grep -A10 -B10 "+=" | grep "string" > /dev/null 2>&1; then
        log_issue "$file" "?" "Potential string concatenation in loop (use strings.Builder for Pi efficiency)" "ERROR"
    fi
    
    # Check for defer in loops
    if grep -n -A3 -B3 "for.*{" "$file" | grep "defer" > /dev/null 2>&1; then
        log_issue "$file" "?" "Defer statement inside loop (Pi memory pressure concern)" "ERROR"
    fi
    
    # Check for large struct copies
    if grep -n "func.*) \w\+(" "$file" | grep -v "\*" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            # Only flag if the struct is likely large (has multiple fields)
            if [[ $(echo "$match" | grep -o "{" | wc -l) -gt 5 ]]; then
                log_issue "$file" "$line_num" "Large struct passed by value instead of pointer (Pi performance concern)" "WARNING"
            fi
        done <<< "$(grep -n "func.*) \w\+(" "$file" | grep -v "\*")"
    fi
    
    # Check for inefficient map operations
    if grep -n "_, ok := .*\[" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            if [[ "$match" =~ delete\( ]]; then
                log_issue "$file" "$line_num" "Check before delete is redundant (delete on non-existent key is safe)" "WARNING"
            fi
        done <<< "$(grep -n "_, ok := .*\[" "$file")"
    fi
    
    # Check for goroutine leaks - missing context cancellation
    if grep -n "go func(" "$file" > /dev/null 2>&1; then
        if ! grep -n "context\." "$file" > /dev/null 2>&1; then
            log_issue "$file" "?" "Goroutine launched without context (potential leak on Pi)" "ERROR"
        fi
    fi
    
    # Check for sync.Mutex in struct without pointer receiver
    if grep -n -A5 "sync\.Mutex" "$file" | grep "func.*) " | grep -v "\*" > /dev/null 2>&1; then
        log_issue "$file" "?" "Mutex in struct with value receiver (copy will break locking)" "ERROR"
    fi
    
    # Check for inefficient JSON parsing
    if grep -n "json\.Unmarshal.*interface{}" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "JSON unmarshal to interface{} is inefficient (use specific struct on Pi)" "WARNING"
        done <<< "$(grep -n "json\.Unmarshal.*interface{}" "$file")"
    fi
    
    # Check for inefficient error handling
    if grep -n "errors\.New.*fmt\.Sprintf" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Use fmt.Errorf instead of errors.New(fmt.Sprintf(...)) for Pi efficiency" "WARNING"
        done <<< "$(grep -n "errors\.New.*fmt\.Sprintf" "$file")"
    fi
    
    # Check for large buffer allocations
    if grep -n "make(\[\]byte, [0-9]\{6,\}" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Large buffer allocation (>100KB) may cause Pi memory pressure" "WARNING"
        done <<< "$(grep -n "make(\[\]byte, [0-9]\{6,\}" "$file")"
    fi
    
    # Check for time.After in select without cleanup
    if grep -n -A10 "select {" "$file" | grep "time\.After" > /dev/null 2>&1; then
        log_issue "$file" "?" "time.After in select creates timer that may leak (use time.NewTimer with Stop())" "WARNING"
    fi
    
    # Check for inefficient regular expressions
    if grep -n "regexp\.MustCompile" "$file" | grep -v "var.*=" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Regexp compiled in function (move to package level variable for Pi efficiency)" "WARNING"
        done <<< "$(grep -n "regexp\.MustCompile" "$file" | grep -v "var.*=")"
    fi
    
    # Check for inefficient logging
    if grep -n "fmt\.Printf.*%.*%.*%" "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Multiple formatting operations in single printf (Pi CPU concern)" "WARNING"
        done <<< "$(grep -n "fmt\.Printf.*%.*%.*%" "$file")"
    fi
    
    # Check for missing buffer pools
    if grep -n "bytes\.Buffer" "$file" | grep -v "sync\.Pool" > /dev/null 2>&1; then
        if grep -c "bytes\.Buffer" "$file" > 5 2>/dev/null; then
            log_issue "$file" "?" "Multiple Buffer allocations detected (consider sync.Pool for Pi memory efficiency)" "WARNING"
        fi
    fi
    
    # Check for inefficient slice operations
    if grep -n "append.*\.\.\." "$file" | grep -v "make.*cap" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            log_issue "$file" "$line_num" "Slice append with spread may cause multiple reallocations (Pi memory concern)" "WARNING"
        done <<< "$(grep -n "append.*\.\.\." "$file" | grep -v "make.*cap")"
    fi
    
done

# Summary
echo -e "\n${BLUE}=== Performance Check Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Performance check failed! Critical Pi performance issues found.${NC}"
    echo -e "${YELLOW}💡 These issues are critical on Raspberry Pi hardware with limited resources.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Performance check passed with warnings. Consider addressing for optimal Pi performance.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ No performance anti-patterns detected!${NC}"
    exit 0
fi