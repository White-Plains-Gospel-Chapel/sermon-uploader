#!/bin/bash

# Load CloudFlare configuration if available
if [ -f "/tmp/cloudflare-config.env" ]; then
    source /tmp/cloudflare-config.env
    echo "📋 Using configuration from CloudFlare setup"
    echo "   Domain: $DOMAIN"
    echo "   MinIO: $MINIO_SUBDOMAIN"
    echo ""
else
    echo "⚠️  CloudFlare config not found. Please enter details manually."
    read -p "Enter your MinIO subdomain (e.g., minio.yourdomain.com): " MINIO_SUBDOMAIN
fi

PI_USER="gaius"
PI_HOST="192.168.1.127"

echo "🌐 Setting up MinIO for Global Access"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Create the MinIO global setup script
cat > /tmp/minio-global-setup.sh << 'EOF'
#!/bin/bash

echo "🛑 Stopping current MinIO container..."
docker stop minio-standalone 2>/dev/null || true
docker rm minio-standalone 2>/dev/null || true

echo "🚀 Starting MinIO with global configuration..."

# Get the domain from environment or use default
MINIO_DOMAIN=${1:-minio.localhost}

docker run -d \
  --name minio-standalone \
  --restart unless-stopped \
  -p 9000:9000 \
  -p 9001:9001 \
  -v /home/gaius/minio/data:/data \
  -e MINIO_ROOT_USER=gaius \
  -e MINIO_ROOT_PASSWORD="John 3:16" \
  -e MINIO_DOMAIN="$MINIO_DOMAIN" \
  -e MINIO_SERVER_URL="http://$MINIO_DOMAIN:9000" \
  -e MINIO_BROWSER_REDIRECT_URL="http://$MINIO_DOMAIN:9001" \
  minio/minio:latest server /data --console-address ":9001"

echo "⏳ Waiting for MinIO to start..."
sleep 5

echo "🔧 Configuring MinIO for global browser access..."

# Configure MinIO client inside container
docker exec -it minio-standalone sh -c '
# Set up mc alias
mc alias set local http://localhost:9000 gaius "John 3:16"

# Create CORS configuration
cat > /tmp/cors.json << CORS_EOF
{
  "CORSRules": [{
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag", "x-amz-server-side-encryption", "x-amz-request-id", "x-amz-id-2", "x-amz-version-id"],
    "MaxAgeSeconds": 3000
  }]
}
CORS_EOF

# Apply CORS settings
echo "📡 Applying CORS settings for browser access..."
mc cors set /tmp/cors.json local/sermons

# Set bucket policy for uploads
echo "🔓 Setting bucket policy for public uploads..."
mc anonymous set upload local/sermons

# Verify settings
echo "✅ CORS settings applied:"
mc cors get local/sermons

echo "🔍 Bucket policy:"
mc anonymous get local/sermons
'

echo ""
echo "✅ MinIO Global Configuration Complete!"
echo ""
echo "📊 MinIO Status:"
docker ps | grep minio

echo ""
echo "🌐 Global Access URLs:"
echo "   MinIO API:     http://'$MINIO_DOMAIN':9000"
echo "   MinIO Console: http://'$MINIO_DOMAIN':9001"
echo ""
echo "🧪 Test Commands:"
echo "   Health Check:  curl -I http://'$MINIO_DOMAIN':9000/minio/health/live"
echo "   CORS Test:     curl -H \"Origin: https://example.com\" -I http://'$MINIO_DOMAIN':9000/sermons/"
EOF

chmod +x /tmp/minio-global-setup.sh

echo "📤 Deploying MinIO global setup to Pi..."
scp /tmp/minio-global-setup.sh $PI_USER@$PI_HOST:/tmp/

echo "🔧 Executing MinIO global setup on Pi..."
ssh $PI_USER@$PI_HOST "bash /tmp/minio-global-setup.sh '$MINIO_SUBDOMAIN'"

echo ""
echo "✅ MinIO Global Setup Complete!"
echo ""
echo "🧪 Testing MinIO access..."

# Test MinIO health endpoint
if curl -s -I "http://$MINIO_SUBDOMAIN:9000/minio/health/live" | grep -q "200 OK"; then
    echo "✅ MinIO health check: PASSED"
else
    echo "❌ MinIO health check: FAILED"
    echo "   This might be due to DNS propagation delay or firewall settings"
fi

# Test CORS headers
echo "🔍 Testing CORS headers..."
CORS_TEST=$(curl -s -H "Origin: https://example.com" -I "http://$MINIO_SUBDOMAIN:9000/sermons/" 2>/dev/null | grep -i "access-control")
if [ ! -z "$CORS_TEST" ]; then
    echo "✅ CORS headers: FOUND"
    echo "   $CORS_TEST"
else
    echo "⚠️  CORS headers: NOT FOUND (may take a few minutes to propagate)"
fi

echo ""
echo "🎯 Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🌐 MinIO Global URL:  http://$MINIO_SUBDOMAIN:9000"
echo "📱 MinIO Console:     http://$MINIO_SUBDOMAIN:9001"
echo "🔓 CORS:              Enabled for all origins"  
echo "📤 Upload Policy:     Public uploads allowed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "⚠️  IMPORTANT: Ensure your router forwards ports 9000 and 9001 to $PI_HOST"
echo ""
echo "Next: Update backend and frontend to use the global MinIO endpoint"