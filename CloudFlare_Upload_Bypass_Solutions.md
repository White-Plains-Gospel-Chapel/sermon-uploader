# CloudFlare Upload Bypass Solutions - Complete Guide

## Overview
This document outlines comprehensive solutions to bypass CloudFlare's 100MB upload limit for web applications, specifically for the Sermon Uploader project.

## The Problem
CloudFlare's free tier imposes a 100MB upload limit that affects:
- Direct uploads through CloudFlare proxy
- Workers (100MB request limit)
- Any proxied traffic through CloudFlare's edge servers

## Solution Architecture Options

### 1. 🏆 **Dual-Domain Architecture (Recommended)**

**Concept**: Keep the web app on CloudFlare, expose MinIO on a separate subdomain without proxy.

```
┌─────────────────────────────────────────────────────────────┐
│ Internet Users                                              │
└──────────┬──────────────────────────────┬───────────────────┘
           │                              │
  ┌────────▼────────┐            ┌────────▼────────┐
  │ CloudFlare      │            │ Direct DNS      │
  │ (Web App)       │            │ (MinIO Only)    │
  │ 🟠 Proxied      │            │ ⚪ DNS Only     │
  └────────┬────────┘            └────────┬────────┘
           │                              │
    ┌──────▼──────┐                ┌──────▼──────┐
    │ Pi Backend  │                │ Pi MinIO    │
    │ Port 8000   │                │ Port 9000   │
    └─────────────┘                └─────────────┘
```

**Implementation**:
```dns
# CloudFlare DNS Settings
sermon-uploader.yourdomain.com  A  YOUR_PI_IP  🟠 Proxied
minio.yourdomain.com             A  YOUR_PI_IP  ⚪ DNS Only
```

**Advantages**:
- Web app stays protected by CloudFlare
- Uploads bypass CloudFlare completely
- No file size limits
- Global accessibility

**Disadvantages**:
- Exposes MinIO directly to internet
- Requires router port forwarding
- Two domains to manage

### 2. **CloudFlare Workers with R2 Multipart API**

**Concept**: Use CloudFlare Workers' multipart upload API to chunk files into <100MB pieces.

```typescript
// Worker implementation
export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const { searchParams } = new URL(request.url)
    const action = searchParams.get('action')

    switch (action) {
      case 'create':
        // Initialize multipart upload
        const upload = await env.R2_BUCKET.createMultipartUpload(filename)
        return Response.json({ uploadId: upload.uploadId })
        
      case 'upload-part':
        // Upload individual part (<100MB)
        const partNumber = searchParams.get('partNumber')
        const uploadId = searchParams.get('uploadId')
        
        const part = await env.R2_BUCKET.uploadPart(filename, uploadId, partNumber, request.body)
        return Response.json({ etag: part.etag })
        
      case 'complete':
        // Complete multipart upload
        const parts = await request.json()
        const upload = await env.R2_BUCKET.completeMultipartUpload(filename, uploadId, parts)
        return Response.json({ success: true })
    }
  }
}
```

**Advantages**:
- Stays within CloudFlare ecosystem
- Built-in R2 integration
- Automatic scaling

**Disadvantages**:
- Requires CloudFlare R2 storage
- More complex state management
- Worker execution time limits

### 3. **Chunked Upload Implementation**

**Concept**: Split files client-side into <100MB chunks, upload sequentially.

```typescript
// Frontend chunking implementation
const CHUNK_SIZE = 90 * 1024 * 1024 // 90MB chunks

export async function uploadFileInChunks(file: File, onProgress?: (progress: number) => void) {
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  
  for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
    const start = chunkIndex * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const chunk = file.slice(start, end)
    
    // Upload chunk
    await uploadChunk(chunk, chunkIndex, totalChunks, file.name)
    
    // Report progress
    const progress = ((chunkIndex + 1) / totalChunks) * 100
    onProgress?.(progress)
  }
  
  // Complete upload
  await completeChunkedUpload(file.name, totalChunks)
}
```

