# 🎵 Audio Quality Preservation Guide

This document outlines the CRITICAL requirements for maintaining audio quality in the White Plains Gospel Chapel sermon uploader system.

## 🚨 Critical Audio Quality Requirements

### 1. Content-Type Enforcement
**REQUIREMENT:** All WAV file uploads MUST use `ContentType: "audio/wav"`

```go
// ✅ CORRECT - Preserves audio quality
minio.PutObjectOptions{
    ContentType: "audio/wav",
    // NO compression options
}

// ❌ INCORRECT - May compromise quality
minio.PutObjectOptions{
    ContentType: "application/octet-stream", // Generic binary
}
```

### 2. Zero-Compression Policy
**REQUIREMENT:** NO compression algorithms anywhere in upload/storage paths

```go
// ✅ CORRECT - No compression
_, err := client.PutObject(ctx, bucket, filename,
    io.NopCloser(bytes.NewReader(data)),
    int64(len(data)),
    minio.PutObjectOptions{
        ContentType: "audio/wav",
        // CRITICAL: No compression options
    })

// ❌ INCORRECT - Introduces compression
_, err := client.PutObject(ctx, bucket, filename,
    gzip.NewReader(bytes.NewReader(data)), // NEVER compress audio
    int64(len(data)),
    minio.PutObjectOptions{
        ContentType: "audio/wav",
        ContentEncoding: "gzip", // NEVER use encoding
    })
```

### 3. File Integrity Verification
**REQUIREMENT:** Every upload MUST verify file integrity using SHA256

```go
// ✅ CORRECT - Integrity verification
func verifyFileIntegrity(original, downloaded []byte) error {
    originalHash := sha256.Sum256(original)
    downloadedHash := sha256.Sum256(downloaded)
    
    if originalHash != downloadedHash {
        return fmt.Errorf("CRITICAL: File integrity compromised")
    }
    return nil
}
```

### 4. Presigned URL Safety
**REQUIREMENT:** Presigned URLs MUST NOT include transformation parameters

```go
// ✅ CORRECT - Safe presigned URL
func (m *MinIOService) GeneratePresignedUploadURL(filename string, expiry time.Duration) (string, error) {
    return m.client.PresignedPutObject(
        context.Background(),
        m.config.MinioBucket,
        filename,
        expiry,
    )
}

// ❌ INCORRECT - May alter audio data
func (m *MinIOService) GeneratePresignedUploadURL(filename string, expiry time.Duration) (string, error) {
    // NEVER add query parameters that could transform audio
    url, err := m.client.PresignedPutObject(context.Background(), m.config.MinioBucket, filename, expiry)
    url += "?transform=compress" // NEVER DO THIS
    return url, err
}
```

## 🛡️ Protected Code Paths

### Backend Services (CRITICAL)
These files are CRITICAL for audio quality preservation:

- `backend/services/minio.go` - Core MinIO operations
- `backend/services/minio_duplicates.go` - Duplicate detection with quality preservation
- `backend/services/minio_presigned.go` - Presigned URL generation
- `backend/handlers/presigned.go` - Upload endpoint handlers
- `backend/services/file_service.go` - File processing logic

### Frontend Components (IMPORTANT)
These components must NOT modify file data:

- `frontend/src/components/upload/` - Upload UI components
- `frontend/src/hooks/useUpload*` - Upload logic hooks
- `frontend/src/lib/upload*` - Upload utility functions

## 🧪 Testing Requirements

### 1. Audio Quality Tests
**REQUIRED:** Every PR affecting audio paths must include:

```go
func TestAudioQualityPreservation(t *testing.T) {
    // Test file integrity through upload/download cycle
    originalData := loadTestWAVFile()
    uploadedData := uploadAndDownloadFile(originalData)
    
    // CRITICAL: Verify exact byte-for-byte match
    assert.Equal(t, originalData, uploadedData)
}
```

