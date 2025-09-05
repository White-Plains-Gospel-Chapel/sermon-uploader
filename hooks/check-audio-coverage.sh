#!/bin/bash
# Pre-commit hook to validate test coverage for audio-related code
# Ensures critical audio handling paths are properly tested

set -e

echo "📊 Validating test coverage for audio handling code..."

# Configuration
MIN_COVERAGE=80  # Minimum coverage percentage for audio code
BACKEND_DIR="./backend"
FRONTEND_DIR="./frontend"

COVERAGE_VIOLATIONS=0

# Files to check for coverage
FILES_TO_CHECK=""
if [ $# -eq 0 ]; then
    FILES_TO_CHECK=$(find . -type f \( -name "*.go" -o -name "*.ts" -o -name "*.tsx" \) \
        -not -path "./node_modules/*" \
        -not -path "./.git/*" \
        -not -path "./vendor/*")
else
    FILES_TO_CHECK="$@"
fi

# Function to check if file handles audio
is_audio_related() {
    local file="$1"
    
    # Check for audio-related keywords
    if grep -q -i "wav\|audio\|minio.*upload\|file.*service\|upload.*service" "$file"; then
        return 0
    fi
    
    # Check for critical functions
    if grep -q -i "putobject\|uploadfile\|processfiles\|calculatehash" "$file"; then
        return 0  
    fi
    
    return 1
}

# Function to check Go test coverage
check_go_coverage() {
    echo "🔍 Checking Go backend test coverage..."
    
    if [[ ! -d "$BACKEND_DIR" ]]; then
        echo "  ⚠️  Backend directory not found, skipping Go coverage"
        return 0
    fi
    
    cd "$BACKEND_DIR"
    
    # Find audio-related Go files
    local audio_files=$(find . -name "*.go" -not -name "*_test.go" -exec grep -l -i "wav\|audio\|minio\|upload" {} \; 2>/dev/null)
    
    if [ -z "$audio_files" ]; then
        echo "  ⚠️  No audio-related Go files found"
        cd - >/dev/null
        return 0
    fi
    
    echo "  Audio-related Go files found:"
    for file in $audio_files; do
        echo "    - $file"
    done
    
    # Generate coverage report
    echo "  📈 Generating coverage report..."
    if go test -coverprofile=coverage.out -covermode=atomic ./... >/dev/null 2>&1; then
        
        # Check overall coverage
        local total_coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')
        echo "  📊 Overall Go coverage: ${total_coverage}%"
        
        # Check coverage for each audio-related file
        echo "  🔍 Audio file coverage details:"
        local violations=0
        
        for file in $audio_files; do
            local clean_file=$(echo "$file" | sed 's/^\.\///')
            local file_coverage=$(go tool cover -func=coverage.out | grep "$clean_file" | awk '{print $3}' | sed 's/%//' | head -1)
            
            if [ -n "$file_coverage" ]; then
                printf "    - %-30s %6s%%\n" "$clean_file" "$file_coverage"
                
                if (( $(echo "$file_coverage < $MIN_COVERAGE" | bc -l) )); then
                    echo "      ❌ Below minimum coverage threshold (${MIN_COVERAGE}%)"
                    violations=$((violations + 1))
                fi
            else
                echo "    - $clean_file: No coverage data"
                echo "      ❌ No test coverage found"
                violations=$((violations + 1))
            fi
        done
        
        # Check for specific critical functions
        echo "  🎯 Critical function coverage:"
        local critical_functions=("UploadFile" "ProcessFiles" "CalculateFileHash" "EnsureBucketExists")
        
        for func in "${critical_functions[@]}"; do
            if go tool cover -func=coverage.out | grep -q "$func"; then
                local func_coverage=$(go tool cover -func=coverage.out | grep "$func" | awk '{print $3}' | sed 's/%//')
                if [ -n "$func_coverage" ]; then
                    printf "    - %-25s %6s%%\n" "$func" "$func_coverage"
                    if (( $(echo "$func_coverage < 100" | bc -l) )); then
                        echo "      ⚠️  Critical function not fully covered"
                    fi
                fi
            else
                echo "    - $func: Not found in coverage"
            fi
        done
        
        # Clean up
        rm -f coverage.out
        
        COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + violations))
        
    else
        echo "  ❌ Failed to generate Go coverage report"
        COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + 1))
    fi
    
    cd - >/dev/null
}

