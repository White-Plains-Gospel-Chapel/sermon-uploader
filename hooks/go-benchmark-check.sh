#!/bin/bash
# Go Performance Benchmark Validation for Raspberry Pi
# This script runs benchmarks and validates performance regressions

set -euo pipefail

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Go Benchmark Validation (Pi Optimized) ===${NC}"

# Pi performance thresholds (adjusted for Pi hardware)
MAX_MEMORY_ALLOCATION=50 # MB per operation
MAX_ALLOCATIONS_PER_OP=1000
MAX_NS_PER_OP=10000000 # 10ms max per operation
MIN_OPS_PER_SEC=100

# Initialize counters
ISSUES_FOUND=0
WARNINGS_FOUND=0
BENCHMARKS_RUN=0

# Function to log benchmark issue
log_issue() {
    local issue="$1"
    local severity="$2"
    local details="${3:-}"
    
    if [[ "$severity" == "ERROR" ]]; then
        echo -e "${RED}ERROR${NC}: $issue"
        [[ -n "$details" ]] && echo -e "  ${BLUE}📊 Details${NC}: $details"
        ((ISSUES_FOUND++))
    else
        echo -e "${YELLOW}WARNING${NC}: $issue"
        [[ -n "$details" ]] && echo -e "  ${BLUE}📊 Details${NC}: $details"
        ((WARNINGS_FOUND++))
    fi
}

# Function to parse benchmark output
parse_benchmark_result() {
    local benchmark_line="$1"
    local benchmark_name ns_per_op mb_per_sec allocs_per_op bytes_per_op
    
    if [[ "$benchmark_line" =~ ^(Benchmark[^[:space:]]+)[[:space:]]+([0-9]+)[[:space:]]+([0-9.]+)[[:space:]]+ns/op ]]; then
        benchmark_name="${BASH_REMATCH[1]}"
        ns_per_op="${BASH_REMATCH[3]%.*}" # Remove decimal part
        
        echo "Analyzing: $benchmark_name"
        
        # Check performance thresholds
        if [[ ${ns_per_op%.*} -gt $MAX_NS_PER_OP ]]; then
            log_issue "Slow benchmark: $benchmark_name" "ERROR" \
                "${ns_per_op} ns/op exceeds Pi threshold of ${MAX_NS_PER_OP} ns/op"
        fi
        
        # Extract memory allocations if present
        if [[ "$benchmark_line" =~ ([0-9]+)[[:space:]]+allocs/op ]]; then
            allocs_per_op="${BASH_REMATCH[1]}"
            if [[ $allocs_per_op -gt $MAX_ALLOCATIONS_PER_OP ]]; then
                log_issue "High allocation benchmark: $benchmark_name" "WARNING" \
                    "$allocs_per_op allocs/op exceeds Pi threshold of $MAX_ALLOCATIONS_PER_OP"
            fi
        fi
        
        # Extract bytes per operation if present
        if [[ "$benchmark_line" =~ ([0-9]+)[[:space:]]+B/op ]]; then
            bytes_per_op="${BASH_REMATCH[1]}"
            local mb_per_op=$((bytes_per_op / 1024 / 1024))
            if [[ $mb_per_op -gt $MAX_MEMORY_ALLOCATION ]]; then
                log_issue "High memory benchmark: $benchmark_name" "ERROR" \
                    "$mb_per_op MB/op exceeds Pi threshold of $MAX_MEMORY_ALLOCATION MB"
            fi
        fi
        
        ((BENCHMARKS_RUN++))
    fi
}

# Change to backend directory where Go code is located
cd "/Users/gaius/Documents/WPGC web/sermon-uploader/backend" || {
    echo -e "${RED}Error: Could not change to backend directory${NC}"
    exit 1
}

# Check if there are any benchmark tests
if ! find . -name "*_test.go" -exec grep -l "func Benchmark" {} \; | head -1 > /dev/null; then
    echo -e "${YELLOW}No benchmark tests found. Creating basic performance benchmarks...${NC}"
    
    # Create basic benchmark file if none exists
    if [[ ! -f "performance_test.go" ]]; then
        cat > performance_test.go << 'EOF'
