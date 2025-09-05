#!/bin/bash
# Pre-commit hook to validate WAV file handling as binary data
# Ensures WAV files are processed without data transformation

set -e

echo "🎼 Validating WAV file binary handling..."

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
BINARY_HANDLING_FOUND=0

# Patterns that indicate proper binary handling
GOOD_BINARY_PATTERNS=(
    "bytes\.NewReader"                # Go binary reader
    "io\.Copy.*reader"               # Go binary copy
    "arrayBuffer()"                  # JS binary buffer
    "application/octet-stream"       # Binary content type
    "binary\.Read\|binary\.Write"    # Go binary operations
    "new Uint8Array"                 # JS typed array
    "Buffer\.from"                   # Node.js buffer
    "ReadAll.*bytes"                 # Go read all bytes
)

# Patterns that could corrupt binary data
BAD_BINARY_PATTERNS=(
    "toString().*wav"                # Converting binary to string
    "JSON\.stringify.*wav"           # JSON serialization of binary
    "\.split.*wav"                   # String splitting on binary
    "\.replace.*wav"                 # String replacement on binary
    "text/plain.*wav"               # Text content type for binary
    "charset=utf-8.*wav"            # UTF-8 encoding for binary
    "encoding.*utf8.*wav"           # UTF-8 encoding
    "ioutil\.ReadFile.*string"      # Go string read of binary
)

# Text processing patterns that shouldn't be used on WAV files
TEXT_PROCESSING_PATTERNS=(
    "strings\..*wav"                # Go string operations on WAV
    "regexp\..*wav"                 # Regex on binary data
    "\.trim().*wav"                 # String trimming
    "\.toLowerCase().*wav"          # String case conversion
    "\.normalize().*wav"            # String normalization
)

# Function to check binary handling patterns
check_binary_handling() {
    local file="$1"
    local violations=0
    
    echo "  Checking binary handling in: $file"
    
    # Skip if file doesn't handle WAV/audio
    if ! grep -q -i "wav\|audio" "$file"; then
        return 0
    fi
    
    # Check for bad binary patterns
    for pattern in "${BAD_BINARY_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Binary corruption risk '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check for text processing on binary data
    for pattern in "${TEXT_PROCESSING_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ VIOLATION: Text processing on binary data '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check for good binary patterns
    local has_good_pattern=0
    for pattern in "${GOOD_BINARY_PATTERNS[@]}"; do
        if grep -q -i "$pattern" "$file"; then
            echo "    ✅ Found proper binary handling '$pattern' in $file"
            has_good_pattern=1
            BINARY_HANDLING_FOUND=$((BINARY_HANDLING_FOUND + 1))
            break
        fi
    done
    
    # Language-specific checks
    if [[ "$file" == *.go ]]; then
        check_go_binary_handling "$file"
        violations=$((violations + $?))
    elif [[ "$file" == *.ts ]] || [[ "$file" == *.tsx ]] || [[ "$file" == *.js ]]; then
        check_js_binary_handling "$file"  
        violations=$((violations + $?))
    fi
    
    return $violations
}

# Go-specific binary handling checks
check_go_binary_handling() {
    local file="$1"
    local violations=0
    
    # Check for proper byte array handling
    if grep -q "wav" "$file"; then
        # Should use []byte for WAV data
        if grep -q "func.*string.*wav\|wav.*string" "$file"; then
            echo "    ⚠️  WARNING: String parameter for WAV data in $file"
            echo "       Consider using []byte for binary data"
        fi
        
        # Check for proper file reading
        if grep -q "ioutil\.ReadFile\|os\.ReadFile" "$file"; then
            if ! grep -q "[]byte" "$file"; then
                echo "    ⚠️  WARNING: File reading without explicit byte handling"
            fi
        fi
        
        # Check for proper MinIO upload
        if grep -q "PutObject" "$file"; then
            if ! grep -q "bytes\.NewReader\|io\.Reader" "$file"; then
                echo "    ❌ VIOLATION: MinIO upload without proper reader in $file"
                violations=$((violations + 1))
            fi
        fi
        
        # Check multipart file handling
        if grep -q "multipart\.File" "$file"; then
            if grep -q "Read.*string\|string.*Read" "$file"; then
                echo "    ❌ VIOLATION: Reading multipart file as string in $file"
                violations=$((violations + 1))
            fi
        fi
    fi
    
    return $violations
}

