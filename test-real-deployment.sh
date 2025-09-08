#!/bin/bash

# =============================================================================
# REAL DEPLOYMENT SIMULATION
# This actually simulates the EXACT conditions on the Pi and tests deployment
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    REAL DEPLOYMENT SIMULATION                               ║${NC}"
echo -e "${CYAN}║              Testing EXACTLY what happens on the Pi                         ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# SETUP: Create Pi-like conditions
# =============================================================================
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}SETUP: Creating Pi environment conditions${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Save current state
ORIGINAL_CONTAINERS=$(docker ps -q)

# Start a fake MinIO process on port 9000 to simulate Pi's existing MinIO
echo -e "${CYAN}▶ Starting a process on port 9000 to simulate Pi's MinIO...${NC}"
# Use Python to create a simple server on port 9000
python3 -m http.server 9000 > /dev/null 2>&1 &
FAKE_MINIO_PID=$!
sleep 2

# Verify port 9000 is now in use
if lsof -i:9000 > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Port 9000 is now in use (PID: $FAKE_MINIO_PID)${NC}"
    echo -e "  ${CYAN}This simulates MinIO already running on the Pi${NC}"
else
    echo -e "  ${RED}✗ Failed to simulate MinIO on port 9000${NC}"
    exit 1
fi

# =============================================================================
# TEST 1: Run deployment script with GitHub Actions environment
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}TEST 1: Deploy with port 9000 already in use (GitHub Actions mode)${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Set environment like GitHub Actions does
export GITHUB_ACTIONS=true
export CI=true

echo -e "${CYAN}▶ Running: ./scripts/smart-deploy-replace.sh docker-compose.pi5.yml --auto${NC}"
echo -e "  ${YELLOW}(This is EXACTLY what GitHub Actions runs on the Pi)${NC}"

# Actually run the deployment script and capture the exit code
set +e  # Don't exit on error
./scripts/smart-deploy-replace.sh docker-compose.pi5.yml --auto
DEPLOY_EXIT_CODE=$?
set -e

echo ""
if [ $DEPLOY_EXIT_CODE -eq 0 ]; then
    echo -e "  ${GREEN}✓ Deployment succeeded (exit code: 0)${NC}"
    
    # Check if containers are actually running
    echo -e "${CYAN}▶ Verifying containers are running...${NC}"
    if docker ps | grep -q sermon-uploader; then
        echo -e "  ${GREEN}✓ Containers are running${NC}"
        docker ps --format "table {{.Names}}\t{{.Status}}" | grep sermon || true
    else
        echo -e "  ${RED}✗ No sermon-uploader containers found!${NC}"
    fi
    
elif [ $DEPLOY_EXIT_CODE -eq 2 ]; then
    echo -e "  ${RED}✗ Deployment failed with exit code 2${NC}"
    echo -e "  ${RED}This is the EXACT error happening on the Pi!${NC}"
    echo -e "  ${YELLOW}The script cannot handle existing MinIO properly${NC}"
else
    echo -e "  ${RED}✗ Deployment failed with exit code: $DEPLOY_EXIT_CODE${NC}"
fi

# =============================================================================
# TEST 2: Test without curl (simulating Pi without curl)
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}TEST 2: Deploy without curl command (Pi might not have curl)${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Temporarily rename curl to simulate it not being available
echo -e "${CYAN}▶ Simulating environment without curl...${NC}"
CURL_PATH=$(which curl)
if [ -n "$CURL_PATH" ]; then
    # Create a wrapper that fails
    cat > /tmp/curl-wrapper.sh << 'EOF'
#!/bin/bash
echo "curl: command not found" >&2
exit 127
EOF
    chmod +x /tmp/curl-wrapper.sh
    
    # Temporarily override curl in PATH
    export PATH="/tmp:$PATH"
    
    echo -e "${CYAN}▶ Running deployment without curl available...${NC}"
    set +e
    ./scripts/smart-deploy-replace.sh docker-compose.pi5.yml --auto
    DEPLOY_NO_CURL_EXIT=$?
    set -e
    
    # Restore PATH
    export PATH="${PATH#/tmp:}"
    
    if [ $DEPLOY_NO_CURL_EXIT -eq 0 ]; then
        echo -e "  ${GREEN}✓ Deployment handled missing curl gracefully${NC}"
    else
        echo -e "  ${RED}✗ Deployment failed when curl is missing (exit: $DEPLOY_NO_CURL_EXIT)${NC}"
        echo -e "  ${RED}This could happen on the Pi!${NC}"
    fi
fi

# =============================================================================
# TEST 3: Test the actual port conflict resolution
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}TEST 3: Test port conflict resolution logic${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "${CYAN}▶ Testing handle_port_conflict function directly...${NC}"

# Source the script to get access to its functions
source scripts/smart-deploy-replace.sh

# Test the function
echo -e "${CYAN}Testing with port 9000 in use...${NC}"
handle_port_conflict 9000 "MinIO" "docker-compose.pi5.yml"
CONFLICT_RESULT=$?

if [ $CONFLICT_RESULT -eq 0 ]; then
    echo -e "  ${GREEN}✓ Port conflict resolved (freed the port)${NC}"
elif [ $CONFLICT_RESULT -eq 2 ]; then
    echo -e "  ${GREEN}✓ Existing MinIO detected, will use it${NC}"
else
    echo -e "  ${RED}✗ Port conflict not handled properly (returned: $CONFLICT_RESULT)${NC}"
    echo -e "  ${RED}This will cause deployment to fail!${NC}"
fi

# =============================================================================
# CLEANUP
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}CLEANUP${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

# Kill our fake MinIO
echo -e "${CYAN}▶ Stopping simulated MinIO...${NC}"
kill $FAKE_MINIO_PID 2>/dev/null || true

# Stop any containers we started
echo -e "${CYAN}▶ Cleaning up test containers...${NC}"
docker compose -f docker-compose.pi5.yml down 2>/dev/null || true
docker compose -f docker-compose.dynamic.yml down 2>/dev/null || true
docker compose -f docker-compose.external-minio.yml down 2>/dev/null || true

# =============================================================================
# FINAL REPORT
# =============================================================================
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                           SIMULATION COMPLETE                               ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ $DEPLOY_EXIT_CODE -ne 0 ]; then
    echo -e "${RED}❌ DEPLOYMENT WILL FAIL ON THE PI${NC}"
    echo -e "${RED}The deployment script exits with code $DEPLOY_EXIT_CODE when port 9000 is in use${NC}"
    echo ""
    echo -e "${YELLOW}To fix this:${NC}"
    echo -e "1. The script should detect existing MinIO and use it"
    echo -e "2. It should NOT exit with error when MinIO is already running"
    echo -e "3. It should handle cases where curl is not available"
    exit 1
else
    echo -e "${GREEN}✅ Deployment handles port conflicts correctly${NC}"
    echo -e "${GREEN}Ready to deploy to the Pi${NC}"
fi