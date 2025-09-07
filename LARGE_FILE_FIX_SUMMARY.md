# Large File Upload Fix - Implementation Summary

## Problem
CloudFlare free tier has a 100MB upload limit, causing files larger than 100MB to fail during upload. The application was always routing uploads through CloudFlare proxy (`sermons.wpgc.church`), even for large files.

## Solution
Implemented intelligent URL routing based on file size:
- **Files ≤100MB**: Use CloudFlare URLs (`sermons.wpgc.church`) for CDN benefits
- **Files >100MB**: Use direct MinIO URLs (`192.168.1.127:9000`) to bypass CloudFlare limits

## Implementation Details

### Backend Changes

#### 1. Configuration (`backend/config/config.go`)
```go
// Large File Upload Configuration
LargeFileThresholdMB int64 // Files larger than this (MB) use direct MinIO URLs
```
- Default: 100MB (configurable via `LARGE_FILE_THRESHOLD_MB` environment variable)

#### 2. MinIO Service (`backend/services/minio_presigned.go`)
**New Methods:**
- `GeneratePresignedUploadURLDirect()`: Always uses direct MinIO endpoint
- `GeneratePresignedUploadURLSmart()`: Intelligently chooses URL type based on file size
- `GetLargeFileThreshold()`: Returns configured threshold in bytes

#### 3. Handlers (`backend/handlers/presigned.go`)
**Updated Methods:**
- `GetPresignedURL()`: Now uses smart URL generation
- `GetPresignedURLsBatch()`: Handles mixed file sizes in batch uploads

**Response Format:**
```json
{
  "success": true,
  "uploadUrl": "http://192.168.1.127:9000/sermons/large_file.wav?signature=...",
  "isLargeFile": true,
  "uploadMethod": "direct_minio",
  "largeFileThreshold": 104857600,
  "message": "Large file (150.0 MB) will use direct MinIO upload to bypass CloudFlare 100MB limit"
}
```

### Frontend Changes

#### 1. API Client (`frontend/lib/api.ts`)
**Enhanced `uploadToMinIO()` method:**
- Detects direct MinIO vs CloudFlare uploads
- Different timeout values (30min for direct, 10min for CloudFlare)
- Better error messages and logging
- CORS guidance for direct MinIO uploads

#### 2. Upload Hooks (`frontend/hooks/useUploadQueueOptimized.ts`)
- Logs large file detection
- Provides user feedback for large file uploads

## Test Results

✅ **Integration Test Results:**
```
🔬 Testing: Large file (150MB)
   ✅ isLargeFile: true (correct)
   ✅ uploadMethod: direct_minio (correct) 
   ✅ URL contains '192.168.1.127' (correct)
   ✅ URL doesn't contain 'sermons.wpgc.church' (correct)
   📋 URL: http://192.168.1.127:9000/sermons/large_sermon.wav?X-Amz-Algorithm=AWS4-HMAC-SHA256...
   ℹ️  Message: Large file (150.0 MB) will use direct MinIO upload to bypass CloudFlare 100MB limit
   📏 Threshold: 104857600 bytes

🔬 Testing: Very large file (500MB)  
   ✅ isLargeFile: true (correct)
   ✅ uploadMethod: direct_minio (correct)
   ✅ URL contains '192.168.1.127' (correct) 
   ✅ URL doesn't contain 'sermons.wpgc.church' (correct)
```

## Configuration Requirements

### MinIO Server CORS Configuration
For direct MinIO uploads to work from the frontend, ensure MinIO has proper CORS configuration:

```bash
mc cors set-json /path/to/cors-config.json myminio/sermons
```

**cors-config.json:**
```json
{
  "CORSRules": [
    {
      "AllowedOrigins": ["http://localhost:3000", "https://sermons.wpgc.church"],
      "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
      "AllowedHeaders": ["*"],
      "MaxAgeSeconds": 3000
    }
  ]
}
```

### Environment Variables
```bash
LARGE_FILE_THRESHOLD_MB=100           # Default threshold in MB
MINIO_PUBLIC_ENDPOINT=sermons.wpgc.church  # CloudFlare proxy endpoint
MINIO_ENDPOINT=192.168.1.127:9000     # Direct MinIO endpoint
```

## Benefits

1. **No More 100MB Limit**: Large files now bypass CloudFlare restrictions
2. **Optimal Performance**: Small files still benefit from CloudFlare CDN
3. **Configurable**: Threshold can be adjusted via environment variables
4. **Backward Compatible**: Existing uploads continue to work
5. **Better UX**: Clear feedback for large file uploads
6. **Robust Error Handling**: Different timeouts and error messages

## Files Modified

### Backend
- `backend/config/config.go` - Added large file threshold configuration
- `backend/services/minio_presigned.go` - New smart URL generation methods
- `backend/handlers/presigned.go` - Updated single and batch upload handlers

### Frontend  
- `frontend/lib/api.ts` - Enhanced upload method with better error handling
- `frontend/hooks/useUploadQueueOptimized.ts` - Added large file detection logging

### Tests
- `backend/handlers/presigned_large_file_test.go` - TDD Red phase tests (failing behavior)
- `backend/handlers/presigned_large_file_fixed_test.go` - TDD Green phase tests (fixed behavior)
- `backend/services/minio_presigned_large_file_test.go` - Service-level tests

## Usage Examples

### Single File Upload
```javascript
// Large file will automatically use direct MinIO
const response = await api.getPresignedURL("large_sermon.wav", 150 * 1024 * 1024);
// response.uploadMethod === "direct_minio" 
// response.isLargeFile === true
```

### Batch Upload
```javascript  
// Mixed file sizes handled automatically
const files = [
  { filename: "small.wav", fileSize: 50 * 1024 * 1024 },    // Uses CloudFlare
  { filename: "large.wav", fileSize: 200 * 1024 * 1024 }    // Uses direct MinIO
];
const response = await api.getPresignedURLsBatch(files);
```

## Monitoring

The fix includes extensive logging:
- Backend: Smart URL generation decisions
- Frontend: Upload method detection and progress
- Console: Large file detection and routing decisions

## Success Criteria ✅

- [x] Files >100MB use direct MinIO URLs
- [x] Files ≤100MB continue using CloudFlare URLs  
- [x] Configurable threshold
- [x] Backward compatibility maintained
- [x] Enhanced error handling and user feedback
- [x] Comprehensive test coverage
- [x] Both single and batch uploads supported