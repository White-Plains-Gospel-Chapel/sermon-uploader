#!/bin/bash

# Setup webhook listener for automated Docker deployment on Raspberry Pi

set -e

echo "🔧 Setting up webhook listener for automated deployment..."

# Install Python dependencies
echo "📦 Installing Python dependencies..."
sudo apt-get update
sudo apt-get install -y python3-pip python3-venv

# Create virtual environment
echo "🐍 Creating Python virtual environment..."
cd /opt/sermon-uploader
python3 -m venv webhook-env
source webhook-env/bin/activate

# Install Flask and requests
pip install flask requests

# Install webhook listener service
echo "📝 Installing systemd service..."
sudo cp webhook-listener.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable webhook-listener

# Configure webhook secret
echo ""
echo "⚠️  IMPORTANT: Configure your webhook secret!"
echo ""
echo "1. Generate a secure webhook secret:"
echo "   openssl rand -hex 32"
echo ""
echo "2. Edit the service file to add your secret:"
echo "   sudo nano /etc/systemd/system/webhook-listener.service"
echo "   Update: Environment=\"WEBHOOK_SECRET=your-generated-secret\""
echo ""
echo "3. Add the same secret to GitHub repository secrets:"
echo "   Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/secrets/actions"
echo "   Add secret: PI_WEBHOOK_URL = http://YOUR_PI_IP:9001/webhook"
echo "   Add secret: WEBHOOK_SECRET = your-generated-secret"
echo ""
echo "4. Start the webhook listener:"
echo "   sudo systemctl start webhook-listener"
echo "   sudo systemctl status webhook-listener"
echo ""
echo "5. Check logs:"
echo "   sudo journalctl -u webhook-listener -f"
echo ""
echo "The webhook will listen on port 9001"
echo "Make sure to configure port forwarding if needed!"