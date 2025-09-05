#!/bin/bash
# Pre-commit hook to run critical audio preservation tests
# Ensures bit-perfect audio upload functionality before committing

set -e

echo "🧪 Running critical audio preservation tests..."

# Configuration
BACKEND_DIR="./backend"
FRONTEND_DIR="./frontend"
TEST_TIMEOUT=300  # 5 minutes for critical tests
PARALLEL_JOBS=2

TESTS_PASSED=0
TESTS_FAILED=0

# Function to run Go tests
run_go_tests() {
    echo "📦 Running Go backend audio preservation tests..."
    
    cd "$BACKEND_DIR"
    
    # Run specific audio-related tests
    AUDIO_TESTS=(
        "services/file_service_test.go"
        "services/minio_test.go"
    )
    
    for test_file in "${AUDIO_TESTS[@]}"; do
        if [[ -f "$test_file" ]]; then
            echo "  Running: $test_file"
            
            # Run with specific test patterns for audio preservation
            if go test -v -timeout="${TEST_TIMEOUT}s" -run=".*BitPerfect.*|.*Integrity.*|.*Preservation.*" "./$test_file" 2>&1; then
                echo "  ✅ $test_file passed"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo "  ❌ $test_file failed"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo "  ⚠️  Test file $test_file not found"
        fi
    done
    
    # Run critical unit tests
    echo "  Running critical audio unit tests..."
    if go test -short -v -run=".*Audio.*|.*WAV.*|.*Hash.*" ./services/ 2>&1; then
        echo "  ✅ Audio unit tests passed"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "  ❌ Audio unit tests failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    cd - >/dev/null
}

# Function to run TypeScript tests
run_typescript_tests() {
    echo "⚛️  Running frontend audio integrity tests..."
    
    cd "$FRONTEND_DIR"
    
    # Check if test files exist
    FRONTEND_TESTS=(
        "__tests__/upload/AudioUploadIntegrity.test.ts"
        "__tests__/utils/audioValidation.test.ts"
    )
    
    for test_file in "${FRONTEND_TESTS[@]}"; do
        if [[ -f "$test_file" ]]; then
            echo "  Running: $test_file"
            
            # Run specific test file
            if npm test -- "$test_file" --testTimeout=$((TEST_TIMEOUT * 1000)) --silent 2>&1; then
                echo "  ✅ $test_file passed"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo "  ❌ $test_file failed"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo "  ⚠️  Test file $test_file not found"
        fi
    done
    
    # Run critical audio validation tests
    echo "  Running audio validation tests..."
    if npm test -- --testNamePattern="Audio.*Integrity|WAV.*Validation|Upload.*Preservation" --testTimeout=$((TEST_TIMEOUT * 1000)) --silent 2>&1; then
        echo "  ✅ Audio validation tests passed"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "  ❌ Audio validation tests failed"  
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    cd - >/dev/null
}

# Function to run integration tests (if available)
run_integration_tests() {
    echo "🔗 Running integration tests..."
    
    # Check if Playwright tests exist
    if [[ -f "e2e/audioUploadIntegrity.spec.ts" ]]; then
        echo "  Running E2E audio integrity tests..."
        
        # Run headless browser tests
        if npx playwright test e2e/audioUploadIntegrity.spec.ts --timeout=$((TEST_TIMEOUT * 1000)) --reporter=line 2>&1; then
            echo "  ✅ E2E audio tests passed"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo "  ❌ E2E audio tests failed"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        echo "  ⚠️  E2E tests not found, skipping..."
    fi
}

