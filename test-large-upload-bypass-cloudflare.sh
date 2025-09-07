#!/bin/bash

# Test script for uploading 500MB+ files bypassing CloudFlare
# This script uses presigned URLs to upload directly to MinIO

API_HOST="https://sermons.wpgc.church"
PI_HOST="192.168.1.195"
TEST_DIR="/home/gaius/data/sermon-test-wavs"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Large File Upload Test - CloudFlare Bypass${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Function to test API health
test_api_health() {
    echo -e "${YELLOW}Testing API health...${NC}"
    response=$(curl -s -o /dev/null -w "%{http_code}" "$API_HOST/api/health")
    
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✓ API is healthy${NC}"
        return 0
    else
        echo -e "${RED}✗ API health check failed (HTTP $response)${NC}"
        return 1
    fi
}

# Function to get presigned URL for upload
get_presigned_url() {
    local filename="$1"
    local filesize="$2"
    
    echo -e "${YELLOW}Requesting presigned URL for: $filename ($(( filesize / 1024 / 1024 ))MB)${NC}"
    
    response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
        -H "Content-Type: application/json" \
        -d "{\"filename\":\"$filename\",\"fileSize\":$filesize}")
    
    # Check if response contains error
    if echo "$response" | grep -q '"error":true'; then
        echo -e "${RED}✗ Failed to get presigned URL${NC}"
        echo "$response" | jq '.'
        return 1
    fi
    
    # Check if duplicate
    if echo "$response" | grep -q '"isDuplicate":true'; then
        echo -e "${YELLOW}⚠ File is duplicate, skipping${NC}"
        return 2
    fi
    
    # Extract upload URL
    upload_url=$(echo "$response" | jq -r '.uploadUrl')
    
    if [ -z "$upload_url" ] || [ "$upload_url" = "null" ]; then
        echo -e "${RED}✗ No upload URL received${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ Got presigned URL${NC}"
    echo "$upload_url"
    return 0
}

# Function to upload file using presigned URL
upload_with_presigned_url() {
    local file_path="$1"
    local upload_url="$2"
    local filename=$(basename "$file_path")
    
    echo -e "${YELLOW}Uploading file directly to MinIO (bypassing CloudFlare)...${NC}"
    
    # Upload with progress bar
    start_time=$(date +%s)
    
    # Use curl with progress and timeout settings for large files
    if curl -X PUT "$upload_url" \
        --data-binary "@$file_path" \
        --progress-bar \
        --max-time 3600 \
        --connect-timeout 30 \
        -o /tmp/upload_response.txt 2>&1; then
        
        end_time=$(date +%s)
        duration=$((end_time - start_time))
        file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null)
        speed=$(( file_size / duration / 1024 / 1024 ))
        
        echo -e "${GREEN}✓ Upload successful!${NC}"
        echo -e "  Duration: ${duration}s"
        echo -e "  Speed: ~${speed} MB/s"
        
        # Notify completion
        curl -s -X POST "$API_HOST/api/upload/complete" \
            -H "Content-Type: application/json" \
            -d "{\"filename\":\"$filename\"}" > /dev/null
        
        return 0
    else
        echo -e "${RED}✗ Upload failed${NC}"
        cat /tmp/upload_response.txt
        return 1
    fi
}

# Function to test TUS resumable upload
test_tus_upload() {
    local file_path="$1"
    local filename=$(basename "$file_path")
    local file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null)
    
    echo -e "${YELLOW}Testing TUS resumable upload for: $filename${NC}"
    
    # Create TUS upload session
    response=$(curl -s -X POST "$API_HOST/api/tus" \
        -H "Tus-Resumable: 1.0.0" \
        -H "Upload-Length: $file_size" \
        -H "Upload-Metadata: filename $(echo -n "$filename" | base64)")
    
    # Extract upload ID from Location header
    upload_id=$(echo "$response" | jq -r '.id' 2>/dev/null)
    
    if [ -z "$upload_id" ] || [ "$upload_id" = "null" ]; then
        echo -e "${RED}✗ Failed to create TUS session${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ TUS session created: $upload_id${NC}"
    
    # Upload in chunks (10MB chunks for reliability)
    chunk_size=$((10 * 1024 * 1024))
    offset=0
    
    while [ $offset -lt $file_size ]; do
        remaining=$((file_size - offset))
        current_chunk=$((remaining < chunk_size ? remaining : chunk_size))
        
        echo -n "  Uploading chunk at offset $offset..."
        
        # Extract chunk and upload
        dd if="$file_path" bs=1 skip=$offset count=$current_chunk 2>/dev/null | \
        curl -s -X PATCH "$API_HOST/api/tus/$upload_id" \
            -H "Tus-Resumable: 1.0.0" \
            -H "Upload-Offset: $offset" \
            -H "Content-Type: application/offset+octet-stream" \
            --data-binary @- > /dev/null
        
        if [ $? -eq 0 ]; then
            echo -e " ${GREEN}✓${NC}"
            offset=$((offset + current_chunk))
        else
            echo -e " ${RED}✗${NC}"
            return 1
        fi
    done
    
    echo -e "${GREEN}✓ TUS upload complete${NC}"
    return 0
}

