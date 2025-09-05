#!/bin/bash
# Pre-commit hook to validate audio content-type headers
# Ensures proper content-type handling for bit-perfect audio uploads

set -e

echo "🎵 Validating audio content-type headers..."

# Files to check
FILES_TO_CHECK=""
if [ $# -eq 0 ]; then
    FILES_TO_CHECK=$(find . -type f \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" \) \
        -not -path "./node_modules/*" \
        -not -path "./.git/*" \
        -not -path "./vendor/*")
else
    FILES_TO_CHECK="$@"
fi

VIOLATIONS_FOUND=0
GOOD_PATTERNS_FOUND=0

# Required content-type patterns for audio uploads
REQUIRED_CONTENT_TYPES=(
    "audio/wav"
    "application/octet-stream"
)

# Forbidden content-type patterns that could cause compression
FORBIDDEN_CONTENT_TYPES=(
    "text/plain.*wav"
    "application/json.*wav"
    "multipart/form-data.*audio"
    "text/.*audio"
)

# Forbidden headers that enable compression
FORBIDDEN_HEADERS=(
    "Content-Encoding.*gzip"
    "Content-Encoding.*deflate"
    "Transfer-Encoding.*gzip"
    "Accept-Encoding.*gzip.*audio"
)

# Function to check content-type patterns in a file
check_content_types() {
    local file="$1"
    local violations=0
    
    echo "  Checking content-types in: $file"
    
    # Check for forbidden content-types
    for pattern in "${FORBIDDEN_CONTENT_TYPES[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Forbidden content-type pattern '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check for forbidden compression headers
    for pattern in "${FORBIDDEN_HEADERS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Compression header pattern '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check for required content-types in audio handling code
    if grep -q -i "wav\|audio" "$file"; then
        local has_good_pattern=0
        for pattern in "${REQUIRED_CONTENT_TYPES[@]}"; do
            if grep -q -i "$pattern" "$file"; then
                echo "    ✅ Found required content-type '$pattern' in $file"
                has_good_pattern=1
                GOOD_PATTERNS_FOUND=$((GOOD_PATTERNS_FOUND + 1))
                break
            fi
        done
        
        # If file handles audio but doesn't have proper content-type
        if [ $has_good_pattern -eq 0 ] && grep -q -i "upload.*wav\|wav.*upload" "$file"; then
            echo "    ⚠️  WARNING: Audio upload code without proper content-type in $file"
            echo "       Consider adding 'audio/wav' or 'application/octet-stream'"
        fi
    fi
    
    # Special checks for Go files
    if [[ "$file" == *.go ]]; then
        # Check for proper MinIO content-type setting
        if grep -q "PutObject" "$file" && grep -q "wav" "$file"; then
            if ! grep -q "ContentType.*audio/wav" "$file"; then
                echo "    ⚠️  WARNING: MinIO PutObject for WAV without audio/wav content-type"
            fi
        fi
        
        # Check for proper HTTP response headers
        if grep -q "\.Header\(\)\.Set\|w\.Header\(\)\.Add" "$file" && grep -q "wav" "$file"; then
            if ! grep -q "Content-Type.*audio" "$file"; then
                echo "    ⚠️  WARNING: HTTP headers set for audio without proper content-type"
            fi
        fi
    fi
    
    # Special checks for TypeScript/JavaScript files
    if [[ "$file" == *.ts ]] || [[ "$file" == *.tsx ]] || [[ "$file" == *.js ]]; then
        # Check for proper fetch/axios headers
        if grep -q "fetch\|axios" "$file" && grep -q "wav\|audio" "$file"; then
            if grep -q "Content-Type.*text\|Content-Type.*json" "$file"; then
                echo "    ❌ VIOLATION: Text/JSON content-type for audio upload in $file"
                violations=$((violations + 1))
            fi
        fi
        
        # Check for proper FormData content-type handling
        if grep -q "FormData\|multipart" "$file" && grep -q "wav" "$file"; then
            # FormData should set proper content-type automatically
            if grep -q "Content-Type.*multipart" "$file"; then
                echo "    ⚠️  WARNING: Manual multipart content-type (should be automatic)"
            fi
        fi
    fi
    
    return $violations
}

# Check each file
for file in $FILES_TO_CHECK; do
    if [[ -f "$file" ]]; then
        check_content_types "$file"
        if [ $? -gt 0 ]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + $?))
        fi
    fi
done

# Additional validation for specific patterns
echo ""
echo "🔍 Additional Content-Type Validation:"

# Check for proper WAV file handling
WAV_HANDLERS=$(grep -l -i "\.wav\|audio/wav" $FILES_TO_CHECK 2>/dev/null || true)
if [ -n "$WAV_HANDLERS" ]; then
    echo "  Files handling WAV content:"
    for handler in $WAV_HANDLERS; do
        echo "    - $handler"
        
        # Verify proper content-type usage
        if grep -q "audio/wav" "$handler"; then
            echo "      ✅ Uses audio/wav content-type"
        elif grep -q "application/octet-stream" "$handler"; then
            echo "      ✅ Uses application/octet-stream content-type" 
        else
            echo "      ⚠️  No explicit audio content-type found"
        fi
    done
fi

# Check for HTTP compression middleware
MIDDLEWARE_FILES=$(grep -l -i "compression\|gzip.*middleware" $FILES_TO_CHECK 2>/dev/null || true)
if [ -n "$MIDDLEWARE_FILES" ]; then
    echo ""
    echo "  ❌ Files with compression middleware (may affect audio uploads):"
    for middleware in $MIDDLEWARE_FILES; do
        echo "    - $middleware"
        grep -n -i "compression\|gzip.*middleware" "$middleware" | head -2
    done
    VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
fi

echo ""
echo "🎵 Content-Type Check Summary:"
echo "  Files checked: $(echo $FILES_TO_CHECK | wc -w)"
echo "  Violations found: $VIOLATIONS_FOUND" 
echo "  Good patterns found: $GOOD_PATTERNS_FOUND"

if [ $VIOLATIONS_FOUND -gt 0 ]; then
    echo ""
    echo "❌ CRITICAL: Content-type violations found!"
    echo "   Improper content-type handling can compromise audio upload integrity."
    echo ""
    echo "   Required for audio uploads:"
    echo "   - Use 'audio/wav' for WAV files"
    echo "   - Use 'application/octet-stream' for binary uploads"
    echo "   - Avoid text/plain or application/json for audio"
    echo "   - Never use compression headers with audio content"
    echo ""
    echo "   Example fixes:"
    echo "   Go: minio.PutObjectOptions{ContentType: \"audio/wav\"}"
    echo "   JS: fetch(url, { headers: { 'Content-Type': 'application/octet-stream' } })"
    echo ""
    exit 1
fi

echo "✅ All content-type patterns valid for bit-perfect audio uploads!"
exit 0