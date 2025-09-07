#!/bin/bash

# Run this script ON YOUR RASPBERRY PI to deploy the latest code
# This pulls the latest changes and sets up everything

set -e

echo "🚀 Sermon Uploader Deployment Script"
echo "====================================="
echo "Running on: $(hostname)"
echo "Date: $(date)"
echo ""

# Configuration
REPO_PATH="/home/gaius/sermon-uploader"
GITHUB_REPO="https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Step 1: Pull latest code
echo "📦 Step 1: Pulling latest code from GitHub..."
cd $REPO_PATH

# Stash any local changes
git stash || true

# Pull latest
git pull origin master

echo -e "${GREEN}✓ Code updated${NC}"

# Step 2: Set up TLS certificates if not exists
echo ""
echo "🔐 Step 2: Setting up TLS certificates..."

if [ ! -f "$HOME/.minio/certs/private.key" ]; then
    echo "Generating TLS certificate..."
    
    # Create directories
    mkdir -p ~/.minio/certs
    mkdir -p $REPO_PATH/certs
    
    # Generate ECDSA certificate
    openssl ecparam -genkey -name prime256v1 -out ~/.minio/certs/private.key
    
    # Generate self-signed certificate
    openssl req -new -x509 -days 365 \
        -key ~/.minio/certs/private.key \
        -out ~/.minio/certs/public.crt \
        -subj "/C=US/ST=NY/L=White Plains/O=WPGC/CN=MinIO" \
        -addext "subjectAltName=IP:192.168.1.127,IP:127.0.0.1,DNS:localhost,DNS:minio.local"
    
    # Copy to Docker mount
    cp ~/.minio/certs/private.key $REPO_PATH/certs/
    cp ~/.minio/certs/public.crt $REPO_PATH/certs/
    
    # Set permissions
    chmod 600 ~/.minio/certs/private.key $REPO_PATH/certs/private.key
    chmod 644 ~/.minio/certs/public.crt $REPO_PATH/certs/public.crt
    
    echo -e "${GREEN}✓ TLS certificates generated${NC}"
else
    echo -e "${YELLOW}Certificates already exist${NC}"
fi

# Step 3: Update Docker Compose configuration
echo ""
echo "🐳 Step 3: Updating Docker configuration..."

# Create docker-compose.pi.yml if it doesn't exist or update it
cat > $REPO_PATH/docker-compose.pi.yml << 'EOF'
version: '3.8'

services:
  minio:
    image: minio/minio:latest
    container_name: sermon-minio
    restart: unless-stopped
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=gaius
      - MINIO_ROOT_PASSWORD=John 3:16
      - MINIO_API_CORS_ALLOW_ORIGIN=*
      - MINIO_BROWSER=on
      - MINIO_BROWSER_REDIRECT_URL=https://192.168.1.127:9001
    volumes:
      - ./minio-data:/data
      - ./certs:/root/.minio/certs:ro
    command: server /data --console-address ":9001"
    healthcheck:
      test: ["CMD", "curl", "-f", "-k", "https://localhost:9000/minio/health/live"]
      interval: 30s
      timeout: 20s
      retries: 3
    networks:
      - sermon-network

  backend:
    build: ./backend
    container_name: sermon-backend
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      - MINIO_ENDPOINT=minio:9000
      - MINIO_SECURE=true
      - MINIO_ACCESS_KEY=gaius
      - MINIO_SECRET_KEY=John 3:16
      - MINIO_BUCKET=sermons
      - MINIO_PUBLIC_ENDPOINT=192.168.1.127:9000
      - MINIO_PUBLIC_SECURE=true
      - PORT=8000
      - NODE_TLS_REJECT_UNAUTHORIZED=0
    depends_on:
      - minio
    networks:
      - sermon-network

networks:
  sermon-network:
    driver: bridge
EOF

echo -e "${GREEN}✓ Docker Compose updated${NC}"

# Step 4: Update backend configuration
echo ""
echo "🔧 Step 4: Configuring backend..."

