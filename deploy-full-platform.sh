#!/bin/bash

# WPGC Full Platform Deployment Script
# Deploys both admin dashboard and backend to Raspberry Pi

set -e  # Exit on any error

# Configuration
PI_HOST="192.168.1.127"
PI_USER="gaius"
PROJECT_ROOT="/Users/gaius/Documents/WPGC web/sermon-uploader"
TEMP_DIR="/tmp/wpgc-deploy"

echo "🚀 WPGC Full Platform Deployment"
echo "================================"
echo "Target: $PI_USER@$PI_HOST"
echo "Project: $PROJECT_ROOT"
echo ""

# Cleanup any existing temp files
rm -rf "$TEMP_DIR"
mkdir -p "$TEMP_DIR"

# Step 1: Build and package backend
echo "📦 Building backend..."
cd "$PROJECT_ROOT/backend"

# Check if Go binary exists, build if needed
if [ ! -f "sermon-uploader-fast" ] || [ "$1" == "--rebuild" ]; then
    echo "Building Go binary..."
    go build -ldflags="-s -w" -o sermon-uploader-fast
fi

# Create backend package
echo "Packaging backend..."
tar -czf "$TEMP_DIR/wpgc-backend.tar.gz" \
    --exclude="*.log" \
    --exclude=".git" \
    --exclude="logs/*" \
    .

echo "✅ Backend package created ($(du -h "$TEMP_DIR/wpgc-backend.tar.gz" | cut -f1))"

# Step 2: Build and package frontend
echo "📦 Building admin dashboard..."
cd "$PROJECT_ROOT/frontend-react"

# Clean build
rm -rf .next

# Build for production
npm run build

# Create frontend package
echo "Packaging admin dashboard..."
tar -czf "$TEMP_DIR/wpgc-admin.tar.gz" \
    .next package.json next.config.js public components app lib tailwind.config.ts tsconfig.json

echo "✅ Admin dashboard package created ($(du -h "$TEMP_DIR/wpgc-admin.tar.gz" | cut -f1))"

# Step 3: Upload packages to Pi
echo "🚀 Uploading packages to Pi..."
scp "$TEMP_DIR/wpgc-backend.tar.gz" "$PI_USER@$PI_HOST:/tmp/"
scp "$TEMP_DIR/wpgc-admin.tar.gz" "$PI_USER@$PI_HOST:/tmp/"

echo "✅ Packages uploaded successfully"

# Step 4: Deploy on Pi
echo "🔧 Deploying on Pi..."

ssh "$PI_USER@$PI_HOST" << 'ENDSSH'
set -e

echo "Setting up directories..."
sudo mkdir -p /opt/wpgc/backend
sudo mkdir -p /var/www/admin.wpgc.church
sudo mkdir -p /var/log/wpgc

echo "Deploying backend..."
cd /opt/wpgc
sudo rm -rf backend.old
[ -d backend ] && sudo mv backend backend.old
sudo mkdir -p backend
cd backend
sudo tar -xzf /tmp/wpgc-backend.tar.gz
sudo chown -R gaius:gaius /opt/wpgc/backend
sudo chmod +x /opt/wpgc/backend/sermon-uploader-fast

echo "Deploying admin dashboard..."
cd /var/www/admin.wpgc.church
sudo tar -xzf /tmp/wpgc-admin.tar.gz
sudo chown -R www-data:www-data /var/www/admin.wpgc.church

# Install/update npm dependencies
sudo -u www-data npm install --production --silent

echo "✅ Deployment complete"
ENDSSH

# Step 5: Create/update services if they don't exist
echo "⚙️ Setting up services..."

ssh "$PI_USER@$PI_HOST" << 'ENDSSH'
# Create backend service if it doesn't exist
if [ ! -f /etc/systemd/system/wpgc-backend.service ]; then
    echo "Creating backend service..."
    sudo tee /etc/systemd/system/wpgc-backend.service > /dev/null << 'EOF'
[Unit]
Description=WPGC Backend API
After=network.target
Wants=network.target

[Service]
Type=simple
User=gaius
Group=gaius
WorkingDirectory=/opt/wpgc/backend
ExecStart=/opt/wpgc/backend/sermon-uploader-fast
Restart=always
RestartSec=5
Environment=ENV=production
Environment=PORT=8000

# Pi Optimizations
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
fi

# Create admin service if it doesn't exist
if [ ! -f /etc/systemd/system/wpgc-admin.service ]; then
    echo "Creating admin service..."
    sudo tee /etc/systemd/system/wpgc-admin.service > /dev/null << 'EOF'
[Unit]
Description=WPGC Admin Dashboard
After=network.target wpgc-backend.service
Wants=wpgc-backend.service

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
fi

# Reload systemd and enable services
sudo systemctl daemon-reload
sudo systemctl enable wpgc-backend
sudo systemctl enable wpgc-admin

echo "✅ Services configured"
ENDSSH

# Step 6: Restart services
echo "🔄 Restarting services..."

ssh "$PI_USER@$PI_HOST" << 'ENDSSH'
# Restart backend
echo "Restarting backend..."
sudo systemctl restart wpgc-backend
sleep 2

# Restart admin dashboard  
echo "Restarting admin dashboard..."
sudo systemctl restart wpgc-admin
sleep 2

# Check service status
echo "Checking service status..."
if sudo systemctl is-active --quiet wpgc-backend; then
    echo "✅ Backend service is running"
else
    echo "❌ Backend service failed to start"
    sudo systemctl status wpgc-backend --no-pager
    exit 1
fi

if sudo systemctl is-active --quiet wpgc-admin; then
    echo "✅ Admin service is running"
else
    echo "❌ Admin service failed to start"
    sudo systemctl status wpgc-admin --no-pager
    exit 1
fi

# Show listening ports
echo "Active services:"
sudo netstat -tulpn | grep -E ":(3000|8000|9000)" | head -5
ENDSSH

# Step 7: Test deployment
echo "🧪 Testing deployment..."

echo "Testing backend API..."
if curl -sf "http://$PI_HOST:8000/api/health" > /dev/null; then
    echo "✅ Backend API is responding"
else
    echo "⚠️ Backend API not responding (might need time to start)"
fi

echo "Testing admin dashboard..."
if curl -sf "http://$PI_HOST:3000" > /dev/null; then
    echo "✅ Admin dashboard is responding"
else
    echo "⚠️ Admin dashboard not responding (might need time to start)"
fi

# Cleanup
rm -rf "$TEMP_DIR"

echo ""
echo "🎉 Deployment Complete!"
echo "========================"
echo ""
echo "Services:"
echo "  • Backend API: http://$PI_HOST:8000"
echo "  • Admin Dashboard: http://$PI_HOST:3000"
echo ""
echo "Next Steps:"
echo "1. Configure your router to forward ports 80/443 to $PI_HOST"
echo "2. Setup DNS records:"
echo "   • admin.wpgc.church → $PI_HOST"  
echo "   • api.wpgc.church → $PI_HOST"
echo "3. Setup SSL with: sudo certbot --nginx -d admin.wpgc.church -d api.wpgc.church"
echo ""
echo "Monitor logs:"
echo "  • Backend: ssh $PI_USER@$PI_HOST 'sudo journalctl -u wpgc-backend -f'"
echo "  • Admin: ssh $PI_USER@$PI_HOST 'sudo journalctl -u wpgc-admin -f'"
ENDSSH