#!/bin/bash
set -e

echo "🔍 Pre-deployment verification - catching issues before GitHub Actions"
echo "=================================================================="

# Load environment variables if .env exists
if [ -f ".env" ]; then
  echo "📋 Loading environment variables from .env..."
  # Check if .env has any syntax issues before sourcing
  if bash -n .env 2>/dev/null; then
    set -a
    source .env 2>/dev/null || {
      echo "⚠️ Error loading .env file, using environment variables instead"
    }
    set +a
  else
    echo "⚠️ .env file has syntax errors, using environment variables instead"
  fi
else
  echo "⚠️ No .env file found - using environment variables"
fi

# Check required variables
echo ""
echo "🔧 Checking required environment variables..."
REQUIRED_VARS=("PI_HOST" "PI_USER" "PI_SSH_KEY" "MINIO_ACCESS_KEY" "MINIO_SECRET_KEY" "DISCORD_WEBHOOK_URL")
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var}" ]; then
    MISSING_VARS+=("$var")
    echo "❌ $var is not set"
  else
    echo "✅ $var is set"
  fi
done

if [ ${#MISSING_VARS[@]} -ne 0 ]; then
  echo ""
  if [[ " ${MISSING_VARS[*]} " == *" PI_SSH_KEY "* ]]; then
    echo "⚠️ PI_SSH_KEY missing - will skip SSH authentication test"
    echo "💡 SSH key is in GitHub Secrets and will work for deployment"
    echo "💡 To enable full verification, export your SSH key:"
    echo "    export PI_SSH_KEY=\"\$(cat ~/.ssh/id_rsa)\""
    echo ""
    # Remove PI_SSH_KEY from missing vars for other checks
    MISSING_VARS=($(printf '%s\n' "${MISSING_VARS[@]}" | grep -v '^PI_SSH_KEY$'))
  fi
  
  if [ ${#MISSING_VARS[@]} -ne 0 ]; then
    echo "❌ Missing required environment variables: ${MISSING_VARS[*]}"
    echo "💡 Add these to your .env file or export them as environment variables"
    exit 1
  fi
fi

echo ""
echo "🖥️ Pi connectivity tests..."

# Test 1: Basic network connectivity
echo "📡 Test 1: Network connectivity (ping)"
if ping -c 3 -W 3 "$PI_HOST" >/dev/null 2>&1; then
  echo "✅ Pi responds to ping - network connectivity OK"
else
  echo "❌ Pi does not respond to ping"
  echo "💡 Possible issues:"
  echo "   - Pi is powered off"
  echo "   - Network connection issue"
  echo "   - Pi IP address changed ($PI_HOST)"
  echo "   - Firewall blocking ICMP"
  echo ""
  echo "🔧 Suggestions:"
  echo "   1. Check if Pi is powered on and network cable connected"
  echo "   2. Verify Pi IP address: ssh $PI_USER@$PI_HOST 'hostname -I'"
  echo "   3. Check router/network settings"
  exit 1
fi

# Test 2: SSH port accessibility
echo ""
echo "🔌 Test 2: SSH port accessibility"
PI_PORT="${PI_PORT:-22}"
if timeout 10 nc -z "$PI_HOST" "$PI_PORT" >/dev/null 2>&1; then
  echo "✅ SSH port $PI_PORT is open and accessible"
else
  echo "❌ SSH port $PI_PORT is not accessible"
  
  # Try common SSH ports
  echo "🔍 Checking common SSH ports..."
  FOUND_PORT=""
  for port in 22 2222 22000; do
    if timeout 5 nc -z "$PI_HOST" "$port" >/dev/null 2>&1; then
      echo "✅ Found SSH service on port $port"
      FOUND_PORT="$port"
      break
    else
      echo "❌ No SSH service on port $port"
    fi
  done
  
  if [ -n "$FOUND_PORT" ]; then
    echo ""
    echo "💡 SSH is running on port $FOUND_PORT instead of $PI_PORT"
    echo "🔧 Update your PI_PORT environment variable to $FOUND_PORT"
    exit 1
  else
    echo ""
    echo "💡 No SSH service found on common ports"
    echo "🔧 Suggestions:"
    echo "   1. SSH into Pi manually: ssh $PI_USER@$PI_HOST"
    echo "   2. Start SSH service: sudo systemctl start ssh"
    echo "   3. Enable SSH: sudo systemctl enable ssh"
    echo "   4. Check SSH config: sudo systemctl status ssh"
    exit 1
  fi
fi

# Test 3: SSH key authentication
echo ""
echo "🔑 Test 3: SSH key authentication"

if [ -z "$PI_SSH_KEY" ]; then
  echo "⚠️ PI_SSH_KEY not set - skipping authentication test"
  echo "💡 SSH key is configured in GitHub Secrets for deployment"
  echo "💡 Assuming authentication will work based on GitHub Secrets"
else
  # Create temporary key file
  TEMP_KEY_FILE=$(mktemp)
  echo "$PI_SSH_KEY" > "$TEMP_KEY_FILE"
  chmod 600 "$TEMP_KEY_FILE"

  # Test SSH connection
  SSH_OUTPUT=$(timeout 15 ssh -i "$TEMP_KEY_FILE" -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o PasswordAuthentication=no -v "$PI_USER@$PI_HOST" -p "$PI_PORT" exit 2>&1 || true)

  rm -f "$TEMP_KEY_FILE"

  if echo "$SSH_OUTPUT" | grep -q "Authentication succeeded"; then
    echo "✅ SSH key authentication successful"
  elif echo "$SSH_OUTPUT" | grep -q "Permission denied"; then
    echo "❌ SSH key authentication failed"
    echo ""
    echo "💡 Possible issues:"
    echo "   - SSH key is incorrect or not authorized"
    echo "   - Key format might be wrong (needs to be OpenSSH format)"
    echo "   - Pi user account issues"
    echo "   - SSH key not added to Pi's ~/.ssh/authorized_keys"
    echo ""
    echo "🔧 Suggestions:"
    echo "   1. Copy your public key to Pi: ssh-copy-id $PI_USER@$PI_HOST"
    echo "   2. Check authorized_keys: ssh $PI_USER@$PI_HOST 'cat ~/.ssh/authorized_keys'"
    echo "   3. Verify key format matches what's in GitHub Secrets"
    exit 1
  else
    echo "❌ SSH connection issue"
    echo "SSH output (last 10 lines):"
    echo "$SSH_OUTPUT" | tail -10
    exit 1
  fi
fi

# Test 4: Pi system readiness
echo ""
echo "🖥️ Test 4: Pi system readiness"
SYSTEM_CHECK=$(timeout 30 ssh -i <(echo "$PI_SSH_KEY") -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" -p "$PI_PORT" '
  echo "=== System Info ==="
  echo "Hostname: $(hostname)"
  echo "Uptime: $(uptime)"
  echo "Disk space: $(df -h / | tail -1 | awk "{print \$5\" used, \"\$4\" available\"}")"
  echo "Memory: $(free -h | grep Mem | awk "{print \$3\"/\"\$2\" used\"}")"
  echo ""
  echo "=== Docker Status ==="
  if command -v docker >/dev/null 2>&1; then
    echo "Docker installed: $(docker --version)"
    if docker info >/dev/null 2>&1; then
      echo "Docker daemon: Running"
      echo "Running containers: $(docker ps --format "table {{.Names}}\t{{.Status}}" | tail -n +2 | wc -l)"
    else
      echo "Docker daemon: Not running"
    fi
  else
    echo "Docker: Not installed"
  fi
  echo ""
  echo "=== Project Directory ==="
  if [ -d "/opt/sermon-uploader" ]; then
    echo "Project dir: Exists"
    cd /opt/sermon-uploader
    echo "Git status: $(git status --porcelain | wc -l) changes"
    echo "Current branch: $(git branch --show-current 2>/dev/null || echo 'unknown')"
    echo "Last commit: $(git log -1 --format='%h %s' 2>/dev/null || echo 'none')"
  else
    echo "Project dir: Missing (/opt/sermon-uploader)"
  fi
' 2>&1 || echo "Failed to connect to Pi")

if echo "$SYSTEM_CHECK" | grep -q "Hostname:"; then
  echo "✅ Pi system check completed"
  echo "$SYSTEM_CHECK"
else
  echo "❌ Failed to check Pi system status"
  echo "$SYSTEM_CHECK"
  exit 1
fi

# Test 5: Docker and project readiness
echo ""
echo "🐳 Test 5: Docker and deployment readiness"
if echo "$SYSTEM_CHECK" | grep -q "Docker daemon: Running"; then
  echo "✅ Docker is running on Pi"
else
  echo "❌ Docker is not running on Pi"
  echo "🔧 Fix: ssh $PI_USER@$PI_HOST 'sudo systemctl start docker'"
  exit 1
fi

if echo "$SYSTEM_CHECK" | grep -q "Project dir: Exists"; then
  echo "✅ Project directory exists on Pi"
else
  echo "❌ Project directory missing on Pi"
  echo "🔧 Fix: ssh $PI_USER@$PI_HOST 'sudo mkdir -p /opt/sermon-uploader && sudo chown $USER:$USER /opt/sermon-uploader'"
  exit 1
fi

# Test 6: GitHub Container Registry access
echo ""
echo "📦 Test 6: GitHub Container Registry access"
if timeout 10 docker pull ghcr.io/white-plains-gospel-chapel/sermon-uploader:latest >/dev/null 2>&1; then
  echo "✅ Can pull latest container image"
else
  echo "⚠️ Cannot pull latest container image (will be built during deployment)"
  echo "💡 This is normal for first deployments or when no image exists yet"
fi

echo ""
echo "✅ All pre-deployment checks passed!"
echo ""
echo "🚀 Ready to deploy to Pi at $PI_HOST"
echo "💰 GitHub Actions deployment should succeed (saving costs!)"
echo ""
echo "To deploy, run:"
echo "  git push origin master"
echo ""
echo "To watch deployment:"
echo "  ./watch-deployment.sh"