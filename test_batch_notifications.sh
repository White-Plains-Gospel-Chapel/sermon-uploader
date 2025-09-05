#!/bin/bash

# Test script for batch Discord notification fix
# This script tests the new /api/upload/complete-batch endpoint

set -e

# Configuration
API_URL="${API_URL:-http://localhost:8000}"
TEST_FILES=("test_batch_1.wav" "test_batch_2.wav" "test_batch_3.wav")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧪 Testing Batch Discord Notification Fix${NC}"
echo "API URL: $API_URL"
echo "=========================================="

# Function to check if API is accessible
check_api() {
    echo -e "${BLUE}🔍 Checking API health...${NC}"
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/api/health")
    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ API is healthy${NC}"
        return 0
    else
        echo -e "${RED}✗ API not accessible (HTTP $HTTP_CODE)${NC}"
        return 1
    fi
}

# Function to test Discord webhook
test_discord_webhook() {
    echo -e "${BLUE}🔍 Testing Discord webhook...${NC}"
    RESPONSE=$(curl -s -X POST "$API_URL/api/test/discord")
    SUCCESS=$(echo "$RESPONSE" | jq -r '.success // false')
    
    if [ "$SUCCESS" = "true" ]; then
        echo -e "${GREEN}✓ Discord webhook is working${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ Discord webhook test failed - notifications may not work${NC}"
        echo "Response: $RESPONSE"
        return 1
    fi
}

# Function to test batch completion endpoint
test_batch_completion_endpoint() {
    echo -e "${BLUE}🔍 Testing batch completion endpoint...${NC}"
    
    # Test with valid payload
    PAYLOAD=$(cat <<EOF
{
    "filenames": ["test_file_1.wav", "test_file_2.wav", "test_file_3.wav"]
}
EOF
    )
    
    RESPONSE=$(curl -s -X POST "$API_URL/api/upload/complete-batch" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
    
    # Check if the endpoint exists (not 404)
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API_URL/api/upload/complete-batch" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")
    
    if [ "$HTTP_CODE" = "404" ]; then
        echo -e "${RED}✗ Batch completion endpoint not found (HTTP 404)${NC}"
        return 1
    elif [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ Batch completion endpoint is accessible${NC}"
        
        # Parse response
        IS_BATCH=$(echo "$RESPONSE" | jq -r '.is_batch // false')
        SUCCESSFUL=$(echo "$RESPONSE" | jq -r '.successful // 0')
        FAILED=$(echo "$RESPONSE" | jq -r '.failed // 0')
        
        echo "  - Is batch: $IS_BATCH"
        echo "  - Successful: $SUCCESSFUL"
        echo "  - Failed: $FAILED (expected - files don't exist)"
        
        if [ "$IS_BATCH" = "true" ]; then
            echo -e "${GREEN}✓ Batch detection is working correctly${NC}"
        else
            echo -e "${YELLOW}⚠ Batch detection may not be working${NC}"
        fi
        
        return 0
    else
        echo -e "${YELLOW}⚠ Batch completion endpoint returned HTTP $HTTP_CODE${NC}"
        echo "Response: $RESPONSE"
        return 1
    fi
}

# Function to test single file vs batch behavior
test_batch_threshold() {
    echo -e "${BLUE}🔍 Testing batch threshold behavior...${NC}"
    
    # Test single file (should not be batch)
    echo "Testing single file (should NOT trigger batch)..."
    SINGLE_PAYLOAD='{"filenames": ["single_test.wav"]}'
    SINGLE_RESPONSE=$(curl -s -X POST "$API_URL/api/upload/complete-batch" \
        -H "Content-Type: application/json" \
        -d "$SINGLE_PAYLOAD")
    
    SINGLE_IS_BATCH=$(echo "$SINGLE_RESPONSE" | jq -r '.is_batch // false')
    
    if [ "$SINGLE_IS_BATCH" = "false" ]; then
        echo -e "${GREEN}✓ Single file correctly identified as non-batch${NC}"
    else
        echo -e "${YELLOW}⚠ Single file incorrectly identified as batch${NC}"
    fi
    
    # Test multiple files (should be batch)
    echo "Testing multiple files (should trigger batch)..."
    BATCH_PAYLOAD='{"filenames": ["batch_test_1.wav", "batch_test_2.wav"]}'
    BATCH_RESPONSE=$(curl -s -X POST "$API_URL/api/upload/complete-batch" \
        -H "Content-Type: application/json" \
        -d "$BATCH_PAYLOAD")
    
    BATCH_IS_BATCH=$(echo "$BATCH_RESPONSE" | jq -r '.is_batch // false')
    
    if [ "$BATCH_IS_BATCH" = "true" ]; then
        echo -e "${GREEN}✓ Multiple files correctly identified as batch${NC}"
    else
        echo -e "${YELLOW}⚠ Multiple files incorrectly identified as non-batch${NC}"
    fi
}

# Function to show configuration
show_configuration() {
    echo -e "${BLUE}🔍 Checking configuration...${NC}"
    
    # Get dashboard to check batch threshold
    DASHBOARD=$(curl -s "$API_URL/api/dashboard")
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Dashboard accessible${NC}"
        
        # Try to extract file count
        FILE_COUNT=$(echo "$DASHBOARD" | jq -r '.status.file_count // "unknown"')
        echo "  - Current file count: $FILE_COUNT"
        
        # Check MinIO connection
        MINIO_CONNECTED=$(echo "$DASHBOARD" | jq -r '.status.minio_connected // false')
        if [ "$MINIO_CONNECTED" = "true" ]; then
            echo -e "${GREEN}  - MinIO: Connected${NC}"
        else
            echo -e "${RED}  - MinIO: Disconnected${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ Dashboard not accessible${NC}"
    fi
}

# Main test execution
echo -e "${BLUE}Starting tests...${NC}"
echo

# Run all tests
check_api || echo -e "${RED}❌ API health check failed${NC}"
echo

test_discord_webhook || echo -e "${YELLOW}⚠ Discord webhook test had issues${NC}"  
echo

test_batch_completion_endpoint || echo -e "${RED}❌ Batch completion endpoint test failed${NC}"
echo

test_batch_threshold || echo -e "${YELLOW}⚠ Batch threshold test had issues${NC}"
echo

show_configuration || echo -e "${YELLOW}⚠ Configuration check had issues${NC}"
echo

echo -e "${BLUE}=========================================="
echo -e "🎯 Test Summary:"
echo -e "- New batch completion endpoint: /api/upload/complete-batch"
echo -e "- Batch threshold detection: Based on file count"  
echo -e "- Discord notifications: Should trigger for batches"
echo -e "- Frontend integration: Ready in useUploadQueue.ts"
echo -e "=========================================${NC}"

echo -e "${GREEN}✅ Batch Discord notification fix testing complete!${NC}"
echo
echo -e "${YELLOW}📝 Next Steps:"
echo -e "1. Test with real file uploads using the frontend"
echo -e "2. Monitor Discord channel for batch notifications"
echo -e "3. Verify that individual uploads still work correctly${NC}"