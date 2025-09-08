#!/bin/bash

# Optimized deployment script for Raspberry Pi 5
# Uses Docker with multi-architecture support

set -e

echo "🚀 Deploying sermon-uploader on Raspberry Pi 5..."

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" != "aarch64" ]; then
    echo "⚠️  Warning: This script is optimized for ARM64 (detected: $ARCH)"
fi

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Installing..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker $USER
    rm get-docker.sh
    echo "✅ Docker installed. Please log out and back in, then run this script again."
    exit 0
fi

# Enable Docker BuildKit for better performance
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

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
docker compose -f docker-compose.pi5.yml down 2>/dev/null || true

# Prune old data to save space on Pi
echo "🧹 Cleaning up Docker resources..."
docker system prune -af --volumes

# Build with specific platform
echo "🔨 Building images for ARM64..."
docker compose -f docker-compose.pi5.yml build \
    --build-arg BUILDPLATFORM=linux/arm64 \
    --build-arg TARGETPLATFORM=linux/arm64 \
    --build-arg TARGETOS=linux \
    --build-arg TARGETARCH=arm64

# Start services
echo "🚀 Starting services..."
docker compose -f docker-compose.pi5.yml up -d

# Wait for services
echo "⏳ Waiting for services to be healthy..."
timeout 60 bash -c 'until docker compose -f docker-compose.pi5.yml ps | grep -q "healthy"; do sleep 2; done' || true

# Check status
echo ""
echo "📊 Container status:"
docker compose -f docker-compose.pi5.yml ps

# Test endpoints
echo ""
echo "🔍 Testing endpoints..."
sleep 5

echo -n "Backend API: "
if curl -s -f http://localhost:8000/api/health > /dev/null 2>&1; then
    echo "✅ Running"
    curl -s http://localhost:8000/api/health | python3 -m json.tool 2>/dev/null || true
else
    echo "❌ Not responding"
fi

echo -n "Frontend: "
if curl -s -f http://localhost:3000 > /dev/null 2>&1; then
    echo "✅ Running"
else
    echo "❌ Not responding"
fi

# Show resource usage
echo ""
echo "📈 Resource usage:"
docker stats --no-stream

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Services are available at:"
echo "  - Backend API: http://localhost:8000"
echo "  - Frontend: http://localhost:3000"
echo "  - Admin Dashboard: https://admin.wpgc.church"
echo "  - API Endpoint: https://api.wpgc.church"
echo ""
echo "Useful commands:"
echo "  - View logs: docker compose -f docker-compose.pi5.yml logs -f"
echo "  - Stop services: docker compose -f docker-compose.pi5.yml down"
echo "  - Restart services: docker compose -f docker-compose.pi5.yml restart"
echo "  - Check resources: docker stats"