# JavaScript/TypeScript-specific binary handling checks  
check_js_binary_handling() {
    local file="$1"
    local violations=0
    
    if grep -q "wav\|audio" "$file"; then
        # Check for proper File API usage
        if grep -q "File\|Blob" "$file"; then
            if ! grep -q "arrayBuffer\|stream" "$file"; then
                echo "    ⚠️  WARNING: File handling without binary methods in $file"
                echo "       Use .arrayBuffer() or .stream() for binary data"
            fi
        fi
        
        # Check for FormData usage
        if grep -q "FormData" "$file"; then
            if grep -q "JSON\.stringify.*append\|toString.*append" "$file"; then
                echo "    ❌ VIOLATION: Converting binary to text in FormData in $file"
                violations=$((violations + 1))
            fi
        fi
        
        # Check for fetch/upload handling
        if grep -q "fetch.*wav\|upload.*wav" "$file"; then
            if grep -q "JSON\.stringify\|\.toString()" "$file"; then
                echo "    ❌ VIOLATION: Text serialization of WAV data in $file"
                violations=$((violations + 1))
            fi
        fi
        
        # Check for proper typed arrays
        if grep -q "audio.*process\|wav.*process" "$file"; then
            if ! grep -q "Uint8Array\|ArrayBuffer\|DataView" "$file"; then
                echo "    ⚠️  WARNING: Audio processing without typed arrays in $file"
            fi
        fi
    fi
    
    return $violations
}

# Check each file
for file in $FILES_TO_CHECK; do
    if [[ -f "$file" ]]; then
        check_binary_handling "$file"
        if [ $? -gt 0 ]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + $?))
        fi
    fi
done

# Additional comprehensive checks
echo ""
echo "🔍 Additional Binary Handling Validation:"

# Look for WAV-specific handling code
WAV_FILES=$(grep -l "\.wav\|WAV\|audio/wav" $FILES_TO_CHECK 2>/dev/null || true)
if [ -n "$WAV_FILES" ]; then
    echo "  Files with WAV handling:"
    for wav_file in $WAV_FILES; do
        echo "    - $wav_file"
        
        # Check specific risky patterns
        if grep -q "base64.*wav\|btoa.*wav" "$wav_file"; then
            echo "      ❌ Base64 encoding found (increases size, not bit-perfect)"
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        fi
        
        if grep -q "JSON.*wav" "$wav_file"; then
            echo "      ❌ JSON processing of WAV data found"
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        fi
        
        if grep -q "encodeURI.*wav\|decodeURI.*wav" "$wav_file"; then
            echo "      ❌ URI encoding of binary data found"
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        fi
    done
fi

# Check for hash verification code
HASH_FILES=$(grep -l "sha256\|hash.*wav" $FILES_TO_CHECK 2>/dev/null || true)
if [ -n "$HASH_FILES" ]; then
    echo ""
    echo "  Files with hash verification:"
    for hash_file in $HASH_FILES; do
        echo "    - $hash_file"
        
        # Verify hash is calculated on binary data
        if grep -q "hash.*toString\|hash.*string" "$hash_file"; then
            echo "      ❌ Hash calculated on string representation"
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + 1))
        else
            echo "      ✅ Hash appears to be calculated on binary data"
        fi
    done
fi

echo ""
echo "🎼 WAV Binary Handling Summary:"
echo "  Files checked: $(echo $FILES_TO_CHECK | wc -w)"
echo "  Violations found: $VIOLATIONS_FOUND"
echo "  Proper binary handling found: $BINARY_HANDLING_FOUND"

if [ $VIOLATIONS_FOUND -gt 0 ]; then
    echo ""
    echo "❌ CRITICAL: Binary handling violations found!"
    echo "   Improper binary handling will corrupt WAV files and break bit-perfect uploads."
    echo ""
    echo "   Required for WAV files:"
    echo "   - Always handle as binary data ([]byte in Go, ArrayBuffer in JS)"
    echo "   - Never convert to string or apply text operations"
    echo "   - Use binary readers/writers for I/O operations"
    echo "   - Calculate hashes on raw binary data, not string representations"
    echo "   - Use application/octet-stream content-type for uploads"
    echo ""
    echo "   Example proper patterns:"
    echo "   Go: bytes.NewReader(wavData), io.Copy(writer, reader)"
    echo "   JS: file.arrayBuffer(), new Uint8Array(buffer)"
    echo ""
    exit 1
fi

echo "✅ All WAV files handled as binary data. Bit-perfect integrity maintained!"
exit 0