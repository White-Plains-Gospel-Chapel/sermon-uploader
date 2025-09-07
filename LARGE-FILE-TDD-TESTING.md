# Large File TDD Testing Suite (500MB+)

## Overview

This comprehensive TDD test suite validates the sermon uploader's ability to handle large files (500MB+) using both standard HTTP uploads and TUS resumable uploads. It's specifically designed for Sunday sermon recordings and batch upload scenarios.

## Prerequisites

### System Requirements
- **Raspberry Pi** at IP `192.168.1.195` (or modify scripts for your environment)
- **Test files** located at: `/home/gaius/data/sermon-test-wavs/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files`
- **Production API** accessible at: `https://sermons.wpgc.church`
- **WAV files** of 500MB or larger for testing

### Required Tools
Install these tools on the Pi:
```bash
sudo apt-get update
sudo apt-get install curl bc jq hexdump
```

### File Structure
```
sermon-uploader/
├── test-large-files-tdd.sh           # Standard HTTP upload tests
├── test-tus-resumable-tdd.sh         # TUS resumable upload tests
├── run-all-large-file-tests.sh       # Master test runner
└── LARGE-FILE-TDD-TESTING.md         # This documentation
```

## Test Suites

### 1. Standard HTTP Upload Tests (`test-large-files-tdd.sh`)

Tests the traditional HTTP multipart upload method:

- **Test 1: API Health Check** - Verifies system readiness for large uploads
- **Test 2: Single Large File Upload** - Uploads one 500MB+ file via `/api/upload`
- **Test 3: Batch Upload** - Uploads 3-5 files (500MB+ each) simultaneously
- **Test 4: Timeout Handling** - Tests resilience with connection timeouts
- **Test 5: Progress Tracking** - Monitors upload progress and speed
- **Test 6: Error Recovery** - Tests retry mechanisms and resilience

### 2. TUS Resumable Upload Tests (`test-tus-resumable-tdd.sh`)

Tests the TUS protocol for resumable uploads (best for large files):

- **Test 1: TUS Configuration Discovery** - Discovers server TUS capabilities
- **Test 2: Create Upload Session** - Creates TUS upload session for large file
- **Test 3: Chunked Upload** - Uploads file in configurable chunks (default 10MB)
- **Test 4: Resume Capability** - Tests resuming interrupted uploads
- **Test 5: Performance Analysis** - Tests different chunk sizes for optimization
- **Test 6: Error Handling** - Tests TUS-specific error scenarios

### 3. Master Test Runner (`run-all-large-file-tests.sh`)

Orchestrates both test suites and provides comprehensive analysis:

- Runs both standard and TUS test suites
- Compares performance between upload methods
- Generates unified reports and recommendations
- Determines production readiness for Sunday uploads

## Usage

### Quick Start
Run the comprehensive test suite:
```bash
cd /path/to/sermon-uploader
./run-all-large-file-tests.sh
```

### Individual Test Suites
Run only standard HTTP tests:
```bash
./test-large-files-tdd.sh
```

Run only TUS resumable tests:
```bash
./test-tus-resumable-tdd.sh
```

### View Help
```bash
./run-all-large-file-tests.sh --help
```

## Expected Results vs Actual Results

### Expected Results for Production-Ready System

#### Standard HTTP Upload Tests
1. **API Health Check**: PASS - Health endpoint returns 200 OK
2. **Single Large File Upload**: PASS - 500MB+ file uploads successfully
3. **Batch Upload**: PASS - 3-5 files upload without memory issues
4. **Timeout Handling**: PASS - Graceful handling of connection issues
5. **Progress Tracking**: PASS - Upload progress reported accurately
6. **Error Recovery**: PASS - Retry mechanisms work correctly

#### TUS Resumable Upload Tests  
1. **TUS Configuration**: PASS - TUS headers present and valid
2. **Create Upload Session**: PASS - Upload URLs generated successfully
3. **Chunked Upload**: PASS - Files upload in chunks with progress
4. **Resume Capability**: PASS - Uploads can resume from interruption
5. **Performance Analysis**: PASS - Optimal chunk sizes identified
6. **Error Handling**: PASS - Proper error codes returned

### Performance Metrics Expected

For a properly configured system on typical Pi hardware with good network:

- **Upload Speed**: 5-15 MB/s (depending on network and Pi model)
- **Memory Usage**: Stable during large uploads (chunked processing)
- **Success Rate**: 95%+ for uploads under optimal conditions
- **Timeout Resistance**: Handles 10+ minute uploads without failure
- **Resume Success**: 100% success rate for resume operations

## Performance Metrics Captured

