# Comprehensive Testing Strategy for Bit-Perfect Audio Preservation

This document outlines the complete testing strategy for the sermon uploader system, with a **critical focus on maintaining bit-perfect audio quality** throughout the upload process.

## 🎯 Core Testing Objective

**ZERO COMPRESSION**: All audio files must maintain bit-perfect quality. No compression at any level - API responses only, never files.

## 📁 Testing Structure

```
sermon-uploader/
├── backend/
│   └── services/
│       ├── file_service_test.go      # File processing integrity tests
│       └── minio_test.go             # MinIO bit-perfect storage tests
├── frontend/
│   └── __tests__/
│       ├── upload/
│       │   └── AudioUploadIntegrity.test.ts    # Frontend integrity tests
│       └── utils/
│           └── audioValidation.test.ts         # WAV validation tests
├── e2e/
│   └── audioUploadIntegrity.spec.ts  # End-to-end browser tests
├── test-utils/
│   └── wav-generator.go              # Test data generator
├── hooks/                            # Pre-commit validation hooks
└── .pre-commit-config.yaml          # Pre-commit configuration
```

## 🧪 Test Categories

### 1. Backend Tests (Go)

#### File Service Tests (`backend/services/file_service_test.go`)
- **Bit-perfect preservation validation**
- **Hash verification for integrity**
- **Large file handling without corruption**
- **Duplicate detection by content hash**
- **Concurrent upload isolation**
- **Error handling without data loss**

Key test methods:
```go
TestFileService_ProcessFiles_BitPerfectPreservation()
TestFileService_DuplicateDetection_PreservesIntegrity()
TestFileService_LargeFileHandling_BitPerfect()
TestFileService_WAVHeaderIntegrity()
```

#### MinIO Service Tests (`backend/services/minio_test.go`)
- **Raw binary upload verification**
- **Integration tests with real MinIO container**
- **Large file chunked upload integrity**
- **Metadata preservation**
- **Concurrent upload isolation**
- **Hash calculation accuracy**

Key test methods:
```go
TestMinIOService_UploadFile_BitPerfectPreservation()
TestMinIOService_MultipleFiles_NoCrossContamination()
TestMinIOService_LargeFile_ChunkedUpload()
TestMinIOService_GetExistingHashes_Accuracy()
```

### 2. Frontend Tests (TypeScript)

#### Upload Integrity Tests (`frontend/__tests__/upload/AudioUploadIntegrity.test.ts`)
- **Client-side file integrity validation**
- **Upload process bit-perfect preservation**
- **Progress tracking without data modification**
- **Error handling with original file preservation**
- **WebSocket communication validation**
- **Binary data handling verification**

Key test suites:
```typescript
describe('File Validation - Audio Format Integrity')
describe('Upload Process - Bit-Perfect Preservation')  
describe('Hash Verification and Duplicate Detection')
describe('Error Handling - Data Preservation')
```

#### Audio Validation Tests (`frontend/__tests__/utils/audioValidation.test.ts`)
- **WAV header structure validation**
- **File size and duration accuracy**
- **Bit depth and sample rate verification**
- **Audio content integrity validation**
- **Performance with large files**
- **Binary data immutability**

### 3. End-to-End Tests (Playwright)

#### Browser Integration Tests (`e2e/audioUploadIntegrity.spec.ts`)
- **Real browser file upload workflows**
- **Drag-and-drop functionality**
- **Progress tracking accuracy**
- **Large file handling in browsers**
- **Duplicate detection UI/UX**
- **Error scenario handling**
- **WebSocket real-time updates**

Key test scenarios:
```typescript
test('uploads single WAV file with bit-perfect preservation')
test('uploads multiple WAV files concurrently maintaining individual integrity')
test('handles large WAV file upload with progress tracking')
test('detects and handles duplicate files correctly')
```

### 4. Test Data Generation

#### WAV Generator (`test-utils/wav-generator.go`)
Generates predictable test WAV files for consistent testing:

- **Multiple audio qualities** (16-bit/44kHz to 24-bit/192kHz)
- **Various file sizes** (small to 1GB+)
- **Different patterns** (sine waves, noise, predictable data)
- **Benchmark capabilities** for performance testing
- **Hash verification** for integrity validation

Usage:
```bash
# Generate test suite
go run wav-generator.go -testsuite -output ./test-wavs

# Run benchmarks
go run wav-generator.go -benchmark
```

### 5. Pre-commit Hooks

Automated validation before each commit to prevent compression-introducing code:

#### `hooks/check-no-compression.sh`
- Prevents compression library imports
- Blocks compression middleware
- Validates content-type headers
- Ensures binary upload handling

#### `hooks/check-content-types.sh`
- Validates proper audio content-types
- Prevents text-based content-types for audio
- Checks for compression headers
- Validates MinIO upload configurations

#### `hooks/check-wav-handling.sh`
- Ensures WAV files handled as binary data
- Prevents string operations on binary data
- Validates proper byte array usage
- Checks for text processing on audio files

#### `hooks/run-audio-tests.sh`
- Runs critical audio preservation tests
- Validates test data integrity
- Checks test coverage for audio code
- Verifies benchmark performance

#### `hooks/check-audio-coverage.sh`
- Validates test coverage for audio-related code
- Checks for missing test files
- Verifies critical test scenarios
- Ensures minimum coverage thresholds

#### `hooks/check-quality-settings.sh`
- Prevents hardcoded quality settings
- Validates configurable parameters
- Checks for appropriate file size limits
- Ensures flexibility for different audio formats

