#!/bin/bash

# Run this script directly on your Raspberry Pi
# This sets up MinIO with HTTPS for secure browser uploads

set -e

echo "🔐 MinIO HTTPS Setup for Sermon Uploader"
echo "========================================"
echo ""

# Step 1: Create certificate directories
echo "Step 1: Creating certificate directories..."
mkdir -p ~/.minio/certs
mkdir -p ~/sermon-uploader/certs
chmod 700 ~/.minio/certs ~/sermon-uploader/certs

# Step 2: Generate ECDSA certificate (more efficient for Pi)
echo "Step 2: Generating ECDSA TLS certificate..."

# Generate ECDSA private key
openssl ecparam -genkey -name prime256v1 -out ~/.minio/certs/private.key

# Generate self-signed certificate
openssl req -new -x509 -days 365 \
    -key ~/.minio/certs/private.key \
    -out ~/.minio/certs/public.crt \
    -subj "/C=US/ST=NY/L=White Plains/O=WPGC/CN=MinIO" \
    -addext "subjectAltName=IP:192.168.1.127,IP:127.0.0.1,DNS:localhost,DNS:minio.local"

# Copy to Docker mount location
cp ~/.minio/certs/private.key ~/sermon-uploader/certs/
cp ~/.minio/certs/public.crt ~/sermon-uploader/certs/

# Set permissions
chmod 600 ~/.minio/certs/private.key ~/sermon-uploader/certs/private.key
chmod 644 ~/.minio/certs/public.crt ~/sermon-uploader/certs/public.crt

echo "✓ Certificates generated"

# Step 3: Update Docker Compose
echo "Step 3: Updating Docker Compose configuration..."

cd ~/sermon-uploader

# Backup existing file
cp docker-compose.pi.yml docker-compose.pi.yml.backup 2>/dev/null || true

# Create new configuration
cat > docker-compose.pi.yml << 'EOF'
version: '3.8'

services:
  minio:
    image: minio/minio:latest
    container_name: sermon-minio
    restart: unless-stopped
    ports:
      - "9000:9000"  # HTTPS API
      - "9001:9001"  # HTTPS Console
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
    volumes:
      - ./backend:/app
EOF

echo "✓ Docker Compose updated"

# Step 4: Update backend .env
echo "Step 4: Configuring backend for HTTPS..."

cat > backend/.env << 'EOF'
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

echo "✓ Backend configured"

# Step 5: Restart services
echo "Step 5: Restarting services..."

docker-compose -f docker-compose.pi.yml down
docker-compose -f docker-compose.pi.yml up -d

echo "Waiting for services to start..."
sleep 15

# Step 6: Configure MinIO client
echo "Step 6: Configuring MinIO client..."

# Install mc if not present
if ! command -v mc &> /dev/null; then
    wget https://dl.min.io/client/mc/release/linux-arm/mc
    chmod +x mc
    sudo mv mc /usr/local/bin/
fi

# Configure mc for HTTPS
mc alias set local https://localhost:9000 gaius "John 3:16" --insecure

# Create bucket
mc mb local/sermons --ignore-existing --insecure

echo "✓ MinIO client configured"

# Step 7: Test HTTPS
echo "Step 7: Testing HTTPS configuration..."

if curl -k -s https://localhost:9000/minio/health/live | grep -q "OK"; then
    echo "✅ MinIO HTTPS is working!"
else
    echo "❌ MinIO HTTPS test failed"
    docker logs sermon-minio --tail 20
fi

echo ""
echo "========================================"
echo "✅ Setup Complete!"
echo "========================================"
echo ""
echo "MinIO is now running with HTTPS:"
echo "  API:     https://192.168.1.127:9000"
echo "  Console: https://192.168.1.127:9001"
echo ""
echo "Next steps:"
echo "1. Open https://192.168.1.127:9000 in your browser"
echo "2. Accept the security certificate warning"
echo "3. Your frontend can now upload to HTTPS endpoints"
echo ""
echo "To test: curl -k https://192.168.1.127:9000/minio/health/live"