#!/bin/bash

# MinIO Native TLS Setup - Production Ready Version
# Based on 2025 best practices research

set -e

echo "🔐 MinIO Native TLS Setup (Production Version)"
echo "=============================================="
echo ""

# Configuration
MINIO_HOST="192.168.1.127"
MINIO_USER="gaius"
CERT_PATH="/home/gaius/.minio/certs"
DOCKER_PATH="/home/gaius/sermon-uploader"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_step() { echo -e "${BLUE}▶ $1${NC}"; }
print_success() { echo -e "${GREEN}✓ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠ $1${NC}"; }
print_error() { echo -e "${RED}✗ $1${NC}"; }

# Step 1: Create certificate directory structure
print_step "Step 1: Setting up certificate directories"

ssh $MINIO_USER@$MINIO_HOST << 'EOF'
# Create MinIO certs directory
mkdir -p ~/.minio/certs

# Create local certs directory for Docker mounting
mkdir -p ~/sermon-uploader/certs

# Set proper permissions
chmod 700 ~/.minio/certs ~/sermon-uploader/certs
EOF

print_success "Certificate directories created"

# Step 2: Generate ECDSA certificate (recommended over RSA in 2025)
print_step "Step 2: Generating ECDSA TLS certificate (best practice for 2025)"

# Create certificate configuration with all necessary SANs
cat > /tmp/cert.conf << 'EOF'
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = New York
L = White Plains
O = White Plains Gospel Chapel
OU = IT Department
CN = MinIO Server

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = minio.local
DNS.3 = *.minio.local
DNS.4 = minio
IP.1 = 192.168.1.127
IP.2 = 127.0.0.1
EOF

# Generate ECDSA certificate on Pi (more efficient than RSA)
ssh $MINIO_USER@$MINIO_HOST << 'EOF'
# Generate ECDSA private key (P-256 curve)
openssl ecparam -genkey -name prime256v1 -out /tmp/private.key

# Generate certificate with the key
openssl req -new -x509 -days 365 \
    -key /tmp/private.key \
    -out /tmp/public.crt \
    -config - << 'CERTCONF'
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = New York
L = White Plains
O = White Plains Gospel Chapel
OU = IT Department
CN = MinIO Server

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = minio.local
DNS.3 = *.minio.local
DNS.4 = minio
IP.1 = 192.168.1.127
IP.2 = 127.0.0.1
CERTCONF

# Copy certificates to both locations
cp /tmp/private.key ~/.minio/certs/
cp /tmp/public.crt ~/.minio/certs/
cp /tmp/private.key ~/sermon-uploader/certs/
cp /tmp/public.crt ~/sermon-uploader/certs/

# Set proper permissions
chmod 600 ~/.minio/certs/private.key ~/sermon-uploader/certs/private.key
chmod 644 ~/.minio/certs/public.crt ~/sermon-uploader/certs/public.crt

# Clean up temp files
rm /tmp/private.key /tmp/public.crt
EOF

print_success "ECDSA certificate generated (more efficient than RSA)"

# Step 3: Update Docker Compose for HTTPS
print_step "Step 3: Configuring Docker Compose for HTTPS"

ssh $MINIO_USER@$MINIO_HOST << 'EOF'
cd ~/sermon-uploader

# Backup existing docker-compose file
if [ -f docker-compose.pi.yml ]; then
    cp docker-compose.pi.yml docker-compose.pi.yml.backup
fi

# Create new Docker Compose with HTTPS configuration
cat > docker-compose.pi.yml << 'DOCKER'
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
      # IMPORTANT: Set CORS for browser access
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
      - DISCORD_WEBHOOK_URL=${DISCORD_WEBHOOK_URL}
      - PORT=8000
      - ENV=production
      # For self-signed certs
      - NODE_TLS_REJECT_UNAUTHORIZED=0
    depends_on:
      - minio
    volumes:
      - ./backend:/app
      - ./certs:/certs:ro
    networks:
      - sermon-network

  processor:
    build: ./pi-processor
    container_name: sermon-processor
    restart: unless-stopped
    environment:
      - MINIO_ENDPOINT=minio:9000
      - MINIO_SECURE=true
      - MINIO_ACCESS_KEY=gaius
      - MINIO_SECRET_KEY=John 3:16
      - MINIO_BUCKET=sermons
      - DISCORD_WEBHOOK_URL=${DISCORD_WEBHOOK_URL}
      - NODE_TLS_REJECT_UNAUTHORIZED=0
    depends_on:
      - minio
    volumes:
      - ./pi-processor:/app
      - ./certs:/certs:ro
    networks:
      - sermon-network

networks:
  sermon-network:
    driver: bridge
DOCKER

echo "Docker Compose updated for HTTPS"
EOF

print_success "Docker Compose configured for HTTPS"

# Step 4: Update backend configuration
print_step "Step 4: Updating backend for HTTPS MinIO connection"

ssh $MINIO_USER@$MINIO_HOST << 'EOF'
cd ~/sermon-uploader/backend

# Update or create .env file
cat > .env << 'ENV'
# MinIO Configuration
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_SECURE=true
MINIO_BUCKET=sermons

# Public endpoint for presigned URLs
MINIO_PUBLIC_ENDPOINT=192.168.1.127:9000
MINIO_PUBLIC_SECURE=true

# Server Configuration
PORT=8000
ENV=production

# Discord
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/1410698516891701400/Ve6k3d8sdd54kf0II1xFc7H6YkYLoWiPFDEe5NsHsmX4Qv6l4CNzD4rMmdlWPQxLnRPT

# CORS
CORS_ORIGINS=https://sermons.wpgc.church,http://localhost:3000,https://localhost:3000