### 2. Content-Type Verification
```go
func TestContentTypePreservation(t *testing.T) {
    // Upload WAV file
    filename := "test.wav"
    uploadWAVFile(filename)
    
    // Verify content type is preserved
    objInfo := getObjectInfo(filename)
    assert.Equal(t, "audio/wav", objInfo.ContentType)
}
```

### 3. Performance with Quality
```go
func BenchmarkLargeFileUpload(b *testing.B) {
    largeWAVData := generateLargeWAVFile(100 * 1024 * 1024) // 100MB
    
    for i := 0; i < b.N; i++ {
        // Upload must be fast AND preserve quality
        uploadedData := uploadAndDownloadFile(largeWAVData)
        
        // CRITICAL: No quality compromise for performance
        if !bytes.Equal(largeWAVData, uploadedData) {
            b.Fatal("Quality compromised for performance")
        }
    }
}
```

## 🚨 Prohibited Patterns

### ❌ NEVER Use These Patterns:

1. **Content Encoding**
```go
// NEVER use content encoding for audio
minio.PutObjectOptions{
    ContentType: "audio/wav",
    ContentEncoding: "gzip", // ❌ FORBIDDEN
}
```

2. **File Transformation**
```go
// NEVER transform audio files
func processAudioFile(data []byte) []byte {
    // ❌ FORBIDDEN - Any transformation
    return compress(data)
    return convert(data, "mp3")
    return optimize(data)
}
```

3. **Generic Binary Content-Type**
```go
// NEVER use generic content types for audio
minio.PutObjectOptions{
    ContentType: "application/octet-stream", // ❌ Use "audio/wav"
}
```

4. **Client-Side File Modification**
```typescript
// NEVER modify files in frontend
const processFile = (file: File) => {
    // ❌ FORBIDDEN - Any file modification
    return compressFile(file);
    return convertFile(file);
    return optimizeFile(file);
}
```

## 🔍 Code Review Checklist

When reviewing PRs, verify:

### ✅ Audio Quality Checklist
- [ ] All WAV uploads use `ContentType: "audio/wav"`
- [ ] No compression algorithms introduced
- [ ] File integrity verification present
- [ ] Presigned URLs don't alter data
- [ ] No client-side file modification
- [ ] Tests verify quality preservation
- [ ] Performance doesn't compromise quality

### ✅ Critical Code Paths
- [ ] `minio.PutObjectOptions` preserves quality
- [ ] Upload handlers maintain integrity
- [ ] Duplicate detection preserves original
- [ ] Metadata extraction doesn't alter files
- [ ] Frontend only uploads, never transforms

## 🚨 Emergency Procedures

### If Audio Quality is Compromised:

1. **IMMEDIATE ACTIONS:**
   - Halt all deployments
   - Revert problematic changes
   - Verify backup audio files
   - Test upload/download integrity

2. **INVESTIGATION:**
   - Check recent commits for compression code
   - Verify content-type headers
   - Test with known-good audio files
   - Validate MinIO operations

3. **RECOVERY:**
   - Restore quality-preserving code
   - Re-run all audio quality tests
   - Verify production integrity
   - Update monitoring alerts

## 📊 Monitoring and Alerts

### Automated Monitoring
- **Every 6 hours:** Audio quality health check
- **On every PR:** Quality preservation verification  
- **On deployment:** File integrity validation
- **Weekly:** Comprehensive audio quality audit

### Alert Triggers
- Content-type changes from `audio/wav`
- Compression algorithms introduced
- File integrity verification failures
- Upload/download size mismatches
- Performance degradation in audio paths

## 🔒 Enforcement

### Branch Protection
- Audio Quality Preservation Check (REQUIRED)
- Backend Tests with Audio Validation (REQUIRED)
- Integration Tests with MinIO (REQUIRED)
- Code owner review for audio-critical changes

### Automated Prevention
- PR blocks for dangerous patterns
- Content-type enforcement
- Compression detection
- File integrity verification
- Performance regression detection

---

**⚠️ REMEMBER: Audio quality is NON-NEGOTIABLE for sermon preservation.**
**🎵 Every sermon must be preserved with perfect fidelity for future generations.**