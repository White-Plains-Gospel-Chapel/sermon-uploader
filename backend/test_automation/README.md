# API Upload Testing Suite

Comprehensive API upload testing using real 40GB WAV files from the ridgepoint Pi to validate the sermon uploader API endpoints with production-scale data.

## Overview

This testing suite validates the API upload functionality by:

- **Connecting to ridgepoint Pi** and accessing real church WAV files (40GB collection)
- **Testing API endpoints** (`/api/upload`, `/api/upload/presigned`, `/api/upload/presigned-batch`)
- **Measuring performance** and success rates under real load conditions
- **Simulating Sunday morning scenarios** with 20-30 concurrent uploads
- **Validating file integrity** and bit-perfect audio preservation

## Test Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   ridgepoint    │    │  Test Runner    │    │ sermon-uploader │
│    Pi (WAVs)    │────│   (Python)      │────│   API Server    │
│   40GB files    │    │                 │    │  (Go/Fiber)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                       ┌─────────────────┐
                       │  Test Reports   │
                       │ JSON + Analysis │
                       └─────────────────┘
```

## File Structure

```
backend/test_automation/
├── api_upload_tests.py           # Main API testing framework
├── sunday_morning_stress_test.py # Sunday morning scenario simulator
├── run_api_tests.py             # Test suite orchestrator
├── api_test_config.json         # Configuration file
├── requirements.txt             # Python dependencies
├── README.md                    # This file
└── reports/                     # Generated test reports
    ├── master_api_test_report.json
    ├── stress_test_results.json
    └── api_upload_test_report.json
```

## Quick Start

### 1. Setup Environment

```bash
# Install Python dependencies
pip install -r requirements.txt

# Ensure SSH access to ridgepoint Pi
ssh gaius@ridgepoint.local "echo 'Connection test successful'"

# Start the sermon-uploader API server
cd ../
go run main.go
```

### 2. Configure Tests

Update `api_test_config.json`:

```json
{
  "api": {
    "base_url": "http://localhost:8000"
  },
  "ridgepoint": {
    "hostname": "ridgepoint.local",
    "username": "gaius"
  },
  "testing": {
    "max_concurrent_uploads": 5
  }
}
```

### 3. Run Tests

```bash
# Quick validation (5-10 minutes)
python3 run_api_tests.py --quick-validation

# Full test suite (30-60 minutes)
python3 run_api_tests.py --full-suite

