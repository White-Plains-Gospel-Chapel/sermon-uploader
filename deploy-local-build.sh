#!/bin/bash

# Deploy sermon-uploader by building Docker images locally on Pi
# No Docker Hub account required!

set -e

echo "🚀 Deploying sermon-uploader with local Docker build..."

# Navigate to project directory
cd /opt/sermon-uploader || {
    echo "📁 Project directory not found. Cloning..."
    sudo mkdir -p /opt
    cd /opt
    sudo git clone https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git
    sudo chown -R $USER:$USER sermon-uploader
    cd sermon-uploader
}

# Pull latest changes
echo "📥 Pulling latest code..."
git pull origin master

# Stop existing containers
echo "🛑 Stopping existing containers..."
docker compose down 2>/dev/null || true

# Build images locally
echo "🔨 Building Docker images locally (this may take a while)..."
docker compose build --no-cache

# Start services
echo "🚀 Starting services..."
docker compose up -d

# Wait for services
echo "⏳ Waiting for services to start..."
sleep 10

# Check status
echo ""
echo "📊 Container status:"
docker compose ps

# Test endpoints
echo ""
echo "🔍 Testing endpoints..."
echo -n "Backend API: "
curl -s http://localhost:8000/api/health > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"

echo -n "Frontend: "
curl -s http://localhost:3000 > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Services are available at:"
echo "  - Backend API: http://localhost:8000"
echo "  - Frontend: http://localhost:3000"
echo "  - Admin Dashboard: https://admin.wpgc.church"
echo "  - API Endpoint: https://api.wpgc.church"