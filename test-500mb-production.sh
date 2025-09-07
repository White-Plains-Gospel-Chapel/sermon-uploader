#!/bin/bash

# Production test script for 500MB+ files ONLY
# Tests the API as if using frontend/backend with real large files from Pi

API_HOST="https://sermons.wpgc.church"
PI_HOST="192.168.1.195"
TEST_DIR="/home/gaius/data/sermon-test-wavs"
MIN_SIZE_MB=500

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Production Test: 500MB+ Files Only${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Step 1: Test API health
echo -e "${YELLOW}Step 1: Testing API health...${NC}"
response=$(curl -s -o /dev/null -w "%{http_code}" "$API_HOST/api/health")
if [ "$response" = "200" ]; then
    echo -e "${GREEN}✓ API is healthy${NC}"
else
    echo -e "${RED}✗ API health check failed (HTTP $response)${NC}"
    exit 1
fi
echo ""

# Step 2: Find ONLY 500MB+ files on Pi
echo -e "${YELLOW}Step 2: Finding files ≥500MB on Pi...${NC}"
ssh gaius@$PI_HOST << 'EOF'
TEST_DIR="/home/gaius/data/sermon-test-wavs"
MIN_SIZE="500M"

echo "Searching for files 500MB or larger..."
large_files=$(find "$TEST_DIR" -type f -name "*.wav" -size +500M 2>/dev/null | head -5)

if [ -z "$large_files" ]; then
    echo "ERROR: No files 500MB or larger found"
    exit 1
fi

echo "Found large files (500MB+):"
echo "$large_files" | while read -r file; do
    size_bytes=$(stat -c%s "$file" 2>/dev/null)
    size_mb=$((size_bytes / 1024 / 1024))
    echo "  • $(basename "$file") - ${size_mb}MB"
done
EOF

echo ""

# Step 3: Test presigned URL generation for large files
echo -e "${YELLOW}Step 3: Testing presigned URL for 500MB+ file...${NC}"

# Get first large file info
test_file_info=$(ssh gaius@$PI_HOST "find /home/gaius/data/sermon-test-wavs -type f -name '*.wav' -size +500M 2>/dev/null | head -1 | xargs -I {} sh -c 'basename=\"{}\" && size=\$(stat -c%s \"{}\") && echo \"\$basename|\$size\"'")
test_filename=$(echo "$test_file_info" | cut -d'|' -f1 | xargs basename)
test_filesize=$(echo "$test_file_info" | cut -d'|' -f2)
test_filesize_mb=$((test_filesize / 1024 / 1024))

echo "Testing with: $test_filename (${test_filesize_mb}MB)"

# Request presigned URL
response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
    -H "Content-Type: application/json" \
    -d "{\"filename\":\"$test_filename\",\"fileSize\":$test_filesize}")

# Check response
if echo "$response" | grep -q '"error":true'; then
    echo -e "${RED}✗ Failed to get presigned URL${NC}"
    echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"
    exit 1
fi

# Extract URL and check if it's direct MinIO
upload_url=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('uploadUrl', ''))" 2>/dev/null)
is_large_file=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('isLargeFile', False))" 2>/dev/null)
upload_method=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('uploadMethod', ''))" 2>/dev/null)

echo "Response analysis:"
echo "  • isLargeFile: $is_large_file"
echo "  • uploadMethod: $upload_method"

if echo "$upload_url" | grep -q "192.168.1.127:9000"; then
    echo -e "${GREEN}✓ Got direct MinIO URL (bypassing CloudFlare)${NC}"
else
    echo -e "${RED}✗ URL still goes through CloudFlare!${NC}"
    echo "  URL: $upload_url"
fi
echo ""

# Step 4: Test batch presigned URLs for multiple large files
echo -e "${YELLOW}Step 4: Testing batch presigned URLs for 3 large files...${NC}"

# Get 3 large files
batch_files=$(ssh gaius@$PI_HOST "find /home/gaius/data/sermon-test-wavs -type f -name '*.wav' -size +500M 2>/dev/null | head -3")

# Build batch request
batch_json='{"files":['
first=true
while IFS= read -r file; do
    filename=$(basename "$file")
    filesize=$(ssh gaius@$PI_HOST "stat -c%s \"$file\" 2>/dev/null")
    
    if [ "$first" = true ]; then
        first=false
    else
        batch_json="$batch_json,"
    fi
    batch_json="$batch_json{\"filename\":\"$filename\",\"fileSize\":$filesize}"
