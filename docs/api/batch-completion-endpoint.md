# Batch Completion API Endpoint

## `/api/upload/complete-batch`

**Method**: `POST`  
**Purpose**: Complete batch upload processing and trigger Discord batch notifications  
**Added**: September 5, 2025

### Request Format

```json
{
    "filenames": [
        "sermon_20250905_1.wav",
        "sermon_20250905_2.wav", 
        "sermon_20250905_3.wav"
    ]
}
```

### Response Format

```json
{
    "success": true,
    "total_files": 3,
    "successful": 3,
    "duplicates": 0,
    "failed": 0,
    "is_batch": true,
    "batch_threshold": 2,
    "results": {
        "sermon_20250905_1.wav": {
            "error": false,
            "message": "File processed successfully",
            "size": 1073741824
        },
        "sermon_20250905_2.wav": {
            "error": false,
            "message": "File processed successfully", 
            "size": 1073741824
        },
        "sermon_20250905_3.wav": {
            "error": false,
            "message": "File processed successfully",
            "size": 1073741824
        }
    },
    "message": "Processed 3 files: 3 successful, 0 failed"
}
```

### Error Responses

#### 400 - Bad Request
```json
{
    "error": true,
    "message": "Invalid request format"
}
```

#### 400 - No Files
```json
{
    "error": true,
    "message": "No filenames provided"
}
```

### Batch Threshold Logic

- **Individual**: `fileCount < BATCH_THRESHOLD` → `is_batch: false`
- **Batch**: `fileCount >= BATCH_THRESHOLD` → `is_batch: true`
- **Default Threshold**: 2 files (configurable via `BATCH_THRESHOLD` env var)

### Discord Notifications

#### Batch Start Notification
```
📤 Batch Upload Started
Processing 3 file(s)
```

#### Batch Completion Notification
```
✅ Success - Batch Upload Complete
✅ 3 uploaded

Successful: 3
Duplicates: 0 
Failed: 0
```

### Usage in Frontend

```typescript
import { uploadService } from '@/services/uploadService'

// After all files in batch are uploaded successfully
const successfulFilenames = ['file1.wav', 'file2.wav', 'file3.wav']
const result = await uploadService.completeUploadBatch(successfulFilenames)

console.log(`Batch processed: ${result.successful} successful, ${result.failed} failed`)
```

### Comparison with Individual Endpoint

| Feature | `/api/upload/complete` | `/api/upload/complete-batch` |
|---------|------------------------|------------------------------|
| Input | Single filename | Array of filenames |
| Discord | Individual notification | Batch notification (if >= threshold) |
| Metadata | Immediate + background | Background only |
| Use Case | Single file uploads | Batch uploads via presigned URLs |

### Error Handling

The endpoint gracefully handles:
- **Missing files**: Counts as failed, continues processing others
- **MinIO errors**: Logged and counted as failed
- **Discord failures**: Logged but doesn't block file processing
- **Empty requests**: Returns 400 error

### Performance Notes

- **Efficient**: Single API call vs N individual calls
- **Async metadata**: Background processing doesn't block response
- **Batch optimized**: Better resource utilization for multiple files
- **Fault tolerant**: Partial failures don't break entire batch

### Testing

```bash
# Test with curl
curl -X POST "http://localhost:8000/api/upload/complete-batch" \
  -H "Content-Type: application/json" \
  -d '{
    "filenames": [
      "test_batch_1.wav",
      "test_batch_2.wav", 
      "test_batch_3.wav"
    ]
  }'
```

### Related Endpoints

- [`/api/upload/complete`](./individual-completion.md) - Individual file completion
- [`/api/upload/presigned-batch`](./presigned-urls.md) - Batch presigned URL generation
- [`/api/test/discord`](./test-endpoints.md) - Discord webhook testing