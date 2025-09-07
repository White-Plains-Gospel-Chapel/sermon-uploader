#!/bin/bash

PI_USER="gaius"
PI_HOST="192.168.1.127"

echo "🚀 Deploying Direct Pi Access (Bypassing CloudFlare)..."

# Deploy backend
echo "📦 Deploying backend..."
scp backend/sermon-uploader-direct $PI_USER@$PI_HOST:/home/$PI_USER/sermon-uploader-direct

# Deploy frontend
echo "📦 Deploying frontend..."
cd frontend
tar -czf out-direct.tar.gz out/
scp out-direct.tar.gz $PI_USER@$PI_HOST:/home/$PI_USER/
rm out-direct.tar.gz
cd ..

# Apply changes on Pi
echo "🔄 Applying changes on Pi..."
ssh $PI_USER@$PI_HOST << 'ENDSSH'
# Stop current backend
pkill -f sermon-uploader

# Move new backend
chmod +x /home/gaius/sermon-uploader-direct
mv /home/gaius/sermon-uploader-direct /home/gaius/sermon-uploader-current

# Extract frontend
cd /home/gaius/frontend
rm -rf out
tar -xzf /home/gaius/out-direct.tar.gz
rm /home/gaius/out-direct.tar.gz

# Start backend with proper logging
cd /home/gaius
nohup ./sermon-uploader-current > sermon-uploader.log 2>&1 &
echo "✅ Backend started with PID: $!"

# Check if it's running
sleep 2
if pgrep -f sermon-uploader-current > /dev/null; then
    echo "✅ Backend is running!"
    echo "📡 Direct access available at http://192.168.1.127:8000"
    echo "🚫 CloudFlare 100MB limit bypassed!"
else
    echo "❌ Backend failed to start. Check sermon-uploader.log"
fi
ENDSSH

echo ""
echo "✅ Deployment complete!"
echo "📡 Access directly at: http://192.168.1.127:8000"
echo "🎯 This bypasses CloudFlare - no 100MB upload limit!"
echo ""
echo "⚠️  Note: When accessing from browser, use http://192.168.1.127:8000"
echo "    NOT your CloudFlare domain to avoid the 100MB limit"
