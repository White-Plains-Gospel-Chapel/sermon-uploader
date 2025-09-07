# CORS Fix Deployment Guide

## 🎯 Overview

This document outlines the deployment process for the comprehensive CORS configuration fixes implemented in PR #55. These fixes resolve browser-based bulk upload issues and enable full cross-origin functionality.

## 📋 Deployment Summary

**Date**: September 7, 2025  
**PR**: #55 - Fix CORS configuration for browser-based bulk uploads  
**Branch**: `fix/cors-browser-bulk-uploads` → `master`  
**Target**: Raspberry Pi 5 (ARM64) at `192.168.1.127`  
**Binary**: `sermon-uploader-cors-fix` (10MB ARM64)

## 🔧 What's Being Deployed

### CORS Fixes Included
- ✅ Proper preflight OPTIONS request handling
- ✅ Support for all required headers (`Content-Type`, `Authorization`, `X-Amz-*`)
- ✅ Multiple HTTP methods (`GET`, `POST`, `PUT`, `DELETE`, `OPTIONS`)
- ✅ Environment-specific origin configuration
- ✅ Credentials handling for authenticated requests
- ✅ Browser bulk upload compatibility

### Test Coverage
- ✅ 15+ comprehensive CORS test cases
- ✅ Integration testing with real MinIO scenarios
- ✅ Cross-browser compatibility validation
- ✅ Bulk upload stress testing

## 🚀 Deployment Process

### Prerequisites
1. SSH access to Raspberry Pi (`pi@192.168.1.127`)
2. Binary built for ARM64 architecture
3. Network connectivity to Pi
4. Backup capability enabled

### Step 1: Automated Deployment
```bash
# Run the deployment script
./deploy-cors-fix.sh

# Or with custom Pi settings
PI_HOST=192.168.1.127 PI_USER=pi ./deploy-cors-fix.sh
```

### Step 2: Manual Deployment (if needed)
```bash
# 1. Copy binary to Pi
scp bin/sermon-uploader-cors-fix pi@192.168.1.127:/tmp/

# 2. Install on Pi
ssh pi@192.168.1.127 "
    sudo mkdir -p /opt/sermon-uploader
    sudo mv /tmp/sermon-uploader-cors-fix /opt/sermon-uploader/sermon-uploader
    sudo chmod +x /opt/sermon-uploader/sermon-uploader
    sudo chown root:root /opt/sermon-uploader/sermon-uploader
"

# 3. Restart service
ssh pi@192.168.1.127 "
    sudo pkill -f sermon-uploader || true
    cd /opt/sermon-uploader
    nohup sudo ./sermon-uploader > ./service.log 2>&1 &
"
```

## ✅ Verification Steps

### 1. Service Health Check
```bash
curl -f http://192.168.1.127:8000/api/health
```

### 2. CORS Preflight Test
```bash
curl -v \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -X OPTIONS \
  http://192.168.1.127:8000/api/upload/presigned
```

**Expected Response Headers:**
```
Access-Control-Allow-Origin: http://localhost:3000
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-Amz-*
Access-Control-Allow-Credentials: true
```

### 3. Browser Test
1. Open browser to frontend application
2. Navigate to bulk upload interface
3. Select multiple files for upload
4. Verify no CORS errors in browser console
5. Confirm all files upload successfully

### 4. Integration Test
```bash
# Run CORS test utility
cd /opt/sermon-uploader
./sermon-uploader corstest
```

## 🔄 Rollback Procedure

### Automatic Rollback
```bash
# Rollback using deployment script (replace timestamp)
./deploy-cors-fix.sh rollback 20250907_014500
```

### Manual Rollback
```bash
ssh pi@192.168.1.127 "
    cd /opt/sermon-uploader
    sudo pkill -f sermon-uploader || true
    
    # Restore backup (replace timestamp)
    sudo cp sermon-uploader.backup.20250907_014500 sermon-uploader
    sudo chmod +x sermon-uploader
    
    # Restart service
    nohup sudo ./sermon-uploader > ./service.log 2>&1 &
"
```

## 📊 Expected Impact