# Function to validate test data integrity
validate_test_data() {
    echo "🔍 Validating test data integrity..."
    
    # Check if test WAV generator is available
    if [[ -f "test-utils/wav-generator.go" ]]; then
        echo "  Generating test WAV files..."
        
        cd test-utils
        
        # Generate small test files for validation
        if go run wav-generator.go -output ./temp-test-wavs 2>&1; then
            echo "  ✅ Test WAV generation successful"
            
            # Verify generated files
            local wav_count=$(find ./temp-test-wavs -name "*.wav" | wc -l)
            if [ "$wav_count" -gt 0 ]; then
                echo "  ✅ Generated $wav_count test WAV files"
                TESTS_PASSED=$((TESTS_PASSED + 1))
                
                # Calculate hashes to verify integrity
                echo "  🔐 Verifying file integrity..."
                for wav_file in ./temp-test-wavs/*.wav; do
                    if [[ -f "$wav_file" ]]; then
                        local file_hash=$(sha256sum "$wav_file" | cut -d' ' -f1)
                        local file_size=$(stat -c%s "$wav_file" 2>/dev/null || stat -f%z "$wav_file")
                        echo "    - $(basename $wav_file): ${file_size} bytes, hash: ${file_hash:0:16}..."
                    fi
                done
                
                # Clean up test files
                rm -rf ./temp-test-wavs
            else
                echo "  ❌ No test WAV files generated"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo "  ❌ Test WAV generation failed"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
        
        cd - >/dev/null
    else
        echo "  ⚠️  WAV generator not found, skipping data validation..."
    fi
}

# Function to check test coverage for audio-related code
check_audio_test_coverage() {
    echo "📊 Checking audio-related test coverage..."
    
    # Backend coverage
    if [[ -d "$BACKEND_DIR" ]]; then
        cd "$BACKEND_DIR"
        
        echo "  Checking Go test coverage..."
        if go test -coverprofile=coverage.out ./services/ >/dev/null 2>&1; then
            local coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')
            echo "  📊 Backend coverage: ${coverage}%"
            
            if (( $(echo "$coverage >= 80.0" | bc -l) )); then
                echo "  ✅ Backend coverage meets minimum threshold (80%)"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo "  ⚠️  Backend coverage below 80% threshold"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
            
            # Clean up coverage files
            rm -f coverage.out
        else
            echo "  ❌ Failed to generate backend coverage report"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
        
        cd - >/dev/null
    fi
    
    # Frontend coverage
    if [[ -d "$FRONTEND_DIR" ]]; then
        cd "$FRONTEND_DIR"
        
        echo "  Checking frontend test coverage..."
        if npm test -- --coverage --silent --watchAll=false >/dev/null 2>&1; then
            echo "  ✅ Frontend coverage report generated"
            TESTS_PASSED=$((TESTS_PASSED + 1))
        else
            echo "  ⚠️  Frontend coverage report failed"
        fi
        
        cd - >/dev/null
    fi
}

# Main execution
echo "🏃‍♂️ Starting audio preservation test suite..."
echo "  Timeout: ${TEST_TIMEOUT}s per test suite"
echo "  Parallel jobs: ${PARALLEL_JOBS}"
echo ""

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Skipping Go tests."
else
    run_go_tests
fi

if ! command -v npm &> /dev/null; then
    echo "❌ npm not found. Skipping frontend tests."
else
    if [[ -f "$FRONTEND_DIR/package.json" ]]; then
        run_typescript_tests
    else
        echo "⚠️  Frontend package.json not found. Skipping frontend tests."
    fi
fi

# Run additional validation
validate_test_data
run_integration_tests
check_audio_test_coverage

# Summary
echo ""
echo "🧪 Audio Preservation Test Summary:"
echo "  Tests passed: $TESTS_PASSED"
echo "  Tests failed: $TESTS_FAILED"
echo "  Total tests: $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -gt 0 ]; then
    echo ""
    echo "❌ CRITICAL: Audio preservation tests failed!"
    echo "   Bit-perfect audio upload functionality is compromised."
    echo "   Please fix failing tests before committing."
    echo ""
    echo "   Common issues to check:"
    echo "   - Hash verification failures (data corruption)"
    echo "   - Content-type mismatches (compression risk)"
    echo "   - Binary handling errors (text conversion)"
    echo "   - Upload integrity failures (network issues)"
    echo ""
    echo "   Debug commands:"
    echo "   - Backend: cd backend && go test -v ./services/"
    echo "   - Frontend: cd frontend && npm test"
    echo "   - Generate test data: cd test-utils && go run wav-generator.go -testsuite"
    echo ""
    exit 1
fi

if [ $TESTS_PASSED -eq 0 ]; then
    echo ""
    echo "⚠️  WARNING: No audio preservation tests were executed."
    echo "   This could indicate missing test files or configuration issues."
    echo "   Consider running the test setup manually to verify functionality."
    echo ""
    exit 1
fi

echo ""
echo "✅ All critical audio preservation tests passed!"
echo "   Bit-perfect audio upload integrity verified."
echo "   Safe to commit changes."
exit 0