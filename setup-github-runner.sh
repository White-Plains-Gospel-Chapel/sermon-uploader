#!/bin/bash

# GitHub Actions Self-Hosted Runner Setup for Raspberry Pi
# This is the industry-standard way to deploy to private infrastructure

set -e

echo "🚀 GitHub Actions Self-Hosted Runner Setup"
echo "=========================================="
echo ""
echo "This script will set up your Raspberry Pi as a self-hosted GitHub Actions runner."
echo "This allows GitHub Actions to deploy directly to your Pi on your private network."
echo ""

# Configuration
RUNNER_VERSION="2.319.1"
RUNNER_ARCH="linux-arm64"  # For Raspberry Pi 4
REPO_OWNER="White-Plains-Gospel-Chapel"
REPO_NAME="sermon-uploader"
RUNNER_NAME="pi-runner-$(hostname)"
RUNNER_WORK_DIR="/home/gaius/actions-runner/_work"

echo "📋 Prerequisites:"
echo "  - GitHub Personal Access Token with 'repo' and 'workflow' permissions"
echo "  - Run this script ON YOUR RASPBERRY PI"
echo ""
read -p "Do you have your GitHub token ready? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Please create a token at: https://github.com/settings/tokens"
    echo "Required scopes: repo, workflow"
    exit 1
fi

echo ""
read -p "Enter your GitHub Personal Access Token: " GITHUB_TOKEN
echo ""

# Step 1: Download and extract runner
echo "📦 Step 1: Downloading GitHub Actions runner..."
cd /home/gaius
mkdir -p actions-runner && cd actions-runner

# Download the latest runner package
curl -o actions-runner-linux-arm64.tar.gz -L \
  https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-${RUNNER_ARCH}-${RUNNER_VERSION}.tar.gz

echo "📂 Extracting runner..."
tar xzf ./actions-runner-linux-arm64.tar.gz
rm actions-runner-linux-arm64.tar.gz

# Step 2: Get registration token
echo ""
echo "🔑 Step 2: Getting registration token..."
REGISTRATION_TOKEN=$(curl -sX POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/actions/runners/registration-token \
  | grep '"token"' | cut -d'"' -f4)

if [ -z "$REGISTRATION_TOKEN" ]; then
    echo "❌ Failed to get registration token. Check your GitHub token permissions."
    exit 1
fi

echo "✅ Got registration token"

# Step 3: Configure the runner
echo ""
echo "⚙️ Step 3: Configuring runner..."
./config.sh --url https://github.com/${REPO_OWNER}/${REPO_NAME} \
  --token ${REGISTRATION_TOKEN} \
  --name ${RUNNER_NAME} \
  --work ${RUNNER_WORK_DIR} \
  --labels "self-hosted,Linux,ARM64,raspberry-pi" \
  --unattended \
  --replace

# Step 4: Install as a service
echo ""
echo "🔧 Step 4: Installing as a service..."
sudo ./svc.sh install
sudo ./svc.sh start

# Step 5: Verify runner is running
echo ""
echo "✅ Step 5: Verifying runner status..."
sudo ./svc.sh status

echo ""
echo "======================================"
echo "✅ Self-Hosted Runner Setup Complete!"
echo "======================================"
echo ""
echo "Your Raspberry Pi is now a GitHub Actions runner!"
echo ""
echo "🎯 What this means:"
echo "  - GitHub Actions can now deploy directly to your Pi"
echo "  - No VPN or port forwarding needed"
echo "  - Workflows run on YOUR hardware"
echo "  - Full access to your local network"
echo ""
echo "📝 To use this runner in your workflows, add:"
echo ""
echo "  jobs:"
echo "    deploy:"
echo "      runs-on: self-hosted  # <-- This uses your Pi!"
echo ""
echo "🔍 Check runner status at:"
echo "  https://github.com/${REPO_OWNER}/${REPO_NAME}/settings/actions/runners"
echo ""
echo "🛑 To stop/start the runner:"
echo "  sudo /home/gaius/actions-runner/svc.sh stop"
echo "  sudo /home/gaius/actions-runner/svc.sh start"
echo ""