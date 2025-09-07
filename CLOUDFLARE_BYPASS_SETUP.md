# CloudFlare Bypass Setup for MinIO Direct Uploads

## Architecture Overview

```
Internet Users
     ↓
CloudFlare (for web app)           Direct DNS (for MinIO)
sermon-uploader.yourdomain.com      minio.yourdomain.com
     ↓                                    ↓
Your Pi Backend (8000)              Your Pi MinIO (9000)
```

## Step 1: CloudFlare DNS Configuration

### In CloudFlare Dashboard:

1. **Keep your main app on CloudFlare proxy** (orange cloud ON):
   - Type: A
   - Name: sermon-uploader (or your subdomain)
   - Content: Your Pi's public IP
   - Proxy: ✅ ON (orange cloud)

2. **Add MinIO subdomain with proxy DISABLED** (grey cloud):
   - Type: A
   - Name: minio
   - Content: Your Pi's public IP  
   - Proxy: ❌ OFF (grey cloud) ← CRITICAL: This bypasses CloudFlare!

## Step 2: Router Port Forwarding

Forward these ports to your Pi (192.168.1.127):
- Port 8000 → 8000 (Backend API)
- Port 9000 → 9000 (MinIO)
- Port 9001 → 9001 (MinIO Console, optional)

## Step 3: Update MinIO Configuration

```bash
# SSH to your Pi
ssh gaius@192.168.1.127

# Update MinIO to accept external connections
docker stop minio-standalone
docker rm minio-standalone

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
  -e MINIO_BROWSER_REDIRECT_URL=http://minio.yourdomain.com:9001 \
  minio/minio server /data --console-address ":9001"
```

## Step 4: Configure MinIO CORS for Global Access

```bash
# Inside MinIO container
docker exec -it minio-standalone sh

# Set up MinIO client
mc alias set local http://localhost:9000 gaius "John 3:16"

# Create CORS policy
cat > /tmp/cors.json << 'EOF'
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

# Apply CORS to bucket
mc cors set /tmp/cors.json local/sermons

# Set bucket policy for public uploads
mc policy set upload local/sermons
```

## Step 5: Backend Configuration

Update the backend to generate URLs using the public MinIO domain:

```go
// In config/config.go or .env
MINIO_PUBLIC_ENDPOINT=http://minio.yourdomain.com:9000
MINIO_INTERNAL_ENDPOINT=http://localhost:9000
```

## Step 6: Frontend Configuration

```typescript
// Frontend uses different URLs based on upload method
const CLOUDFLARE_API = 'https://sermon-uploader.yourdomain.com'
const DIRECT_MINIO = 'http://minio.yourdomain.com:9000'
```

## How It Works

1. **Web App Access**: Users visit `https://sermon-uploader.yourdomain.com` (through CloudFlare)
2. **Get Upload URL**: Frontend calls backend API to get presigned MinIO URL
3. **Direct Upload**: Browser uploads directly to `http://minio.yourdomain.com:9000` (bypasses CloudFlare!)
4. **No Size Limit**: Since MinIO subdomain has CloudFlare proxy OFF, no 100MB limit!

## Security Considerations

- MinIO is exposed directly to internet (necessary for bypassing CloudFlare)
- Use presigned URLs with expiration (24 hours recommended)
- Consider adding rate limiting at router level
- Monitor MinIO access logs
- Use HTTPS with Let's Encrypt (optional but recommended)

## Testing

1. Check DNS propagation:
```bash
nslookup minio.yourdomain.com
# Should return your Pi's public IP directly
```

2. Test MinIO access:
```bash
curl -I http://minio.yourdomain.com:9000/minio/health/live
```

3. Test CORS:
```javascript
// Run in browser console
fetch('http://minio.yourdomain.com:9000/sermons/', {
  method: 'HEAD'
}).then(r => console.log('CORS OK:', r.ok))
```