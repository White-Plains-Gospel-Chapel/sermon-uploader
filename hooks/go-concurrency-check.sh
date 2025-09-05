#!/bin/bash
# Go Concurrency and Goroutine Safety Check for Raspberry Pi
# This script validates concurrent programming patterns for Pi deployment

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Concurrency Safety Check (Pi Optimized) ===${NC}"

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0

# Function to log concurrency issue
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

# Check for files to analyze
if [[ $# -eq 0 ]]; then
    echo "No Go files to check"
    exit 0
fi

echo "Analyzing Go files for concurrency safety..."

for file in "$@"; do
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    echo "Checking: $file"
    
    # Check for goroutines without proper lifecycle management
    if grep -n "go func(" "$file" > /dev/null 2>&1; then
        local goroutine_count=$(grep -c "go func(" "$file")
        local context_usage=$(grep -c "context\." "$file" || echo 0)
        local channel_usage=$(grep -c "chan\|<-.*chan\|chan.*<-" "$file" || echo 0)
        
        if [[ $context_usage -eq 0 && $channel_usage -eq 0 ]]; then
            log_issue "$file" "?" "$goroutine_count goroutines without lifecycle management" "ERROR" \
                "Use context.Context or channels for goroutine coordination"
        fi
        
        # Check for goroutines in loops
        if grep -n -B2 -A2 "for.*{" "$file" | grep "go func(" > /dev/null 2>&1; then
            log_issue "$file" "?" "Goroutine creation inside loop" "ERROR" \
                "Use worker pool pattern to limit goroutine count on Pi"
        fi
    fi
    
    # Check for race conditions with shared variables
    if grep -n "var.*=.*" "$file" | grep -v "const\|func" > /dev/null 2>&1; then
        local global_vars=$(grep -c "var.*=" "$file" | head -1)
        local mutex_usage=$(grep -c "sync\.Mutex\|sync\.RWMutex" "$file" || echo 0)
        local atomic_usage=$(grep -c "sync/atomic\|atomic\." "$file" || echo 0)
        
        if [[ $global_vars -gt 0 && $mutex_usage -eq 0 && $atomic_usage -eq 0 ]]; then
            local goroutine_count=$(grep -c "go func(" "$file" || echo 0)
            if [[ $goroutine_count -gt 0 ]]; then
                log_issue "$file" "?" "Global variables without synchronization in concurrent code" "ERROR" \
                    "Use sync.Mutex, sync.RWMutex, or atomic operations"
            fi
        fi
    fi
    
    # Check for channel operations without proper handling
    if grep -n "<-" "$file" > /dev/null 2>&1; then
        # Check for channel operations without select or timeout
        if ! grep -n "select\|context\.WithTimeout\|time\.After" "$file" > /dev/null 2>&1; then
            local channel_ops=$(grep -c "<-" "$file")
            if [[ $channel_ops -gt 1 ]]; then
                log_issue "$file" "?" "Channel operations without timeout handling" "WARNING" \
                    "Use select with timeout for robust Pi operation"
            fi
        fi
        
        # Check for channel closing without synchronization
        if grep -n "close(" "$file" > /dev/null 2>&1; then
            if ! grep -n "sync\.\|atomic\." "$file" > /dev/null 2>&1; then
                log_issue "$file" "?" "Channel close without synchronization" "WARNING" \
                    "Ensure only one goroutine closes the channel"
            fi
        fi
    fi
    
    # Check for mutex usage patterns
    if grep -n "sync\.Mutex\|sync\.RWMutex" "$file" > /dev/null 2>&1; then
        # Check for mutex in struct without pointer receiver
        if grep -n -A10 "type.*struct" "$file" | grep -A10 "sync\..*Mutex" | grep -A5 "func.*)" | grep -v "\*" > /dev/null 2>&1; then
            log_issue "$file" "?" "Mutex in struct with value receiver" "ERROR" \
                "Use pointer receivers for methods on types with mutexes"
        fi
        
        # Check for defer unlock pattern
        local mutex_count=$(grep -c "\.Lock()" "$file" || echo 0)
        local defer_unlock_count=$(grep -c "defer.*\.Unlock()" "$file" || echo 0)
        
        if [[ $mutex_count -gt $defer_unlock_count && $defer_unlock_count -gt 0 ]]; then
            log_issue "$file" "?" "Not all mutex locks use defer unlock pattern" "WARNING" \
                "Always use 'defer mu.Unlock()' after 'mu.Lock()'"
        fi
    fi
    
    # Check for context usage patterns
    if grep -n "context\." "$file" > /dev/null 2>&1; then
        # Check for context.Background() usage in non-main functions
        if grep -n "context\.Background()" "$file" | grep -v "func main\|func init" > /dev/null 2>&1; then
            log_issue "$file" "?" "context.Background() used outside main/init" "WARNING" \
                "Propagate context from caller instead"
        fi
        
        # Check for context not being first parameter
        if grep -n "func.*context\." "$file" | grep -v "func.*ctx.*context\." > /dev/null 2>&1; then
            log_issue "$file" "?" "Context not as first parameter" "WARNING" \
                "Context should be the first parameter in function signature"
        fi
    fi
    
    # Check for waitgroup usage
    if grep -n "sync\.WaitGroup" "$file" > /dev/null 2>&1; then
        local wg_add_count=$(grep -c "\.Add(" "$file" || echo 0)
        local wg_done_count=$(grep -c "\.Done()" "$file" || echo 0)
        local wg_wait_count=$(grep -c "\.Wait()" "$file" || echo 0)
        
        if [[ $wg_wait_count -eq 0 && $wg_add_count -gt 0 ]]; then
            log_issue "$file" "?" "WaitGroup Add() without Wait()" "ERROR" \
                "Always call Wait() after adding to WaitGroup"
        fi
        
        if [[ $wg_add_count -ne $wg_done_count && $wg_done_count -gt 0 ]]; then
            log_issue "$file" "?" "WaitGroup Add/Done count mismatch" "ERROR" \
                "Ensure Done() is called exactly once for each Add()"
        fi
    fi
    
    # Check for atomic operations
    if grep -n "sync/atomic\|atomic\." "$file" > /dev/null 2>&1; then
        # Check for mixing atomic and non-atomic access
        if grep -n "atomic\." "$file" | grep "Load\|Store" > /dev/null 2>&1; then
            log_issue "$file" "?" "Using atomic operations (verify consistent usage)" "WARNING" \
                "Ensure all access to variable uses atomic operations"
        fi
    fi
    
    # Check for channel buffer size considerations
    if grep -n "make(chan.*," "$file" > /dev/null 2>&1; then
        while IFS=: read -r line_num match; do
            if [[ "$match" =~ make\(chan.*,\ ([0-9]+)\) ]]; then
                local buffer_size="${BASH_REMATCH[1]}"
                if [[ $buffer_size -gt 1000 ]]; then
                    log_issue "$file" "$line_num" "Large channel buffer ($buffer_size) may consume Pi memory" "WARNING" \
                        "Consider smaller buffer or streaming approach"
                fi
            fi
        done <<< "$(grep -n "make(chan.*," "$file")"
    fi
    
    # Check for goroutine leak potential
    if grep -n "for {" "$file" > /dev/null 2>&1; then
        if grep -n -A10 "for {" "$file" | grep "go func" > /dev/null 2>&1; then
            if ! grep -n -A10 "for {" "$file" | grep "context\|select\|channel" > /dev/null 2>&1; then
                log_issue "$file" "?" "Infinite loop spawning goroutines without exit condition" "ERROR" \
                    "Add context or channel-based exit condition"
            fi
        fi
    fi
    
    # Check for timer/ticker resource leaks
    if grep -n "time\.NewTimer\|time\.NewTicker" "$file" > /dev/null 2>&1; then
        local timer_count=$(grep -c "time\.NewTimer\|time\.NewTicker" "$file")
        local stop_count=$(grep -c "\.Stop()" "$file" || echo 0)
        
        if [[ $stop_count -lt $timer_count ]]; then
            log_issue "$file" "?" "Timer/Ticker created without Stop() calls" "WARNING" \
                "Always call Stop() on timers/tickers to free resources on Pi"
        fi
    fi
    
    # Check for select statement best practices
    if grep -n "select {" "$file" > /dev/null 2>&1; then
        # Check for select without default and timeout
        local select_blocks=$(grep -n -A20 "select {" "$file")
        if echo "$select_blocks" | grep -v "default\|time\.After\|context\.Done" > /dev/null 2>&1; then
            log_issue "$file" "?" "Select statement without timeout or default case" "WARNING" \
                "Add timeout or default case to prevent blocking on Pi"
        fi
    fi
    
    # Check for shared slice/map modifications
    if grep -n "append\|delete" "$file" > /dev/null 2>&1; then
        local goroutine_count=$(grep -c "go func(" "$file" || echo 0)
        local mutex_count=$(grep -c "sync\." "$file" || echo 0)
        
        if [[ $goroutine_count -gt 0 && $mutex_count -eq 0 ]]; then
            log_issue "$file" "?" "Potential race condition on slice/map modifications" "WARNING" \
                "Protect shared data structures with mutexes"
        fi
    fi
    
done

# Concurrency best practices summary
echo -e "\n${BLUE}=== Pi Concurrency Guidelines ===${NC}"
echo -e "• Limit goroutines (Pi has 4-8 CPU cores)"
echo -e "• Use context for lifecycle management"
echo -e "• Prefer channels over shared memory"
echo -e "• Always use defer for cleanup"
echo -e "• Monitor goroutine count in production"

# Summary
echo -e "\n${BLUE}=== Concurrency Check Summary ===${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Concurrency check failed! Critical race conditions or deadlock potential found.${NC}"
    echo -e "${YELLOW}💡 These issues can cause data corruption or hangs on Pi.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Concurrency check passed with warnings. Review concurrent code carefully.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ No concurrency issues detected!${NC}"
    exit 0
fi