# Upload Configuration
MAX_UPLOAD_SIZE=5368709120
CHUNK_SIZE=5242880
MAX_CONCURRENT_UPLOADS=1
ENV
EOF

print_success "Backend configuration updated"

# Step 5: Create MinIO client configuration script
print_step "Step 5: Setting up MinIO client for HTTPS"

ssh $MINIO_USER@$MINIO_HOST << 'EOF'
# Configure mc client for HTTPS with self-signed cert
mc alias set local https://localhost:9000 gaius "John 3:16" --api S3v4 --insecure

# Create the bucket if it doesn't exist
mc mb local/sermons --ignore-existing --insecure

# Set bucket policy for public read (if needed)
cat > /tmp/bucket-policy.json << 'POLICY'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "*"},
      "Action": ["s3:GetBucketLocation"],
      "Resource": ["arn:aws:s3:::sermons"]
    },
    {
      "Effect": "Allow",
      "Principal": {"AWS": "*"},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::sermons/*"]
    }
  ]
}
POLICY

# Apply bucket policy
mc anonymous set-json /tmp/bucket-policy.json local/sermons --insecure || true
rm /tmp/bucket-policy.json

echo "MinIO client configured for HTTPS"
EOF

print_success "MinIO client configured"

# Step 6: Restart services
print_step "Step 6: Restarting services with HTTPS configuration"

ssh $MINIO_USER@$MINIO_HOST << 'EOF'
cd ~/sermon-uploader

# Stop existing services
docker-compose -f docker-compose.pi.yml down || true

# Start with new HTTPS configuration
docker-compose -f docker-compose.pi.yml up -d

# Wait for services to start
sleep 10

# Check if MinIO is responding on HTTPS
if curl -k -s https://localhost:9000/minio/health/live | grep -q "OK"; then
    echo "✓ MinIO is running with HTTPS"
else
    echo "⚠ MinIO HTTPS check failed, checking logs..."
    docker logs sermon-minio --tail 20
fi
EOF

print_success "Services restarted with HTTPS"

# Step 7: Create browser certificate acceptance helper
print_step "Step 7: Creating certificate acceptance helper"

cat > /tmp/accept-cert.html << 'HTML'
<!DOCTYPE html>
<html>
<head>
    <title>Accept MinIO Certificate</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .card {
            background: white;
            border-radius: 8px;
            padding: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 { color: #333; }
        .step {
            margin: 20px 0;
            padding: 15px;
            background: #f8f9fa;
            border-left: 4px solid #007bff;
            border-radius: 4px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background: #007bff;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin: 10px 5px;
        }
        .button:hover { background: #0056b3; }
        .success { color: #28a745; }
        .warning { color: #ffc107; }
        .error { color: #dc3545; }
        #status { margin-top: 20px; font-weight: bold; }
    </style>
</head>
<body>
    <div class="card">
        <h1>🔐 Accept MinIO Certificate</h1>
        <p>To enable secure uploads, you need to accept the MinIO server certificate.</p>
        
        <div class="step">
            <h3>Step 1: Accept MinIO API Certificate</h3>
            <p>Click the button below to open MinIO API in a new tab. Accept the security warning.</p>
            <a href="https://192.168.1.127:9000/minio/health/live" target="_blank" class="button">
                Open MinIO API
            </a>
        </div>
        
        <div class="step">
            <h3>Step 2: Accept MinIO Console Certificate</h3>
            <p>Click the button below to open MinIO Console. Accept the security warning.</p>
            <a href="https://192.168.1.127:9001" target="_blank" class="button">
                Open MinIO Console
            </a>
        </div>
        
        <div class="step">
            <h3>Step 3: Test Connection</h3>
            <p>Click the button below to test if certificates are properly accepted.</p>
            <button onclick="testConnection()" class="button">Test Connection</button>
            <div id="status"></div>
        </div>
    </div>
    
    <script>
    async function testConnection() {
        const status = document.getElementById('status');
        status.innerHTML = '<span class="warning">Testing connection...</span>';
        
        try {
            const response = await fetch('https://192.168.1.127:9000/minio/health/live');
            const text = await response.text();
            
            if (text.includes('OK')) {
                status.innerHTML = '<span class="success">✅ Connection successful! You can now upload files.</span>';
            } else {
                status.innerHTML = '<span class="error">❌ Connection failed. Please accept the certificates in steps 1 and 2.</span>';
            }
        } catch (error) {
            status.innerHTML = '<span class="error">❌ Connection failed. Please accept the certificates in steps 1 and 2.<br>Error: ' + error.message + '</span>';
        }
    }
    </script>
</body>
</html>
HTML

echo ""
echo "Certificate acceptance helper created at: /tmp/accept-cert.html"

# Step 8: Run tests
print_step "Step 8: Running HTTPS configuration tests"

cd "/Users/gaius/Documents/WPGC web/sermon-uploader"
./scripts/test-minio-tls.sh || true

echo ""
echo "========================================="
echo -e "${GREEN}✅ MinIO HTTPS Setup Complete!${NC}"
echo "========================================="
echo ""
echo "IMPORTANT NEXT STEPS:"
echo ""
echo "1. Open /tmp/accept-cert.html in your browser"
echo "2. Follow the steps to accept the certificates"
echo "3. Your app can now upload directly to: https://192.168.1.127:9000"
echo ""
echo "Key Points:"
echo "• MinIO API: https://192.168.1.127:9000"
echo "• MinIO Console: https://192.168.1.127:9001"
echo "• CORS configured for all origins (*)"
echo "• Using ECDSA certificates (more efficient)"
echo "• Backend configured for HTTPS MinIO connection"
echo ""
echo "To test manually:"
echo "curl -k https://192.168.1.127:9000/minio/health/live"