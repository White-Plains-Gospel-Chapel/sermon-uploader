#!/bin/bash
set -e

echo "🔑 Setting up SSH key for pre-deployment checks..."

if [ ! -f ~/.ssh/id_rsa ]; then
  echo "❌ No SSH key found at ~/.ssh/id_rsa"
  echo "💡 Generate one with: ssh-keygen -t rsa -b 4096 -C 'your@email.com'"
  exit 1
fi

echo "📋 Adding SSH key to .env file..."

# Read the private key and escape it properly for .env
SSH_KEY=$(cat ~/.ssh/id_rsa | sed ':a;N;$!ba;s/\n/\\n/g')

# Update .env file
if grep -q "PI_SSH_KEY=" .env; then
  # Replace existing line using a different approach for multiline
  sed -i.bak '/^PI_SSH_KEY=/d' .env
fi

# Add the SSH key to .env file
echo "PI_SSH_KEY=\"$SSH_KEY\"" >> .env

echo "✅ SSH key added to .env file"
echo ""
echo "🧪 Test the setup:"
echo "  ./pre-deploy-check.sh"
echo ""
echo "⚠️ Make sure this key matches your GitHub Secrets PI_SSH_KEY"
echo "💡 Public key to verify: cat ~/.ssh/id_rsa.pub"