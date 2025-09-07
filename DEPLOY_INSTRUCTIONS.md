# 🚀 Deployment Instructions

Since your Raspberry Pi is on a private network, GitHub Actions cannot reach it directly. 
Follow these steps to deploy the multipart upload system:

## Option 1: Quick Deploy (If you have SSH access from your current location)

Run this single command from your Mac:

```bash
ssh gaius@192.168.1.127 "cd /home/gaius/sermon-uploader && git pull && chmod +x deploy-from-pi.sh && ./deploy-from-pi.sh"
```

## Option 2: Manual Deploy (If you need to SSH into the Pi first)

1. SSH into your Pi:
```bash
ssh gaius@192.168.1.127
```

2. Navigate to the project:
```bash
cd /home/gaius/sermon-uploader
```

3. Pull the latest code:
```bash
git pull origin master
```

4. Run the deployment script:
```bash
chmod +x deploy-from-pi.sh
./deploy-from-pi.sh
```

## What the Deployment Does:

✅ Sets up HTTPS/TLS certificates for MinIO  
✅ Configures Docker services with HTTPS  
✅ Builds the backend with multipart upload support  
✅ Tests all endpoints automatically  
✅ Shows you the service status  

## After Deployment:

1. **Accept the Certificate**: Open https://192.168.1.127:9000 in your browser and accept the security warning

2. **Test the Multipart Endpoint**:
```bash
curl -k -X POST http://192.168.1.127:8000/api/upload/multipart/init \
  -H "Content-Type: application/json" \
  -d '{"filename":"test.wav","fileSize":734003200,"fileHash":"test123"}'
```

3. **Check Service Status**:
```bash
docker ps
```

## Service URLs After Deployment:

- **MinIO API**: https://192.168.1.127:9000
- **MinIO Console**: https://192.168.1.127:9001  
- **Backend API**: http://192.168.1.127:8000

## Troubleshooting:

If the deployment fails, check:

1. **Docker logs**:
```bash
docker logs sermon-minio
docker logs sermon-backend
```

2. **Service status**:
```bash
docker-compose -f docker-compose.pi.yml ps
```

3. **Test HTTPS**:
```bash
curl -k https://localhost:9000/minio/health/live
```

## Frontend Integration:

After deployment, update your frontend to use the new multipart endpoints:

```javascript
// Initialize multipart upload
const response = await fetch('http://192.168.1.127:8000/api/upload/multipart/init', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    filename: 'sermon.wav',
    fileSize: file.size,
    fileHash: await calculateHash(file)
  })
});

const { uploadId, totalParts } = await response.json();
```

## Features Now Available:

- **5MB Chunks**: MinIO minimum chunk size
- **Server Queue**: 1 file at a time (prevents Pi overload)  
- **Resumability**: Uploads can be resumed if interrupted
- **HTTPS**: Secure uploads with TLS
- **No Browser Freezing**: Chunked uploads prevent freezing
- **700MB+ Files**: Handles your large sermon files

---

Last updated: September 7, 2025