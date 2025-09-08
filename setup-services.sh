#!/bin/bash

# Setup script for sermon-uploader services on Raspberry Pi
# This script builds and configures both backend and frontend for Pi deployment

set -e

echo "🚀 Setting up sermon-uploader services for Raspberry Pi..."

# Check system architecture
ARCH=$(uname -m)
echo "📊 System architecture: $ARCH"

# Navigate to project directory
PROJECT_DIR="/opt/sermon-uploader"
if [ ! -d "$PROJECT_DIR" ]; then
    echo "❌ Project directory not found at $PROJECT_DIR"
    echo "Please clone the repository first:"
    echo "  sudo mkdir -p /opt && cd /opt"
    echo "  git clone https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git"
    exit 1
fi

cd $PROJECT_DIR

# Pull latest changes
echo "📥 Pulling latest code..."
git pull origin master

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "📦 Installing Go..."
    wget https://go.dev/dl/go1.21.6.linux-arm64.tar.gz
    sudo tar -C /usr/local -xzf go1.21.6.linux-arm64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    rm go1.21.6.linux-arm64.tar.gz
    echo "✅ Go installed"
fi

# Install Node.js if not present
if ! command -v node &> /dev/null; then
    echo "📦 Installing Node.js..."
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
    echo "✅ Node.js installed"
fi

# Build backend for ARM64
echo "🔨 Building backend for ARM64..."
cd $PROJECT_DIR/backend

# Create .env file if it doesn't exist
if [ ! -f .env ]; then
    echo "📝 Creating backend .env file..."
    cat > .env << 'EOF'
PORT=8000
MINIO_ENDPOINT=192.168.1.127:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_BUCKET=sermons
MINIO_SECURE=false
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/1411012857985892412/dMzxtUtXiOCvFR0w8IuzL8mGYwZqFXuwGucT3CnBNjnXgkVxcWPLk5Vlm9lwh72YWP38
EOF
fi

# Build the backend
echo "🔨 Compiling backend..."
go mod download
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o sermon-uploader-arm64 .
chmod +x sermon-uploader-arm64

# Build frontend
echo "🔨 Building frontend..."
cd $PROJECT_DIR/frontend-react

# Create .env.local if it doesn't exist
if [ ! -f .env.local ]; then
    echo "📝 Creating frontend .env.local file..."
    cat > .env.local << 'EOF'
NEXT_PUBLIC_API_URL=http://localhost:8000
EOF
fi

# Install dependencies and build
npm ci
npm run build

# Create systemd service for backend
echo "📝 Creating backend systemd service..."
sudo tee /etc/systemd/system/sermon-backend.service > /dev/null << EOF
[Unit]
Description=Sermon Uploader Backend API
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=$PROJECT_DIR/backend
ExecStart=$PROJECT_DIR/backend/sermon-uploader-arm64
Restart=always
RestartSec=10
EnvironmentFile=$PROJECT_DIR/backend/.env

[Install]
WantedBy=multi-user.target
EOF

# Create systemd service for frontend
echo "📝 Creating frontend systemd service..."
sudo tee /etc/systemd/system/sermon-frontend.service > /dev/null << EOF
[Unit]
Description=Sermon Uploader Frontend
After=network.target sermon-backend.service

[Service]
Type=simple
User=pi
WorkingDirectory=$PROJECT_DIR/frontend-react
ExecStart=/usr/bin/npm start
Restart=always
RestartSec=10
Environment="NODE_ENV=production"
Environment="PORT=3000"

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start services
echo "🚀 Starting services..."
sudo systemctl daemon-reload
sudo systemctl enable sermon-backend sermon-frontend
sudo systemctl restart sermon-backend sermon-frontend

# Wait for services to start
sleep 5

# Check status
echo ""
echo "📊 Service status:"
echo "Backend:"
sudo systemctl status sermon-backend --no-pager | head -10
echo ""
echo "Frontend:"
sudo systemctl status sermon-frontend --no-pager | head -10

# Test endpoints
echo ""
echo "🔍 Testing endpoints..."
echo -n "Backend API: "
curl -s http://localhost:8000/api/health > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"
echo -n "Frontend: "
curl -s http://localhost:3000 > /dev/null 2>&1 && echo "✅ Running" || echo "❌ Not responding"

echo ""
echo "✅ Setup complete!"
echo ""
echo "Services are running:"
echo "  - Backend API: http://localhost:8000"
echo "  - Frontend: http://localhost:3000"
echo ""
echo "Access via domain:"
echo "  - Admin Dashboard: https://admin.wpgc.church"
echo "  - API: https://api.wpgc.church"
echo ""
echo "Useful commands:"
echo "  - View backend logs: sudo journalctl -u sermon-backend -f"
echo "  - View frontend logs: sudo journalctl -u sermon-frontend -f"
echo "  - Restart backend: sudo systemctl restart sermon-backend"
echo "  - Restart frontend: sudo systemctl restart sermon-frontend"