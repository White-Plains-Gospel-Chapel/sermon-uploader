#!/bin/bash

# GitHub Actions Self-Hosted Runner Setup Script for Raspberry Pi
# This script installs and configures a GitHub Actions runner on your Raspberry Pi

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
RUNNER_NAME="${RUNNER_NAME:-pi-runner}"
RUNNER_WORKDIR="${RUNNER_WORKDIR:-/opt/actions-runner}"
RUNNER_USER="${RUNNER_USER:-$USER}"
GITHUB_OWNER="White-Plains-Gospel-Chapel"
GITHUB_REPO="sermon-uploader"

echo -e "${GREEN}GitHub Actions Self-Hosted Runner Setup${NC}"
echo "========================================"

# Check if running on Raspberry Pi
if ! grep -q "Raspberry Pi" /proc/device-tree/model 2>/dev/null && ! uname -m | grep -q "aarch64\|armv"; then
    echo -e "${YELLOW}Warning: This doesn't appear to be a Raspberry Pi. Continue anyway? (y/n)${NC}"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Step 1: Install dependencies
echo -e "${GREEN}Step 1: Installing dependencies...${NC}"
sudo apt-get update
sudo apt-get install -y curl jq build-essential libssl-dev libffi-dev python3 python3-venv python3-dev

# Step 2: Create runner directory
echo -e "${GREEN}Step 2: Creating runner directory...${NC}"
sudo mkdir -p $RUNNER_WORKDIR
sudo chown -R $RUNNER_USER:$RUNNER_USER $RUNNER_WORKDIR
cd $RUNNER_WORKDIR

# Step 3: Get runner token from GitHub
echo -e "${GREEN}Step 3: Getting registration token...${NC}"
echo -e "${YELLOW}Please go to: https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/settings/actions/runners/new${NC}"
echo -e "${YELLOW}Click on 'Linux' and 'ARM64' (for Pi 4/5) or 'ARM' (for older Pi)${NC}"
echo -e "${YELLOW}You'll see a token in the configuration command. It looks like: AXXXXXXXXXXXXXXXXXXXXX${NC}"
echo ""
read -p "Enter the registration token: " RUNNER_TOKEN

if [ -z "$RUNNER_TOKEN" ]; then
    echo -e "${RED}Error: Token cannot be empty${NC}"
    exit 1
fi

# Step 4: Download and extract the runner
echo -e "${GREEN}Step 4: Downloading GitHub Actions runner...${NC}"

# Detect architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "aarch64" ]] || [[ "$ARCH" == "arm64" ]]; then
    # 64-bit ARM (Pi 4/5 with 64-bit OS)
    RUNNER_ARCH="arm64"
elif [[ "$ARCH" == "armv7l" ]]; then
    # 32-bit ARM (older Pi or 32-bit OS)
    RUNNER_ARCH="arm"
else
    echo -e "${RED}Unsupported architecture: $ARCH${NC}"
    exit 1
fi

# Get latest runner version
LATEST_VERSION=$(curl -s https://api.github.com/repos/actions/runner/releases/latest | jq -r '.tag_name' | sed 's/v//')
RUNNER_FILE="actions-runner-linux-${RUNNER_ARCH}-${LATEST_VERSION}.tar.gz"
RUNNER_URL="https://github.com/actions/runner/releases/download/v${LATEST_VERSION}/${RUNNER_FILE}"

echo "Downloading runner version ${LATEST_VERSION} for ${RUNNER_ARCH}..."
curl -L -o runner.tar.gz $RUNNER_URL

echo "Extracting runner..."
tar xzf runner.tar.gz
rm runner.tar.gz

# Step 5: Configure the runner
echo -e "${GREEN}Step 5: Configuring the runner...${NC}"
./config.sh \
    --url "https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}" \
    --token "$RUNNER_TOKEN" \
    --name "$RUNNER_NAME" \
    --labels "self-hosted,Linux,${RUNNER_ARCH},raspberry-pi" \
    --work "_work" \
    --unattended \
    --replace

# Step 6: Install as a service
echo -e "${GREEN}Step 6: Installing runner as a service...${NC}"
sudo ./svc.sh install $RUNNER_USER
sudo ./svc.sh start

# Step 7: Verify installation
echo -e "${GREEN}Step 7: Verifying installation...${NC}"
sleep 3
if sudo ./svc.sh status | grep -q "active (running)"; then
    echo -e "${GREEN}✓ Runner service is running!${NC}"
else
    echo -e "${RED}✗ Runner service failed to start. Check logs with: sudo journalctl -u actions.runner.${GITHUB_OWNER}-${GITHUB_REPO}.${RUNNER_NAME} -f${NC}"
    exit 1
fi

# Step 8: Docker permissions (if Docker is installed)
if command -v docker &> /dev/null; then
    echo -e "${GREEN}Step 8: Setting up Docker permissions...${NC}"
    sudo usermod -aG docker $RUNNER_USER
    echo -e "${YELLOW}Note: You may need to restart the runner service for Docker permissions to take effect:${NC}"
    echo -e "${YELLOW}  sudo ./svc.sh stop && sudo ./svc.sh start${NC}"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✓ GitHub Actions Runner Setup Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Runner Name: $RUNNER_NAME"
echo "Runner Directory: $RUNNER_WORKDIR"
echo "Runner User: $RUNNER_USER"
echo "Repository: https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}"
echo ""
echo "Useful commands:"
echo "  Check status:  sudo $RUNNER_WORKDIR/svc.sh status"
echo "  View logs:     sudo journalctl -u actions.runner.${GITHUB_OWNER}-${GITHUB_REPO}.${RUNNER_NAME} -f"
echo "  Stop runner:   sudo $RUNNER_WORKDIR/svc.sh stop"
echo "  Start runner:  sudo $RUNNER_WORKDIR/svc.sh start"
echo "  Uninstall:     sudo $RUNNER_WORKDIR/svc.sh uninstall"
echo ""
echo -e "${YELLOW}Next step: Update your GitHub Actions workflow to use 'runs-on: [self-hosted, raspberry-pi]'${NC}"