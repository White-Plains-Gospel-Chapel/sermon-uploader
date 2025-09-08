#!/bin/bash

# WPGC Admin Dashboard - Pi Deployment Script
# This script deploys the Next.js admin dashboard to your Raspberry Pi

# Configuration - UPDATE THESE VALUES
PI_HOST="192.168.1.127"  # Your Pi's IP address
PI_USER="gaius"          # SSH username on Pi
DEPLOY_PATH="/home/gaius/wpgc-admin"  # Where to deploy on Pi
NGINX_SITE="admin.wpgc.church"  # Domain name

echo "🚀 WPGC Admin Dashboard Deployment"
echo "=================================="
echo "Target: $PI_USER@$PI_HOST:$DEPLOY_PATH"
echo "Domain: $NGINX_SITE"
echo ""

# Check if build exists
if [ ! -d ".next" ]; then
    echo "❌ Build not found. Running npm run build first..."
    npm run build
    if [ $? -ne 0 ]; then
        echo "❌ Build failed. Please fix build errors first."
        exit 1
    fi
fi

echo "✅ Build found"

# Create deployment package
echo "📦 Creating deployment package..."
tar -czf admin-dashboard.tar.gz .next package.json next.config.* public components app

# Copy to Pi
echo "🚀 Uploading to Pi..."
scp admin-dashboard.tar.gz $PI_USER@$PI_HOST:/tmp/

# Deploy on Pi
echo "🔧 Deploying on Pi..."
ssh $PI_USER@$PI_HOST << 'ENDSSH'
    # Create deployment directory
    sudo mkdir -p /var/www/admin.wpgc.church
    cd /var/www/admin.wpgc.church
    
    # Extract and setup
    sudo tar -xzf /tmp/admin-dashboard.tar.gz
    sudo chown -R www-data:www-data /var/www/admin.wpgc.church
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        sudo -u www-data npm install --production
    fi
    
    echo "✅ Files deployed to /var/www/admin.wpgc.church"
ENDSSH

# Create Nginx configuration
echo "🌐 Creating Nginx configuration..."
cat > nginx-admin-site.conf << 'EOF'
server {
    listen 80;
    server_name admin.wpgc.church;
    
    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name admin.wpgc.church;

    # SSL Configuration (Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/admin.wpgc.church/fullchain.pem;
    ssl_private_key /etc/letsencrypt/live/admin.wpgc.church/privkey.pem;
    
    # Next.js app
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

    # API requests to backend
    location /api {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Handle large uploads
        client_max_body_size 10G;
        proxy_request_buffering off;
    }
}
EOF

# Upload Nginx config
echo "📝 Uploading Nginx configuration..."
scp nginx-admin-site.conf $PI_USER@$PI_HOST:/tmp/

# Configure Nginx on Pi
ssh $PI_USER@$PI_HOST << 'ENDSSH'
    # Install Nginx config
    sudo cp /tmp/nginx-admin-site.conf /etc/nginx/sites-available/admin.wpgc.church
    sudo ln -sf /etc/nginx/sites-available/admin.wpgc.church /etc/nginx/sites-enabled/
    
    # Test Nginx config
    sudo nginx -t
    if [ $? -eq 0 ]; then
        echo "✅ Nginx configuration valid"
    else
        echo "❌ Nginx configuration error"
        exit 1
    fi
ENDSSH

# Create systemd service for the admin dashboard
cat > admin-dashboard.service << 'EOF'
[Unit]
Description=WPGC Admin Dashboard
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/var/www/admin.wpgc.church
ExecStart=/usr/bin/npm start
Restart=on-failure
RestartSec=5
Environment=NODE_ENV=production
Environment=PORT=3000

[Install]
WantedBy=multi-user.target
EOF

# Upload and install systemd service
echo "⚙️ Installing systemd service..."
scp admin-dashboard.service $PI_USER@$PI_HOST:/tmp/

ssh $PI_USER@$PI_HOST << 'ENDSSH'
    # Install systemd service
    sudo cp /tmp/admin-dashboard.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable admin-dashboard
    
    echo "✅ Systemd service installed"
ENDSSH

# Clean up
rm admin-dashboard.tar.gz nginx-admin-site.conf admin-dashboard.service

echo ""
echo "🎉 Deployment Complete!"
echo "========================"
echo ""
echo "Next Steps:"
echo "1. SSH to your Pi: ssh $PI_USER@$PI_HOST"
echo "2. Start the admin dashboard: sudo systemctl start admin-dashboard"
echo "3. Restart Nginx: sudo systemctl restart nginx"
echo "4. Setup SSL with Let's Encrypt:"
echo "   sudo certbot --nginx -d admin.wpgc.church"
echo "5. Point admin.wpgc.church DNS to your Pi IP: $PI_HOST"
echo ""
echo "🌐 Once DNS is setup, visit: https://admin.wpgc.church"
echo "📊 Check status: sudo systemctl status admin-dashboard"
echo "📝 View logs: sudo journalctl -u admin-dashboard -f"