#!/bin/bash
# Pre-commit hook to check for hardcoded audio quality settings
# Prevents hardcoded compression parameters that could affect audio quality

set -e

echo "⚙️ Checking for hardcoded audio quality settings..."

# Files to check
FILES_TO_CHECK=""
if [ $# -eq 0 ]; then
    FILES_TO_CHECK=$(find . -type f \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" -o -name "*.js" -o -name "*.json" -o -name "*.yaml" -o -name "*.yml" \) \
        -not -path "./node_modules/*" \
        -not -path "./.git/*" \
        -not -path "./vendor/*")
else
    FILES_TO_CHECK="$@"
fi

VIOLATIONS_FOUND=0

# Hardcoded quality settings that should be configurable or avoided
HARDCODED_PATTERNS=(
    # Audio quality/bitrate hardcoding
    "bitrate.*[0-9]+"
    "quality.*[0-9]+"
    "compression.*[0-9]+"
    "sample.*rate.*[0-9]+"
    "bit.*depth.*[0-9]+"
    
    # FFmpeg hardcoded parameters
    "-ab [0-9]+"           # Audio bitrate
    "-ar [0-9]+"           # Audio sample rate  
    "-ac [0-9]+"           # Audio channels
    "-q:a [0-9]+"          # Audio quality
    "-compression_level [0-9]+"
    
    # Hardcoded buffer sizes for audio
    "buffer.*size.*[0-9]+.*audio"
    "chunk.*size.*[0-9]+.*audio"
    "audio.*buffer.*[0-9]+"
    
    # HTTP request size limits (could affect large WAV uploads)
    "maxRequestSize.*[0-9]+"
    "bodyLimit.*[0-9]+"
    "uploadLimit.*[0-9]+"
)

# Dangerous hardcoded values that should never appear
DANGEROUS_HARDCODED=(
    # Compression levels
    "gzip.*level.*[0-9]+"
    "deflate.*level.*[0-9]+"
    "compress.*level.*[0-9]+"
    
    # Audio processing that implies quality loss
    "normalize.*[0-9]+"
    "volume.*[0-9]+"
    "gain.*[0-9]+"
    
    # Specific codec parameters
    "mp3.*bitrate"
    "aac.*bitrate"  
    "opus.*bitrate"
    "vorbis.*quality"
)

# File size limits that might be too restrictive for audio
SIZE_LIMIT_PATTERNS=(
    "max.*file.*size.*[0-9]+[KMG]?B?"
    "size.*limit.*[0-9]+[KMG]?B?"
    "upload.*limit.*[0-9]+[KMG]?B?"
    "body.*limit.*[0-9]+[KMG]?B?"
)

# Function to check for hardcoded patterns
check_hardcoded_settings() {
    local file="$1"
    local violations=0
    
    echo "  Checking: $file"
    
    # Check for hardcoded audio quality patterns
    for pattern in "${HARDCODED_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ⚠️  HARDCODED: '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check for dangerous hardcoded patterns
    for pattern in "${DANGEROUS_HARDCODED[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            echo "    ❌ DANGEROUS: '$pattern' in $file"
            grep -n -i --color=always "$pattern" "$file" | head -2
            violations=$((violations + 1))
        fi
    done
    
    # Check file size limits
    for pattern in "${SIZE_LIMIT_PATTERNS[@]}"; do
        if grep -n -i "$pattern" "$file" >/dev/null 2>&1; then
            local matches=$(grep -n -i "$pattern" "$file")
            echo "    🔍 SIZE LIMIT: '$pattern' in $file"
            echo "$matches" | while read -r match; do
                local line_num=$(echo "$match" | cut -d':' -f1)
                local content=$(echo "$match" | cut -d':' -f2-)
                
                # Extract numeric value
                local size_value=$(echo "$content" | grep -o '[0-9]\+[KMG]\?B\?' | head -1)
                if [ -n "$size_value" ]; then
                    # Convert to MB for comparison
                    local size_mb
                    case "$size_value" in
                        *GB|*G) size_mb=$(echo "$size_value" | sed 's/G.*//' | awk '{print $1 * 1024}') ;;
                        *MB|*M) size_mb=$(echo "$size_value" | sed 's/M.*//' | awk '{print $1}') ;;
                        *KB|*K) size_mb=$(echo "$size_value" | sed 's/K.*//' | awk '{print $1 / 1024}') ;;
                        *B) size_mb=$(echo "$size_value" | sed 's/B.*//' | awk '{print $1 / 1024 / 1024}') ;;
                        *) size_mb=$(echo "$size_value" | awk '{print $1 / 1024 / 1024}') ;;
                    esac
                    
                    # WAV files can be large (1GB+ for long sermons)
                    if (( $(echo "$size_mb < 1000" | bc -l 2>/dev/null || echo "0") )); then
                        echo "      ⚠️  Size limit ${size_value} may be too small for large WAV files"
                        violations=$((violations + 1))
                    else
                        echo "      ✅ Size limit ${size_value} adequate for large audio files"
                    fi
                fi
            done
        fi
    done
    
    # File-specific checks
    if [[ "$file" == *.json ]]; then
        check_json_settings "$file"
        violations=$((violations + $?))
    elif [[ "$file" == *.yaml ]] || [[ "$file" == *.yml ]]; then
        check_yaml_settings "$file"
        violations=$((violations + $?))
    elif [[ "$file" == *.go ]]; then
        check_go_settings "$file"
        violations=$((violations + $?))
    elif [[ "$file" == *.ts ]] || [[ "$file" == *.tsx ]] || [[ "$file" == *.js ]]; then
        check_javascript_settings "$file"
        violations=$((violations + $?))
    fi
    
    return $violations
}