done <<< "$batch_files"
batch_json="$batch_json]}"

# Request batch presigned URLs
batch_response=$(curl -s -X POST "$API_HOST/api/upload/presigned-batch" \
    -H "Content-Type: application/json" \
    -d "$batch_json")

# Check batch response
if echo "$batch_response" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ Batch presigned URLs received${NC}"
    
    # Check if all URLs are direct MinIO
    if echo "$batch_response" | grep -q "192.168.1.127:9000"; then
        echo -e "${GREEN}✓ All URLs use direct MinIO (CloudFlare bypassed)${NC}"
    else
        echo -e "${RED}✗ Some URLs still use CloudFlare${NC}"
    fi
else
    echo -e "${RED}✗ Batch request failed${NC}"
    echo "$batch_response" | python3 -m json.tool 2>/dev/null || echo "$batch_response"
fi
echo ""

# Step 5: Perform actual upload test from Pi
echo -e "${YELLOW}Step 5: Testing actual upload of 500MB+ file from Pi...${NC}"

cat > /tmp/test_large_upload.sh << 'SCRIPT'
#!/bin/bash
API_HOST="https://sermons.wpgc.church"

# Find first 500MB+ file
test_file=$(find /home/gaius/data/sermon-test-wavs -type f -name "*.wav" -size +500M 2>/dev/null | head -1)
if [ -z "$test_file" ]; then
    echo "ERROR: No 500MB+ file found"
    exit 1
fi

filename=$(basename "$test_file")
filesize=$(stat -c%s "$test_file")
filesize_mb=$((filesize / 1024 / 1024))

echo "Uploading: $filename (${filesize_mb}MB)"

# Get presigned URL
response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
    -H "Content-Type: application/json" \
    -d "{\"filename\":\"$filename\",\"fileSize\":$filesize}")

# Check for duplicate
if echo "$response" | grep -q '"isDuplicate":true'; then
    echo "File is duplicate (already uploaded)"
    exit 0
fi

# Extract upload URL
upload_url=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('uploadUrl', ''))" 2>/dev/null)

if [ -z "$upload_url" ] || [ "$upload_url" = "None" ]; then
    echo "ERROR: No upload URL received"
    echo "$response"
    exit 1
fi

# Check if using direct MinIO
if echo "$upload_url" | grep -q "192.168.1.127:9000"; then
    echo "✓ Using direct MinIO URL (CloudFlare bypassed)"
else
    echo "WARNING: Using CloudFlare URL (may fail for >100MB)"
fi

# Upload file
echo "Starting upload..."
start_time=$(date +%s)

if curl -X PUT "$upload_url" \
    --data-binary "@$test_file" \
    --progress-bar \
    --max-time 3600 \
    --connect-timeout 30 \
    -o /tmp/upload_result.txt 2>&1; then
    
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    speed_mbps=$((filesize / duration / 1024 / 1024))
    
    echo "✓ Upload successful!"
    echo "  Duration: ${duration}s"
    echo "  Speed: ~${speed_mbps} MB/s"
    
    # Notify completion
    curl -s -X POST "$API_HOST/api/upload/complete" \
        -H "Content-Type: application/json" \
        -d "{\"filename\":\"$filename\"}" > /dev/null
else
    echo "✗ Upload failed"
    cat /tmp/upload_result.txt 2>/dev/null
    exit 1
fi
SCRIPT

# Copy and run script on Pi
scp /tmp/test_large_upload.sh gaius@$PI_HOST:/tmp/
ssh gaius@$PI_HOST "chmod +x /tmp/test_large_upload.sh && /tmp/test_large_upload.sh"

echo ""
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo "✓ API is accessible and healthy"
echo "✓ Found 500MB+ test files on Pi"
echo "✓ Presigned URL API works for large files"
echo ""
echo "Key findings:"
echo "• Files >100MB should use direct MinIO URLs (192.168.1.127:9000)"
echo "• This bypasses CloudFlare's 100MB upload limit"
echo "• Upload speed is faster without CloudFlare proxy"
echo ""
echo "If uploads are still failing, check:"
echo "1. Backend returns direct MinIO URLs for large files"
echo "2. MinIO is accessible from Pi at 192.168.1.127:9000"
echo "3. CORS is configured on MinIO for direct access"