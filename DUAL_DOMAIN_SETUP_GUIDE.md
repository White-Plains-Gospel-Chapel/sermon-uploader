# Dual-Domain Setup Guide - Bypass CloudFlare for Uploads

## Overview
This guide implements a dual-domain architecture where:
- 📱 **Web App**: Uses CloudFlare (protection, global CDN, caching)
- 💾 **MinIO Uploads**: Bypasses CloudFlare (no 100MB limit, direct access)

## Architecture Diagram
```
Global Users
     ↓
┌─────────────────────┬─────────────────────┐
│ CloudFlare (🟠 ON)  │ Direct DNS (⚪ OFF) │
│ Web App Protection  │ MinIO Direct Access │
└─────────┬───────────┴─────────┬───────────┘
          │                     │
    ┌─────▼─────┐         ┌─────▼─────┐
    │ Pi Backend│         │ Pi MinIO  │
    │ Port 8000 │         │ Port 9000 │
    └───────────┘         └───────────┘
```

## Step-by-Step Setup

### Step 1: Configure CloudFlare DNS
```bash
# Run the CloudFlare DNS setup script
./setup-cloudflare-dns.sh

# You'll need:
# - CloudFlare API Token (from dashboard)
# - Your domain name (e.g., wpgcservices.com)
# - Your Pi's public IP address
```

This creates:
- `sermon-uploader.wpgcservices.com` → Your Pi IP (🟠 Proxied through CloudFlare)
- `minio.wpgcservices.com` → Your Pi IP (⚪ DNS Only - bypasses CloudFlare)

### Step 2: Configure Router Port Forwarding
Forward these ports from your router to your Pi (192.168.1.127):
```
External Port → Internal Port
8000         → 192.168.1.127:8000  (Backend API)
9000         → 192.168.1.127:9000  (MinIO Storage)
9001         → 192.168.1.127:9001  (MinIO Console)
```

### Step 3: Setup MinIO for Global Access
```bash
# Configure MinIO to accept global connections with CORS
./setup-minio-global.sh
```

This:
- ✅ Restarts MinIO with public domain configuration
- ✅ Sets up CORS for browser uploads
- ✅ Configures bucket policies for direct uploads
- ✅ Tests global accessibility

### Step 4: Deploy Dual-Domain Application
```bash
# Build and deploy both frontend and backend
./deploy-dual-domain.sh
```

This:
- ✅ Builds backend with public MinIO support
- ✅ Builds frontend for dual-domain architecture  
- ✅ Deploys both to Pi
- ✅ Starts services
- ✅ Tests connectivity

## How It Works

### Upload Flow
1. **User visits**: `https://sermon-uploader.wpgcservices.com` (through CloudFlare)
2. **Frontend requests**: Upload URL from backend API (through CloudFlare)
3. **Backend generates**: Presigned URL for `http://minio.wpgcservices.com:9000` (direct)
4. **Browser uploads**: Directly to MinIO subdomain (bypasses CloudFlare!)
5. **No size limit**: MinIO subdomain doesn't go through CloudFlare proxy

### API Calls vs Uploads
- 📡 **API Calls**: Go through CloudFlare (better performance, protection)
- 📤 **File Uploads**: Go direct to MinIO (bypass 100MB limit)

## Testing the Setup

### 1. DNS Propagation
```bash
# Check that DNS is working
nslookup sermon-uploader.wpgcservices.com  # Should show CloudFlare IP
nslookup minio.wpgcservices.com            # Should show your Pi's public IP
```

### 2. Service Health
```bash
# Test backend API (through CloudFlare)
curl https://sermon-uploader.wpgcservices.com/api/health

# Test MinIO direct access
curl -I http://minio.wpgcservices.com:9000/minio/health/live
```

### 3. CORS Configuration
```bash
# Test CORS headers for browser uploads
curl -H "Origin: https://sermon-uploader.wpgcservices.com" \
     -I http://minio.wpgcservices.com:9000/sermons/
```

Should return `Access-Control-Allow-Origin: *` header.

### 4. Full Upload Test
1. Visit: `https://sermon-uploader.wpgcservices.com`
2. Upload a file larger than 100MB
3. Check browser network tab - upload should go to `minio.wpgcservices.com:9000`
4. Verify file appears in MinIO

