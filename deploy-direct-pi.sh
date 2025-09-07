#!/bin/bash

PI_USER="gaius"
PI_HOST="192.168.1.127"
KEY_PATH="$HOME/.ssh/WPGCSermonUploader"

echo "🚀 Deploying Direct Pi Access Solution (Bypassing CloudFlare)..."

# Deploy backend
echo "📦 Deploying backend..."
scp -i "$KEY_PATH" backend/sermon-uploader-direct $PI_USER@$PI_HOST:/home/$PI_USER/sermon-uploader/backend/sermon-uploader

# Deploy frontend
echo "📦 Creating frontend archive..."
cd frontend
tar -czf out-direct.tar.gz out/
scp -i "$KEY_PATH" out-direct.tar.gz $PI_USER@$PI_HOST:/home/$PI_USER/sermon-uploader/frontend/
rm out-direct.tar.gz
cd ..

# Restart services
echo "🔄 Restarting services on Pi..."
ssh -i "$KEY_PATH" $PI_USER@$PI_HOST << 'ENDSSH'
cd /home/gaius/sermon-uploader/frontend
tar -xzf out-direct.tar.gz
rm out-direct.tar.gz

# Restart backend service
sudo systemctl restart sermon-uploader || (
  cd /home/gaius/sermon-uploader/backend
  pkill -f sermon-uploader
  nohup ./sermon-uploader > app.log 2>&1 &
  echo "✅ Backend restarted with direct access support"
)
ENDSSH

echo "✅ Deployment complete! Access directly at http://192.168.1.127:8000"
echo "📡 This bypasses CloudFlare completely - no 100MB limit!"
