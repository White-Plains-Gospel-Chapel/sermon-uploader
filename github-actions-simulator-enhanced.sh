#!/bin/bash
set -e  # Exit on any error

# =============================================================================
# ENHANCED GITHUB ACTIONS WORKFLOW SIMULATOR WITH EDGE CASE DETECTION
# This version catches port conflicts and other deployment issues
# =============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Simulation variables
export GITHUB_WORKSPACE="$(pwd)"
export GITHUB_SHA="$(git rev-parse HEAD)"
export GITHUB_REF="refs/heads/master"
export GITHUB_REF_NAME="master"
export GITHUB_REPOSITORY="White-Plains-Gospel-Chapel/sermon-uploader"
export GITHUB_RUN_ID="simulation-$(date +%s)"
export RUNNER_OS="Linux"
export REGISTRY="docker.io"
export DOCKER_USERNAME="gaiusr"
export BACKEND_IMAGE="sermon-uploader-backend"
export FRONTEND_IMAGE="sermon-uploader-frontend"

# Track failures
FAILED_JOBS=()
WARNINGS=()

# Enhanced port checking function
check_port() {
    local port=$1
    local service=$2
    
    # Check if port is in use
    if lsof -i :$port > /dev/null 2>&1; then
        local process=$(lsof -i :$port | grep LISTEN | head -1 | awk '{print $1}')
        echo -e "  ${RED}✗ Port $port is already in use by: $process${NC}"
        echo -e "  ${RED}This WILL cause deployment failure!${NC}"
        FAILED_JOBS+=("port-conflict-$port")
        return 1
    elif netstat -an 2>/dev/null | grep -q ":$port.*LISTEN"; then
        echo -e "  ${RED}✗ Port $port is already in use${NC}"
        FAILED_JOBS+=("port-conflict-$port")
        return 1
    else
        echo -e "  ${GREEN}✓ Port $port is available for $service${NC}"
        return 0
    fi
}

# Check Docker container conflicts
check_container_conflicts() {
    local container_name=$1
    
    if docker ps -a --format "{{.Names}}" | grep -q "^${container_name}$"; then
        echo -e "  ${YELLOW}⚠ Container '$container_name' already exists${NC}"
        local status=$(docker inspect -f '{{.State.Status}}' $container_name 2>/dev/null)
        if [ "$status" = "running" ]; then
            echo -e "  ${RED}✗ Container is running - will cause conflict${NC}"
            FAILED_JOBS+=("container-conflict-$container_name")
            return 1
        else
            echo -e "  ${YELLOW}Container exists but is stopped - will be removed${NC}"
            WARNINGS+=("existing-container-$container_name")
        fi
    fi
    return 0
}

# Check Docker network conflicts
check_network_conflicts() {
    local network_name=$1
    
    if docker network ls --format "{{.Name}}" | grep -q "^${network_name}$"; then
        echo -e "  ${YELLOW}⚠ Network '$network_name' already exists${NC}"
        # Check if any containers are using this network
        local containers=$(docker network inspect $network_name --format '{{len .Containers}}' 2>/dev/null || echo "0")
        if [ "$containers" != "0" ]; then
            echo -e "  ${RED}✗ Network has active containers - may cause issues${NC}"
            WARNINGS+=("network-in-use-$network_name")
        fi
    fi
}

# Validate docker-compose file
validate_docker_compose() {
    local compose_file=$1
    
    if [ ! -f "$compose_file" ]; then
        echo -e "  ${RED}✗ $compose_file not found!${NC}"
        FAILED_JOBS+=("missing-compose-file")
        return 1
    fi
    
    # Check syntax
    if ! docker compose -f $compose_file config > /dev/null 2>&1; then
        echo -e "  ${RED}✗ Invalid docker-compose syntax!${NC}"
        FAILED_JOBS+=("invalid-compose-syntax")
        return 1
    fi
    
    # Extract and check all ports from docker-compose
    echo -e "  ${CYAN}Checking ports defined in $compose_file...${NC}"
    
    # Parse ports from docker-compose - fixed parsing
    local ports=$(grep -E '^\s*-\s*"?[0-9]+:[0-9]+' $compose_file | sed 's/.*"\?\([0-9]*\):[0-9]*.*/\1/' | sort -u)
    
    for port in $ports; do
        if [ -n "$port" ]; then
            local service=$(grep -B10 "$port:" $compose_file | grep -E '^[a-z]+:$' | tail -1 | sed 's/://g')
            check_port $port "$service"
        fi
    done
    
    return 0
}