package main

import (
    "bytes"
    "encoding/json"
    "strings"
    "testing"
    "time"
)

// BenchmarkStringConcatenation tests string building performance
func BenchmarkStringConcatenation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var result string
        for j := 0; j < 100; j++ {
            result += "test string "
        }
    }
}

// BenchmarkStringBuilder tests optimized string building
func BenchmarkStringBuilder(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var builder strings.Builder
        for j := 0; j < 100; j++ {
            builder.WriteString("test string ")
        }
        _ = builder.String()
    }
}

// BenchmarkJSONMarshal tests JSON serialization performance
func BenchmarkJSONMarshal(b *testing.B) {
    data := map[string]interface{}{
        "filename": "test.wav",
        "size": 1024000,
        "timestamp": time.Now(),
        "metadata": map[string]string{
            "format": "wav",
            "duration": "60s",
        },
    }
    
    for i := 0; i < b.N; i++ {
        _, err := json.Marshal(data)
        if err != nil {
            b.Error(err)
        }
    }
}

// BenchmarkBufferOperations tests buffer performance
func BenchmarkBufferOperations(b *testing.B) {
    data := make([]byte, 1024)
    for i := range data {
        data[i] = byte(i % 256)
    }
    
    for i := 0; i < b.N; i++ {
        var buf bytes.Buffer
        for j := 0; j < 100; j++ {
            buf.Write(data)
        }
        _ = buf.Bytes()
    }
}

// BenchmarkSliceAppend tests slice append performance
func BenchmarkSliceAppend(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var slice []int
        for j := 0; j < 1000; j++ {
            slice = append(slice, j)
        }
    }
}

// BenchmarkSlicePrealloc tests preallocated slice performance
func BenchmarkSlicePrealloc(b *testing.B) {
    for i := 0; i < b.N; i++ {
        slice := make([]int, 0, 1000)
        for j := 0; j < 1000; j++ {
            slice = append(slice, j)
        }
    }
}
EOF
        echo -e "${GREEN}Created basic benchmark file: performance_test.go${NC}"
    fi
fi

echo "Running Go benchmarks..."

# Run benchmarks with memory statistics
benchmark_output=$(go test -bench=. -benchmem -timeout=5m ./... 2>&1) || {
    echo -e "${RED}Benchmark execution failed${NC}"
    echo "$benchmark_output"
    exit 1
}

echo -e "\n${BLUE}=== Benchmark Results ===${NC}"
echo "$benchmark_output"

# Parse benchmark results
echo -e "\n${BLUE}=== Analyzing Benchmark Performance ===${NC}"
while IFS= read -r line; do
    if [[ "$line" =~ ^Benchmark ]]; then
        parse_benchmark_result "$line"
    fi
done <<< "$benchmark_output"