# Function to check TypeScript test coverage
check_typescript_coverage() {
    echo "🔍 Checking TypeScript frontend test coverage..."
    
    if [[ ! -d "$FRONTEND_DIR" ]]; then
        echo "  ⚠️  Frontend directory not found, skipping TypeScript coverage"
        return 0
    fi
    
    cd "$FRONTEND_DIR"
    
    # Check if package.json exists
    if [[ ! -f "package.json" ]]; then
        echo "  ⚠️  package.json not found, skipping frontend coverage"
        cd - >/dev/null
        return 0
    fi
    
    # Find audio-related TypeScript files
    local audio_files=$(find . -name "*.ts" -o -name "*.tsx" -not -path "./node_modules/*" -not -name "*.test.*" -not -name "*.spec.*" | xargs grep -l -i "wav\|audio\|upload\|validation" 2>/dev/null || true)
    
    if [ -z "$audio_files" ]; then
        echo "  ⚠️  No audio-related TypeScript files found"
        cd - >/dev/null
        return 0
    fi
    
    echo "  Audio-related TypeScript files found:"
    for file in $audio_files; do
        echo "    - $file"
    done
    
    # Generate coverage report
    echo "  📈 Generating frontend coverage report..."
    if npm test -- --coverage --silent --watchAll=false --collectCoverageFrom="**/*.{ts,tsx}" --collectCoverageFrom="!**/*.{test,spec}.*" >/dev/null 2>&1; then
        
        # Check if coverage directory exists
        if [[ -d "coverage" ]]; then
            echo "  ✅ Coverage report generated successfully"
            
            # Try to extract coverage data from lcov.info if it exists
            if [[ -f "coverage/lcov.info" ]]; then
                echo "  🔍 Audio file coverage details:"
                local violations=0
                
                for file in $audio_files; do
                    local clean_file=$(echo "$file" | sed 's/^\.\///')
                    
                    # Extract coverage for this file from lcov.info
                    if grep -q "SF:.*$clean_file" coverage/lcov.info; then
                        local lines_found=$(grep -A 20 "SF:.*$clean_file" coverage/lcov.info | grep "LF:" | cut -d':' -f2)
                        local lines_hit=$(grep -A 20 "SF:.*$clean_file" coverage/lcov.info | grep "LH:" | cut -d':' -f2)
                        
                        if [ -n "$lines_found" ] && [ -n "$lines_hit" ] && [ "$lines_found" -gt 0 ]; then
                            local coverage=$(echo "scale=1; $lines_hit * 100 / $lines_found" | bc)
                            printf "    - %-30s %6s%%\n" "$clean_file" "$coverage"
                            
                            if (( $(echo "$coverage < $MIN_COVERAGE" | bc -l) )); then
                                echo "      ❌ Below minimum coverage threshold (${MIN_COVERAGE}%)"
                                violations=$((violations + 1))
                            fi
                        else
                            echo "    - $clean_file: No valid coverage data"
                            violations=$((violations + 1))
                        fi
                    else
                        echo "    - $clean_file: Not found in coverage report"
                        violations=$((violations + 1))
                    fi
                done
                
                COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + violations))
            else
                echo "  ⚠️  lcov.info not found, cannot analyze detailed coverage"
            fi
            
        else
            echo "  ❌ Coverage directory not found"
            COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + 1))
        fi
        
    else
        echo "  ❌ Failed to generate frontend coverage report"
        COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + 1))
    fi
    
    cd - >/dev/null
}

