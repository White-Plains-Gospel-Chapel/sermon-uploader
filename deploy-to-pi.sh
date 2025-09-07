#!/bin/bash
# Direct deployment to Raspberry Pi

echo "🚀 Starting direct deployment to Pi..."

# Build for ARM64
echo "📦 Building ARM64 binary..."
cd backend
GOOS=linux GOARCH=arm64 go build -o sermon-uploader-arm64 .

echo "✅ Binary built successfully"

# Create deployment package
echo "📦 Creating deployment package..."
cd ..
tar -czf deployment.tar.gz \
  backend/sermon-uploader-arm64 \
  docker-compose.prod.yml \
  .env.example \
  scripts/configure-minio-cors.sh

echo "📤 Deploying to Pi at 192.168.1.127..."

# Try to deploy using expect for password automation
expect -c "
spawn scp deployment.tar.gz pi@192.168.1.127:/tmp/
expect \"password:\"
send \"raspberry\r\"
expect eof
"

# Execute deployment commands on Pi
expect -c "
spawn ssh pi@192.168.1.127
expect \"password:\"
send \"raspberry\r\"
expect \"$ \"
send \"cd /home/pi/sermon-uploader && tar -xzf /tmp/deployment.tar.gz\r\"
expect \"$ \"
send \"chmod +x backend/sermon-uploader-arm64\r\"
expect \"$ \"
send \"sudo systemctl stop sermon-uploader || true\r\"
expect \"$ \"
send \"sudo cp backend/sermon-uploader-arm64 /usr/local/bin/sermon-uploader\r\"
expect \"$ \"
send \"sudo systemctl start sermon-uploader || true\r\"
expect \"$ \"
send \"docker-compose down && docker-compose up -d\r\"
expect \"$ \"
send \"exit\r\"
expect eof
"

echo "✅ Deployment complete!"
echo "🔍 Verifying deployment..."
curl -s http://192.168.1.127:8000/api/status | jq .