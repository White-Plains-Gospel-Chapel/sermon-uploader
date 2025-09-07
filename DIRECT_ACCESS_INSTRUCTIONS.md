# Direct Pi Access Instructions (Bypass CloudFlare)

## IMPORTANT: How to Access the Application

### ✅ CORRECT WAY (Bypasses CloudFlare - No 100MB Limit)
Open your browser and go to:
```
http://192.168.1.127:8000
```

This connects DIRECTLY to your Raspberry Pi, completely bypassing CloudFlare's 100MB upload restriction.

### ❌ WRONG WAY (Goes through CloudFlare - 100MB Limit Applies)
Do NOT access via:
- Your CloudFlare domain (e.g., sermon-uploader.yourdomain.com)
- Any HTTPS URL
- Any public domain name

## Why This Works

1. **Direct Connection**: Your browser connects directly to the Pi on your local network
2. **No CloudFlare**: Completely bypasses CloudFlare's proxy and its 100MB limit
3. **No Upload Restrictions**: Can upload files of any size (limited only by Pi storage)

## Troubleshooting

### If you see "Load Failed" or network errors:

1. **Check you're using the correct URL**: Must be `http://192.168.1.127:8000`
2. **Check Pi is accessible**: 
   ```bash
   ping 192.168.1.127
   ```
3. **Check backend is running**:
   ```bash
   ssh gaius@192.168.1.127 "pgrep -f sermon-uploader"
   ```

### If uploads still fail:

1. **Clear browser cache**: The old CloudFlare settings might be cached
2. **Use incognito/private window**: Ensures no cached settings
3. **Check browser console**: Look for CORS or network errors

## Current Status

✅ **Backend**: Configured with CORS to accept all origins
✅ **Frontend**: Configured to use direct Pi IP (192.168.1.127:8000)
✅ **Deployment**: Both frontend and backend deployed to Pi
✅ **CloudFlare Bypass**: Complete - no 100MB limit when accessed directly

## Testing

To verify everything works:

1. Open browser to `http://192.168.1.127:8000`
2. Try uploading a file larger than 100MB
3. Check browser console for any errors
4. Monitor backend logs:
   ```bash
   ssh gaius@192.168.1.127 "tail -f /home/gaius/sermon-uploader.log"
   ```

## Architecture

```
Your Browser (on local network)
     ↓ Direct HTTP connection
Raspberry Pi (192.168.1.127:8000)
     ├── Frontend (Next.js static files)
     └── Backend API (Go Fiber)
          ↓
     MinIO Storage (192.168.1.127:9000)
```

No CloudFlare in this path = No upload limits!