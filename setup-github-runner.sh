#!/bin/bash

# GitHub Self-Hosted Runner Setup for Raspberry Pi
# This script sets up the Pi as a GitHub Actions runner

set -e

echo "🔧 Setting up GitHub Actions self-hosted runner on Pi..."

# Create runner directory
sudo mkdir -p /opt/github-runner
cd /opt/github-runner

# Download the latest runner for ARM64
echo "📥 Downloading GitHub Actions runner..."
curl -o actions-runner-linux-arm64-2.311.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-arm64-2.311.0.tar.gz

# Verify the hash (optional but recommended)
echo "3d357cf7-6d18-4a6b-83b7-1ca67c1f9fd1  actions-runner-linux-arm64-2.311.0.tar.gz" | shasum -a 256 -c

# Extract the installer
tar xzf actions-runner-linux-arm64-2.311.0.tar.gz

# Create a user for the runner
sudo useradd -m -s /bin/bash github-runner || true
sudo chown -R github-runner:github-runner /opt/github-runner

echo "✅ GitHub Actions runner downloaded and extracted."
echo ""
echo "🔑 Next steps:"
echo "1. Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/actions/runners"
echo "2. Click 'New self-hosted runner'"
echo "3. Select Linux ARM64"
echo "4. Copy the token from the configuration command"
echo "5. Run the following commands on your Pi:"
echo ""
echo "   sudo su - github-runner"
echo "   cd /opt/github-runner"
echo "   ./config.sh --url https://github.com/White-Plains-Gospel-Chapel/sermon-uploader --token YOUR_TOKEN"
echo "   sudo ./svc.sh install"
echo "   sudo ./svc.sh start"
echo ""
echo "After configuration, create the systemd service file:"

# Create systemd service file
sudo tee /etc/systemd/system/github-runner.service > /dev/null << 'EOF'
[Unit]
Description=GitHub Actions Runner
After=network.target

[Service]
Type=simple
User=github-runner
WorkingDirectory=/opt/github-runner
ExecStart=/opt/github-runner/runsvc.sh
Restart=always
RestartSec=15
KillMode=process
KillSignal=SIGTERM
TimeoutStopSec=5min

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
sudo chown -R github-runner:github-runner /opt/github-runner

echo "📝 Systemd service created: github-runner.service"
echo ""
echo "🎯 After configuring with your token, enable the service:"
echo "   sudo systemctl daemon-reload"
echo "   sudo systemctl enable github-runner"
echo "   sudo systemctl start github-runner"
echo ""
echo "🔍 Check status with:"
echo "   sudo systemctl status github-runner"