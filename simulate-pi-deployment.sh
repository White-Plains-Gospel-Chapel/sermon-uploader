#!/bin/bash
set -e  # Exit on any error

echo "=== SIMULATING EXACT GITHUB ACTIONS DEPLOYMENT TO PI ==="
echo "This script replicates what happens on your Raspberry Pi during deployment"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Simulate environment variables from GitHub Actions
export REGISTRY=docker.io
export DOCKER_USERNAME=gaiusr
export BACKEND_IMAGE=sermon-uploader-backend
export FRONTEND_IMAGE=sermon-uploader-frontend

echo "Step 1: 🔍 System Information (same as GitHub Actions)"
echo "- Hostname: $(hostname)"
echo "- OS: $(uname -a)"
echo "- Docker Version: $(docker --version)"
echo "- Docker Compose Version: $(docker compose version || docker-compose --version)"
echo "- Current Directory: $(pwd)"
echo "- Disk Space: $(df -h / | tail -1)"
echo ""

echo "Step 2: 🐳 Pull latest Docker images"
echo "Pulling backend image..."
docker pull ${REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:pi5
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to pull backend image${NC}"
    exit 1
fi

echo "Pulling frontend image..."
docker pull ${REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:pi5
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to pull frontend image${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Docker images pulled successfully!${NC}"
echo ""

echo "Step 3: 🔄 Stop existing containers"
if [ -f docker-compose.pi5.yml ]; then
    docker compose -f docker-compose.pi5.yml down || true
else
    echo -e "${RED}❌ docker-compose.pi5.yml not found!${NC}"
    exit 1
fi
echo ""

echo "Step 4: 🚀 Start new containers"
echo "Starting containers with docker-compose..."
docker compose -f docker-compose.pi5.yml up -d
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to start containers${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Containers started successfully!${NC}"
echo ""

echo "Step 5: ⏳ Wait for services to be healthy (EXACTLY like GitHub Actions)"
echo "Waiting for services to be healthy..."
sleep 10

# Check backend health - EXACTLY as in the workflow
echo "Checking backend health at http://localhost:8000/api/health..."
for i in {1..30}; do
    if curl -f http://localhost:8000/api/health 2>/dev/null; then
        echo ""
        echo -e "${GREEN}✅ Backend is healthy!${NC}"
        break
    fi
    echo "Waiting for backend... ($i/30)"
    sleep 2
    if [ $i -eq 30 ]; then
        echo -e "${RED}❌ Backend health check failed after 30 attempts${NC}"
        echo "Backend logs:"
        docker logs sermon-uploader-backend --tail 50
        exit 1
    fi
done

# Check frontend - EXACTLY as in the workflow
echo "Checking frontend at http://localhost:3000..."
for i in {1..30}; do
    if curl -f http://localhost:3000 2>/dev/null; then
        echo ""
        echo -e "${GREEN}✅ Frontend is accessible!${NC}"
        break
    fi
    echo "Waiting for frontend... ($i/30)"
    sleep 2
    if [ $i -eq 30 ]; then
        echo -e "${RED}❌ Frontend check failed after 30 attempts${NC}"
        echo "Frontend logs:"
        docker logs sermon-uploader-frontend --tail 50
        exit 1
    fi
done
echo ""

echo "Step 6: 📊 Show running containers"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""

echo "Step 7: 🧹 Clean up old images"
echo "Cleaning up old Docker images..."
docker image prune -f
echo -e "${GREEN}✅ Cleanup complete!${NC}"
echo ""

echo "Step 8: 📝 Deployment Summary"
echo -e "${GREEN}## ✅ Deployment Successful!${NC}"
echo ""
echo "### 🌐 Access URLs:"
echo "- Frontend: http://localhost:3000"
echo "- Backend API: http://localhost:8000"
echo "- MinIO Console: http://localhost:9001"
echo ""
echo "### 📅 Deployment Time:"
echo "- Timestamp: $(date '+%Y-%m-%d %H:%M:%S %Z')"
echo ""

echo -e "${GREEN}=== SIMULATION COMPLETE ===${NC}"
echo "This is EXACTLY what will happen on your Raspberry Pi during GitHub Actions deployment."