**Backend chunk reassembly**:
```go
func (h *Handlers) CompleteChunkedUpload(c *fiber.Ctx) error {
    // Reassemble chunks
    var completeFile bytes.Buffer
    for i := 0; i < totalChunks; i++ {
        chunkKey := fmt.Sprintf("%s_chunk_%d", filename, i)
        chunk := h.chunkStore[chunkKey]
        completeFile.Write(chunk)
        delete(h.chunkStore, chunkKey) // Clean up
    }
    
    // Upload complete file to MinIO
    return h.minioService.PutObject(filename, &completeFile)
}
```

### 4. **Temporary Proxy Disable Method**

**Concept**: Temporarily disable CloudFlare proxy for uploads.

```bash
# Using CloudFlare API
curl -X PATCH "https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records/{record_id}" \
     -H "Authorization: Bearer {api_token}" \
     -H "Content-Type: application/json" \
     --data '{"proxied": false}'

# Upload file

# Re-enable proxy
curl -X PATCH "https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records/{record_id}" \
     -H "Authorization: Bearer {api_token}" \
     -H "Content-Type: application/json" \
     --data '{"proxied": true}'
```

## Implementation for Sermon Uploader

### Current Architecture Decision: **Dual-Domain Approach**

#### Step 1: DNS Configuration
```bash
# In CloudFlare Dashboard:
# 1. sermon-uploader.yourdomain.com → YOUR_PI_IP (🟠 Proxied)
# 2. minio.yourdomain.com → YOUR_PI_IP (⚪ DNS Only)
```

#### Step 2: Router Configuration
```
Port Forwarding Rules:
- 8000 → Pi:8000 (Backend API)
- 9000 → Pi:9000 (MinIO Storage)
```

#### Step 3: MinIO Public Configuration
```bash
# Update MinIO for global access
docker run -d \
  --name minio-standalone \
  --restart unless-stopped \
  -p 9000:9000 \
  -p 9001:9001 \
  -v /home/gaius/minio/data:/data \
  -e MINIO_ROOT_USER=gaius \
  -e MINIO_ROOT_PASSWORD="John 3:16" \
  -e MINIO_DOMAIN=minio.yourdomain.com \
  -e MINIO_SERVER_URL=http://minio.yourdomain.com:9000 \
  minio/minio server /data --console-address ":9001"
```

#### Step 4: CORS Configuration
```bash
# Configure MinIO CORS for global browser access
docker exec -it minio-standalone sh -c '
cat > /tmp/cors.json << EOF
{
  "CORSRules": [{
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag", "x-amz-server-side-encryption", "x-amz-request-id"],
    "MaxAgeSeconds": 3000
  }]
}
EOF

mc alias set local http://localhost:9000 gaius "John 3:16"
mc cors set /tmp/cors.json local/sermons
mc policy set upload local/sermons
'
```

#### Step 5: Backend Updates
```go
// In handlers/direct_upload.go
func (h *Handlers) GetDirectMinIOUploadURL(c *fiber.Ctx) error {
    // Generate presigned URL using public endpoint
    publicEndpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT") // http://minio.yourdomain.com:9000
    presignedURL := generatePresignedURL(publicEndpoint, filename)
    
    return c.JSON(fiber.Map{
        "uploadUrl": presignedURL,
        "uploadMethod": "direct_minio_global",
        "message": "Direct MinIO upload - bypasses CloudFlare globally",
    })
}
```

#### Step 6: Frontend Updates
```typescript
// In lib/direct-upload.ts
const MINIO_ENDPOINT = 'http://minio.yourdomain.com:9000'
const API_ENDPOINT = 'https://sermon-uploader.yourdomain.com'

export async function uploadDirectToMinIO(file: File, presignedURL: string) {
  // Direct browser → MinIO upload (bypasses CloudFlare)
  const xhr = new XMLHttpRequest()
  xhr.open('PUT', presignedURL)
  xhr.setRequestHeader('Content-Type', file.type)
  xhr.send(file)
}
```

