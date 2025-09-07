#!/bin/bash

echo "🌐 Dual-Domain Architecture Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "This will set up:"
echo "📱 Web App: sermon-uploader.wpgcservices.com (via CloudFlare)"
echo "💾 MinIO:   minio.wpgcservices.com (direct, bypasses CloudFlare)"
echo ""

PI_USER="gaius"
PI_HOST="192.168.1.127"

# Step 1: Build backend with public MinIO support
echo "🔧 Building backend with dual-domain support..."
cd backend
GOOS=linux GOARCH=arm64 go build -o sermon-uploader-dual .
if [ $? -ne 0 ]; then
    echo "❌ Backend build failed"
    exit 1
fi
echo "✅ Backend built successfully"

# Step 2: Build frontend with dual-domain configuration
echo "🔧 Building frontend with dual-domain support..."
cd ../frontend

# Create production environment file
cat > .env.production << 'EOF'
NEXT_PUBLIC_CLOUDFLARE_URL=https://sermon-uploader.wpgcservices.com
NEXT_PUBLIC_MINIO_DOMAIN=minio.wpgcservices.com
EOF

npm run build
if [ $? -ne 0 ]; then
    echo "❌ Frontend build failed"
    exit 1
fi
echo "✅ Frontend built successfully"
cd ..

# Step 3: Deploy backend
echo "📤 Deploying backend to Pi..."
scp backend/sermon-uploader-dual $PI_USER@$PI_HOST:/home/$PI_USER/sermon-uploader-dual

# Step 4: Deploy frontend
echo "📤 Deploying frontend to Pi..."
cd frontend
tar -czf out-dual-domain.tar.gz out/
scp out-dual-domain.tar.gz $PI_USER@$PI_HOST:/home/$PI_USER/
rm out-dual-domain.tar.gz
cd ..

# Step 5: Deploy configuration
echo "📤 Deploying configuration..."
scp backend/.env $PI_USER@$PI_HOST:/home/$PI_USER/.env

# Step 6: Setup on Pi
echo "🔧 Setting up services on Pi..."
ssh $PI_USER@$PI_HOST << 'ENDSSH'
# Stop existing services
pkill -f sermon-uploader

# Extract frontend
cd /home/gaius/frontend
rm -rf out
tar -xzf /home/gaius/out-dual-domain.tar.gz
rm /home/gaius/out-dual-domain.tar.gz

# Move backend
chmod +x /home/gaius/sermon-uploader-dual
cp /home/gaius/sermon-uploader-dual /home/gaius/sermon-uploader-current

# Copy environment
cp /home/gaius/.env /home/gaius/backend.env

# Start backend with environment
cd /home/gaius
nohup ./sermon-uploader-current > sermon-uploader.log 2>&1 &
echo "Backend started with PID: $!"

# Check if backend is running
sleep 3
if pgrep -f sermon-uploader-current > /dev/null; then
    echo "✅ Backend is running"
else
    echo "❌ Backend failed to start"
    tail -20 sermon-uploader.log
fi
ENDSSH

echo ""
echo "🧪 Testing deployment..."

# Test backend API
if curl -s -f "http://192.168.1.127:8000/api/health" > /dev/null; then
    echo "✅ Backend API: ONLINE"
else
    echo "❌ Backend API: OFFLINE"
fi

# Test MinIO (local)
if curl -s -f "http://192.168.1.127:9000/minio/health/live" > /dev/null; then
    echo "✅ MinIO (local): ONLINE"
else
    echo "❌ MinIO (local): OFFLINE"
fi

echo ""
echo "🎯 Deployment Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📱 Web App URL:      https://sermon-uploader.wpgcservices.com"
echo "💾 MinIO Direct:     http://minio.wpgcservices.com:9000"
echo "🔧 Backend Local:    http://192.168.1.127:8000"
echo "📊 MinIO Console:    http://minio.wpgcservices.com:9001"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "⚠️  IMPORTANT NEXT STEPS:"
echo "1. Ensure DNS is configured (run ./setup-cloudflare-dns.sh)"
echo "2. Configure router port forwarding:"
echo "   - 8000 → 192.168.1.127:8000 (Backend)"  
echo "   - 9000 → 192.168.1.127:9000 (MinIO)"
echo "   - 9001 → 192.168.1.127:9001 (MinIO Console)"
echo "3. Run MinIO global setup: ./setup-minio-global.sh"
echo "4. Test uploads from https://sermon-uploader.wpgcservices.com"
echo ""
echo "🔍 To troubleshoot:"
echo "ssh $PI_USER@$PI_HOST 'tail -f /home/gaius/sermon-uploader.log'"