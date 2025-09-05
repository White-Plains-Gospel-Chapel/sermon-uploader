#!/bin/bash
# Test Sunday morning batch upload scenario - 5 large files (300MB+ each)

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
echo -e "${BLUE}SUNDAY MORNING BATCH TEST - 5 LARGE FILES${NC}"
echo -e "${BLUE}========================================${NC}"

# Select 5 test files
FILES=(
    "generated-1gb/sermon_batch_001_1GB.wav"
    "generated-1gb/sermon_batch_002_1GB.wav"
    "generated-1gb/sermon_batch_003_1GB.wav"
    "generated-1gb/sermon_batch_004_1GB.wav"
    "generated-1gb/sermon_batch_005_1GB.wav"
)

TOTAL_SIZE=0
SUCCESSFUL_UPLOADS=0
FAILED_UPLOADS=0
TOTAL_START_TIME=$(date +%s)

echo -e "${YELLOW}Testing with 5 x 1.8GB files (9GB total)${NC}\n"

# Process each file
for i in "${!FILES[@]}"; do
    FILE="${FILES[$i]}"
    FILE_NUM=$((i+1))
    
    echo -e "${BLUE}--- File $FILE_NUM/5: $(basename $FILE) ---${NC}"
    
    # Get file size
    FILE_SIZE=$(ssh gaius@${RIDGEPOINT} "stat -c '%s' '${REMOTE_PATH}/${FILE}'" 2>/dev/null || echo "0")
    
    if [ "$FILE_SIZE" -eq 0 ]; then
        echo -e "${RED}✗ Cannot get file size${NC}"
        FAILED_UPLOADS=$((FAILED_UPLOADS+1))
        continue
    fi
    
    SIZE_MB=$(echo "scale=1; $FILE_SIZE / 1024 / 1024" | bc)
    TOTAL_SIZE=$(echo "scale=1; $TOTAL_SIZE + $SIZE_MB" | bc)
    
    # Generate unique filename
    UNIQUE_NAME="sunday_batch_${FILE_NUM}_$(date +%s).wav"
    
    # Get presigned URL
    echo "Getting presigned URL..."
    RESPONSE=$(curl -s -X POST "$API_URL/api/upload/presigned" \
        -H "Content-Type: application/json" \
        -d "{\"filename\": \"$UNIQUE_NAME\", \"fileSize\": $FILE_SIZE}")
    
    UPLOAD_URL=$(echo "$RESPONSE" | jq -r '.uploadUrl')
    SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
    
    if [ "$SUCCESS" != "true" ] || [ "$UPLOAD_URL" = "null" ]; then
        echo -e "${RED}✗ Failed to get presigned URL${NC}"
        FAILED_UPLOADS=$((FAILED_UPLOADS+1))
        continue
    fi
    
    # Upload the file
    echo "Uploading ${SIZE_MB}MB..."
    START_TIME=$(date +%s)
    
    # Upload from ridgepoint
    HTTP_CODE=$(ssh gaius@${RIDGEPOINT} "curl -s -w '%{http_code}' -X PUT -T '${REMOTE_PATH}/${FILE}' '${UPLOAD_URL}' --max-time 600 -o /dev/null")
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    
    if [ "$HTTP_CODE" = "200" ]; then
        SPEED=$(echo "scale=2; $SIZE_MB / $DURATION" | bc)
        echo -e "${GREEN}✓ Upload successful - ${DURATION}s @ ${SPEED} MB/s${NC}"
        SUCCESSFUL_UPLOADS=$((SUCCESSFUL_UPLOADS+1))
        
        # Process the file
        curl -s -X POST "$API_URL/api/upload/process" \
            -H "Content-Type: application/json" \
            -d "{\"filename\": \"$UNIQUE_NAME\"}" > /dev/null
    else
        echo -e "${RED}✗ Upload failed - HTTP $HTTP_CODE${NC}"
        FAILED_UPLOADS=$((FAILED_UPLOADS+1))
    fi
    
    echo ""
done

TOTAL_END_TIME=$(date +%s)
TOTAL_DURATION=$((TOTAL_END_TIME - TOTAL_START_TIME))

# Calculate statistics
AVG_SPEED=$(echo "scale=2; $TOTAL_SIZE / $TOTAL_DURATION" | bc)
TOTAL_MINUTES=$(echo "scale=1; $TOTAL_DURATION / 60" | bc)

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}BATCH TEST RESULTS${NC}"
echo -e "${BLUE}========================================${NC}"
echo "Files Uploaded: $SUCCESSFUL_UPLOADS/5"
echo "Failed: $FAILED_UPLOADS"
echo "Total Size: ${TOTAL_SIZE} MB"
echo "Total Time: ${TOTAL_DURATION} seconds (${TOTAL_MINUTES} minutes)"
echo -e "${GREEN}Average Speed: ${AVG_SPEED} MB/s${NC}"

echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}SUNDAY MORNING ASSESSMENT${NC}"
echo -e "${BLUE}========================================${NC}"

if [ $SUCCESSFUL_UPLOADS -eq 5 ]; then
    echo -e "${GREEN}✓ ALL FILES UPLOADED SUCCESSFULLY${NC}"
    echo -e "${GREEN}✓ System is READY for Sunday morning${NC}"
    
    if [ $(echo "$AVG_SPEED > 10" | bc) -eq 1 ]; then
        echo -e "${GREEN}✓ Excellent performance (${AVG_SPEED} MB/s)${NC}"
    elif [ $(echo "$AVG_SPEED > 5" | bc) -eq 1 ]; then
        echo -e "${GREEN}✓ Good performance (${AVG_SPEED} MB/s)${NC}"
    else
        echo -e "${YELLOW}⚠ Performance needs improvement (${AVG_SPEED} MB/s)${NC}"
    fi
    
    # Estimate time for typical Sunday (10 files)
    TEN_FILE_TIME=$(echo "scale=0; (10 * 1816.8) / $AVG_SPEED" | bc)
    TEN_FILE_MINUTES=$(echo "scale=1; $TEN_FILE_TIME / 60" | bc)
    echo -e "${GREEN}✓ Estimated time for 10 files: ${TEN_FILE_MINUTES} minutes${NC}"
    
elif [ $SUCCESSFUL_UPLOADS -gt 2 ]; then
    echo -e "${YELLOW}⚠ PARTIAL SUCCESS (${SUCCESSFUL_UPLOADS}/5 uploaded)${NC}"
    echo -e "${YELLOW}⚠ System needs optimization for reliability${NC}"
else
    echo -e "${RED}✗ BATCH TEST FAILED${NC}"
    echo -e "${RED}✗ System NOT ready for Sunday morning${NC}"
fi

echo -e "${BLUE}========================================${NC}"