# Check JSON configuration files
check_json_settings() {
    local file="$1"
    local violations=0
    
    # Check for hardcoded audio settings in JSON
    if grep -q -i "bitrate\|quality\|compression" "$file"; then
        echo "    🔍 JSON audio settings found in $file"
        
        # Extract potential quality settings
        local quality_settings=$(grep -n -i "\".*\(bitrate\|quality\|compression\).*\":" "$file" | head -5)
        if [ -n "$quality_settings" ]; then
            echo "      Settings found:"
            echo "$quality_settings" | while read -r setting; do
                echo "        $setting"
            done
            
            # Check if settings are hardcoded values vs environment variables
            if ! grep -q "process\.env\|\${.*}\|getenv" "$file"; then
                echo "      ⚠️  Settings appear to be hardcoded, consider using environment variables"
                violations=$((violations + 1))
            fi
        fi
    fi
    
    return $violations
}

# Check YAML configuration files
check_yaml_settings() {
    local file="$1"
    local violations=0
    
    # Check for hardcoded values in YAML
    if grep -q -i "bitrate\|quality\|compression\|limit" "$file"; then
        echo "    🔍 YAML configuration settings found in $file"
        
        # Look for hardcoded numeric values
        local numeric_settings=$(grep -n -i -E "(bitrate|quality|compression|limit).*[0-9]+" "$file")
        if [ -n "$numeric_settings" ]; then
            echo "      Numeric settings found:"
            echo "$numeric_settings" | head -5
            
            # Check if using environment variable substitution
            if ! grep -q "\${.*}\|%{.*}\|{{.*}}" "$file"; then
                echo "      ⚠️  Consider using environment variable substitution"
                violations=$((violations + 1))
            fi
        fi
    fi
    
    return $violations
}

# Check Go source files
check_go_settings() {
    local file="$1"
    local violations=0
    
    # Check for hardcoded constants
    if grep -q "const.*[0-9]\+" "$file" && grep -q -i "audio\|wav\|bitrate\|quality" "$file"; then
        echo "    🔍 Go constants with audio-related values in $file"
        
        local constants=$(grep -n "const.*[0-9]\+" "$file" | head -3)
        if [ -n "$constants" ]; then
            echo "      Constants found:"
            echo "$constants"
            
            # Check if constants are related to audio quality
            if grep -q -i "const.*\(bitrate\|quality\|sample.*rate\)" "$file"; then
                echo "      ⚠️  Consider making audio quality settings configurable"
                violations=$((violations + 1))
            fi
        fi
    fi
    
    # Check for hardcoded magic numbers in audio processing
    if grep -q -i "wav\|audio" "$file"; then
        local magic_numbers=$(grep -n -E "[^a-zA-Z][0-9]{4,}[^a-zA-Z]" "$file" | grep -v -E "test|Test" | head -3)
        if [ -n "$magic_numbers" ]; then
            echo "    🔍 Potential magic numbers in audio-related Go file:"
            echo "$magic_numbers"
            echo "      ℹ️  Consider using named constants for clarity"
        fi
    fi
    
    return $violations
}