# Function to check for missing test files
check_missing_tests() {
    echo "🔍 Checking for missing test files..."
    
    local violations=0
    
    # Check for Go files without corresponding tests
    local go_files=$(find . -name "*.go" -not -name "*_test.go" -not -path "./vendor/*" -not -path "./.git/*")
    
    for go_file in $go_files; do
        if is_audio_related "$go_file"; then
            local test_file="${go_file%%.go}_test.go"
            if [[ ! -f "$test_file" ]]; then
                echo "  ❌ Missing test file for audio-related Go file: $go_file"
                echo "    Expected: $test_file"
                violations=$((violations + 1))
            fi
        fi
    done
    
    # Check for TypeScript files without corresponding tests
    local ts_files=$(find . -name "*.ts" -o -name "*.tsx" -not -name "*.test.*" -not -name "*.spec.*" -not -path "./node_modules/*" -not -path "./.git/*")
    
    for ts_file in $ts_files; do
        if is_audio_related "$ts_file"; then
            local base_name=$(basename "$ts_file" | sed 's/\.(ts|tsx)$//')
            local dir_name=$(dirname "$ts_file")
            
            # Look for test files in common patterns
            local test_patterns=(
                "${ts_file%%.ts*}.test.ts"
                "${ts_file%%.ts*}.spec.ts"
                "${dir_name}/__tests__/${base_name}.test.ts"
                "${dir_name}/__tests__/${base_name}.spec.ts"
            )
            
            local test_found=false
            for pattern in "${test_patterns[@]}"; do
                if [[ -f "$pattern" ]]; then
                    test_found=true
                    break
                fi
            done
            
            if [ "$test_found" = false ]; then
                echo "  ⚠️  No test file found for audio-related TypeScript file: $ts_file"
                echo "    Consider adding tests at: ${dir_name}/__tests__/${base_name}.test.ts"
                violations=$((violations + 1))
            fi
        fi
    done
    
    COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + violations))
}

# Function to check for critical test scenarios
check_critical_test_scenarios() {
    echo "🎯 Checking for critical test scenarios..."
    
    local violations=0
    
    # Critical scenarios that must be tested
    local critical_scenarios=(
        "bit.*perfect\|bit-perfect"
        "hash.*verification\|hash.*integrity"
        "compression.*prevent\|no.*compression"
        "binary.*handling\|raw.*data"
        "large.*file.*upload"
        "concurrent.*upload"
        "duplicate.*detection"
        "error.*handling.*upload"
    )
    
    # Find all test files
    local test_files=$(find . -name "*_test.go" -o -name "*.test.ts" -o -name "*.spec.ts" -not -path "./node_modules/*")
    
    echo "  Critical scenarios to test:"
    for scenario in "${critical_scenarios[@]}"; do
        local found=false
        
        for test_file in $test_files; do
            if grep -q -i "$scenario" "$test_file"; then
                found=true
                break
            fi
        done
        
        if [ "$found" = true ]; then
            echo "    ✅ $scenario"
        else
            echo "    ❌ Missing: $scenario"
            violations=$((violations + 1))
        fi
    done
    
    COVERAGE_VIOLATIONS=$((COVERAGE_VIOLATIONS + violations))
}

# Main execution
echo "📊 Starting audio code coverage validation..."
echo "  Minimum coverage threshold: ${MIN_COVERAGE}%"
echo ""

# Check prerequisites
if ! command -v bc &> /dev/null; then
    echo "❌ 'bc' calculator not found. Please install for coverage calculations."
    exit 1
fi

# Run coverage checks
check_go_coverage
check_typescript_coverage
check_missing_tests
check_critical_test_scenarios

# Summary
echo ""
echo "📊 Audio Test Coverage Summary:"
echo "  Coverage violations found: $COVERAGE_VIOLATIONS"

if [ $COVERAGE_VIOLATIONS -gt 0 ]; then
    echo ""
    echo "❌ CRITICAL: Audio code coverage violations found!"
    echo "   Critical audio handling code lacks adequate test coverage."
    echo ""
    echo "   Required actions:"
    echo "   - Add tests for all audio-related functions"
    echo "   - Achieve minimum ${MIN_COVERAGE}% coverage for audio code"
    echo "   - Test all critical scenarios (bit-perfect, hash verification, etc.)"
    echo "   - Add integration tests for upload workflows"
    echo ""
    echo "   Test file naming conventions:"
    echo "   - Go: filename_test.go (same package)"
    echo "   - TypeScript: filename.test.ts or __tests__/filename.test.ts"
    echo ""
    echo "   Focus on testing:"
    echo "   - File upload integrity (hash verification)"
    echo "   - Binary data handling (no text conversion)"
    echo "   - Error scenarios (network failures, corruption)"
    echo "   - Large file handling (memory efficiency)"
    echo ""
    exit 1
fi

echo "✅ All audio-related code has adequate test coverage!"
echo "   Critical audio handling functionality is properly tested."
exit 0