## 🔧 Setup Instructions

### 1. Install Dependencies

Backend (Go):
```bash
cd backend
go mod tidy
go install github.com/stretchr/testify
```

Frontend (TypeScript):
```bash
cd frontend
npm install
npm install --save-dev @testing-library/react @testing-library/jest-dom
```

End-to-end (Playwright):
```bash
npm install -g @playwright/test
npx playwright install
```

Pre-commit hooks:
```bash
pip install pre-commit
pre-commit install
```

### 2. Generate Test Data

```bash
cd test-utils
go run wav-generator.go -testsuite -output ../test-data
```

### 3. Run Test Suites

#### Backend Tests
```bash
cd backend
go test -v ./services/
go test -run=".*BitPerfect.*|.*Integrity.*" ./services/
```

#### Frontend Tests
```bash
cd frontend
npm test
npm test -- --coverage
npm test -- __tests__/upload/AudioUploadIntegrity.test.ts
```

#### End-to-End Tests
```bash
npx playwright test e2e/audioUploadIntegrity.spec.ts
```

#### Pre-commit Hook Testing
```bash
# Test all hooks
pre-commit run --all-files

# Test specific hooks
hooks/check-no-compression.sh
hooks/run-audio-tests.sh
```

## 📊 Test Coverage Requirements

### Minimum Coverage Targets
- **Backend audio code**: 90%+ line coverage
- **Frontend upload code**: 85%+ line coverage  
- **Critical functions**: 100% coverage
- **Error paths**: 80%+ coverage

### Critical Functions That Must Be 100% Tested
1. `UploadFile()` - MinIO upload with hash verification
2. `ProcessFiles()` - Multi-file batch processing
3. `CalculateFileHash()` - SHA256 integrity verification
4. `validateFile()` - Frontend file validation
5. `uploadToPresignedURL()` - Binary upload handling

## 🚨 Critical Test Scenarios

### Must Pass Before Deployment
1. **Bit-Perfect Upload**: SHA256 hash identical before/after upload
2. **Large File Handling**: 1GB+ WAV files upload successfully  
3. **Concurrent Uploads**: Multiple files maintain individual integrity
4. **Error Recovery**: Network failures don't corrupt files
5. **Duplicate Detection**: Content-based hash comparison
6. **Binary Handling**: No text conversion anywhere in pipeline
7. **Memory Efficiency**: Large files don't cause memory issues
8. **Progress Tracking**: Real-time updates without affecting data

### Performance Benchmarks
- **Small files** (< 10MB): < 2 seconds upload + verification
- **Medium files** (100MB): < 30 seconds upload + verification  
- **Large files** (1GB): < 5 minutes upload + verification
- **Hash calculation**: < 1 second per 100MB
- **Concurrent uploads**: 5 files simultaneously without degradation

## 🔍 Debugging Failed Tests

### Common Failure Patterns

#### Hash Mismatch (Data Corruption)
```bash
# Check for text processing on binary data
grep -r "toString.*wav\|string.*wav" backend/ frontend/

# Verify content-type headers
grep -r "Content-Type.*text\|application/json.*audio" .
```

#### Upload Failures
```bash
# Check MinIO configuration
go test -v -run="TestMinIOService_TestConnection" ./backend/services/

# Verify network settings  
docker logs <minio-container>
```

#### Large File Issues
```bash
# Test memory usage
go test -v -run="TestLarge" -memprofile=mem.prof ./backend/services/
go tool pprof mem.prof
```

#### Performance Degradation
```bash
# Run benchmarks
cd test-utils
go run wav-generator.go -benchmark

# Profile critical paths
go test -bench=. -cpuprofile=cpu.prof ./backend/services/
```

## 📈 Continuous Integration

### GitHub Actions Integration
```yaml
name: Audio Integrity Tests
on: [push, pull_request]
jobs:
  audio-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.21
      - name: Setup Node.js  
        uses: actions/setup-node@v3
        with:
          node-version: 18
      - name: Run Backend Tests
        run: cd backend && go test -v ./services/
      - name: Run Frontend Tests
        run: cd frontend && npm test -- --coverage
      - name: Run E2E Tests
        run: npx playwright test
```

### Pre-deployment Checklist
- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] E2E tests pass
- [ ] Pre-commit hooks pass
- [ ] Performance benchmarks meet targets
- [ ] Test coverage meets minimums
- [ ] Large file tests completed successfully
- [ ] Hash verification working across all paths

## 🛡️ Security Considerations

### Test Data Security
- Use predictable, non-sensitive audio patterns
- Generate test files programmatically
- Never commit large test files to repository
- Clean up temporary test data

### Hash Verification Security
- Use SHA256 for cryptographic strength
- Calculate hashes on raw binary data only
- Verify hashes at every upload stage
- Store hashes securely in metadata

## 📚 Additional Resources

- [Go Testing Best Practices](https://golang.org/doc/tutorial/add-a-test)
- [React Testing Library Docs](https://testing-library.com/docs/react-testing-library/intro/)
- [Playwright Documentation](https://playwright.dev/)
- [Pre-commit Hook Documentation](https://pre-commit.com/)
- [WAV File Format Specification](http://soundfile.sapp.org/doc/WaveFormat/)

---

**Remember**: The primary goal is **bit-perfect audio preservation**. Every test should validate that audio files maintain exactly the same binary content from upload to storage. No compression, no transformation, no quality loss - ever.