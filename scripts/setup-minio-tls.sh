#!/bin/bash

# Setup script for MinIO Native TLS Configuration
# This implements Option C from our design decisions

set -e

echo "🔐 MinIO Native TLS Setup Script"
echo "================================="
echo ""
echo "This script will configure MinIO with native TLS support for HTTPS access."
echo "No CloudFlare, no proxy - direct HTTPS to MinIO."
echo ""

# Configuration
MINIO_HOST="192.168.1.127"
MINIO_USER="gaius"
CERT_PATH="/home/gaius/.minio/certs"
MINIO_PORT="9000"
MINIO_CONSOLE_PORT="9001"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_step() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Step 1: Create certificate directory
print_step "Step 1: Creating certificate directory on Raspberry Pi"
ssh $MINIO_USER@$MINIO_HOST "mkdir -p $CERT_PATH"
print_success "Certificate directory created"

# Step 2: Generate self-signed certificate
print_step "Step 2: Generating TLS certificate"
echo ""
echo "Choose certificate type:"
echo "1) Self-signed certificate (for development/testing)"
echo "2) Let's Encrypt certificate (for production)"
echo "3) Use existing certificate"
read -p "Enter choice (1-3): " cert_choice

case $cert_choice in
    1)
        print_step "Generating self-signed certificate..."
        
        # Create certificate configuration
        cat > /tmp/cert.conf <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = NY
L = White Plains
O = WPGC
CN = $MINIO_HOST

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = minio.local
DNS.3 = *.minio.local
IP.1 = $MINIO_HOST
IP.2 = 127.0.0.1
EOF
        
        # Generate certificate on Pi
        ssh $MINIO_USER@$MINIO_HOST "
            openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
                -keyout $CERT_PATH/private.key \
                -out $CERT_PATH/public.crt \
                -config - <<'CERTEOF'