# Main test execution
echo "Step 1: Testing API connectivity"
echo "================================="
if ! test_api_health; then
    echo -e "${RED}API is not accessible. Exiting.${NC}"
    exit 1
fi
echo ""

echo "Step 2: Finding large test files (500MB+)"
echo "========================================="

# Create test script to run on Pi
cat > /tmp/test_upload_from_pi.sh << 'EOF'
#!/bin/bash

TEST_DIR="/home/gaius/data/sermon-test-wavs"
API_HOST="https://sermons.wpgc.church"

# Find files 500MB or larger
echo "Searching for files 500MB or larger..."
large_files=$(find "$TEST_DIR" -type f -name "*.wav" -size +500M 2>/dev/null | head -5)

if [ -z "$large_files" ]; then
    echo "No files 500MB or larger found"
    exit 1
fi

echo "Found large files:"
echo "$large_files" | while read -r file; do
    size=$(stat -c%s "$file" 2>/dev/null)
    size_mb=$((size / 1024 / 1024))
    echo "  - $(basename "$file") (${size_mb}MB)"
done

# Test with first large file
test_file=$(echo "$large_files" | head -1)
filename=$(basename "$test_file")
filesize=$(stat -c%s "$test_file")

echo ""
echo "Testing with: $filename ($(( filesize / 1024 / 1024 ))MB)"
echo ""

# Method 1: Presigned URL
echo "Method 1: Presigned URL Upload"
echo "------------------------------"

# Get presigned URL
echo "Requesting presigned URL..."
response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
    -H "Content-Type: application/json" \
    -d "{\"filename\":\"$filename\",\"fileSize\":$filesize}")

if echo "$response" | grep -q '"isDuplicate":true'; then
    echo "File is duplicate, skipping"
else
    upload_url=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('uploadUrl', ''))")
    
    if [ -n "$upload_url" ] && [ "$upload_url" != "null" ]; then
        echo "Got presigned URL, uploading..."
        start_time=$(date +%s)
        
        # Upload directly to MinIO
        if curl -X PUT "$upload_url" \
            --data-binary "@$test_file" \
            --progress-bar \
            --max-time 3600 \
            --connect-timeout 30; then
            
            end_time=$(date +%s)
            duration=$((end_time - start_time))
            speed=$(( filesize / duration / 1024 / 1024 ))
            
            echo "✓ Upload successful!"
            echo "  Duration: ${duration}s"
            echo "  Speed: ~${speed} MB/s"
            
            # Notify completion
            curl -s -X POST "$API_HOST/api/upload/complete" \
                -H "Content-Type: application/json" \
                -d "{\"filename\":\"$filename\"}" > /dev/null
        else
            echo "✗ Upload failed"
        fi
    else
        echo "Failed to get presigned URL"
        echo "$response"
    fi
fi

echo ""
echo "Test complete!"
EOF

echo -e "${YELLOW}Copying test script to Pi...${NC}"
scp /tmp/test_upload_from_pi.sh gaius@$PI_HOST:/tmp/

echo -e "${YELLOW}Running test on Pi...${NC}"
echo ""
ssh gaius@$PI_HOST "chmod +x /tmp/test_upload_from_pi.sh && /tmp/test_upload_from_pi.sh"

echo ""
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Test Complete${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo "Summary:"
echo "--------"
echo "The workaround for CloudFlare's 100MB limit uses:"
echo "1. Presigned URLs - Upload directly to MinIO at 192.168.1.127:9000"
echo "2. TUS Protocol - Resumable uploads in chunks"
echo "3. Both methods bypass CloudFlare's proxy"
echo ""
echo "Files go directly to MinIO, avoiding CloudFlare's restrictions!"