# Check for benchmark regressions if previous results exist
BENCHMARK_HISTORY_FILE="../.benchmark_history"
if [[ -f "$BENCHMARK_HISTORY_FILE" ]]; then
    echo -e "\n${BLUE}=== Checking for Performance Regressions ===${NC}"
    
    # Store current benchmarks
    current_benchmarks=$(echo "$benchmark_output" | grep "^Benchmark")
    
    # Compare with previous results
    while IFS= read -r current_line; do
        if [[ "$current_line" =~ ^(Benchmark[^[:space:]]+) ]]; then
            benchmark_name="${BASH_REMATCH[1]}"
            
            # Find corresponding line in history
            if grep -q "^$benchmark_name" "$BENCHMARK_HISTORY_FILE"; then
                previous_line=$(grep "^$benchmark_name" "$BENCHMARK_HISTORY_FILE" | tail -1)
                
                # Extract ns/op from both lines
                if [[ "$current_line" =~ ([0-9.]+)[[:space:]]+ns/op ]] && \
                   [[ "$previous_line" =~ ([0-9.]+)[[:space:]]+ns/op ]]; then
                    
                    current_ns="${BASH_REMATCH[1]%.*}"
                    previous_ns=$(echo "$previous_line" | sed -n 's/.*\([0-9.]\+\) ns\/op.*/\1/p' | cut -d. -f1)
                    
                    if [[ -n "$current_ns" && -n "$previous_ns" && $previous_ns -gt 0 ]]; then
                        # Calculate percentage change
                        local percent_change=$(( (current_ns - previous_ns) * 100 / previous_ns ))
                        
                        if [[ $percent_change -gt 20 ]]; then
                            log_issue "Performance regression in $benchmark_name" "ERROR" \
                                "$percent_change% slower than previous run ($previous_ns -> $current_ns ns/op)"
                        elif [[ $percent_change -gt 10 ]]; then
                            log_issue "Performance degradation in $benchmark_name" "WARNING" \
                                "$percent_change% slower than previous run ($previous_ns -> $current_ns ns/op)"
                        fi
                    fi
                fi
            fi
        fi
    done <<< "$current_benchmarks"
fi

# Store current results for future comparisons
echo "$benchmark_output" | grep "^Benchmark" > "$BENCHMARK_HISTORY_FILE.tmp" && \
    mv "$BENCHMARK_HISTORY_FILE.tmp" "$BENCHMARK_HISTORY_FILE"

# Check for missing benchmarks in critical code
echo -e "\n${BLUE}=== Checking Benchmark Coverage ===${NC}"

critical_files=(
    "handlers/handlers.go"
    "services/file_service.go"
    "services/minio.go"
    "services/streaming_service.go"
)

for file in "${critical_files[@]}"; do
    if [[ -f "$file" ]]; then
        # Check if corresponding benchmark file exists
        benchmark_file="${file%.*}_test.go"
        if [[ ! -f "$benchmark_file" ]] || ! grep -q "func Benchmark" "$benchmark_file" 2>/dev/null; then
            log_issue "Missing benchmarks for $file" "WARNING" \
                "Critical code should have performance benchmarks for Pi validation"
        fi
    fi
done

# Pi-specific benchmark recommendations
echo -e "\n${BLUE}=== Pi Benchmark Recommendations ===${NC}"
echo -e "🔧 Memory: Keep allocations under $MAX_MEMORY_ALLOCATION MB per operation"
echo -e "🔧 Speed: Target under $MAX_NS_PER_OP ns per operation"
echo -e "🔧 Allocations: Minimize to under $MAX_ALLOCATIONS_PER_OP per operation"
echo -e "🔧 Coverage: Benchmark critical paths for Pi performance validation"

# Summary
echo -e "\n${BLUE}=== Benchmark Summary ===${NC}"
echo -e "Benchmarks run: ${GREEN}$BENCHMARKS_RUN${NC}"
echo -e "Errors found: ${RED}$ISSUES_FOUND${NC}"
echo -e "Warnings found: ${YELLOW}$WARNINGS_FOUND${NC}"

if [[ $BENCHMARKS_RUN -eq 0 ]]; then
    echo -e "\n${RED}❌ No benchmarks were run!${NC}"
    echo -e "${YELLOW}💡 Add benchmark tests to validate Pi performance.${NC}"
    exit 1
elif [[ $ISSUES_FOUND -gt 0 ]]; then
    echo -e "\n${RED}❌ Benchmark validation failed! Performance issues found.${NC}"
    echo -e "${YELLOW}💡 These performance issues are critical on Pi hardware.${NC}"
    exit 1
elif [[ $WARNINGS_FOUND -gt 0 ]]; then
    echo -e "\n${YELLOW}⚠️  Benchmark validation passed with warnings. Monitor Pi performance.${NC}"
    exit 0
else
    echo -e "\n${GREEN}✅ All benchmarks passed Pi performance thresholds!${NC}"
    exit 0
fi