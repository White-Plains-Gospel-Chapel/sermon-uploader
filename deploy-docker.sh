#!/bin/bash

# Deploy sermon-uploader with Docker on Raspberry Pi
# Run this script on the Pi: ./deploy-docker.sh

set -e

echo "🚀 Starting Docker deployment for sermon-uploader..."

# Navigate to project directory
cd /opt/sermon-uploader

# Pull latest changes
echo "📥 Pulling latest code from GitHub..."
git pull origin master

# Stop existing containers
echo "🛑 Stopping existing containers..."
docker compose down 2>/dev/null || true

# Clean up old images
echo "🧹 Cleaning up old images..."
docker system prune -f

# Build images
echo "🔨 Building Docker images..."
docker compose build --no-cache

# Start containers
echo "🚀 Starting containers..."
docker compose up -d

# Wait for services to be ready
echo "⏳ Waiting for services to start..."
sleep 10

# Check container status
echo "📊 Container status:"
docker compose ps

# Check if services are responding
echo "🔍 Checking service health..."
echo -n "Backend API: "
curl -s http://localhost:8000/api/health > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"

echo -n "Frontend: "
curl -s http://localhost:3000 > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"

# Show container logs
echo ""
echo "📋 Recent logs:"
docker compose logs --tail=20

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Access the services at:"
echo "  - Admin Dashboard: https://admin.wpgc.church"
echo "  - API: https://api.wpgc.church"
echo ""
echo "To view logs: docker compose logs -f"
echo "To stop: docker compose down"