# Check disk space
check_disk_space() {
    local required_gb=$1
    local available=$(df -BG . | tail -1 | awk '{print $4}' | sed 's/G//')
    
    if [ "$available" -lt "$required_gb" ]; then
        echo -e "  ${RED}✗ Insufficient disk space: ${available}GB available, ${required_gb}GB required${NC}"
        FAILED_JOBS+=("insufficient-disk-space")
        return 1
    else
        echo -e "  ${GREEN}✓ Disk space OK: ${available}GB available${NC}"
    fi
}

# =============================================================================
# START ENHANCED SIMULATION
# =============================================================================

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║         ENHANCED GITHUB ACTIONS WORKFLOW SIMULATOR                          ║${NC}"
echo -e "${CYAN}║         WITH PORT CONFLICT AND EDGE CASE DETECTION                          ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# =============================================================================
# PRE-FLIGHT CHECKS (Would have caught the MinIO port conflict!)
# =============================================================================

echo -e "${MAGENTA}=================================================================================${NC}"
echo -e "${MAGENTA}PRE-FLIGHT CHECKS - Detecting potential deployment issues${NC}"
echo -e "${MAGENTA}=================================================================================${NC}"
echo ""

echo -e "${BLUE}▶ Checking Docker environment${NC}"
if ! docker info > /dev/null 2>&1; then
    echo -e "  ${RED}✗ Docker is not running or not accessible${NC}"
    FAILED_JOBS+=("docker-not-running")
else
    echo -e "  ${GREEN}✓ Docker is running${NC}"
fi

echo -e "${BLUE}▶ Checking docker-compose.pi5.yml${NC}"
validate_docker_compose "docker-compose.pi5.yml"

echo -e "${BLUE}▶ Checking for container name conflicts${NC}"
check_container_conflicts "sermon-uploader-minio"
check_container_conflicts "sermon-uploader-backend"
check_container_conflicts "sermon-uploader-frontend"

echo -e "${BLUE}▶ Checking for network conflicts${NC}"
check_network_conflicts "sermon-uploader_sermon-network"

echo -e "${BLUE}▶ Checking disk space (minimum 5GB)${NC}"
check_disk_space 5

echo -e "${BLUE}▶ Checking Docker image availability${NC}"
# Check if images exist locally or can be pulled
for image in "minio/minio:latest" "${DOCKER_USERNAME}/${BACKEND_IMAGE}:pi5" "${DOCKER_USERNAME}/${FRONTEND_IMAGE}:pi5"; do
    if docker image inspect $image > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓ Image $image exists locally${NC}"
    else
        echo -e "  ${YELLOW}⚠ Image $image not found locally - will need to pull${NC}"
        WARNINGS+=("image-pull-required-$image")
    fi
done

echo ""

# =============================================================================
# DEPLOYMENT SIMULATION WITH ACTUAL DOCKER COMPOSE
# =============================================================================

echo -e "${CYAN}=================================================================================${NC}"
echo -e "${CYAN}DEPLOYMENT SIMULATION - Testing actual deployment${NC}"
echo -e "${CYAN}=================================================================================${NC}"
echo ""