# Create .env file
cat > $REPO_PATH/backend/.env << 'EOF'
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_SECURE=true
MINIO_BUCKET=sermons
MINIO_PUBLIC_ENDPOINT=192.168.1.127:9000
MINIO_PUBLIC_SECURE=true
PORT=8000
ENV=production
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/1410698516891701400/Ve6k3d8sdd54kf0II1xFc7H6YkYLoWiPFDEe5NsHsmX4Qv6l4CNzD4rMmdlWPQxLnRPT
EOF

# Use the updated main.go if it exists
if [ -f $REPO_PATH/backend/main_updated.go ]; then
    mv $REPO_PATH/backend/main.go $REPO_PATH/backend/main.go.original 2>/dev/null || true
    cp $REPO_PATH/backend/main_updated.go $REPO_PATH/backend/main.go
fi

echo -e "${GREEN}✓ Backend configured${NC}"

# Step 5: Build and restart services
echo ""
echo "🚀 Step 5: Building and starting services..."

cd $REPO_PATH

# Stop existing services
docker-compose -f docker-compose.pi.yml down || true

# Build backend
cd backend
go mod tidy
go build -o sermon-backend

# Return to repo root
cd $REPO_PATH

# Start services
docker-compose -f docker-compose.pi.yml up -d

echo -e "${GREEN}✓ Services started${NC}"

# Step 6: Wait for services to be ready
echo ""
echo "⏳ Step 6: Waiting for services to start..."
sleep 10

# Step 7: Configure MinIO client
echo ""
echo "📊 Step 7: Configuring MinIO..."

# Install mc if not present
if ! command -v mc &> /dev/null; then
    wget https://dl.min.io/client/mc/release/linux-arm/mc
    chmod +x mc
    sudo mv mc /usr/local/bin/
fi

# Configure mc
mc alias set local https://localhost:9000 gaius "John 3:16" --insecure

# Create bucket if it doesn't exist
mc mb local/sermons --ignore-existing --insecure

echo -e "${GREEN}✓ MinIO configured${NC}"

# Step 8: Test the deployment
echo ""
echo "🧪 Step 8: Testing deployment..."

# Test MinIO HTTPS
if curl -k -s https://localhost:9000/minio/health/live | grep -q "OK"; then
    echo -e "${GREEN}✅ MinIO HTTPS is working${NC}"
else
    echo -e "${RED}❌ MinIO HTTPS test failed${NC}"
    docker logs sermon-minio --tail 20
fi

# Test backend
if curl -k -s http://localhost:8000/api/health | grep -q "healthy"; then
    echo -e "${GREEN}✅ Backend API is working${NC}"
else
    echo -e "${RED}❌ Backend API test failed${NC}"
    docker logs sermon-backend --tail 20
fi

# Test multipart endpoint
echo ""
echo "Testing multipart upload endpoint..."
RESPONSE=$(curl -k -s -X POST http://localhost:8000/api/upload/multipart/init \
    -H "Content-Type: application/json" \
    -d '{"filename":"test.wav","fileSize":1048576,"fileHash":"test123"}' || echo "Failed")

if echo "$RESPONSE" | grep -q "uploadId"; then
    echo -e "${GREEN}✅ Multipart upload endpoint working${NC}"
else
    echo -e "${RED}❌ Multipart upload endpoint failed${NC}"
    echo "Response: $RESPONSE"
fi

# Step 9: Show status
echo ""
echo "====================================="
echo -e "${GREEN}🎉 Deployment Complete!${NC}"
echo "====================================="
echo ""
echo "Service URLs:"
echo "  MinIO API:     https://192.168.1.127:9000"
echo "  MinIO Console: https://192.168.1.127:9001"
echo "  Backend API:   http://192.168.1.127:8000"
echo ""
echo "Next steps:"
echo "1. Open https://192.168.1.127:9000 in your browser"
echo "2. Accept the security certificate"
echo "3. Test file upload from your frontend"
echo ""

# Show container status
echo "Container Status:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"