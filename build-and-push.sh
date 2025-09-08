#!/bin/bash

# Build and push Docker images for Raspberry Pi 5
# This builds multi-arch images and pushes to Docker Hub

set -e

# Configuration
DOCKER_REGISTRY=${DOCKER_REGISTRY:-"docker.io"}
DOCKER_USERNAME=${DOCKER_USERNAME:-"wpgcparish"}
IMAGE_PREFIX="sermon-uploader"
VERSION=${VERSION:-"latest"}

echo "🔨 Building multi-architecture Docker images for Raspberry Pi 5..."

# Check if logged into Docker Hub
if ! docker info | grep -q "Username"; then
    echo "📝 Please log in to Docker Hub:"
    docker login
fi

# Enable BuildKit
export DOCKER_BUILDKIT=1

# Create builder for multi-platform builds if not exists
if ! docker buildx ls | grep -q "multiarch"; then
    echo "🔧 Creating multi-platform builder..."
    docker buildx create --name multiarch --driver docker-container --use
    docker buildx inspect --bootstrap
fi

# Build and push backend
echo "🔨 Building backend for ARM64..."
docker buildx build \
    --platform linux/arm64,linux/amd64 \
    --tag ${DOCKER_USERNAME}/${IMAGE_PREFIX}-backend:${VERSION} \
    --tag ${DOCKER_USERNAME}/${IMAGE_PREFIX}-backend:pi5 \
    --push \
    ./backend

# Build and push frontend
echo "🔨 Building frontend for ARM64..."
docker buildx build \
    --platform linux/arm64,linux/amd64 \
    --tag ${DOCKER_USERNAME}/${IMAGE_PREFIX}-frontend:${VERSION} \
    --tag ${DOCKER_USERNAME}/${IMAGE_PREFIX}-frontend:pi5 \
    --push \
    ./frontend-react

echo ""
echo "✅ Images built and pushed successfully!"
echo ""
echo "Images available at:"
echo "  - ${DOCKER_USERNAME}/${IMAGE_PREFIX}-backend:pi5"
echo "  - ${DOCKER_USERNAME}/${IMAGE_PREFIX}-frontend:pi5"
echo ""
echo "To deploy on Pi 5, update docker-compose.pi5.yml to use:"
echo "  backend image: ${DOCKER_USERNAME}/${IMAGE_PREFIX}-backend:pi5"
echo "  frontend image: ${DOCKER_USERNAME}/${IMAGE_PREFIX}-frontend:pi5"