#!/bin/bash

# Script to set up GitHub secrets for automated deployment
# Run this locally to configure your repository

echo "🔐 Setting up GitHub Secrets for Automated Deployment"
echo "====================================================="
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed."
    echo "Install it with: brew install gh"
    echo "Then run: gh auth login"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub."
    echo "Run: gh auth login"
    exit 1
fi

echo "This script will set up the following GitHub secrets:"
echo "  - PI_HOST: Your Raspberry Pi IP address"
echo "  - PI_SSH_KEY: SSH private key for Pi access"
echo "  - PI_HOST_KEY: Pi's SSH host key (for known_hosts)"
echo "  - DISCORD_WEBHOOK_URL: Discord webhook for notifications"
echo ""

# Get Pi host
read -p "Enter your Raspberry Pi IP address [192.168.1.127]: " PI_HOST
PI_HOST=${PI_HOST:-192.168.1.127}

# Get or generate SSH key
echo ""
echo "SSH Key Setup:"
echo "1) Use existing SSH key"
echo "2) Generate new SSH key for deployment"
read -p "Choose option (1-2): " ssh_option

case $ssh_option in
    1)
        read -p "Enter path to private key [~/.ssh/id_rsa]: " KEY_PATH
        KEY_PATH=${KEY_PATH:-~/.ssh/id_rsa}
        
        if [ ! -f "$KEY_PATH" ]; then
            echo "❌ Key file not found: $KEY_PATH"
            exit 1
        fi
        
        SSH_KEY=$(cat "$KEY_PATH")
        ;;
    2)
        echo "Generating new SSH key pair..."
        ssh-keygen -t rsa -b 4096 -f /tmp/deploy_key -N "" -C "github-actions-deploy"
        SSH_KEY=$(cat /tmp/deploy_key)
        
        echo ""
        echo "📋 Add this public key to your Pi's ~/.ssh/authorized_keys:"
        echo "=================================================="
        cat /tmp/deploy_key.pub
        echo "=================================================="
        echo ""
        read -p "Press Enter after adding the key to your Pi..."
        
        rm /tmp/deploy_key /tmp/deploy_key.pub
        ;;
esac

# Get Pi's host key
echo ""
echo "Getting Pi's SSH host key..."
HOST_KEY=$(ssh-keyscan -t rsa $PI_HOST 2>/dev/null | grep ssh-rsa | cut -d' ' -f3)

if [ -z "$HOST_KEY" ]; then
    echo "⚠️  Could not retrieve host key automatically."
    echo "Run this command and paste the result:"
    echo "  ssh-keyscan -t rsa $PI_HOST | grep ssh-rsa | cut -d' ' -f3"
    read -p "Host key: " HOST_KEY
fi

# Get Discord webhook
echo ""
read -p "Enter Discord webhook URL (or press Enter to use default): " DISCORD_URL
DISCORD_URL=${DISCORD_URL:-"https://discord.com/api/webhooks/1410698516891701400/Ve6k3d8sdd54kf0II1xFc7H6YkYLoWiPFDEe5NsHsmX4Qv6l4CNzD4rMmdlWPQxLnRPT"}

# Set GitHub secrets
echo ""
echo "Setting GitHub secrets..."

gh secret set PI_HOST --body "$PI_HOST"
echo "✅ Set PI_HOST"

gh secret set PI_SSH_KEY --body "$SSH_KEY"
echo "✅ Set PI_SSH_KEY"

gh secret set PI_HOST_KEY --body "$HOST_KEY"
echo "✅ Set PI_HOST_KEY"

gh secret set DISCORD_WEBHOOK_URL --body "$DISCORD_URL"
echo "✅ Set DISCORD_WEBHOOK_URL"

echo ""
echo "====================================================="
echo "✅ GitHub Secrets configured successfully!"
echo "====================================================="
echo ""
echo "You can now:"
echo "1. Push code to trigger automatic deployment"
echo "2. Manually trigger deployment from GitHub Actions tab"
echo "3. Monitor deployments in Discord"
echo ""
echo "To test deployment:"
echo "  git add ."
echo "  git commit -m 'Deploy multipart upload system'"
echo "  git push"
echo ""
echo "Or trigger manually:"
echo "  gh workflow run deploy-to-pi.yml"