## Security Considerations

### For Dual-Domain Approach:
1. **MinIO Exposure**: Direct internet access to MinIO
   - Mitigation: Presigned URLs with short expiration (24 hours)
   - Mitigation: Rate limiting at router level
   - Mitigation: Regular security updates

2. **CORS Configuration**: Wildcard origins (*) allows all domains
   - Mitigation: Restrict to specific origins in production
   - Mitigation: Monitor access logs

3. **Public IP Exposure**: MinIO subdomain reveals Pi's IP
   - Acceptable trade-off for upload functionality
   - Consider VPN for sensitive environments

### For Workers/R2 Approach:
1. **State Management**: Multipart uploads are stateful
   - Store state in client or external database
   - Implement cleanup for abandoned uploads

2. **Cost Implications**: R2 storage costs
   - Monitor usage and costs
   - Implement lifecycle policies

## Testing Checklist

### Local Testing:
- [ ] MinIO accessible at `http://192.168.1.127:9000`
- [ ] CORS headers present in responses
- [ ] Direct upload works from browser
- [ ] File appears in MinIO bucket

### Global Testing:
- [ ] DNS propagation complete (`nslookup minio.yourdomain.com`)
- [ ] MinIO accessible from external network
- [ ] Upload works from different geographic locations
- [ ] Web app loads correctly through CloudFlare

## Monitoring & Maintenance

### Key Metrics:
- Upload success rate
- File sizes uploaded
- Geographic distribution of uploads
- CORS preflight request volume

### Log Analysis:
```bash
# MinIO access logs
docker logs minio-standalone | grep -E "(PUT|POST)"

# Backend upload logs
tail -f /home/gaius/sermon-uploader.log | grep -i upload

# Router logs (if available)
# Monitor port 9000 traffic
```

### Alerts:
- MinIO service down
- Unusual upload patterns
- High bandwidth usage
- Failed authentication attempts

## Fallback Strategy

If global MinIO access fails:
1. **Immediate**: Switch to chunked upload method
2. **Short-term**: Use temporary proxy disable automation
3. **Long-term**: Migrate to CloudFlare R2 with Workers

## Performance Optimization

### For Large Files (>1GB):
1. **Connection Pooling**: Reuse HTTP connections
2. **Parallel Chunks**: Upload multiple chunks simultaneously
3. **Progress Tracking**: Fine-grained progress reporting
4. **Error Recovery**: Resume failed uploads

### Network Optimization:
1. **CDN Benefits**: Static assets still served via CloudFlare
2. **Compression**: Enable gzip for API responses
3. **Keep-Alive**: Maintain persistent connections
4. **DNS Caching**: Optimize DNS resolution times

## Cost Analysis

### Current Solution (Dual-Domain):
- **Cost**: $0 (uses existing infrastructure)
- **Bandwidth**: Home internet upload bandwidth
- **Storage**: Local Pi storage costs
- **Maintenance**: Manual updates and monitoring

### Alternative (CloudFlare R2):
- **Cost**: $0.015/GB stored + $0.01/GB transferred
- **Bandwidth**: Unlimited CloudFlare bandwidth
- **Storage**: Managed R2 storage
- **Maintenance**: Managed service

---

## References
- [CloudFlare CORS Documentation](https://developers.cloudflare.com/workers/examples/cors-header-proxy/)
- [MinIO Browser Upload Guide](https://docs.min.io/docs/minio-client-quickstart-guide.html)
- [CloudFlare Workers R2 API](https://developers.cloudflare.com/r2/api/workers/)
- [DNS Only vs Proxied Mode](https://developers.cloudflare.com/dns/manage-dns-records/reference/proxied-dns-records/)