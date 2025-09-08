# WPGC API Endpoints - Global Access

All endpoints are accessible globally via `https://api.wpgc.church`

## 🚨 Admin Endpoints (Use with Caution)

### Clear Bucket - Delete All Files
```bash
DELETE https://api.wpgc.church/api/admin/clear-bucket
```

**⚠️ WARNING**: This deletes ALL files from the storage bucket!

#### Example Request (from anywhere in the world):
```bash
# Using curl
curl -X DELETE https://api.wpgc.church/api/admin/clear-bucket

# Using Postman
Method: DELETE
URL: https://api.wpgc.church/api/admin/clear-bucket
```

#### Response:
```json
{
  "success": true,
  "files_deleted": 15,
  "space_freed": "2.3GB"
}
```

## 📊 Public API Endpoints (Safe to Use)

### 1. Health Check
```bash
GET https://api.wpgc.church/api/health
```

### 2. Upload Sermon
```bash
POST https://api.wpgc.church/api/uploads/sermon
Content-Type: multipart/form-data
Body: file (binary)
```

### 3. List Files
```bash
GET https://api.wpgc.church/api/files/list
```

### 4. Get File Details
```bash
GET https://api.wpgc.church/api/files/{filename}
```

### 5. Delete Single File
```bash
DELETE https://api.wpgc.church/api/files/{filename}
```

### 6. Check Duplicate (Hash Verification)
```bash
POST https://api.wpgc.church/api/files/verify-hash
Content-Type: application/json

{
  "hash": "sha256..."
}
```

### 7. Processing Queue Status
```bash
GET https://api.wpgc.church/api/processing/queue
```

### 8. Storage Statistics
```bash
GET https://api.wpgc.church/api/storage/stats
```

## 🔐 Security Notes

- No authentication currently required (will be added)
- Rate limiting: 10 requests/minute for admin endpoints
- All endpoints accessible globally (not restricted to local network)
- api.wpgc.church uses DNS-only routing (bypasses Cloudflare proxy)

## 🧪 Testing from Postman

1. **Import this collection**: Create a new Postman collection
2. **Set base URL**: `https://api.wpgc.church`
3. **No authentication needed**: Currently open access
4. **SSL Issues?**: Disable certificate verification in Postman settings

## 📱 Testing from Mobile/External Networks

All endpoints work from:
- ✅ Home network
- ✅ Mobile data (4G/5G)
- ✅ Public WiFi
- ✅ Any global location

## 🛠️ Troubleshooting

### Connection Refused
- Check if backend is running: `ssh gaius@192.168.1.127 "sudo systemctl status sermon-uploader"`

### SSL Certificate Error
- Wait for certificate propagation (can take 5-10 minutes)
- Or use HTTP temporarily: `http://api.wpgc.church`

### 404 Not Found
- Verify endpoint path is correct
- Check API is deployed: `curl https://api.wpgc.church/api/health`

## 🚀 Quick Test Commands

```bash
# Test from anywhere
curl https://api.wpgc.church/api/health

# Clear bucket (DANGER!)
curl -X DELETE https://api.wpgc.church/api/admin/clear-bucket

# Upload file
curl -X POST -F "file=@sermon.wav" https://api.wpgc.church/api/uploads/sermon

# List files
curl https://api.wpgc.church/api/files/list
```