# Stress tests only (20-40 minutes)
python3 run_api_tests.py --stress-only
```

## Test Categories

### Single File Upload Tests

Tests individual file uploads using different methods and file sizes:

- **Direct uploads** via `/api/upload`
- **Presigned URL uploads** via `/api/upload/presigned`
- **File size categories**: small (<100MB), medium (100-500MB), large (500MB-1GB), xlarge (>1GB)

```bash
python3 api_upload_tests.py --test single --method presigned --size large --files 3
```

### Batch Upload Tests

Tests concurrent batch uploads using presigned URLs:

- **Batch sizes**: 3, 5, 8 files per batch
- **Concurrent processing** within batches
- **API response time** for batch presigned URL generation

```bash
python3 api_upload_tests.py --test batch --files 10 --batch-size 5
```

### Sunday Morning Stress Tests

Simulates realistic Sunday morning scenarios:

- **Immediate Rush**: 25 files uploaded within 5 minutes (worst case)
- **Staggered Upload**: 20 files over 15 minutes (typical case)
- **Network Issues**: Simulated connection drops and delays
- **Peak Load**: 30 files with 20 concurrent uploads

```bash
python3 sunday_morning_stress_test.py --scenario Sunday_Immediate_Rush
```

## Available WAV Files

The ridgepoint Pi contains approximately **101 WAV files totaling 90.54 GB**:

- **Small files** (<100MB): 9 files
- **Medium files** (100-500MB): 6 files  
- **Large files** (500MB-1GB): 60 files
- **Extra large files** (>1GB): 26 files

## Test Execution Modes

### Full Suite (`--full-suite`)

Complete validation with all test types:

1. **Environment Validation** - Connectivity and file access
2. **API Health Checks** - Endpoint availability
3. **Single File Tests** - All upload methods and sizes
4. **Batch Upload Tests** - Concurrent batch processing
5. **Stress Tests** - Sunday morning scenarios
6. **Performance Validation** - Against defined targets
7. **Cleanup Verification** - Resource cleanup

**Duration**: 30-60 minutes  
**Files tested**: 50-100 files  
**Data processed**: 10-30 GB

### Quick Validation (`--quick-validation`)

Fast validation for CI/CD:

1. **Basic Connectivity** - API and ridgepoint Pi
2. **Single Small File** - One presigned upload
3. **Small Batch Test** - 3 files batch upload

**Duration**: 5-10 minutes  
**Files tested**: 4-5 files  
**Data processed**: <1 GB

### Stress Only (`--stress-only`)

Focus on Sunday morning scenarios:

1. **All Stress Scenarios** - Rush, staggered, network issues, peak load
2. **Performance Analysis** - Throughput and stability under load
3. **Resource Monitoring** - Memory, CPU, connections

**Duration**: 20-40 minutes  
**Files tested**: 80-120 files  
**Data processed**: 20-40 GB

## Configuration Options

### API Configuration

```json
{
  "api": {
    "base_url": "http://localhost:8000",
    "timeout": 300,
    "max_retries": 3,
    "retry_backoff": 1.0
  }
}
```

### Performance Targets

```json
{
  "performance_targets": {
    "min_throughput_mbps": 5.0,
    "max_api_response_time": 2.0,
    "target_success_rate": 95.0
  }
}
```

### File Selection Limits

```json
{
  "file_selection": {
    "small_files_limit": 3,
    "medium_files_limit": 3,
    "large_files_limit": 2,
    "xlarge_files_limit": 2
  }
}
```

## Test Reports

### Master Report (`master_api_test_report.json`)

Comprehensive test execution summary:

```json
{
  "test_execution": {
    "suite_name": "Complete API Upload Test Suite",
    "duration_minutes": 45.2,
    "environment": {...}
  },
  "test_summary": {
    "total_tests": 7,
    "passed_tests": 6,
    "failed_tests": 1,
    "success_rate": 85.7
  },
  "data_validation": {
    "total_files_tested": 87,
    "total_data_processed_gb": 23.4,
    "overall_upload_success_rate": 94.3
  },
  "performance_analysis": {
    "avg_throughput_mbps": 12.8,
    "avg_api_response_time": 0.45,
    "peak_throughput": 28.3
  },
  "api_validation_checklist": {
    "presigned_url_generation": true,
    "batch_presigned_urls": true,
    "upload_completion_handling": true,
    "duplicate_detection": true,
    "error_handling": true,
    "performance_targets_met": true
  }
}
```

### Stress Test Report (`stress_test_results.json`)

Sunday morning scenario analysis:

```json
{
  "test_type": "Sunday Morning Stress Test",
  "scenario_results": [
    {
      "scenario_name": "Sunday_Immediate_Rush",
      "duration_minutes": 8.3,
      "files_processed": 25,
      "success_rate": 96.0,
      "data_transferred_gb": 18.7,
      "avg_throughput_mbps": 15.2,
      "stress_analysis": {
        "system_stability": "stable",
        "bottlenecks_detected": ["High API response times"]
      }
    }
  ]
}
```

## Performance Validation

### Success Criteria

- **Upload Success Rate**: ≥95%
- **Throughput**: ≥5 MB/s average
- **API Response Time**: ≤2.0 seconds
- **System Stability**: No crashes during stress tests
- **File Integrity**: 100% bit-perfect preservation

### Performance Targets

| Test Type | Min Throughput | Max Response Time | Min Success Rate |
|-----------|----------------|-------------------|------------------|
| Single File | 5 MB/s | 2.0s | 95% |
| Batch Upload | 8 MB/s | 3.0s | 90% |
| Stress Test | 3 MB/s | 5.0s | 85% |

## Troubleshooting

### Common Issues

1. **ridgepoint Pi Connection Failed**
   ```bash
   # Test SSH connectivity
   ssh -o ConnectTimeout=10 gaius@ridgepoint.local "echo test"
   
   # Check SSH keys if needed
   ssh-add ~/.ssh/id_rsa
   ```

2. **API Health Check Failed**
   ```bash
   # Check if server is running
   curl http://localhost:8000/api/health
   
   # Start server if needed
   cd .. && go run main.go
   ```

3. **No WAV Files Found**
   ```bash
   # Check ridgepoint Pi file structure
   ssh gaius@ridgepoint.local "find /home/gaius/data -name '*.wav' | wc -l"
   ```

4. **Upload Timeouts**
   - Increase timeout values in config
   - Reduce concurrent upload count
   - Check network stability

5. **Memory Issues**
   - Reduce `max_concurrent_uploads`
   - Use smaller test file sets
   - Monitor system resources

### Debug Mode

Enable detailed logging:

```bash
# Set debug environment
export PYTHONPATH="."
export LOG_LEVEL="DEBUG"

