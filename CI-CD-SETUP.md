# WPGC CI/CD Pipeline Setup Guide

## Overview

Complete GitHub Actions CI/CD pipeline for automated deployment of the WPGC Admin Platform to Raspberry Pi.

## Architecture

```
GitHub Repository (master branch)
    ↓ (push trigger)
GitHub Actions Workflow
    ↓ (test & build)
Self-Hosted Runner (Raspberry Pi)
    ↓ (deploy)
Production Services (admin.wpgc.church + api.wpgc.church)
```

## Setup Steps

### 1. GitHub Repository Setup ✅

Repository: `https://github.com/White-Plains-Gospel-Chapel/sermon-uploader`

### 2. Pi Self-Hosted Runner Setup

#### Step 2a: Install Runner
```bash
# On your Pi, run:
ssh gaius@192.168.1.127
cd /opt/github-runner
```

#### Step 2b: Configure Runner
1. Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/actions/runners
2. Click **"New self-hosted runner"**
3. Select **Linux ARM64**
4. Copy the configuration token
5. Run on Pi:

```bash
sudo su - github-runner
cd /opt/github-runner
./config.sh --url https://github.com/White-Plains-Gospel-Chapel/sermon-uploader --token YOUR_TOKEN_HERE
# When prompted:
# - Enter name: wpgc-pi-runner
# - Enter work folder: _work
# - Add labels: raspberry-pi,arm64,production
```

#### Step 2c: Install as Service
```bash
sudo ./svc.sh install
sudo systemctl enable github-runner
sudo systemctl start github-runner
```

#### Step 2d: Verify Runner
```bash
sudo systemctl status github-runner
# Should show "active (running)"
```

### 3. GitHub Secrets Setup

Add these secrets in GitHub repository settings:

1. Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/secrets/actions
2. Add secrets:

```
DISCORD_WEBHOOK_URL = https://discord.com/api/webhooks/YOUR_WEBHOOK_URL
```

### 4. Production Directories Setup ✅

Already configured on Pi:
- `/opt/sermon-uploader/` - Backend service
- `/opt/admin-dashboard/` - Frontend application  
- `/etc/nginx/sites-available/wpgc-platform` - Nginx config
- `/etc/systemd/system/sermon-uploader.service` - Backend service

### 5. Workflow Triggers

The pipeline triggers on:
- **Push to master**: Full deployment
- **Pull request to master**: Test only (no deployment)

## Workflow Stages

### Stage 1: Test (Ubuntu Runner)
- ✅ Go backend tests
- ✅ Backend build verification  
- ✅ Frontend build verification
- ✅ Dependency security scan

### Stage 2: Deploy (Self-Hosted Pi Runner)
- ✅ Cross-compile Go backend (ARM64)
- ✅ Build Next.js frontend
- ✅ Deploy backend binary
- ✅ Deploy frontend application
- ✅ Restart services
- ✅ Health checks
- ✅ Discord notifications

## Manual Deployment

If needed, you can deploy manually:

```bash
# From your local machine:
cd "/Users/gaius/Documents/WPGC web/sermon-uploader"
./deploy-full-platform.sh
```

## Monitoring & Health Checks

### Service Status
```bash
# Check services on Pi:
ssh gaius@192.168.1.127
sudo systemctl status sermon-uploader
sudo systemctl status nginx
sudo systemctl status github-runner
```

### Application Health
```bash
# Test endpoints:
curl https://admin.wpgc.church
curl https://api.wpgc.church/api/health
```

### Deployment Logs
```bash
# GitHub Actions logs available at:
# https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/actions

# Pi service logs:
sudo journalctl -u sermon-uploader -f
sudo journalctl -u github-runner -f
```

## Deployment Notifications

Discord notifications are sent to `sermons-uploading-notif` channel for:
- ✅ Successful deployments
- ❌ Failed deployments  
- 🔄 Deployment status updates

## Security Features

- ✅ Secrets stored in GitHub Actions secrets
- ✅ Self-hosted runner on private network
- ✅ SSL certificates auto-renewal
- ✅ Service isolation with systemd
- ✅ Non-root service execution

## Rollback Procedure

If deployment fails:

```bash
# 1. Check service status
ssh gaius@192.168.1.127
sudo systemctl status sermon-uploader

# 2. View logs
sudo journalctl -u sermon-uploader -n 50

# 3. Rollback to previous version (if needed)
cd /opt/sermon-uploader
sudo cp sermon-uploader.backup sermon-uploader
sudo systemctl restart sermon-uploader

# 4. Verify
curl http://localhost:8000/api/health
```

## Performance Optimizations

- ✅ Cross-compilation for ARM64
- ✅ Static binary compilation (`CGO_ENABLED=0`)
- ✅ Build artifacts optimization (`-ldflags="-s -w"`)
- ✅ Frontend production build caching
- ✅ Nginx reverse proxy caching

## Future Enhancements

- [ ] Blue-green deployments
- [ ] Database migration automation  
- [ ] Performance testing in pipeline
- [ ] Automated security scanning
- [ ] Multi-environment support (staging)

## Troubleshooting

### Runner Not Connecting
```bash
ssh gaius@192.168.1.127
sudo systemctl restart github-runner
sudo journalctl -u github-runner -f
```

### Deployment Hanging
```bash
# Check if processes are stuck:
sudo pkill -f "next start"
sudo systemctl restart sermon-uploader
```

### SSL Certificate Issues
```bash
sudo certbot renew --dry-run
sudo systemctl reload nginx
```

## Success Metrics

- ✅ Zero-downtime deployments
- ✅ Automated testing and validation
- ✅ Real-time deployment notifications  
- ✅ Full rollback capability
- ✅ Production-ready SSL/TLS
- ✅ Global API accessibility