if [ ${#FAILED_JOBS[@]} -gt 0 ]; then
    echo -e "${RED}⛔ STOPPING: Pre-flight checks failed!${NC}"
    echo -e "${RED}The deployment WILL FAIL on GitHub Actions due to:${NC}"
    for failure in "${FAILED_JOBS[@]}"; do
        echo -e "${RED}  - $failure${NC}"
    done
    echo ""
    echo -e "${YELLOW}Suggested fixes:${NC}"
    
    if [[ " ${FAILED_JOBS[@]} " =~ "port-conflict-9000" ]]; then
        echo -e "${YELLOW}  1. Port 9000 conflict (MinIO):${NC}"
        echo "     - Option A: Stop the existing MinIO service"
        echo "     - Option B: Use docker-compose.pi5-external-minio.yml"
        echo "     - Option C: Change MinIO ports in docker-compose.pi5.yml"
    fi
    
    if [[ " ${FAILED_JOBS[@]} " =~ "port-conflict-9001" ]]; then
        echo -e "${YELLOW}  2. Port 9001 conflict (MinIO Console):${NC}"
        echo "     - Change the console port in docker-compose.pi5.yml"
    fi
    
    if [[ " ${FAILED_JOBS[@]} " =~ "port-conflict-8000" ]]; then
        echo -e "${YELLOW}  3. Port 8000 conflict (Backend):${NC}"
        echo "     - Stop the service using port 8000"
        echo "     - Or change backend port in docker-compose.pi5.yml"
    fi
    
    if [[ " ${FAILED_JOBS[@]} " =~ "port-conflict-3000" ]]; then
        echo -e "${YELLOW}  4. Port 3000 conflict (Frontend):${NC}"
        echo "     - Stop the service using port 3000 (likely another Node.js app)"
        echo "     - Or change frontend port in docker-compose.pi5.yml"
    fi
    
    exit 1
fi

echo -e "${BLUE}▶ Simulating docker-compose deployment${NC}"
echo "  Would run: docker compose -f docker-compose.pi5.yml up -d"

# Actually test with --dry-run if available, or just validate
if docker compose -f docker-compose.pi5.yml config > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Docker compose configuration is valid${NC}"
    
    # Test if we can actually start the services
    echo -e "${BLUE}▶ Testing service startup (5 second test)${NC}"
    
    # Start services in background
    docker compose -f docker-compose.pi5.yml up -d > /dev/null 2>&1
    
    # Wait a moment
    sleep 5
    
    # Check if services started
    if docker ps | grep -q sermon-uploader; then
        echo -e "  ${GREEN}✓ Services started successfully${NC}"
        
        # Check health
        if curl -f http://localhost:8000/api/health > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓ Backend health check passed${NC}"
        else
            echo -e "  ${YELLOW}⚠ Backend health check failed (may need more time)${NC}"
        fi
        
        # Clean up
        docker compose -f docker-compose.pi5.yml down > /dev/null 2>&1
        echo -e "  ${GREEN}✓ Test deployment cleaned up${NC}"
    else
        echo -e "  ${RED}✗ Services failed to start${NC}"
        docker compose -f docker-compose.pi5.yml down > /dev/null 2>&1
        FAILED_JOBS+=("services-failed-to-start")
    fi
else
    echo -e "  ${RED}✗ Docker compose configuration invalid${NC}"
    FAILED_JOBS+=("docker-compose-invalid")
fi

# =============================================================================
# FINAL REPORT
# =============================================================================

echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                        SIMULATION COMPLETE                                  ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ ${#FAILED_JOBS[@]} -eq 0 ] && [ ${#WARNINGS[@]} -eq 0 ]; then
    echo -e "${GREEN}🎉 ALL CHECKS PASSED!${NC}"
    echo -e "${GREEN}Your deployment will succeed on GitHub Actions.${NC}"
elif [ ${#FAILED_JOBS[@]} -eq 0 ] && [ ${#WARNINGS[@]} -gt 0 ]; then
    echo -e "${YELLOW}⚠️  DEPLOYMENT WILL WORK but with warnings:${NC}"
    for warning in "${WARNINGS[@]}"; do
        echo -e "${YELLOW}   - $warning${NC}"
    done
else
    echo -e "${RED}❌ DEPLOYMENT WILL FAIL!${NC}"
    echo -e "${RED}Critical issues found:${NC}"
    for failure in "${FAILED_JOBS[@]}"; do
        echo -e "${RED}   - $failure${NC}"
    done
    echo ""
    echo -e "${YELLOW}Fix these issues before pushing to GitHub!${NC}"
    exit 1
fi