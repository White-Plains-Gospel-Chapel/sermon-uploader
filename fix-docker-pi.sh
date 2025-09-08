#!/bin/bash

# Fix Docker installation on Raspberry Pi

echo "🔧 Fixing Docker on Raspberry Pi..."

# Stop and disable Docker if it's broken
sudo systemctl stop docker 2>/dev/null || true
sudo systemctl stop docker.socket 2>/dev/null || true

# Clean up Docker data that might be corrupted
echo "🧹 Cleaning up Docker data..."
sudo rm -rf /var/lib/docker/runtimes
sudo rm -rf /var/run/docker.sock

# Try to start Docker again
echo "🚀 Starting Docker..."
sudo systemctl start docker.socket
sudo systemctl start docker

# Check if Docker is running
if sudo docker version > /dev/null 2>&1; then
    echo "✅ Docker is running!"
    sudo docker version
    
    # Add user to docker group if not already
    sudo usermod -aG docker $USER
    echo "You may need to log out and back in for group changes to take effect"
else
    echo "❌ Docker still not working. Try reinstalling:"
    echo ""
    echo "To reinstall Docker:"
    echo "sudo apt-get remove docker docker-engine docker.io containerd runc"
    echo "sudo apt-get update"
    echo "curl -fsSL https://get.docker.com -o get-docker.sh"
    echo "sudo sh get-docker.sh"
    echo "sudo usermod -aG docker $USER"
fi

# Check Docker status
echo ""
echo "📊 Docker status:"
sudo systemctl status docker --no-pager | head -20