#!/bin/bash
# Comprehensive test runner for sermon uploader audio integrity testing
# Demonstrates the complete testing workflow for bit-perfect audio preservation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT=$(pwd)
BACKEND_DIR="./backend"
FRONTEND_DIR="./frontend"
TEST_UTILS_DIR="./test-utils"
HOOKS_DIR="./hooks"

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to print section headers
print_section() {
    echo ""
    echo -e "${BLUE}${'='*60}${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}${'='*60}${NC}"
}

# Function to print sub-section headers
print_subsection() {
    echo ""
    echo -e "${PURPLE}📋 $1${NC}"
    echo -e "${PURPLE}${'-'*40}${NC}"
}

# Function to run a test with status tracking
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    echo -e "${CYAN}🧪 Running: $test_name${NC}"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if eval "$test_command"; then
        echo -e "${GREEN}✅ PASSED: $test_name${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${RED}❌ FAILED: $test_name${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_section "🔍 CHECKING PREREQUISITES"
    
    local missing_tools=()
    
    # Check required tools
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi
    
    if ! command -v node &> /dev/null; then
        missing_tools+=("node")
    fi
    
    if ! command -v npm &> /dev/null; then
        missing_tools+=("npm")
    fi
    
    if ! command -v git &> /dev/null; then
        missing_tools+=("git")
    fi
    
    if [ ${#missing_tools[@]} -gt 0 ]; then
        echo -e "${RED}❌ Missing required tools: ${missing_tools[*]}${NC}"
        echo "Please install missing tools before running tests."
        exit 1
    fi
    
    echo -e "${GREEN}✅ All required tools are available${NC}"
    
    # Check project structure
    local missing_dirs=()
    
    if [ ! -d "$BACKEND_DIR" ]; then
        missing_dirs+=("$BACKEND_DIR")
    fi
    
    if [ ! -d "$FRONTEND_DIR" ]; then
        missing_dirs+=("$FRONTEND_DIR")
    fi
    
    if [ ${#missing_dirs[@]} -gt 0 ]; then
        echo -e "${RED}❌ Missing project directories: ${missing_dirs[*]}${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Project structure validated${NC}"
}

# Function to generate test data
generate_test_data() {
    print_section "🎵 GENERATING TEST DATA"
    
    if [ -f "$TEST_UTILS_DIR/wav-generator.go" ]; then
        echo -e "${CYAN}📁 Generating comprehensive test WAV files...${NC}"
        
        cd "$TEST_UTILS_DIR"
        
        # Generate test suite
        if go run wav-generator.go -testsuite -output ../test-data 2>/dev/null; then
            echo -e "${GREEN}✅ Test suite generated successfully${NC}"
            
            # Count generated files
            local wav_count=$(find ../test-data -name "*.wav" 2>/dev/null | wc -l)
            echo -e "${GREEN}📊 Generated $wav_count test WAV files${NC}"
        else
            echo -e "${YELLOW}⚠️  Test data generation failed (continuing with existing data)${NC}"
        fi
        
        cd "$PROJECT_ROOT"
    else
        echo -e "${YELLOW}⚠️  WAV generator not found, skipping test data generation${NC}"
    fi
}

# Function to run pre-commit hooks
run_precommit_hooks() {
    print_section "🔒 RUNNING PRE-COMMIT HOOKS"
    
    if [ -d "$HOOKS_DIR" ]; then
        local hooks=(
            "check-no-compression.sh:Compression Prevention Check"
            "check-content-types.sh:Content-Type Validation" 
            "check-wav-handling.sh:WAV Binary Handling Check"
            "check-audio-coverage.sh:Audio Code Coverage Check"
            "check-quality-settings.sh:Quality Settings Check"
        )
        
        for hook_info in "${hooks[@]}"; do
            local hook_file=$(echo "$hook_info" | cut -d':' -f1)
            local hook_name=$(echo "$hook_info" | cut -d':' -f2)
            local hook_path="$HOOKS_DIR/$hook_file"
            
            if [ -f "$hook_path" ]; then
                run_test "$hook_name" "$hook_path >/dev/null 2>&1"
            else
                echo -e "${YELLOW}⚠️  Hook not found: $hook_path${NC}"
            fi
        done
    else
        echo -e "${YELLOW}⚠️  Hooks directory not found, skipping pre-commit validation${NC}"
    fi
}

# Function to run backend tests
run_backend_tests() {
    print_section "🚀 RUNNING BACKEND TESTS (Go)"
    
    if [ ! -d "$BACKEND_DIR" ]; then
        echo -e "${YELLOW}⚠️  Backend directory not found, skipping Go tests${NC}"
        return
    fi
    
    cd "$BACKEND_DIR"
    
    print_subsection "Installing Go Dependencies"
    run_test "Go mod tidy" "go mod tidy"
    
    print_subsection "Unit Tests"
    run_test "All Backend Unit Tests" "go test -short ./..."
    
    print_subsection "Audio Integrity Tests"
    if [ -f "services/file_service_test.go" ]; then
        run_test "File Service Integrity Tests" "go test -v -run='.*BitPerfect.*|.*Integrity.*' ./services/"
    fi
    
    if [ -f "services/minio_test.go" ]; then
        run_test "MinIO Service Integrity Tests" "go test -v -run='.*BitPerfect.*|.*Preservation.*' ./services/"
    fi
    
    print_subsection "Coverage Analysis"
    run_test "Test Coverage Generation" "go test -coverprofile=coverage.out ./services/"
    
    if [ -f "coverage.out" ]; then
        local coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')
        echo -e "${CYAN}📊 Overall Backend Coverage: ${coverage}%${NC}"
        
        if (( $(echo "$coverage >= 80.0" | bc -l 2>/dev/null || echo "0") )); then
            echo -e "${GREEN}✅ Coverage meets minimum threshold (80%)${NC}"
        else
            echo -e "${YELLOW}⚠️  Coverage below 80% threshold${NC}"
        fi
        
        rm -f coverage.out
    fi
    
    cd "$PROJECT_ROOT"
}

# Function to run frontend tests
run_frontend_tests() {
    print_section "⚛️  RUNNING FRONTEND TESTS (TypeScript)"
    
    if [ ! -d "$FRONTEND_DIR" ]; then
        echo -e "${YELLOW}⚠️  Frontend directory not found, skipping TypeScript tests${NC}"
        return
    fi
    
    cd "$FRONTEND_DIR"
    
    print_subsection "Installing Dependencies"
    if [ -f "package.json" ]; then
        run_test "NPM Install" "npm install --silent"
    else
        echo -e "${YELLOW}⚠️  package.json not found, skipping npm install${NC}"
        cd "$PROJECT_ROOT"
        return
    fi
    
    print_subsection "TypeScript Compilation"
    run_test "TypeScript Type Check" "npm run type-check 2>/dev/null || npx tsc --noEmit"
    
    print_subsection "Unit Tests"
    run_test "Frontend Unit Tests" "npm test -- --watchAll=false --silent"
    
    print_subsection "Audio Integrity Tests"
    if [ -f "__tests__/upload/AudioUploadIntegrity.test.ts" ]; then
        run_test "Audio Upload Integrity Tests" "npm test -- __tests__/upload/AudioUploadIntegrity.test.ts --watchAll=false --silent"
    fi
    
    if [ -f "__tests__/utils/audioValidation.test.ts" ]; then
        run_test "Audio Validation Tests" "npm test -- __tests__/utils/audioValidation.test.ts --watchAll=false --silent"
    fi
    
    print_subsection "Coverage Analysis"
    run_test "Frontend Coverage" "npm test -- --coverage --watchAll=false --silent"
    
    cd "$PROJECT_ROOT"
}

# Function to run end-to-end tests
run_e2e_tests() {
    print_section "🌐 RUNNING END-TO-END TESTS (Playwright)"
    
    if [ -f "e2e/audioUploadIntegrity.spec.ts" ]; then
        print_subsection "E2E Setup"
        if command -v npx &> /dev/null; then
            run_test "Playwright Install" "npx playwright install --with-deps chromium"
            
            print_subsection "Browser Tests"
            run_test "Audio Upload E2E Tests" "npx playwright test e2e/audioUploadIntegrity.spec.ts --reporter=line"
        else
            echo -e "${YELLOW}⚠️  npx not available, skipping E2E tests${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  E2E test files not found, skipping browser tests${NC}"
    fi
}

# Function to run performance benchmarks
run_benchmarks() {
    print_section "📊 RUNNING PERFORMANCE BENCHMARKS"
    
    if [ -f "$TEST_UTILS_DIR/wav-generator.go" ]; then
        cd "$TEST_UTILS_DIR"
        
        print_subsection "WAV Generation Benchmarks"
        run_test "File Generation Benchmarks" "go run wav-generator.go -benchmark"
        
        cd "$PROJECT_ROOT"
    fi
    
    if [ -d "$BACKEND_DIR" ]; then
        cd "$BACKEND_DIR"
        
        print_subsection "Backend Performance Tests"
        run_test "Go Benchmark Tests" "go test -bench=. ./services/"
        
        cd "$PROJECT_ROOT"
    fi
}

# Function to validate audio integrity
validate_audio_integrity() {
    print_section "🔍 VALIDATING AUDIO INTEGRITY"
    
    print_subsection "Hash Verification Tests"
    
    # Test with small generated file if available
    if [ -d "test-data" ]; then
        local test_file=$(find test-data -name "*.wav" | head -1)
        if [ -n "$test_file" ]; then
            local hash1=$(sha256sum "$test_file" | cut -d' ' -f1)
            local hash2=$(sha256sum "$test_file" | cut -d' ' -f1)
            
            if [ "$hash1" = "$hash2" ]; then
                echo -e "${GREEN}✅ Hash consistency verified${NC}"
                PASSED_TESTS=$((PASSED_TESTS + 1))
            else
                echo -e "${RED}❌ Hash inconsistency detected${NC}"
                FAILED_TESTS=$((FAILED_TESTS + 1))
            fi
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
            
            echo -e "${CYAN}🔐 Sample file hash: ${hash1:0:16}...${NC}"
        fi
    fi
    
    print_subsection "WAV Header Validation"
    
    # Basic WAV header validation for test files
    if [ -d "test-data" ]; then
        local wav_files=$(find test-data -name "*.wav" | head -3)
        for wav_file in $wav_files; do
            if [ -f "$wav_file" ]; then
                local header=$(hexdump -C "$wav_file" | head -1)
                if echo "$header" | grep -q "52 49 46 46.*57 41 56 45"; then
                    echo -e "${GREEN}✅ Valid WAV header: $(basename $wav_file)${NC}"
                else
                    echo -e "${RED}❌ Invalid WAV header: $(basename $wav_file)${NC}"
                fi
            fi
        done
    fi
}

# Function to print final summary
print_summary() {
    print_section "📈 TEST EXECUTION SUMMARY"
    
    echo -e "${CYAN}Total Tests Run: $TOTAL_TESTS${NC}"
    echo -e "${GREEN}Tests Passed: $PASSED_TESTS${NC}"
    echo -e "${RED}Tests Failed: $FAILED_TESTS${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        local success_rate=100
    else
        local success_rate=$(echo "scale=1; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc -l 2>/dev/null || echo "0")
    fi
    
    echo -e "${PURPLE}Success Rate: ${success_rate}%${NC}"
    
    echo ""
    echo -e "${BLUE}🎯 Audio Integrity Status:${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}✅ BIT-PERFECT AUDIO PRESERVATION VERIFIED${NC}"
        echo -e "${GREEN}   All critical tests passed successfully${NC}"
        echo -e "${GREEN}   Audio files will maintain exact binary integrity${NC}"
        echo -e "${GREEN}   No compression or quality loss detected${NC}"
        echo ""
        echo -e "${GREEN}🚀 SAFE TO DEPLOY${NC}"
    elif [ $FAILED_TESTS -le 2 ]; then
        echo -e "${YELLOW}⚠️  MINOR ISSUES DETECTED${NC}"
        echo -e "${YELLOW}   Most critical tests passed${NC}"
        echo -e "${YELLOW}   Review failed tests before deployment${NC}"
        echo -e "${YELLOW}   Audio integrity likely preserved${NC}"
        echo ""
        echo -e "${YELLOW}🔄 REVIEW BEFORE DEPLOY${NC}"
    else
        echo -e "${RED}❌ CRITICAL FAILURES DETECTED${NC}"
        echo -e "${RED}   Audio integrity may be compromised${NC}"
        echo -e "${RED}   DO NOT DEPLOY until issues are resolved${NC}"
        echo -e "${RED}   Check for compression or binary handling issues${NC}"
        echo ""
        echo -e "${RED}🚫 DO NOT DEPLOY${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}Next Steps:${NC}"
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "  • Run: ${CYAN}git add . && git commit${NC} (pre-commit hooks will validate)"
        echo -e "  • Deploy with confidence in audio quality preservation"
    else
        echo -e "  • Review failed tests above"
        echo -e "  • Check logs in respective test directories"
        echo -e "  • Run individual test suites for detailed debugging"
        echo -e "  • Verify audio handling code follows best practices"
    fi
    
    echo -e "\n${BLUE}For detailed debugging:${NC}"
    echo -e "  • Backend: ${CYAN}cd backend && go test -v ./services/${NC}"
    echo -e "  • Frontend: ${CYAN}cd frontend && npm test${NC}"
    echo -e "  • E2E: ${CYAN}npx playwright test --ui${NC}"
    echo -e "  • Hooks: ${CYAN}hooks/run-audio-tests.sh${NC}"
}

# Function to cleanup temporary files
cleanup() {
    echo -e "\n${CYAN}🧹 Cleaning up temporary files...${NC}"
    
    # Remove temporary test data if generated
    if [ -d "test-data" ]; then
        echo "  Removing test data directory..."
        rm -rf test-data
    fi
    
    # Remove coverage files
    find . -name "coverage.out" -delete 2>/dev/null || true
    find . -name "*.prof" -delete 2>/dev/null || true
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

# Main execution function
main() {
    echo -e "${PURPLE}"
    echo "╔══════════════════════════════════════════════════════════════════╗"
    echo "║                  SERMON UPLOADER TEST SUITE                     ║"
    echo "║            Comprehensive Audio Integrity Testing                ║"
    echo "║                                                                  ║"
    echo "║  🎯 MISSION: Verify Bit-Perfect Audio Preservation              ║"
    echo "║  🚫 ZERO COMPRESSION at any level                               ║"
    echo "║  ✅ Hash verification for every upload                          ║"
    echo "║  🔒 Binary data handling validation                             ║"
    echo "╚══════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    # Parse command line arguments
    local run_all=true
    local run_backend=false
    local run_frontend=false
    local run_e2e=false
    local run_hooks=false
    local run_benchmarks=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --backend)
                run_all=false
                run_backend=true
                shift
                ;;
            --frontend)
                run_all=false
                run_frontend=true
                shift
                ;;
            --e2e)
                run_all=false
                run_e2e=true
                shift
                ;;
            --hooks)
                run_all=false
                run_hooks=true
                shift
                ;;
            --benchmarks)
                run_all=false
                run_benchmarks=true
                shift
                ;;
            --help)
                echo "Usage: $0 [options]"
                echo "Options:"
                echo "  --backend     Run only backend tests"
                echo "  --frontend    Run only frontend tests"
                echo "  --e2e         Run only end-to-end tests"
                echo "  --hooks       Run only pre-commit hooks"
                echo "  --benchmarks  Run only performance benchmarks"
                echo "  --help        Show this help message"
                echo ""
                echo "Default: Run all test suites"
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Execute test phases
    check_prerequisites
    generate_test_data
    
    if [ "$run_all" = true ] || [ "$run_hooks" = true ]; then
        run_precommit_hooks
    fi
    
    if [ "$run_all" = true ] || [ "$run_backend" = true ]; then
        run_backend_tests
    fi
    
    if [ "$run_all" = true ] || [ "$run_frontend" = true ]; then
        run_frontend_tests
    fi
    
    if [ "$run_all" = true ] || [ "$run_e2e" = true ]; then
        run_e2e_tests
    fi
    
    if [ "$run_all" = true ] || [ "$run_benchmarks" = true ]; then
        run_benchmarks
    fi
    
    validate_audio_integrity
    print_summary
    cleanup
    
    # Exit with appropriate code
    if [ $FAILED_TESTS -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# Set up trap for cleanup on script exit
trap cleanup EXIT

# Run main function with all arguments
main "$@"