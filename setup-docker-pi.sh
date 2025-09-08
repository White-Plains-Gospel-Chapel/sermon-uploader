#!/bin/bash

# Setup Docker on Raspberry Pi for sermon-uploader
# Run this script on the Pi: ./setup-docker-pi.sh

set -e

echo "🔧 Setting up Docker environment for sermon-uploader on Raspberry Pi..."

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "📦 Installing Docker..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker $USER
    rm get-docker.sh
    echo "✅ Docker installed. Please log out and back in for group changes to take effect."
else
    echo "✅ Docker is already installed"
fi

# Check Docker Compose
if ! docker compose version &> /dev/null 2>&1; then
    echo "📦 Installing Docker Compose plugin..."
    sudo apt-get update
    sudo apt-get install -y docker-compose-plugin
fi

# Install required tools
echo "📦 Installing required tools..."
sudo apt-get update
sudo apt-get install -y git curl ffmpeg

# Create project directory if it doesn't exist
if [ ! -d "/opt/sermon-uploader" ]; then
    echo "📁 Creating project directory..."
    sudo mkdir -p /opt/sermon-uploader
    sudo chown $USER:$USER /opt/sermon-uploader
fi

# Clone or update repository
cd /opt
if [ ! -d "sermon-uploader/.git" ]; then
    echo "📥 Cloning repository..."
    git clone https://github.com/wpgc-parish/sermon-uploader.git
else
    echo "📥 Updating repository..."
    cd sermon-uploader
    git pull origin master
fi

cd /opt/sermon-uploader

# Create .env file for backend if it doesn't exist
if [ ! -f "backend/.env" ]; then
    echo "📝 Creating backend .env file..."
    cat > backend/.env << 'EOF'
PORT=8000
MINIO_ENDPOINT=192.168.1.127:9000
MINIO_ACCESS_KEY=gaius
MINIO_SECRET_KEY=John 3:16
MINIO_BUCKET=sermons
MINIO_SECURE=false
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/1411012857985892412/dMzxtUtXiOCvFR0w8IuzL8mGYwZqFXuwGucT3CnBNjnXgkVxcWPLk5Vlm9lwh72YWP38
EOF
    echo "✅ Backend .env created"
fi

# Create .env file for frontend if it doesn't exist
if [ ! -f "frontend-react/.env.local" ]; then
    echo "📝 Creating frontend .env.local file..."
    cat > frontend-react/.env.local << 'EOF'
NEXT_PUBLIC_API_URL=http://localhost:8000
EOF
    echo "✅ Frontend .env.local created"
fi

# Make deployment script executable
chmod +x deploy-docker.sh 2>/dev/null || true

echo ""
echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "1. If Docker was just installed, log out and back in"
echo "2. Run: ./deploy-docker.sh to build and start the services"
echo ""