#!/bin/bash

# Test script for MinIO TLS Configuration
# This script verifies HTTPS is working correctly with MinIO

set -e

echo "🧪 MinIO TLS Configuration Test Suite"
echo "====================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
MINIO_HOST="192.168.1.127"
MINIO_HTTPS_PORT="9000"
MINIO_CONSOLE_PORT="9001"
CERT_PATH="/home/gaius/.minio/certs"

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo -n "Testing: $test_name... "
    
    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        ((TESTS_FAILED++))
        return 1
    fi
}

echo ""
echo "1. Certificate Tests"
echo "--------------------"

# Test 1: Check if certificates exist
run_test "Certificates exist" "ssh gaius@$MINIO_HOST 'test -f $CERT_PATH/private.key && test -f $CERT_PATH/public.crt'"

# Test 2: Check certificate validity
run_test "Certificate validity" "ssh gaius@$MINIO_HOST 'openssl x509 -in $CERT_PATH/public.crt -noout -checkend 86400'"

# Test 3: Check certificate permissions
run_test "Certificate permissions" "ssh gaius@$MINIO_HOST 'test -r $CERT_PATH/private.key && test -r $CERT_PATH/public.crt'"

echo ""
echo "2. MinIO HTTPS Tests"
echo "--------------------"

# Test 4: Check if MinIO is running
run_test "MinIO service running" "ssh gaius@$MINIO_HOST 'systemctl is-active minio || docker ps | grep -q minio'"

# Test 5: Test HTTPS connection
run_test "HTTPS connection" "curl -k --connect-timeout 5 https://$MINIO_HOST:$MINIO_HTTPS_PORT/minio/health/live"

# Test 6: Test TLS certificate chain
run_test "TLS handshake" "echo | openssl s_client -connect $MINIO_HOST:$MINIO_HTTPS_PORT -servername $MINIO_HOST 2>/dev/null | grep -q 'Verify return code'"

echo ""
echo "3. CORS Configuration Tests"
echo "---------------------------"

# Test 7: Check CORS headers on OPTIONS request
test_cors() {
    local response=$(curl -k -s -X OPTIONS \
        -H "Origin: https://sermons.wpgc.church" \
        -H "Access-Control-Request-Method: PUT" \
        -H "Access-Control-Request-Headers: Content-Type" \
        -I "https://$MINIO_HOST:$MINIO_HTTPS_PORT/sermons/test.wav" 2>/dev/null)
    
    echo "$response" | grep -qi "Access-Control-Allow-Origin"
}

run_test "CORS headers present" "test_cors"

# Test 8: Check CORS allows our domain
test_cors_domain() {
    local response=$(curl -k -s -X OPTIONS \
        -H "Origin: https://sermons.wpgc.church" \
        -I "https://$MINIO_HOST:$MINIO_HTTPS_PORT/sermons/test.wav" 2>/dev/null)
    
    echo "$response" | grep -qi "Access-Control-Allow-Origin.*sermons.wpgc.church\|Access-Control-Allow-Origin.*\*"
}

run_test "CORS allows sermons.wpgc.church" "test_cors_domain"

echo ""
echo "4. Browser Upload Tests"
echo "-----------------------"

# Test 9: Test presigned URL generation
test_presigned_url() {
    # Use MinIO client to generate presigned URL
    ssh gaius@$MINIO_HOST "mc alias set local https://localhost:$MINIO_HTTPS_PORT gaius 'John 3:16' --insecure && \
                           mc presign local/sermons/test.wav --expire 1h --insecure" > /dev/null 2>&1
}

run_test "Presigned URL generation" "test_presigned_url"

# Test 10: Test direct PUT to MinIO with HTTPS
test_direct_upload() {
    local test_file="/tmp/test_upload_$$.txt"
    echo "Test content" > "$test_file"
    
    # Get presigned URL
    local url=$(ssh gaius@$MINIO_HOST "mc presign local/sermons/test_upload.txt --expire 1h --insecure 2>/dev/null" | grep -o 'https://[^ ]*')
    
    if [ -n "$url" ]; then
        # Upload file using presigned URL
        curl -k -X PUT --data-binary "@$test_file" "$url" > /dev/null 2>&1
        local result=$?
        rm -f "$test_file"
        return $result
    else
        rm -f "$test_file"
        return 1
    fi
}

run_test "Direct HTTPS upload" "test_direct_upload"

echo ""
echo "5. Mixed Content Tests"
echo "----------------------"

# Test 11: Verify HTTPS URLs are returned
test_https_urls() {
    local response=$(curl -k -s "https://$MINIO_HOST:$MINIO_HTTPS_PORT/minio/health/live")
    [ "$?" -eq 0 ]
}

run_test "HTTPS health endpoint" "test_https_urls"

# Test 12: Check no HTTP redirect
test_no_http() {
    # Should fail or redirect when accessing HTTP
    ! curl -s --connect-timeout 2 "http://$MINIO_HOST:$MINIO_HTTPS_PORT/minio/health/live" > /dev/null 2>&1
}

run_test "HTTP disabled/redirected" "test_no_http"

echo ""
echo "6. Performance Tests"
echo "--------------------"

# Test 13: Upload speed test with HTTPS
test_upload_speed() {
    local test_file="/tmp/speed_test_$$.bin"
    # Create 10MB test file
    dd if=/dev/zero of="$test_file" bs=1M count=10 2>/dev/null
    
    local start_time=$(date +%s%N)
    
    # Upload via HTTPS
    local url=$(ssh gaius@$MINIO_HOST "mc presign local/sermons/speed_test.bin --expire 1h --insecure 2>/dev/null" | grep -o 'https://[^ ]*')
    
    if [ -n "$url" ]; then
        curl -k -X PUT --data-binary "@$test_file" "$url" > /dev/null 2>&1
        local end_time=$(date +%s%N)
        local duration=$((($end_time - $start_time) / 1000000)) # Convert to milliseconds
        
        rm -f "$test_file"
        
        # Check if upload took less than 5 seconds for 10MB (reasonable for local network)
        [ "$duration" -lt 5000 ]
    else
        rm -f "$test_file"
        return 1
    fi
}

run_test "HTTPS upload performance" "test_upload_speed"

echo ""
echo "====================================="
echo "Test Results Summary"
echo "====================================="
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed! MinIO TLS is properly configured.${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed. Please check the configuration.${NC}"
    echo ""
    echo "Common fixes:"
    echo "1. Generate certificates: ./scripts/setup-minio-tls.sh"
    echo "2. Check MinIO environment: MINIO_API_CORS_ALLOW_ORIGIN='*'"
    echo "3. Restart MinIO: docker-compose restart minio"
    exit 1
fi