# Check JavaScript/TypeScript files
check_javascript_settings() {
    local file="$1"
    local violations=0
    
    # Check for hardcoded values in audio-related JS/TS
    if grep -q -i "wav\|audio" "$file"; then
        # Check for hardcoded buffer sizes
        if grep -q -E "buffer.*size.*[0-9]+|chunk.*size.*[0-9]+" "$file"; then
            echo "    🔍 Buffer size settings in $file"
            local buffer_settings=$(grep -n -E "buffer.*size.*[0-9]+|chunk.*size.*[0-9]+" "$file" | head -3)
            echo "$buffer_settings"
            
            if ! grep -q "process\.env\|config\." "$file"; then
                echo "      ⚠️  Consider making buffer sizes configurable"
                violations=$((violations + 1))
            fi
        fi
        
        # Check for hardcoded quality settings
        if grep -q -E "quality.*[0-9]+|bitrate.*[0-9]+" "$file"; then
            echo "    🔍 Quality settings in $file"
            local quality_settings=$(grep -n -E "quality.*[0-9]+|bitrate.*[0-9]+" "$file" | head -3)
            echo "$quality_settings"
            echo "      ⚠️  Audio quality settings should be configurable"
            violations=$((violations + 1))
        fi
    fi
    
    return $violations
}

# Main execution
echo "⚙️ Checking for hardcoded audio quality settings..."
echo ""

for file in $FILES_TO_CHECK; do
    if [[ -f "$file" ]]; then
        check_hardcoded_settings "$file"
        if [ $? -gt 0 ]; then
            VIOLATIONS_FOUND=$((VIOLATIONS_FOUND + $?))
        fi
    fi
done

# Additional checks for common configuration files
echo ""
echo "🔍 Additional Configuration Checks:"

# Check for Docker configuration issues
DOCKER_FILES=$(find . -name "Dockerfile*" -o -name "docker-compose*.yml" -not -path "./.git/*")
if [ -n "$DOCKER_FILES" ]; then
    echo "  Docker configuration files found:"
    for docker_file in $DOCKER_FILES; do
        echo "    - $docker_file"
        
        # Check for resource limits that might affect large file uploads
        if grep -q -i "mem_limit\|memory\|cpus" "$docker_file"; then
            echo "      ℹ️  Resource limits found - verify they support large WAV uploads"
        fi
        
        # Check for nginx/proxy configurations
        if grep -q -i "client_max_body_size\|proxy_read_timeout" "$docker_file"; then
            echo "      ℹ️  HTTP proxy settings found - verify large file support"
        fi
    done
fi

# Check for environment variable templates
ENV_FILES=$(find . -name ".env*" -o -name "*.env" -not -path "./.git/*")
if [ -n "$ENV_FILES" ]; then
    echo ""
    echo "  Environment files found:"
    for env_file in $ENV_FILES; do
        echo "    - $env_file"
        
        if grep -q -i "quality\|bitrate\|limit" "$env_file"; then
            echo "      ✅ Audio configuration in environment file (good practice)"
        fi
    done
fi

echo ""
echo "⚙️ Quality Settings Check Summary:"
echo "  Files checked: $(echo $FILES_TO_CHECK | wc -w)"
echo "  Violations found: $VIOLATIONS_FOUND"

if [ $VIOLATIONS_FOUND -gt 0 ]; then
    echo ""
    echo "⚠️  WARNINGS: Hardcoded quality settings found!"
    echo "   While not critical, hardcoded settings reduce flexibility."
    echo ""
    echo "   Best practices:"
    echo "   - Use environment variables for quality settings"
    echo "   - Make file size limits configurable"
    echo "   - Avoid hardcoded audio processing parameters"
    echo "   - Use named constants instead of magic numbers"
    echo ""
    echo "   Example improvements:"
    echo "   - Go: const MaxFileSize = getEnvInt(\"MAX_FILE_SIZE\", 1000*1024*1024)"
    echo "   - JS: const bufferSize = process.env.BUFFER_SIZE || 32768"
    echo "   - Docker: client_max_body_size \${MAX_UPLOAD_SIZE:-1000m};"
    echo ""
    echo "   Note: This is a warning, not a blocking error."
    echo "         Consider addressing these for better maintainability."
    echo ""
fi

echo "✅ Quality settings check completed."
echo "   Audio upload integrity preserved through configurable settings."
exit 0