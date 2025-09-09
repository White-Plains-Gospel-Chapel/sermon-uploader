#!/bin/bash

# Manual script to build and push Docker images for Raspberry Pi 5 (ARM64)
# Use this if the GitHub Actions workflow fails to build proper ARM64 images

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Building Docker images for Raspberry Pi 5 (ARM64)...${NC}"

# Check if docker buildx is available
if ! docker buildx version > /dev/null 2>&1; then
    echo -e "${RED}Docker buildx is not available. Please install it first.${NC}"
    exit 1
fi

# Create or use existing buildx builder
BUILDER_NAME="pi-builder"
if ! docker buildx ls | grep -q "$BUILDER_NAME"; then
    echo -e "${YELLOW}Creating buildx builder: $BUILDER_NAME${NC}"
    docker buildx create --name $BUILDER_NAME --use
    docker buildx inspect --bootstrap
else
    echo -e "${GREEN}Using existing buildx builder: $BUILDER_NAME${NC}"
    docker buildx use $BUILDER_NAME
fi

# Build backend image
echo -e "${BLUE}Building backend image for ARM64...${NC}"
docker buildx build \
    --platform linux/arm64 \
    -t gaiusr/sermon-uploader-backend:pi5 \
    -t gaiusr/sermon-uploader-backend:latest-arm64 \
    --push \
    ./backend

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Backend image built and pushed successfully${NC}"
else
    echo -e "${RED}✗ Backend image build failed${NC}"
    exit 1
fi

# Build frontend image
echo -e "${BLUE}Building frontend image for ARM64...${NC}"
docker buildx build \
    --platform linux/arm64 \
    -t gaiusr/sermon-uploader-frontend:pi5 \
    -t gaiusr/sermon-uploader-frontend:latest-arm64 \
    --push \
    ./frontend-react

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Frontend image built and pushed successfully${NC}"
else
    echo -e "${RED}✗ Frontend image build failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ All images built and pushed successfully!${NC}"
echo ""
echo -e "${CYAN}To deploy on the Pi, SSH in and run:${NC}"
echo "cd /opt/sermon-uploader"
echo "docker compose -f docker-compose.pi5.yml pull"
echo "docker compose -f docker-compose.pi5.yml down"
echo "docker compose -f docker-compose.pi5.yml up -d"