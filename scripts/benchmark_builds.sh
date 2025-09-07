#!/bin/bash
# TDD Docker Build Optimization Benchmark Script
# RED PHASE - Define success criteria FIRST (these should FAIL with current setup)

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
RESULTS_FILE="build_benchmark_results.json"
CURRENT_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "🧪 TDD Docker Build Optimization Benchmark - RED PHASE"
echo "=================================================="
echo "📅 Test run: $CURRENT_TIME"
echo ""

# Initialize results file
cat > $RESULTS_FILE << EOF
{
  "test_run": "$CURRENT_TIME",
  "baseline": {},
  "optimized_approaches": {},
  "test_results": {
    "build_time_under_3_minutes": false,
    "image_size_under_100mb": false,
    "memory_usage_under_500mb": false,
    "cache_hit_rate_over_80": false
  },
  "recommendations": []
}
EOF

echo "🎯 TESTING SUCCESS CRITERIA (These should FAIL initially)"
echo "======================================================="

# SUCCESS CRITERION 1: Build time under 3 minutes
echo -e "\n📏 TEST 1: Build time must be under 3 minutes"
echo "Current GitHub Actions baseline: 9m2s (EXPECTED TO FAIL)"

assert_build_time_under_3_minutes() {
    local start_time=$(date +%s)
    local dockerfile_path="${1:-Dockerfile}"
    
    echo "⏱️  Starting build timer for $dockerfile_path..."
    
    # Build the Docker image and capture time
    if timeout 180 docker buildx build \
        --platform linux/arm64 \
        -f "$dockerfile_path" \
        --load \
        -t "benchmark-test:$(date +%s)" \
        . > build_output.log 2>&1; then
        
        local end_time=$(date +%s)
        local build_time=$((end_time - start_time))
        
        echo "⏱️  Build completed in ${build_time}s"
        
        if [ $build_time -lt 180 ]; then
            echo -e "${GREEN}✅ PASS: Build time ${build_time}s < 180s${NC}"
            return 0
        else
            echo -e "${RED}❌ FAIL: Build time ${build_time}s >= 180s${NC}"
            return 1
        fi
    else
        echo -e "${RED}❌ FAIL: Build timed out after 3 minutes${NC}"
        return 1
    fi
}

# SUCCESS CRITERION 2: Image size under 100MB
echo -e "\n📏 TEST 2: Final image size must be under 100MB"

assert_image_size_under_100mb() {
    local image_name="$1"
    local size_bytes=$(docker images "$image_name" --format "table {{.Size}}" | tail -n +2 | head -1)
    
    if [[ $size_bytes == *"GB"* ]]; then
        echo -e "${RED}❌ FAIL: Image size ${size_bytes} (>= 1GB, way over 100MB limit)${NC}"
        return 1
    elif [[ $size_bytes == *"MB"* ]]; then
        local size_mb=${size_bytes%MB}
        if (( $(echo "$size_mb < 100" | bc -l) )); then
            echo -e "${GREEN}✅ PASS: Image size ${size_bytes} < 100MB${NC}"
            return 0
        else
            echo -e "${RED}❌ FAIL: Image size ${size_bytes} >= 100MB${NC}"
            return 1
        fi
    else
        echo -e "${GREEN}✅ PASS: Image size ${size_bytes} < 100MB${NC}"
        return 0
    fi
}

# SUCCESS CRITERION 3: Memory usage under 500MB on Pi 5
echo -e "\n📏 TEST 3: Runtime memory usage must be under 500MB on Pi 5"

assert_memory_usage_under_500mb() {
    local container_name="benchmark-memory-test"
    
    # Start container
    docker run -d --name "$container_name" "$1" sleep 30
    
    # Wait for container to stabilize
    sleep 5
    
    # Get memory usage
    local memory_usage=$(docker stats --no-stream --format "table {{.MemUsage}}" "$container_name" | tail -n +2)
    local memory_mb=$(echo $memory_usage | grep -o '[0-9.]*MiB' | head -1 | sed 's/MiB//')
    
    docker rm -f "$container_name" >/dev/null
    
    if [ -n "$memory_mb" ] && (( $(echo "$memory_mb < 500" | bc -l) )); then
        echo -e "${GREEN}✅ PASS: Memory usage ${memory_mb}MiB < 500MB${NC}"
        return 0
    else
        echo -e "${RED}❌ FAIL: Memory usage ${memory_usage} >= 500MB${NC}"
        return 1
    fi
}

# SUCCESS CRITERION 4: Cache hit rate over 80%
echo -e "\n📏 TEST 4: Docker layer cache hit rate must be over 80%"

assert_cache_hit_rate_over_80() {
    echo -e "${YELLOW}⚠️  SETUP NEEDED: Cache hit rate test requires build history${NC}"
    echo "This test will be implemented after optimized Dockerfiles are created"
    echo -e "${RED}❌ FAIL: Cache optimization not implemented yet${NC}"
    return 1
}

# Run baseline tests (these should FAIL)
echo -e "\n🔥 RUNNING RED PHASE TESTS (Expected to FAIL)"
echo "============================================"

FAILED_TESTS=0

echo -e "\n🧪 Testing current Dockerfile..."
if ! assert_build_time_under_3_minutes "Dockerfile"; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

# Find the last built image for size test
LATEST_IMAGE=$(docker images --format "table {{.Repository}}:{{.Tag}}\t{{.CreatedAt}}" | grep benchmark-test | head -1 | awk '{print $1}')
if [ -n "$LATEST_IMAGE" ]; then
    if ! assert_image_size_under_100mb "$LATEST_IMAGE"; then
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    
    if ! assert_memory_usage_under_500mb "$LATEST_IMAGE"; then
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
else
    echo -e "${YELLOW}⚠️  No image built, skipping size and memory tests${NC}"
    FAILED_TESTS=$((FAILED_TESTS + 2))
fi

if ! assert_cache_hit_rate_over_80; then
    FAILED_TESTS=$((FAILED_TESTS + 1))
fi

# Update results
jq --argjson failed "$FAILED_TESTS" '.baseline.failed_tests = $failed' $RESULTS_FILE > temp.json && mv temp.json $RESULTS_FILE

echo -e "\n📊 RED PHASE RESULTS"
echo "==================="
echo -e "${RED}❌ Failed tests: $FAILED_TESTS/4${NC}"
echo -e "${GREEN}✅ This is EXPECTED in TDD RED phase!${NC}"
echo ""
echo "📋 Next steps:"
echo "1. Create optimized Dockerfiles (GREEN phase)"
echo "2. Re-run benchmarks to see improvements"
echo "3. Iterate until all tests pass"
echo ""
echo "📄 Results saved to: $RESULTS_FILE"
echo "📄 Build output saved to: build_output.log"

# Cleanup test images
echo -e "\n🧹 Cleaning up test images..."
docker images | grep benchmark-test | awk '{print $3}' | xargs -r docker rmi -f >/dev/null 2>&1 || true

echo -e "\n🎯 TDD RED PHASE COMPLETE"
echo "========================="
echo "All tests failed as expected. Ready for GREEN phase optimization!"