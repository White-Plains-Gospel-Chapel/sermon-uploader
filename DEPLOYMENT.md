# WPGC Admin Dashboard + Backend Deployment Guide

Deploy both the new React admin dashboard and Go backend to your Raspberry Pi.

## 🎯 What You're Deploying

### Admin Dashboard (React/Next.js)
- **Pages**: Dashboard, Sermons, Upload, Media, Events, Members, Settings
- **Domain**: `admin.wpgc.church`
- **Port**: 3000

### Backend API (Go/Fiber)
- **All API routes**: Public, Admin, Uploads, Maintenance
- **Domain**: `api.wpgc.church` 
- **Port**: 8000

## 📦 Files to Upload

### 1. Backend Files
Upload your entire backend directory:
```bash
# From your Mac, compress the backend
cd "/Users/gaius/Documents/WPGC web/sermon-uploader"
tar -czf wpgc-backend.tar.gz backend/

# Upload to Pi
scp wpgc-backend.tar.gz gaius@192.168.1.127:/tmp/
```

### 2. Frontend Files
Upload the built admin dashboard:
```bash
# From your Mac, build and compress the frontend
cd "/Users/gaius/Documents/WPGC web/sermon-uploader/frontend-react"
npm run build
tar -czf wpgc-admin.tar.gz .next package.json next.config.js public components app lib

# Upload to Pi
scp wpgc-admin.tar.gz gaius@192.168.1.127:/tmp/
```

## 🚀 Deployment Steps

### Step 1: SSH to Your Pi
```bash
ssh gaius@192.168.1.127
```

### Step 2: Deploy Backend
```bash
# Create deployment directory
sudo mkdir -p /opt/wpgc
cd /opt/wpgc

# Extract backend
sudo tar -xzf /tmp/wpgc-backend.tar.gz
sudo chown -R gaius:gaius /opt/wpgc/backend

# Build the backend
cd /opt/wpgc/backend
go build -o sermon-uploader-prod
```

### Step 3: Deploy Admin Dashboard
```bash
# Create web directory for admin
sudo mkdir -p /var/www/admin.wpgc.church
cd /var/www/admin.wpgc.church

# Extract frontend
sudo tar -xzf /tmp/wpgc-admin.tar.gz
sudo chown -R www-data:www-data /var/www/admin.wpgc.church

# Install production dependencies
sudo -u www-data npm install --production
```

### Step 4: Configure Environment
```bash
# Create backend environment file
sudo tee /opt/wpgc/backend/.env << 'EOF'
# Production Environment
ENV=production
PORT=8000

# MinIO Configuration (your existing setup)
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_USE_SSL=false
MINIO_BUCKET=sermons

# Discord Webhook (your existing webhook)
DISCORD_WEBHOOK_URL=your_discord_webhook_url_here

# Pi Optimizations
PI_OPTIMIZATION=true
MAX_MEMORY_LIMIT_MB=3072
GC_TARGET_PERCENTAGE=50

# Logging
LOG_LEVEL=info
LOG_FILE=/var/log/wpgc/backend.log
EOF

# Create frontend environment file
sudo tee /var/www/admin.wpgc.church/.env.production << 'EOF'
# Admin Dashboard Production
NEXT_PUBLIC_API_URL=https://api.wpgc.church
NEXT_PUBLIC_WS_URL=wss://api.wpgc.church/ws
NODE_ENV=production
EOF
```

### Step 5: Create Systemd Services

#### Backend Service
```bash
sudo tee /etc/systemd/system/wpgc-backend.service << 'EOF'
[Unit]
Description=WPGC Backend API
After=network.target minio.service
Wants=minio.service

[Service]
Type=simple
User=gaius
Group=gaius
WorkingDirectory=/opt/wpgc/backend
ExecStart=/opt/wpgc/backend/sermon-uploader-prod
Restart=always
RestartSec=5
Environment=ENV=production

# Pi Optimizations
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
```

#### Admin Dashboard Service
```bash
sudo tee /etc/systemd/system/wpgc-admin.service << 'EOF'
[Unit]
Description=WPGC Admin Dashboard
After=network.target
Requires=wpgc-backend.service

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/var/www/admin.wpgc.church
ExecStart=/usr/bin/npm start
Restart=always
RestartSec=5
Environment=NODE_ENV=production
Environment=PORT=3000

[Install]
WantedBy=multi-user.target
EOF
```