The test suite captures detailed metrics:

### Upload Performance
- **Transfer Speed**: MB/s for each file upload
- **Duration**: Total time per file and per batch
- **Throughput**: Overall system throughput during batch uploads
- **Memory Efficiency**: Stable memory usage during large transfers

### Reliability Metrics
- **Success Rate**: Percentage of successful uploads
- **Retry Success**: Effectiveness of retry mechanisms
- **Resume Success**: Success rate of resumable uploads
- **Error Recovery**: Time to recover from failures

### Optimization Data
- **Optimal Chunk Size**: Best chunk size for TUS uploads
- **Batch Size Limits**: Maximum effective batch size
- **Timeout Thresholds**: Optimal timeout settings
- **Connection Stability**: Network reliability assessment

## Output Files

Each test run generates several files in `/tmp/`:

### Standard HTTP Tests
- `sermon_upload_test_YYYYMMDD_HHMMSS.log` - Detailed test execution log
- `sermon_upload_results_YYYYMMDD_HHMMSS.json` - Structured results data

### TUS Resumable Tests
- `sermon_tus_test_YYYYMMDD_HHMMSS.log` - TUS test execution log
- `sermon_tus_results_YYYYMMDD_HHMMSS.json` - TUS results data

### Master Test Suite
- `sermon_master_test_YYYYMMDD_HHMMSS.log` - Combined execution log
- `sermon_master_results_YYYYMMDD_HHMMSS.json` - Unified results and recommendations

## Troubleshooting

### Common Issues

#### No Test Files Found
```bash
# Error: No WAV files >= 500MB found
# Solution: Create or download large test files
dd if=/dev/urandom of=/home/gaius/data/sermon-test-wavs/test_500MB.wav bs=1M count=500
```

#### API Unreachable
```bash
# Error: Production API health check failed
# Solution: Check network connectivity
curl -I https://sermons.wpgc.church/api/health
```

#### Permission Denied
```bash
# Error: Permission denied executing scripts
# Solution: Make scripts executable
chmod +x *.sh
```

#### Missing Dependencies
```bash
# Error: bc: command not found
# Solution: Install required tools
sudo apt-get install curl bc jq hexdump
```

### Performance Issues

If uploads are consistently slow (< 1 MB/s):
1. Check Pi network configuration
2. Verify production server is not under load
3. Test with smaller files first
4. Monitor Pi CPU/memory during uploads

If uploads frequently fail:
1. Increase timeout values in scripts
2. Check for network stability issues
3. Verify server-side configuration
4. Test TUS resumable uploads as alternative

## Production Recommendations

Based on test results, the system will provide specific recommendations:

### Both Methods Working
- Use TUS for files > 1GB or unreliable connections
- Use standard HTTP for smaller files or stable connections  
- Implement automatic fallback (TUS first, then HTTP)

### Standard HTTP Only
- Use standard HTTP uploads for production
- Implement retry logic for reliability
- Monitor upload progress and timeout settings

### TUS Resumable Only
- Use TUS resumable uploads for production
- Leverage resumable capability for reliability
- TUS is inherently better for large files

### Neither Method Working
- Check server configuration and connectivity
- Verify MinIO backend accessibility
- Review server logs for detailed errors
- Consider network infrastructure issues

## Integration with Sunday Workflow

### Pre-Service Testing
Run a quick health check before Sunday services:
```bash
curl -s https://sermons.wpgc.church/api/health | jq .
```

### Batch Upload Preparation
For Sunday sermon batch uploads:
1. Verify all sermon files are 500MB+ and WAV format
2. Use TUS resumable uploads for reliability
3. Upload files in batches of 3-5 to avoid overwhelming the system
4. Monitor progress through Discord notifications

### Post-Upload Validation
After Sunday uploads:
1. Check Discord for completion notifications
2. Verify files appear in MinIO bucket
3. Confirm AAC conversion completes successfully
4. Monitor for any processing errors

## Maintenance

### Weekly Testing
Run abbreviated tests weekly:
```bash
# Quick health and single file test
./test-large-files-tdd.sh | grep -E "(PASS|FAIL|MB/s)"
```

### Monthly Full Testing
Run complete test suite monthly:
```bash
./run-all-large-file-tests.sh > monthly_test_$(date +%Y%m%d).log 2>&1
```

### Performance Monitoring
Track performance trends:
```bash
# Extract speeds from recent logs
grep "MB/s" /tmp/sermon_*test*.log | sort -k2 -nr
```

This comprehensive TDD test suite ensures the sermon uploader can reliably handle the large file uploads typical of Sunday sermon recordings, providing confidence in production deployment and ongoing system reliability.