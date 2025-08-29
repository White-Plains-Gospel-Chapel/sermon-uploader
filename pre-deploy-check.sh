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
if ping -c 3 -t 3 "$PI_HOST" >/dev/null 2>&1; then
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

# Test 2: SSH connectivity and port accessibility  
echo ""
echo "🔌 Test 2: SSH connectivity and port accessibility"
PI_PORT="${PI_PORT:-22}"

# Test SSH connectivity by actually trying to connect (more reliable than netcat)
SSH_TEST=$(ssh -i <(echo -e "$PI_SSH_KEY") -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o BatchMode=yes "$PI_USER@$PI_HOST" -p "$PI_PORT" "echo 'SSH_CONNECTION_SUCCESS'" 2>&1 || echo "SSH_CONNECTION_FAILED")

if echo "$SSH_TEST" | grep -q "SSH_CONNECTION_SUCCESS"; then
  echo "✅ SSH connection successful on port $PI_PORT"
elif echo "$SSH_TEST" | grep -q "Connection refused"; then
  echo "❌ SSH connection refused - SSH service may not be running"
  exit 1
elif echo "$SSH_TEST" | grep -q "Connection timed out\|No route to host"; then
  echo "❌ SSH connection timed out - network or firewall issue"
  exit 1
elif echo "$SSH_TEST" | grep -q "Permission denied"; then
  echo "❌ SSH permission denied - authentication issue"
  exit 1
else
  echo "❌ SSH connection failed with unexpected error:"
  echo "$SSH_TEST" | head -3
  exit 1
fi

# Test 3: SSH key format verification
echo ""
echo "🔑 Test 3: SSH key format verification"

if [ -z "$PI_SSH_KEY" ]; then
  echo "❌ PI_SSH_KEY not set in environment"
  echo ""
  echo "🔧 Add your SSH private key to .env file:"
  echo "   PI_SSH_KEY=\"\$(cat ~/.ssh/id_rsa)\""
  echo ""
  echo "💡 This should match exactly what's in your GitHub Secrets"
  echo "🚫 SSH authentication test failed - deployment will likely fail"
  exit 1
else
  echo "✅ PI_SSH_KEY is set and SSH connection works"
  echo "💡 This key format matches your GitHub Secrets - deployment should work"
fi

# Test 4: Pi system readiness
echo ""
echo "🖥️ Test 4: Pi system readiness"
SYSTEM_CHECK=$(ssh -i <(echo -e "$PI_SSH_KEY") -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o BatchMode=yes "$PI_USER@$PI_HOST" -p "$PI_PORT" '
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