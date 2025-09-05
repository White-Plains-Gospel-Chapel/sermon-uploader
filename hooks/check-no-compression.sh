#!/bin/bash
# Pre-commit hook to prevent compression-introducing dependencies and code
# Critical for maintaining bit-perfect audio uploads

set -e

echo "🔍 Checking for compression-introducing code patterns..."

# Define forbidden patterns that could introduce compression
FORBIDDEN_PATTERNS=(
    # Go compression libraries
    "compress/gzip"
    "compress/zlib" 
    "compress/lzw"
    "compress/flate"
    "github.com/klauspost/compress"
    "github.com/golang/snappy"
    
    # JavaScript/TypeScript compression libraries
    "pako"
    "zlib"
    "gzip"
    "compress"
    "lz-string"
    "snappy"
    
    # HTTP compression headers (dangerous for audio uploads)
    "gzip.*encoding"
    "deflate.*encoding"
    "br.*encoding"
    "Content-Encoding.*gzip"
    "Content-Encoding.*deflate"
    "Accept-Encoding.*gzip"
    
    # Compression middleware patterns
    "compression.*middleware"
    "gzip.*middleware"
    "compress.*handler"
    
    # Audio compression formats
    "audio/mp3"
    "audio/mpeg"
    "audio/aac"
    "audio/ogg"
    "audio/flac"
    ".mp3"
    ".aac" 
    ".ogg"
    ".flac"
    
    # Content-Type that allows compression
    "text/plain.*audio"
    "application/json.*audio"
)

# Additional patterns specific to audio handling
AUDIO_COMPRESSION_PATTERNS=(
    # FFmpeg compression parameters
    "-acodec.*mp3"
    "-acodec.*aac"
    "-c:a.*mp3"
    "-c:a.*aac"
    "libmp3lame"
    "libfdk_aac"
    
    # Audio processing that might compress
    "audioconvert"
    "audioresample"
    "volume.*compress"
    "dynamics.*compress"
)

# Files to check
FILES_TO_CHECK=""
if [ $# -eq 0 ]; then
    # Check all relevant files if no specific files provided
    FILES_TO_CHECK=$(find . -type f \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.json" \) \
        -not -path "./node_modules/*" \
        -not -path "./.git/*" \
        -not -path "./vendor/*" \
        -not -path "./test-utils/*" \
        -not -path "./__tests__/*")
else
    FILES_TO_CHECK="$@"
fi

VIOLATIONS_FOUND=0

# Function to check for patterns in a file
check_file() {
    local file="$1"
    local violations=0
    
    echo "  Checking: $file"
    
    # Check for forbidden compression patterns
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Found forbidden compression pattern '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -3
            violations=$((violations + 1))
        fi
    done
    
    # Check for audio compression patterns
    for pattern in "${AUDIO_COMPRESSION_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Found audio compression pattern '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -3
            violations=$((violations + 1))
        fi
    done
    
    # Special checks for package.json files
    if [[ "$file" == *"package.json"* ]]; then
        if grep -q '"compression"' "$file" || grep -q '"express-compression"' "$file"; then
            echo "    ❌ VIOLATION: Compression middleware found in package.json"
            violations=$((violations + 1))
        fi
    fi
    
    # Special checks for Go mod files
    if [[ "$file" == *"go.mod"* ]]; then
        if grep -q "compress/" "$file"; then
            echo "    ❌ VIOLATION: Compression library found in go.mod"
            violations=$((violations + 1))
        fi
    fi
    
    return $violations
}

# Check each file
for file in $FILES_TO_CHECK; do
    if [[ -f "$file" ]]; then
        check_file "$file"
        if [ $? -gt 0 ]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + $?))
        fi
    fi
done

# Check for specific safe patterns that should be preserved
echo "✅ Verifying safe audio handling patterns..."

REQUIRED_PATTERNS=(
    "application/octet-stream"    # Binary upload content type
    "audio/wav"                   # WAV content type
    "Content-Type.*audio/wav"     # WAV header
    "binary.*upload"              # Binary upload handling
)

SAFE_PATTERNS_FOUND=0
for file in $FILES_TO_CHECK; do
    if [[ -f "$file" ]]; then
        for pattern in "${REQUIRED_PATTERNS[@]}"; do
            if grep -q "$pattern" "$file"; then
                echo "  ✅ Found safe pattern '$pattern' in $file"
                SAFE_PATTERNS_FOUND=$((SAFE_PATTERNS_FOUND + 1))
                break
            fi
        done
    fi
done

# Summary
echo ""
echo "🔍 Compression Check Summary:"
echo "  Files checked: $(echo $FILES_TO_CHECK | wc -w)"
echo "  Violations found: $VIOLATIONS_FOUND"
echo "  Safe patterns found: $SAFE_PATTERNS_FOUND"

if [ $VIOLATIONS_FOUND -gt 0 ]; then
    echo ""
    echo "❌ CRITICAL: Compression-related code found!"
    echo "   This could compromise bit-perfect audio upload integrity."
    echo "   Please remove compression libraries, middleware, and headers"
    echo "   that could affect audio file uploads."
    echo ""
    echo "   Safe alternatives:"
    echo "   - Use application/octet-stream for audio uploads"
    echo "   - Implement raw binary transfer without compression"
    echo "   - Ensure MinIO uploads preserve original file integrity"
    echo ""
    exit 1
fi

echo "✅ No compression-related violations found. Audio integrity preserved!"
exit 0