# Run with verbose output
python3 -v run_api_tests.py --quick-validation 2>&1 | tee debug.log
```

### Performance Monitoring

Monitor system resources during tests:

```bash
# In separate terminal
watch -n 2 'ps aux | grep python; free -h; df -h /tmp'
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: API Upload Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  api-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Setup Python
      uses: actions/setup-python@v4
      with:
        python-version: '3.9'
    
    - name: Install dependencies
      run: |
        cd backend/test_automation
        pip install -r requirements.txt
    
    - name: Start API server
      run: |
        cd backend
        go run main.go &
        sleep 10
    
    - name: Run quick validation
      run: |
        cd backend/test_automation
        python3 run_api_tests.py --quick-validation
    
    - name: Upload test results
      uses: actions/upload-artifact@v3
      with:
        name: api-test-results
        path: backend/test_automation/master_api_test_report.json
```

## Development

### Adding New Tests

1. **Create test method** in `APIUploadTester`
2. **Add to test runner** in `run_api_tests.py`
3. **Update configuration** if needed
4. **Add validation criteria**

### Custom Scenarios

Create custom stress scenarios:

```python
custom_scenario = StressScenario(
    name="Custom_Test",
    description="Custom test scenario",
    file_count=10,
    concurrent_uploads=5,
    duration_minutes=10,
    upload_pattern="staggered",
    file_size_preference="large"
)

# Add to scenarios list
tester.scenarios.append(custom_scenario)
```

## API Validation Checklist

- ✅ **Presigned URL Generation** - `/api/upload/presigned` works correctly
- ✅ **Batch Presigned URLs** - `/api/upload/presigned-batch` handles multiple files
- ✅ **Upload Completion** - File processing after upload completes
- ✅ **Duplicate Detection** - `/api/check-duplicate` prevents re-uploads
- ✅ **Error Handling** - API returns proper error responses
- ✅ **Performance Targets** - Meets throughput and response time goals
- ✅ **File Integrity** - Bit-perfect audio preservation
- ✅ **Concurrent Handling** - System stable under concurrent load

## Production Readiness

System is production-ready when:

- ✅ All test suites pass with >90% success rate
- ✅ Performance targets consistently met
- ✅ Sunday morning stress tests stable
- ✅ Error handling graceful and informative
- ✅ File integrity validation 100% successful
- ✅ System resources within acceptable limits

## Support

For issues or questions:

1. **Check logs**: `api_upload_tests.log`
2. **Review configuration**: `api_test_config.json`
3. **Validate environment**: `python3 run_api_tests.py --quick-validation`
4. **Check API health**: `curl http://localhost:8000/api/health`

---

**Note**: This test suite uses real production data (40GB WAV files). Ensure adequate network bandwidth and storage space when running full test suites.