### Before Deployment
- ❌ Browser bulk uploads fail with CORS policy violations
- ❌ Preflight requests rejected or misconfigured
- ❌ Cross-origin requests from frontend blocked
- ❌ Users forced to use alternative upload methods

### After Deployment
- ✅ Seamless browser-based bulk uploads
- ✅ Full cross-origin functionality
- ✅ Proper preflight request handling
- ✅ All modern browsers supported
- ✅ Enhanced user experience

## 🔧 Configuration

### Environment Variables
The following environment variables are configured for CORS:

```bash
# Development
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

# Production  
CORS_ALLOWED_ORIGINS=https://wpgc.org,https://sermon-uploader.wpgc.org

# MinIO CORS (automatically configured)
MINIO_CORS_ENABLED=true
```

### MinIO CORS Policy
The service automatically configures MinIO with the following CORS policy:

```json
{
  "CORSRule": [
    {
      "AllowedOrigin": ["http://localhost:3000", "https://wpgc.org"],
      "AllowedMethod": ["GET", "POST", "PUT", "DELETE", "HEAD"],
      "AllowedHeader": ["*"],
      "ExposeHeader": ["ETag", "x-amz-request-id"]
    }
  ]
}
```

## 🐛 Troubleshooting

### Common Issues

#### 1. Service Won't Start
```bash
# Check logs
ssh pi@192.168.1.127 "tail -f /opt/sermon-uploader/service.log"

# Check process
ssh pi@192.168.1.127 "ps aux | grep sermon-uploader"
```

#### 2. CORS Still Failing
```bash
# Verify environment
ssh pi@192.168.1.127 "cd /opt/sermon-uploader && cat .env | grep CORS"

# Test MinIO directly
curl -v -H "Origin: http://localhost:3000" \
  -X OPTIONS \
  http://192.168.1.127:9000/sermons/
```

#### 3. Binary Architecture Mismatch
```bash
# Check binary architecture
ssh pi@192.168.1.127 "file /opt/sermon-uploader/sermon-uploader"
# Should show: ARM aarch64
```

#### 4. Network Connectivity
```bash
# Test Pi connectivity
ping 192.168.1.127

# Test service ports
nmap -p 8000,9000 192.168.1.127
```

## 📝 Deployment Checklist

### Pre-Deployment
- [ ] Binary built successfully for ARM64
- [ ] All CORS tests passing
- [ ] Pi connectivity verified
- [ ] SSH access confirmed
- [ ] Backup plan ready

### During Deployment
- [ ] Current binary backed up
- [ ] New binary deployed
- [ ] Configuration updated
- [ ] Service restarted successfully
- [ ] Initial health check passed

### Post-Deployment
- [ ] CORS preflight test passed
- [ ] Browser bulk upload tested
- [ ] Integration tests completed
- [ ] No errors in service logs
- [ ] Performance baseline established

### Rollback Ready
- [ ] Rollback procedure tested
- [ ] Backup timestamp recorded
- [ ] Rollback script validated
- [ ] Team notified of deployment

## 🔍 Monitoring

### Key Metrics to Watch
1. **CORS Request Success Rate**: Should be >99%
2. **Upload Success Rate**: Should maintain current levels
3. **Browser Compatibility**: No console errors
4. **Service Uptime**: Continuous availability
5. **Response Times**: <200ms for CORS preflight

### Log Locations
- **Service logs**: `/opt/sermon-uploader/service.log`
- **Deployment logs**: `logs/deployment_TIMESTAMP.log`
- **Backup info**: `backups/backup_TIMESTAMP.info`

## 📞 Support Contacts

For deployment issues:
1. Check this documentation first
2. Review service logs on Pi
3. Test rollback procedure if needed
4. Verify network connectivity

## 🎉 Success Criteria

Deployment is successful when:
- ✅ Service starts without errors
- ✅ Health check returns HTTP 200
- ✅ CORS preflight returns proper headers
- ✅ Browser bulk uploads work without CORS errors
- ✅ All existing functionality preserved
- ✅ No regression in performance

---

**Generated with [Claude Code](https://claude.ai/code)**

**Co-Authored-By: Claude <noreply@anthropic.com>**