$(cat /tmp/cert.conf)
CERTEOF
        "
        
        rm /tmp/cert.conf
        print_success "Self-signed certificate generated"
        print_warning "Note: Browsers will show a security warning with self-signed certificates"
        ;;
        
    2)
        print_step "Setting up Let's Encrypt certificate..."
        
        read -p "Enter your domain name (e.g., minio.wpgc.church): " domain_name
        
        ssh $MINIO_USER@$MINIO_HOST "
            # Install certbot if not present
            if ! command -v certbot &> /dev/null; then
                sudo apt-get update
                sudo apt-get install -y certbot
            fi
            
            # Stop MinIO temporarily
            sudo systemctl stop minio || docker-compose stop minio || true
            
            # Get certificate
            sudo certbot certonly --standalone -d $domain_name --non-interactive --agree-tos -m admin@wpgc.church
            
            # Copy certificates to MinIO directory
            sudo cp /etc/letsencrypt/live/$domain_name/privkey.pem $CERT_PATH/private.key
            sudo cp /etc/letsencrypt/live/$domain_name/fullchain.pem $CERT_PATH/public.crt
            sudo chown $MINIO_USER:$MINIO_USER $CERT_PATH/*.{key,crt}
            sudo chmod 600 $CERT_PATH/private.key
            sudo chmod 644 $CERT_PATH/public.crt
        "
        
        print_success "Let's Encrypt certificate installed"
        ;;
        
    3)
        print_step "Using existing certificate..."
        read -p "Enter path to private key: " private_key_path
        read -p "Enter path to public certificate: " public_cert_path
        
        # Copy certificates to Pi
        scp "$private_key_path" $MINIO_USER@$MINIO_HOST:$CERT_PATH/private.key
        scp "$public_cert_path" $MINIO_USER@$MINIO_HOST:$CERT_PATH/public.crt
        
        # Set permissions
        ssh $MINIO_USER@$MINIO_HOST "
            chmod 600 $CERT_PATH/private.key
            chmod 644 $CERT_PATH/public.crt
        "
        
        print_success "Existing certificates installed"
        ;;
esac

# Step 3: Configure MinIO environment
print_step "Step 3: Configuring MinIO environment for HTTPS and CORS"

# Check if using Docker or systemd
ssh $MINIO_USER@$MINIO_HOST "
if [ -f /home/gaius/sermon-uploader/docker-compose.yml ]; then
    echo 'DOCKER'
else
    echo 'SYSTEMD'
fi
" > /tmp/minio_type.txt

MINIO_TYPE=$(cat /tmp/minio_type.txt)
rm /tmp/minio_type.txt

if [ "$MINIO_TYPE" = "DOCKER" ]; then
    print_step "Updating Docker Compose configuration..."
    
    ssh $MINIO_USER@$MINIO_HOST "
        cd /home/gaius/sermon-uploader
        
        # Backup existing docker-compose.yml
        cp docker-compose.yml docker-compose.yml.backup
        
        # Create updated configuration
        cat > docker-compose.minio-tls.yml <<'EOF'
version: '3.8'

services:
  minio:
    image: minio/minio:latest
    container_name: sermon-minio
    ports:
      - '9000:9000'
      - '9001:9001'
    environment:
      - MINIO_ROOT_USER=gaius
      - MINIO_ROOT_PASSWORD=John 3:16
      - MINIO_API_CORS_ALLOW_ORIGIN=https://sermons.wpgc.church,https://localhost:3000,http://localhost:3000
      - MINIO_BROWSER=on
      - MINIO_BROWSER_REDIRECT_URL=https://$MINIO_HOST:9001
    volumes:
      - ./minio-data:/data
      - $CERT_PATH:/root/.minio/certs:ro
    command: server /data --console-address ':9001'
    restart: unless-stopped
    healthcheck:
      test: ['CMD', 'curl', '-f', '-k', 'https://localhost:9000/minio/health/live']
      interval: 30s
      timeout: 20s
      retries: 3
EOF
        
        # Stop existing MinIO
        docker-compose stop minio || true
        
        # Start with new configuration
        docker-compose -f docker-compose.minio-tls.yml up -d minio
    "
    
    print_success "Docker Compose updated with TLS configuration"
    
else
    print_step "Updating systemd configuration..."
    
    ssh $MINIO_USER@$MINIO_HOST "
        # Update MinIO environment file
        sudo tee /etc/default/minio <<EOF
# MinIO Configuration
MINIO_ROOT_USER=gaius
MINIO_ROOT_PASSWORD='John 3:16'
MINIO_VOLUMES='/home/gaius/minio-data'
MINIO_OPTS='--console-address :9001'
MINIO_API_CORS_ALLOW_ORIGIN='https://sermons.wpgc.church,https://localhost:3000,http://localhost:3000'
EOF
        
        # Restart MinIO
        sudo systemctl restart minio
    "
    
    print_success "Systemd configuration updated with TLS"
fi

# Step 4: Update MinIO client configuration
print_step "Step 4: Configuring MinIO client for HTTPS"

ssh $MINIO_USER@$MINIO_HOST "
    # Configure mc (MinIO client) for HTTPS
    mc alias set local https://localhost:9000 gaius 'John 3:16' --insecure
    
    # Test the connection
    mc admin info local --insecure
" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    print_success "MinIO client configured for HTTPS"
else
    print_warning "MinIO client configuration needs manual attention"
fi

# Step 5: Update backend configuration
print_step "Step 5: Updating backend configuration for HTTPS"

cat > /tmp/backend_env_update.sh <<'EOF'
# Update backend .env file
ENV_FILE="/home/gaius/sermon-uploader/backend/.env"

if [ -f "$ENV_FILE" ]; then
    cp "$ENV_FILE" "$ENV_FILE.backup"
    
    # Update MinIO endpoint to use HTTPS
    sed -i 's|MINIO_ENDPOINT=.*|MINIO_ENDPOINT=localhost:9000|' "$ENV_FILE"
    sed -i 's|MINIO_SECURE=.*|MINIO_SECURE=true|' "$ENV_FILE"
    
    # Add if not exists
    grep -q "MINIO_SECURE" "$ENV_FILE" || echo "MINIO_SECURE=true" >> "$ENV_FILE"
    
    echo "Backend .env updated"
else
    echo "Backend .env not found"
fi
EOF

scp /tmp/backend_env_update.sh $MINIO_USER@$MINIO_HOST:/tmp/
ssh $MINIO_USER@$MINIO_HOST "bash /tmp/backend_env_update.sh && rm /tmp/backend_env_update.sh"
rm /tmp/backend_env_update.sh

print_success "Backend configuration updated"

# Step 6: Test the configuration
print_step "Step 6: Testing HTTPS configuration"
echo ""

# Run the test script
echo "Running TLS tests..."
bash ./scripts/test-minio-tls.sh

echo ""
echo "================================="
echo -e "${GREEN}✅ MinIO TLS Setup Complete!${NC}"
echo "================================="
echo ""
echo "MinIO is now accessible via HTTPS at:"
echo -e "  API:     ${GREEN}https://$MINIO_HOST:9000${NC}"
echo -e "  Console: ${GREEN}https://$MINIO_HOST:9001${NC}"
echo ""
echo "Next steps:"
echo "1. Update your frontend to use: https://$MINIO_HOST:9000"
echo "2. If using self-signed cert, add exception in browser"
echo "3. Test upload with: curl -k https://$MINIO_HOST:9000/minio/health/live"
echo ""
echo "CORS is configured to allow:"
echo "  - https://sermons.wpgc.church"
echo "  - https://localhost:3000"
echo "  - http://localhost:3000"
echo ""

# Create a test HTML file for browser testing
cat > /tmp/test-upload.html <<'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>MinIO HTTPS Upload Test</title>
</head>
<body>
    <h1>MinIO HTTPS Upload Test</h1>
    <input type="file" id="fileInput" />
    <button onclick="testUpload()">Test Upload</button>
    <div id="result"></div>
    
    <script>
    async function testUpload() {
        const file = document.getElementById('fileInput').files[0];
        if (!file) {
            alert('Please select a file');
            return;
        }
        
        try {
            // Get presigned URL from backend
            const response = await fetch('https://192.168.1.127:8000/api/upload/presigned', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ 
                    filename: file.name,
                    fileSize: file.size 
                })
            });
            
            const { url } = await response.json();
            
            // Upload directly to MinIO
            const uploadResponse = await fetch(url, {
                method: 'PUT',
                body: file
            });
            
            if (uploadResponse.ok) {
                document.getElementById('result').innerHTML = 
                    '<p style="color: green;">✅ Upload successful via HTTPS!</p>';
            } else {
                throw new Error('Upload failed');
            }
        } catch (error) {
            document.getElementById('result').innerHTML = 
                '<p style="color: red;">❌ Upload failed: ' + error.message + '</p>';
        }
    }
    </script>
</body>
</html>
EOF

echo "Test HTML file created at: /tmp/test-upload.html"
echo "Open this file in your browser to test HTTPS uploads"