## Security Considerations

### ✅ Secured
- Web app protected by CloudFlare (DDoS, bot protection, SSL)
- Presigned URLs with 24-hour expiration
- CORS limited to necessary origins (can be restricted further)

### ⚠️ Exposed
- MinIO port 9000 directly accessible from internet
- Necessary trade-off to bypass CloudFlare's upload limit
- Mitigated by presigned URL security and bucket policies

## Troubleshooting

### Upload Fails
1. **Check DNS**: Ensure `minio.wpgcservices.com` resolves to Pi's public IP
2. **Check ports**: Verify router forwards port 9000 to Pi
3. **Check CORS**: Use browser dev tools to see CORS errors
4. **Check logs**: `ssh gaius@192.168.1.127 'tail -f /home/gaius/sermon-uploader.log'`

### Web App Doesn't Load
1. **Check CloudFlare**: Ensure `sermon-uploader.wpgcservices.com` is proxied (🟠)
2. **Check backend**: Verify Pi backend is running on port 8000
3. **Check router**: Verify port 8000 forwarding

### MinIO Not Accessible
1. **Check container**: `ssh gaius@192.168.1.127 'docker ps | grep minio'`
2. **Check logs**: `ssh gaius@192.168.1.127 'docker logs minio-standalone'`
3. **Restart if needed**: `./setup-minio-global.sh`

## Performance & Monitoring

### Expected Performance
- 📱 **Web App**: Fast (CloudFlare CDN)
- 📤 **Uploads**: Limited by home internet upload speed
- 🌐 **Global Access**: Works from anywhere in the world

### Monitoring Commands
```bash
# Backend logs
ssh gaius@192.168.1.127 'tail -f /home/gaius/sermon-uploader.log'

# MinIO logs  
ssh gaius@192.168.1.127 'docker logs -f minio-standalone'

# System resources
ssh gaius@192.168.1.127 'htop'
```

## Cost Analysis

### Current Solution (Dual-Domain)
- **CloudFlare**: Free tier (web app protection)
- **MinIO**: Self-hosted storage
- **Bandwidth**: Home internet upload limits
- **Total Cost**: $0/month + electricity

### Alternative (CloudFlare R2)
- **CloudFlare R2**: $0.015/GB stored + $0.01/GB transferred
- **Workers**: $5/month for larger request limits
- **Total Cost**: ~$20-50/month for heavy usage

## Backup & Recovery

### Backup MinIO Data
```bash
# Create backup
ssh gaius@192.168.1.127 'tar -czf minio-backup-$(date +%Y%m%d).tar.gz /home/gaius/minio/data'

# Copy backup off-site
scp gaius@192.168.1.127:/home/gaius/minio-backup-*.tar.gz ./backups/
```

### Disaster Recovery
1. **Pi Hardware Failure**: Restore MinIO data to new Pi
2. **Internet Outage**: Local network uploads still work
3. **CloudFlare Issues**: Can temporarily use direct Pi IP for web app

## Future Improvements

### Short-term
- [ ] Add HTTPS to MinIO (Let's Encrypt)
- [ ] Implement upload progress persistence
- [ ] Add file deduplication

### Long-term  
- [ ] Multi-region MinIO deployment
- [ ] CDN for static file serving
- [ ] Automated backup to cloud storage

---

## Quick Reference

### URLs
- 📱 **Web App**: https://sermon-uploader.wpgcservices.com
- 💾 **MinIO API**: http://minio.wpgcservices.com:9000
- 📊 **MinIO Console**: http://minio.wpgcservices.com:9001
- 🔧 **Backend API**: http://192.168.1.127:8000 (local)

### Scripts
- `./setup-cloudflare-dns.sh` - Configure DNS records
- `./setup-minio-global.sh` - Configure MinIO for global access
- `./deploy-dual-domain.sh` - Build and deploy application

### Key Benefits
✅ **No Upload Limits** - MinIO subdomain bypasses CloudFlare completely  
✅ **Global Access** - Works from anywhere in the world  
✅ **Web App Protected** - Still gets CloudFlare DDoS protection  
✅ **Cost Effective** - Uses existing infrastructure  
✅ **High Performance** - Direct peer-to-peer uploads