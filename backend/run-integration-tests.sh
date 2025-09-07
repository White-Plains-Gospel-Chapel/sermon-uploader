#!/bin/bash

# Run integration tests from Pi to test real uploads

echo "🧪 Running Large File Upload Integration Tests"
echo "=============================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check if running on Pi
if [[ $(hostname) == "ridgepoint" ]]; then
    echo -e "${GREEN}✓ Running on Pi (ridgepoint)${NC}"
else
    echo -e "${YELLOW}⚠ Not running on Pi, but continuing...${NC}"
fi

# Check for test files
TEST_DIR="/home/gaius/data/sermon-test-wavs"
if [ -d "$TEST_DIR" ]; then
    echo -e "${GREEN}✓ Test directory found${NC}"
    echo "  Large files available:"
    find "$TEST_DIR" -type f -name "*.wav" -size +500M 2>/dev/null | head -3 | while read -r file; do
        size_mb=$(($(stat -c%s "$file" 2>/dev/null || stat -f%z "$file") / 1024 / 1024))
        echo "    - $(basename "$file") (${size_mb}MB)"
    done
else
    echo -e "${YELLOW}⚠ Test directory not found at $TEST_DIR${NC}"
fi

echo ""
echo "Running Go integration tests..."
echo "--------------------------------"

# Run the integration tests with verbose output
cd "$(dirname "$0")"
go test -tags=integration -v ./integration/... -run TestHealthCheck 2>&1 | tee test-health.log

if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo -e "${GREEN}✓ Health check passed${NC}"
else
    echo -e "${RED}✗ Health check failed${NC}"
    exit 1
fi

echo ""
echo "Testing single large file upload..."
go test -tags=integration -v -timeout 30m ./integration/... -run TestSingleLargeFileUpload 2>&1 | tee test-single.log

if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo -e "${GREEN}✓ Single file upload test passed${NC}"
else
    echo -e "${RED}✗ Single file upload test failed${NC}"
    cat test-single.log | grep -E "FAIL|Error|error" | tail -10
fi

echo ""
echo "Testing bulk large file upload..."
go test -tags=integration -v -timeout 60m ./integration/... -run TestBulkLargeFileUpload 2>&1 | tee test-bulk.log

if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo -e "${GREEN}✓ Bulk upload test passed${NC}"
else
    echo -e "${RED}✗ Bulk upload test failed${NC}"
    cat test-bulk.log | grep -E "FAIL|Error|error" | tail -10
fi

echo ""
echo "=============================================="
echo "Test Summary"
echo "=============================================="
echo ""

# Count results
PASSED=$(grep -c "✅" test-*.log 2>/dev/null || echo 0)
FAILED=$(grep -c "FAIL" test-*.log 2>/dev/null || echo 0)

echo "Tests passed: $PASSED"
echo "Tests failed: $FAILED"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All integration tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi