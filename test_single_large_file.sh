#!/bin/bash
# Test a single large file upload (1.8GB) with real performance measurements

set -e

API_URL="http://192.168.1.127:8000"
RIDGEPOINT="192.168.1.195"
REMOTE_PATH="/home/gaius/data/sermon-test-wavs"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}SINGLE LARGE FILE UPLOAD TEST (1.8GB)${NC}"
echo -e "${BLUE}========================================${NC}"

# Select a test file
TEST_FILE="generated-1gb/sermon_batch_001_1GB.wav"
FULL_PATH="${REMOTE_PATH}/${TEST_FILE}"

# Get file size from remote
echo "Getting file size from ridgepoint..."
FILE_SIZE=$(ssh gaius@${RIDGEPOINT} "stat -c '%s' '${FULL_PATH}'" 2>/dev/null || echo "0")

if [ "$FILE_SIZE" -eq 0 ]; then
    echo -e "${RED}ERROR: Cannot get file size for ${TEST_FILE}${NC}"
    exit 1
fi

SIZE_MB=$(echo "scale=1; $FILE_SIZE / 1024 / 1024" | bc)
echo -e "${GREEN}File: ${TEST_FILE}${NC}"
echo -e "${GREEN}Size: ${SIZE_MB} MB${NC}"

# Generate unique filename to avoid duplicates
UNIQUE_NAME="sermon_test_$(date +%s)_1GB.wav"

# Get presigned URL
echo -e "\n${YELLOW}Step 1: Getting presigned URL...${NC}"
RESPONSE=$(curl -s -X POST "$API_URL/api/upload/presigned" \
    -H "Content-Type: application/json" \
    -d "{\"filename\": \"$UNIQUE_NAME\", \"fileSize\": $FILE_SIZE}")

UPLOAD_URL=$(echo "$RESPONSE" | jq -r '.uploadUrl')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
IS_LARGE=$(echo "$RESPONSE" | jq -r '.largeFile.isLargeFile // false')
ESTIMATED_TIME=$(echo "$RESPONSE" | jq -r '.largeFile.estimatedUploadTime // "unknown"')

if [ "$SUCCESS" != "true" ] || [ "$UPLOAD_URL" = "null" ]; then
    echo -e "${RED}Failed to get presigned URL${NC}"
    echo "$RESPONSE" | jq
    exit 1
fi

echo -e "${GREEN}✓ Presigned URL obtained${NC}"
echo "  Is Large File: $IS_LARGE"
echo "  Estimated Upload Time: $ESTIMATED_TIME"

# Monitor API logs in background
echo -e "\n${YELLOW}Step 2: Starting API log monitor...${NC}"
ssh gaius@${RIDGEPOINT} "docker logs sermon-uploader --tail=0 --follow 2>&1 | grep -E '\[PRESIGNED\]|\[MINIO\]|\[LARGE_FILE_OPT\]'" &
LOG_PID=$!

# Upload the file directly from ridgepoint using curl
echo -e "\n${YELLOW}Step 3: Uploading ${SIZE_MB}MB file from ridgepoint...${NC}"
echo "Starting upload at $(date '+%Y-%m-%d %H:%M:%S')"

START_TIME=$(date +%s)

# SSH to ridgepoint and upload from there
ssh gaius@${RIDGEPOINT} "curl -s -w '%{http_code}' -X PUT -T '${FULL_PATH}' '${UPLOAD_URL}' --max-time 3600" > /tmp/upload_result 2>&1 &
UPLOAD_PID=$!

# Monitor progress
echo "Upload in progress..."
while kill -0 $UPLOAD_PID 2>/dev/null; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    echo -ne "\rElapsed: ${ELAPSED}s"
    sleep 5
done

# Get result
wait $UPLOAD_PID
HTTP_CODE=$(cat /tmp/upload_result)
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Kill log monitor
kill $LOG_PID 2>/dev/null || true

# Calculate actual speed
SPEED_MBPS=$(echo "scale=2; $SIZE_MB / $DURATION" | bc)

echo -e "\n\n${BLUE}========================================${NC}"
echo -e "${BLUE}RESULTS${NC}"
echo -e "${BLUE}========================================${NC}"

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ UPLOAD SUCCESSFUL${NC}"
    echo "  File: $UNIQUE_NAME"
    echo "  Size: ${SIZE_MB} MB"
    echo "  Duration: ${DURATION} seconds"
    echo -e "  ${GREEN}ACTUAL Speed: ${SPEED_MBPS} MB/s${NC}"
    echo "  HTTP Code: $HTTP_CODE"
    
    # Process the uploaded file
    echo -e "\n${YELLOW}Step 4: Processing uploaded file...${NC}"
    PROCESS_RESPONSE=$(curl -s -X POST "$API_URL/api/upload/process" \
        -H "Content-Type: application/json" \
        -d "{\"filename\": \"$UNIQUE_NAME\"}")
    
    PROCESS_SUCCESS=$(echo "$PROCESS_RESPONSE" | jq -r '.success')
    if [ "$PROCESS_SUCCESS" = "true" ]; then
        echo -e "${GREEN}✓ File processed successfully${NC}"
    else
        echo -e "${YELLOW}⚠ Processing status unknown${NC}"
    fi
else
    echo -e "${RED}✗ UPLOAD FAILED${NC}"
    echo "  HTTP Code: $HTTP_CODE"
    echo "  Duration: ${DURATION} seconds"
fi

# Cleanup
rm -f /tmp/upload_result

echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SUNDAY MORNING READINESS${NC}"
echo -e "${BLUE}========================================${NC}"

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ System can handle 1.8GB uploads${NC}"
    echo -e "${GREEN}✓ Actual measured speed: ${SPEED_MBPS} MB/s${NC}"
    
    # Calculate time for 5 files
    FIVE_FILE_TIME=$(echo "scale=0; (5 * $SIZE_MB) / $SPEED_MBPS" | bc)
    FIVE_FILE_MINUTES=$(echo "scale=1; $FIVE_FILE_TIME / 60" | bc)
    
    echo -e "${GREEN}✓ Estimated time for 5 x 1.8GB files: ${FIVE_FILE_MINUTES} minutes${NC}"
    
    if [ $(echo "$SPEED_MBPS > 5" | bc) -eq 1 ]; then
        echo -e "${GREEN}✓ Speed is sufficient for Sunday morning (>5 MB/s)${NC}"
    else
        echo -e "${YELLOW}⚠ Speed is below optimal (${SPEED_MBPS} MB/s < 5 MB/s)${NC}"
    fi
else
    echo -e "${RED}✗ System cannot handle large uploads reliably${NC}"
fi

echo -e "${BLUE}========================================${NC}"