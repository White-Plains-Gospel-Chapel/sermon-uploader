#!/bin/bash

echo "🌐 CloudFlare Dual-Domain Setup for Sermon Uploader"
echo "This will create a MinIO subdomain that bypasses CloudFlare proxy"
echo ""

# CloudFlare API credentials (you'll need to provide these)
read -p "Enter your CloudFlare API Token: " CF_API_TOKEN
read -p "Enter your domain (e.g., yourdomain.com): " DOMAIN
read -p "Enter your Pi's public IP address: " PI_PUBLIC_IP

# Extract zone from domain
ZONE_NAME="$DOMAIN"

echo ""
echo "🔍 Looking up CloudFlare Zone ID for $DOMAIN..."

# Get Zone ID
ZONE_RESPONSE=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones?name=$ZONE_NAME" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json")

ZONE_ID=$(echo $ZONE_RESPONSE | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['result'][0]['id'] if data['success'] and data['result'] else 'ERROR')")

if [ "$ZONE_ID" = "ERROR" ]; then
  echo "❌ Failed to get Zone ID. Check your API token and domain name."
  echo "Response: $ZONE_RESPONSE"
  exit 1
fi

echo "✅ Zone ID found: $ZONE_ID"
echo ""

# Create MinIO subdomain with proxy DISABLED
echo "📡 Creating MinIO subdomain (minio.$DOMAIN) with proxy DISABLED..."

MINIO_RECORD_RESPONSE=$(curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{
    "type": "A",
    "name": "minio",
    "content": "'$PI_PUBLIC_IP'",
    "proxied": false,
    "comment": "MinIO direct access - bypasses CloudFlare proxy for large uploads"
  }')

MINIO_SUCCESS=$(echo $MINIO_RECORD_RESPONSE | python3 -c "import sys, json; data=json.load(sys.stdin); print('true' if data['success'] else 'false')")

if [ "$MINIO_SUCCESS" = "true" ]; then
  echo "✅ MinIO subdomain created: minio.$DOMAIN → $PI_PUBLIC_IP (proxy DISABLED)"
else
  echo "⚠️  MinIO subdomain creation result: $MINIO_RECORD_RESPONSE"
  echo "This might be okay if the record already exists."
fi

echo ""

# Verify main domain is proxied
echo "🔍 Checking main domain proxy status..."

MAIN_RECORDS_RESPONSE=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?name=sermon-uploader.$DOMAIN" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json")

MAIN_PROXIED=$(echo $MAIN_RECORDS_RESPONSE | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['result'][0]['proxied'] if data['success'] and data['result'] else 'unknown')")

if [ "$MAIN_PROXIED" = "true" ]; then
  echo "✅ Main domain (sermon-uploader.$DOMAIN) is properly proxied through CloudFlare"
elif [ "$MAIN_PROXIED" = "false" ]; then
  echo "⚠️  Main domain is not proxied. You may want to enable proxy for the main app."
else
  echo "ℹ️  Main domain status unclear. Please verify in CloudFlare dashboard."
fi

echo ""
echo "🎯 DNS Configuration Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📱 Main App:    sermon-uploader.$DOMAIN → $PI_PUBLIC_IP (🟠 Proxied)"
echo "💾 MinIO:       minio.$DOMAIN → $PI_PUBLIC_IP (⚪ DNS Only)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "✅ DNS setup complete!"
echo ""
echo "Next steps:"
echo "1. Wait for DNS propagation (2-5 minutes)"
echo "2. Configure router port forwarding:"
echo "   - Port 8000 → Pi:8000 (Backend)"
echo "   - Port 9000 → Pi:9000 (MinIO)"
echo "3. Update MinIO for global access"
echo ""
echo "Test when ready:"
echo "curl -I http://minio.$DOMAIN:9000/minio/health/live"

# Save configuration for later use
cat > /tmp/cloudflare-config.env << EOF
ZONE_ID=$ZONE_ID
DOMAIN=$DOMAIN
PI_PUBLIC_IP=$PI_PUBLIC_IP
MINIO_SUBDOMAIN=minio.$DOMAIN
MAIN_SUBDOMAIN=sermon-uploader.$DOMAIN
EOF

echo ""
echo "💾 Configuration saved to /tmp/cloudflare-config.env"