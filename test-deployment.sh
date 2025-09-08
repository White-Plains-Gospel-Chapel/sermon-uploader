#!/bin/bash

echo "=== Deployment Test Script ==="
echo "Testing deployment configuration..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if a service is reachable
check_service() {
    local service=$1
    local url=$2
    local max_attempts=30
    local attempt=1
    
    echo -n "Checking $service at $url..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$url" > /dev/null 2>&1; then
            echo -e " ${GREEN}✓${NC}"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo -e " ${RED}✗${NC}"
    return 1
}

# Check if MinIO is required but not running
echo ""
echo "Step 1: Checking MinIO availability..."
if ! lsof -i :9000 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠ MinIO is not running on port 9000${NC}"
    echo "The backend expects MinIO to be available at host.docker.internal:9000"
    echo ""
    echo "Options to fix this:"
    echo "1. Start MinIO locally on port 9000"
    echo "2. Update docker-compose.pi5.yml to include MinIO service"
    echo "3. Make backend work without MinIO for health checks"
    echo ""
else
    echo -e "${GREEN}✓ MinIO is running on port 9000${NC}"
fi

# Test with docker-compose
echo ""
echo "Step 2: Testing docker-compose deployment..."
echo "Stopping any existing containers..."
docker compose -f docker-compose.pi5.yml down 2>/dev/null

echo "Starting containers..."
docker compose -f docker-compose.pi5.yml up -d

echo ""
echo "Step 3: Checking service health..."

# Check backend health
if check_service "Backend" "http://localhost:8000/api/health"; then
    echo -e "${GREEN}Backend is healthy!${NC}"
    
    # Get health response
    health_response=$(curl -s http://localhost:8000/api/health)
    echo "Health response: $health_response"
else
    echo -e "${RED}Backend health check failed!${NC}"
    echo ""
    echo "Backend logs:"
    docker logs sermon-uploader-backend --tail 20
    exit 1
fi

# Check frontend
if check_service "Frontend" "http://localhost:3000"; then
    echo -e "${GREEN}Frontend is accessible!${NC}"
else
    echo -e "${RED}Frontend check failed!${NC}"
    echo ""
    echo "Frontend logs:"
    docker logs sermon-uploader-frontend --tail 20
    exit 1
fi

echo ""
echo -e "${GREEN}=== All checks passed! ===${NC}"
echo "Deployment test successful. Services are healthy."
echo ""
echo "To clean up: docker compose -f docker-compose.pi5.yml down"