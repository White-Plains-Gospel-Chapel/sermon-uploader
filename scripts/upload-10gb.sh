#!/bin/bash

# Memory-efficient solution for uploading 10GB files
# No memory loading - pure streaming

API_HOST="https://sermons.wpgc.church"

echo "10GB Upload Solution"
echo "===================="
echo ""
echo "Method 1: Using curl -T (most efficient)"
echo "-----------------------------------------"
echo "curl -X PUT <presigned_url> -T /path/to/10gb.wav"
echo ""
echo "This streams the file without loading into memory."
echo ""
echo "Method 2: Using dd and curl pipe"
echo "---------------------------------"
echo "dd if=/path/to/10gb.wav bs=64M | curl -X PUT <presigned_url> --data-binary @-"
echo ""
echo "Method 3: Using MinIO client directly"
echo "--------------------------------------"
echo "mc cp /path/to/10gb.wav myminio/sermons/wav/"
echo ""

# Function to get presigned URL and upload
upload_large_file() {
    local file="$1"
    local filename=$(basename "$file")
    local filesize=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)
    
    echo "Getting presigned URL for: $filename"
    
    # Get presigned URL from API
    response=$(curl -s -X POST "$API_HOST/api/upload/presigned" \
        -H "Content-Type: application/json" \
        -d "{\"filename\":\"$filename\",\"fileSize\":$filesize}")
    
    url=$(echo "$response" | python3 -c "import sys, json; print(json.load(sys.stdin).get('uploadUrl', ''))" 2>/dev/null)
    
    if [ -n "$url" ]; then
        echo "Uploading with curl -T (streaming)..."
        # This is the KEY - curl -T streams the file
        curl -X PUT "$url" -T "$file" --progress-bar
    fi
}

# If file provided, upload it
if [ $# -gt 0 ]; then
    upload_large_file "$1"
fi