### Step 6: Configure Nginx

```bash
# Create Nginx configuration for both domains
sudo tee /etc/nginx/sites-available/wpgc-platform << 'EOF'
# Admin Dashboard - admin.wpgc.church
server {
    listen 80;
    server_name admin.wpgc.church;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name admin.wpgc.church;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/admin.wpgc.church/fullchain.pem;
    ssl_private_key /etc/letsencrypt/live/admin.wpgc.church/privkey.pem;

    # Admin Dashboard (Next.js)
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}

# API Backend - api.wpgc.church  
server {
    listen 80;
    server_name api.wpgc.church;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.wpgc.church;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/api.wpgc.church/fullchain.pem;
    ssl_private_key /etc/letsencrypt/live/api.wpgc.church/privkey.pem;

    # Backend API (Go)
    location / {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Large file uploads
        client_max_body_size 10G;
        proxy_request_buffering off;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }

    # WebSocket support
    location /ws {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
EOF

# Enable the site
sudo ln -sf /etc/nginx/sites-available/wpgc-platform /etc/nginx/sites-enabled/

# Remove default site if exists
sudo rm -f /etc/nginx/sites-enabled/default

# Test nginx configuration
sudo nginx -t
```

### Step 7: Setup SSL Certificates
```bash
# Install certbot if not already installed
sudo apt-get update && sudo apt-get install -y certbot python3-certbot-nginx

# Get SSL certificates for both domains
sudo certbot --nginx -d admin.wpgc.church -d api.wpgc.church

# Setup automatic renewal
sudo systemctl enable certbot.timer
```

### Step 8: Create Log Directories
```bash
# Create log directories
sudo mkdir -p /var/log/wpgc
sudo chown gaius:gaius /var/log/wpgc

# Setup log rotation
sudo tee /etc/logrotate.d/wpgc << 'EOF'
/var/log/wpgc/*.log {
    daily
    missingok
    rotate 7
    compress
    notifempty
    create 644 gaius gaius
    postrotate
        systemctl reload wpgc-backend || true
    endscript
}
EOF
```

### Step 9: Start All Services
```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services to start on boot
sudo systemctl enable wpgc-backend
sudo systemctl enable wpgc-admin

# Start services
sudo systemctl start wpgc-backend
sudo systemctl start wpgc-admin
sudo systemctl restart nginx

# Check status
sudo systemctl status wpgc-backend
sudo systemctl status wpgc-admin
sudo systemctl status nginx
```

## 🌐 DNS Configuration

Update your DNS records to point to your Pi IP (192.168.1.127):

```
A    admin.wpgc.church    192.168.1.127
A    api.wpgc.church      192.168.1.127
```

## ✅ Testing Deployment

After deployment, test these URLs:

1. **Admin Dashboard**: https://admin.wpgc.church
   - Dashboard: https://admin.wpgc.church/
   - Upload: https://admin.wpgc.church/sermons/upload
   - Sermons: https://admin.wpgc.church/sermons

2. **API Backend**: https://api.wpgc.church
   - Health: https://api.wpgc.church/api/health
   - Upload: https://api.wpgc.church/api/uploads/sermon
   - WebSocket: wss://api.wpgc.church/ws

## 🔧 Troubleshooting Commands

```bash
# Check service logs
sudo journalctl -u wpgc-backend -f
sudo journalctl -u wpgc-admin -f

# Check nginx logs
sudo tail -f /var/log/nginx/error.log
sudo tail -f /var/log/nginx/access.log

# Restart services
sudo systemctl restart wpgc-backend
sudo systemctl restart wpgc-admin
sudo systemctl reload nginx

# Check ports
sudo netstat -tulpn | grep -E ":(3000|8000|9000)"
```

## 🎉 Success!

Once deployed, you'll have:
- ✅ Professional admin dashboard at admin.wpgc.church
- ✅ Full API backend at api.wpgc.church  
- ✅ Your optimized upload process (unchanged!)
- ✅ All new admin pages (sermons, media, events, etc.)
- ✅ HTTPS encryption with auto-renewal
- ✅ Automatic service restart on reboot