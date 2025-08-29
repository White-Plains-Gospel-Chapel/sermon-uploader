# 🚀 Production Deployment Guide

## 🔐 Pre-Deployment Security Checklist

### ✅ Secrets Removed
- [x] Discord webhook URL removed from code
- [x] MinIO credentials removed from docker-compose.yml
- [x] Environment variables properly templated
- [x] .gitignore updated to exclude .env files

### ✅ Container Architecture Decision: **SEPARATE CONTAINERS**

**Recommended Setup:**
```
┌─────────────────┐    ┌─────────────────┐
│  Sermon App     │    │     MinIO       │
│  (Updates)      │◄──►│  (Persistent)   │
│  Port 8000      │    │  Port 9000/9001 │
└─────────────────┘    └─────────────────┘
```

**Benefits:**
- ✅ **Zero data loss** on app updates
- ✅ **Independent scaling** - can restart app without affecting storage
- ✅ **Easier maintenance** - update app container while MinIO runs continuously
- ✅ **Data persistence** - MinIO volume survives app updates

## 📋 Setup Instructions

### 1. Prepare Your Pi

```bash
# SSH into your Pi
ssh pi@your-pi-ip

# Create project directory
sudo mkdir -p /opt/sermon-uploader
sudo chown $USER:$USER /opt/sermon-uploader
cd /opt/sermon-uploader

# Clone repository (after pushing to GitHub)
git clone https://github.com/yourusername/sermon-uploader.git .
```

### 2. Configure Environment

```bash
# Copy environment template
cp .env.example .env

# Edit with your secrets
nano .env
```

**Required environment variables:**
```bash
# MinIO Configuration
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=your-strong-access-key
MINIO_SECRET_KEY=your-strong-secret-key
MINIO_SECURE=false
MINIO_BUCKET=sermons

# Discord Configuration  
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN

# Application Configuration
PORT=8000
```

### 3. GitHub Repository Setup

#### A. Create Repository
```bash
# Initialize git (if not already)
git init
git add .
git commit -m "Initial commit: Sermon uploader with parallel processing"

# Create on GitHub and push
git remote add origin https://github.com/yourusername/sermon-uploader.git
git push -u origin main
```

#### B. Configure GitHub Secrets
Go to **Settings > Secrets and variables > Actions** and add:

| Secret Name | Description | Example |
|-------------|-------------|---------|
| `PI_HOST` | Pi IP address | `192.168.1.100` |
| `PI_USER` | Pi username | `pi` |
| `PI_SSH_KEY` | Private SSH key for Pi | `-----BEGIN RSA PRIVATE KEY-----...` |
| `PI_PORT` | SSH port (optional) | `22` |
| `DISCORD_WEBHOOK_URL` | Discord webhook for notifications | `https://discord.com/api/webhooks/...` |

#### C. Generate SSH Key for Pi
```bash
# On your local machine
ssh-keygen -t rsa -b 4096 -f ~/.ssh/pi-deploy -C "github-deploy"

# Copy public key to Pi
ssh-copy-id -i ~/.ssh/pi-deploy.pub pi@your-pi-ip

# Add private key content to GitHub secret PI_SSH_KEY
cat ~/.ssh/pi-deploy
```

### 4. Initial Pi Deployment

```bash
# Start services for the first time
docker compose -f docker-compose.prod.yml up -d

# Check status
docker compose -f docker-compose.prod.yml ps

# View logs
docker compose -f docker-compose.prod.yml logs -f
```

## 🔄 Automated Deployment Process

### How it Works:
1. **Push to main/master** → Triggers GitHub Actions
2. **Security scan** → TruffleHog scans for secrets
3. **Build multi-arch image** → AMD64, ARM64, ARM/v7 for Pi compatibility
4. **Push to GHCR** → GitHub Container Registry
5. **Deploy to Pi** → SSH into Pi, pull new image, restart only app container
6. **Health check** → Verify deployment success
7. **Discord notification** → Success/failure notification

### Container Update Strategy:
```bash
# Only updates the sermon-uploader service
docker compose -f docker-compose.prod.yml up -d --force-recreate sermon-uploader

# MinIO continues running with persistent data
# No data loss, no downtime for storage
```

## 📊 Performance Optimizations Included

- ✅ **Parallel uploads** (2-5 simultaneous based on device)
- ✅ **Raspberry Pi optimization** (auto-detects capabilities)
- ✅ **Real-time progress tracking**
- ✅ **Smart duplicate detection**
- ✅ **Memory-efficient processing**

## 🔧 Manual Operations

### View Logs
```bash
cd /opt/sermon-uploader
docker compose -f docker-compose.prod.yml logs -f sermon-uploader
```

### Update Manually
```bash
cd /opt/sermon-uploader
git pull
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### Backup MinIO Data
```bash
# MinIO data is in persistent volume
docker volume ls | grep minio
docker run --rm -v sermon-uploader_minio_data:/data -v $(pwd):/backup alpine tar czf /backup/minio-backup.tar.gz -C /data .
```

### Monitor Resources
```bash
# System resources
htop

# Container stats
docker stats

# Disk usage
df -h
docker system df
```

## 🌐 Access Points

- **Upload Interface**: http://your-pi-ip:8000
- **MinIO Console**: http://your-pi-ip:9001
- **Health Check**: http://your-pi-ip:8000/api/health

## 🚨 Troubleshooting

### Container Won't Start
```bash
# Check logs
docker compose -f docker-compose.prod.yml logs

# Check environment
cat .env

# Restart services
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
```

### MinIO Connection Issues
```bash
# Test MinIO health
curl -f http://localhost:9000/minio/health/live

# Check MinIO logs
docker compose -f docker-compose.prod.yml logs minio
```

### GitHub Actions Failing
- Check repository secrets are set correctly
- Verify SSH key has access to Pi
- Check Pi has Docker installed and user in docker group