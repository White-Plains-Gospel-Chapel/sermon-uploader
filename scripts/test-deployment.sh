#!/bin/bash

# Test deployment script - verifies everything works before production deployment

set -e

echo "🧪 Starting Test Deployment"
echo "=========================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check service health
check_health() {
    local service=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    echo -n "Checking $service health"
    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$url" > /dev/null 2>&1; then
            echo -e " ${GREEN}✅ Healthy${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    echo -e " ${RED}❌ Failed${NC}"
    return 1
}

# Function to cleanup test containers
cleanup() {
    echo "🧹 Cleaning up test containers..."
    docker compose -f docker-compose.test.yml down 2>/dev/null || true
    docker rm -f test-sermon-backend test-sermon-frontend 2>/dev/null || true
}

# Cleanup any existing test containers
cleanup

echo ""
echo "📦 Pulling latest images..."
docker pull gaiusr/sermon-uploader-backend:pi5
docker pull gaiusr/sermon-uploader-frontend:pi5

echo ""
echo "🚀 Starting test deployment..."
docker compose -f docker-compose.test.yml up -d

echo ""
echo "⏳ Waiting for services to be ready..."
sleep 10

echo ""
echo "🔍 Checking service health..."

# Check backend health
if check_health "Backend" "http://localhost:8001/api/health"; then
    BACKEND_HEALTHY=true
else
    BACKEND_HEALTHY=false
    echo "Checking backend logs..."
    docker logs test-sermon-backend --tail 20
fi

# Check frontend health
if check_health "Frontend" "http://localhost:3001"; then
    FRONTEND_HEALTHY=true
else
    FRONTEND_HEALTHY=false
    echo "Checking frontend logs..."
    docker logs test-sermon-frontend --tail 20
fi

echo ""
echo "📊 Container Status:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep test-sermon || true

echo ""
echo "🔬 Testing Backend API endpoints..."
if [ "$BACKEND_HEALTHY" = true ]; then
    # Test version endpoint
    echo -n "  /api/version: "
    if curl -s http://localhost:8001/api/version | grep -q "version"; then
        echo -e "${GREEN}✅${NC}"
    else
        echo -e "${RED}❌${NC}"
    fi
    
    # Test health endpoint details
    echo -n "  /api/health: "
    HEALTH_RESPONSE=$(curl -s http://localhost:8001/api/health)
    if echo "$HEALTH_RESPONSE" | grep -q "healthy"; then
        echo -e "${GREEN}✅${NC}"
        echo "    Response: $HEALTH_RESPONSE"
    else
        echo -e "${RED}❌${NC}"
        echo "    Response: $HEALTH_RESPONSE"
    fi
fi

echo ""
echo "=================================="
if [ "$BACKEND_HEALTHY" = true ] && [ "$FRONTEND_HEALTHY" = true ]; then
    echo -e "${GREEN}✅ Test Deployment Successful!${NC}"
    echo ""
    echo "Services are running at:"
    echo "  Backend:  http://localhost:8001"
    echo "  Frontend: http://localhost:3001"
    echo ""
    echo "To stop test containers: docker compose -f docker-compose.test.yml down"
    echo ""
    echo -e "${GREEN}Ready to deploy to production!${NC}"
    exit 0
else
    echo -e "${RED}❌ Test Deployment Failed!${NC}"
    echo ""
    echo "Issues detected:"
    [ "$BACKEND_HEALTHY" = false ] && echo "  - Backend is not healthy"
    [ "$FRONTEND_HEALTHY" = false ] && echo "  - Frontend is not healthy"
    echo ""
    echo "Please fix the issues before deploying to production."
    echo ""
    echo "Keeping test containers running for debugging."
    echo "To stop: docker compose -f docker-compose.test.yml down"
    exit 1
fi