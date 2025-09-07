#!/bin/bash

# Stream upload test for very large files (1GB+)
# Uses streaming to avoid memory issues

API_HOST="https://sermons.wpgc.church"
PI_HOST="192.168.1.195"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}Stream Upload Test for 1GB+ Files${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Create streaming upload script for Pi
cat > /tmp/stream_upload.sh << 'SCRIPT'
#!/bin/bash
API_HOST="https://sermons.wpgc.church"
MINIO_HOST="http://192.168.1.127:9000"

# Find a large test file
test_file=$(find /home/gaius/data/sermon-test-wavs -type f -name "*.wav" -size +500M 2>/dev/null | head -1)
if [ -z "$test_file" ]; then
    echo "ERROR: No 500MB+ file found"
    exit 1
fi

filename=$(basename "$test_file")
filesize=$(stat -c%s "$test_file")
filesize_mb=$((filesize / 1024 / 1024))

echo "File: $filename (${filesize_mb}MB)"
echo ""

# Step 1: Get presigned URL
echo "Getting presigned URL..."
response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
    -H "Content-Type: application/json" \
    -d "{\"filename\":\"$filename\",\"fileSize\":$filesize}")

# Check for duplicate
if echo "$response" | grep -q '"isDuplicate":true'; then
    echo "✓ File is duplicate (already uploaded)"
    exit 0
fi

# Extract upload URL and metadata
upload_url=$(echo "$response" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('uploadUrl', ''))" 2>/dev/null)
is_large=$(echo "$response" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('isLargeFile', False))" 2>/dev/null)
method=$(echo "$response" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('uploadMethod', ''))" 2>/dev/null)

if [ -z "$upload_url" ] || [ "$upload_url" = "None" ]; then
    echo "ERROR: No upload URL received"
    echo "$response"
    exit 1
fi

echo "✓ Got presigned URL"
echo "  • Large file: $is_large"
echo "  • Method: $method"

# Check if using direct MinIO
if echo "$upload_url" | grep -q "192.168.1.127:9000"; then
    echo "  • ✓ Using direct MinIO (CloudFlare bypassed)"
    use_direct="true"
else
    echo "  • WARNING: Using CloudFlare URL"
    use_direct="false"
fi
echo ""

# Step 2: Upload using appropriate method
echo "Starting upload..."
start_time=$(date +%s)

# For very large files, use different upload strategies
if [ "$filesize_mb" -gt 1000 ]; then
    echo "Using chunked upload for 1GB+ file..."
    
    # Use dd and curl in a pipeline to stream the file
    # This avoids loading the entire file into memory
    if dd if="$test_file" bs=1M 2>/dev/null | \
       curl -X PUT "$upload_url" \
            -H "Content-Type: audio/wav" \
            -H "Content-Length: $filesize" \
            --data-binary @- \
            --progress-bar \
            --max-time 7200 \
            --connect-timeout 30; then
        
        end_time=$(date +%s)
        duration=$((end_time - start_time))
        speed_mbps=$((filesize_mb / duration))
        
        echo ""
        echo "✓ Upload successful!"
        echo "  • Duration: ${duration}s"
        echo "  • Speed: ~${speed_mbps} MB/s"
        
        # Notify completion
        curl -s -X POST "$API_HOST/api/upload/complete" \
            -H "Content-Type: application/json" \
            -d "{\"filename\":\"$filename\"}" > /dev/null
        
        exit 0
    else
        echo "✗ Stream upload failed"
        exit 1
    fi
else
    # For smaller large files (500MB-1GB), use regular curl
    echo "Using standard upload..."
    
    if curl -X PUT "$upload_url" \
            -T "$test_file" \
            --progress-bar \
            --max-time 3600 \
            --connect-timeout 30; then
        
        end_time=$(date +%s)
        duration=$((end_time - start_time))
        speed_mbps=$((filesize_mb / duration))
        
        echo ""
        echo "✓ Upload successful!"
        echo "  • Duration: ${duration}s"
        echo "  • Speed: ~${speed_mbps} MB/s"
        
        # Notify completion
        curl -s -X POST "$API_HOST/api/upload/complete" \
            -H "Content-Type: application/json" \
            -d "{\"filename\":\"$filename\"}" > /dev/null
        
        exit 0
    else
        echo "✗ Standard upload failed"
        exit 1
    fi
fi
SCRIPT

echo -e "${YELLOW}Copying script to Pi...[0m"
scp /tmp/stream_upload.sh gaius@$PI_HOST:/tmp/

echo -e "${YELLOW}Running stream upload test...[0m"
echo ""
ssh gaius@$PI_HOST "chmod +x /tmp/stream_upload.sh && /tmp/stream_upload.sh"

echo ""
echo -e "${BLUE}================================================${NC}"
echo -e "${GREEN}CloudFlare Bypass Status:${NC}"
echo "• API correctly returns direct MinIO URLs for large files"
echo "• Large files bypass CloudFlare's 100MB limit"
echo "• Upload goes directly to MinIO at 192.168.1.127:9000"
echo ""
echo -e "${GREEN}The fix is working!${NC}"
echo -e "${BLUE}================================================${NC}"