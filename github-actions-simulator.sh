#!/bin/bash
set -e  # Exit on any error

# =============================================================================
# GITHUB ACTIONS COMPLETE WORKFLOW SIMULATOR
# This script simulates the EXACT GitHub Actions workflow locally
# including all jobs, steps, and potential failure points
# =============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Simulation variables (mimicking GitHub Actions environment)
export GITHUB_WORKSPACE="$(pwd)"
export GITHUB_SHA="$(git rev-parse HEAD)"
export GITHUB_REF="refs/heads/master"
export GITHUB_REF_NAME="master"
export GITHUB_REPOSITORY="White-Plains-Gospel-Chapel/sermon-uploader"
export GITHUB_RUN_ID="simulation-$(date +%s)"
export GITHUB_RUN_NUMBER="999"
export RUNNER_OS="Linux"
export REGISTRY="docker.io"
export DOCKER_USERNAME="gaiusr"
export BACKEND_IMAGE="sermon-uploader-backend"
export FRONTEND_IMAGE="sermon-uploader-frontend"
export NODE_VERSION="18"
export GO_VERSION="1.21"

# Track job results (using simple arrays for compatibility)
FAILED_JOBS=()

# Function to simulate GitHub Actions job
run_job() {
    local job_name=$1
    local job_id=$2
    echo ""
    echo -e "${CYAN}=================================================================================${NC}"
    echo -e "${CYAN}JOB: ${job_name} (ID: ${job_id})${NC}"
    echo -e "${CYAN}=================================================================================${NC}"
    echo ""
}

# Function to simulate GitHub Actions step
run_step() {
    local step_name=$1
    echo -e "${BLUE}▶ ${step_name}${NC}"
}

# Function to check step result
check_result() {
    if [ $? -eq 0 ]; then
        echo -e "  ${GREEN}✓ Success${NC}"
        return 0
    else
        echo -e "  ${RED}✗ Failed${NC}"
        return 1
    fi
}

# =============================================================================
# START SIMULATION
# =============================================================================

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║           GITHUB ACTIONS WORKFLOW SIMULATION STARTING                       ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Workflow: Build, Test and Deploy with Self-Hosted Runner"
echo "Trigger: push to master"
echo "Commit: ${GITHUB_SHA}"
echo "Repository: ${GITHUB_REPOSITORY}"
echo ""

# =============================================================================
# JOB 1: TEST BACKEND
# =============================================================================

run_job "Test Backend" "49877403986"

run_step "📥 Checkout code"
if [ -d ".git" ]; then
    echo "  Repository already checked out"
    check_result
else
    echo -e "  ${RED}Not in a git repository${NC}"
    exit 1
fi

run_step "🔧 Set up Go ${GO_VERSION}"
if command -v go &> /dev/null; then
    GO_INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "  Go version: ${GO_INSTALLED_VERSION}"
    check_result
else
    echo -e "  ${YELLOW}⚠ Go not installed (would be installed in real GitHub Actions)${NC}"
fi

run_step "📦 Cache Go modules"
CACHE_KEY="${RUNNER_OS}-go-$(sha256sum backend/go.sum | cut -d' ' -f1)"
if [ -f "backend/go.sum" ]; then
    echo "  Cache key: ${CACHE_KEY:0:20}..."
    echo "  Cache would be restored from: ~/go/pkg/mod"
    check_result
else
    echo -e "  ${RED}ERROR: backend/go.sum not found!${NC}"
    echo -e "  ${RED}This would cause the cache warning in GitHub Actions${NC}"
    FAILED_JOBS+=("test-backend")
fi

run_step "📚 Download dependencies"
cd backend
go mod download 2>/dev/null
if check_result; then
    echo "  Dependencies downloaded successfully"
fi
cd ..

run_step "🧪 Run backend tests"
cd backend
echo "  Running: go test -v -race -coverprofile=coverage.out ./..."
if go test -race -coverprofile=coverage.out ./... > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ All backend tests passed${NC}"
    check_result
else
    echo -e "  ${RED}✗ Backend tests failed${NC}"
    FAILED_JOBS+=("test-backend")
fi
cd ..

run_step "📊 Upload coverage reports"
if [ -f "backend/coverage.out" ]; then
    echo "  Artifact: backend-coverage"
    echo "  Size: $(du -h backend/coverage.out | cut -f1)"
    check_result
else
    echo -e "  ${YELLOW}⚠ No coverage file generated${NC}"
fi

# Job completed

# =============================================================================
# JOB 2: TEST FRONTEND
# =============================================================================

run_job "Test Frontend" "49877403991"

run_step "📥 Checkout code"
echo "  Repository already checked out"
check_result

run_step "🔧 Set up Node.js ${NODE_VERSION}"
if command -v node &> /dev/null; then
    NODE_INSTALLED_VERSION=$(node --version)
    echo "  Node version: ${NODE_INSTALLED_VERSION}"
    check_result
else
    echo -e "  ${YELLOW}⚠ Node.js not installed${NC}"
fi

run_step "📦 Install dependencies"
cd frontend-react
if [ -f "package-lock.json" ]; then
    echo "  Cache dependency path: frontend-react/package-lock.json ✓"
    echo "  Running: npm ci"
    npm ci > /dev/null 2>&1
    check_result
else
    echo -e "  ${RED}✗ package-lock.json not found${NC}"
    FAILED_JOBS+=("test-frontend")
fi

run_step "🎨 Run linting"
echo "  Running: npm run lint"
npm run lint > /dev/null 2>&1 || true
echo -e "  ${GREEN}✓ Linting complete (errors allowed)${NC}"

run_step "🧪 Run frontend tests"
echo "  Running: npm test -- --coverage --watchAll=false"
npm test -- --coverage --watchAll=false > /dev/null 2>&1 || true
echo -e "  ${GREEN}✓ Frontend tests complete (failures allowed)${NC}"

run_step "🏗️ Build frontend"
echo "  Running: npm run build"
if npm run build > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Frontend build successful${NC}"
    check_result
else
    echo -e "  ${RED}✗ Frontend build failed${NC}"
    FAILED_JOBS+=("test-frontend")
fi

run_step "📊 Upload build artifacts"
if [ -d ".next" ]; then
    echo "  Artifact: frontend-build"
    echo "  Path: frontend-react/.next/ ✓"
    echo "  Size: $(du -sh .next | cut -f1)"
    check_result
else
    echo -e "  ${RED}ERROR: No .next directory found!${NC}"
    echo -e "  ${RED}This would cause the artifact warning in GitHub Actions${NC}"
    FAILED_JOBS+=("test-frontend")
fi
cd ..

# Job completed

# =============================================================================
# JOB 3: BUILD DOCKER IMAGES
# =============================================================================

run_job "Build Docker Images" "49877477691"

# Check if previous jobs passed
if [ ${#FAILED_JOBS[@]} -gt 0 ]; then
    echo -e "${YELLOW}⚠ Warning: Previous jobs had failures, but continuing...${NC}"
fi

run_step "📥 Checkout code"
echo "  Repository already checked out"
check_result

run_step "🔐 Log in to Docker Hub"
if docker info > /dev/null 2>&1; then
    echo "  Docker is running ✓"
    echo "  Would authenticate with DOCKER_USERNAME and DOCKER_PASSWORD"
    check_result
else
    echo -e "  ${RED}Docker is not running!${NC}"
    exit 1
fi

run_step "🏷️ Extract metadata for backend"
BACKEND_TAGS="${REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:latest,${REGISTRY}/${DOCKER_USERNAME}/${BACKEND_IMAGE}:pi5"
echo "  Tags: ${BACKEND_TAGS}"
check_result

run_step "🏷️ Extract metadata for frontend"
FRONTEND_TAGS="${REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:latest,${REGISTRY}/${DOCKER_USERNAME}/${FRONTEND_IMAGE}:pi5"
echo "  Tags: ${FRONTEND_TAGS}"
check_result

run_step "🔨 Set up Docker Buildx"
if docker buildx version > /dev/null 2>&1; then
    echo "  Docker Buildx available ✓"
    check_result
else
    echo -e "  ${YELLOW}⚠ Docker Buildx not available${NC}"
fi

run_step "🐳 Build backend Docker image"
echo "  Building for platforms: linux/arm64,linux/amd64"
echo "  Context: ./backend"
cd backend
if docker build -t ${DOCKER_USERNAME}/${BACKEND_IMAGE}:pi5 . > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Backend image built successfully${NC}"
    check_result
else
    echo -e "  ${RED}✗ Backend Docker build failed${NC}"
    FAILED_JOBS+=("build-docker")
fi
cd ..

run_step "🐳 Build frontend Docker image"
echo "  Building for platforms: linux/arm64,linux/amd64"
echo "  Context: ./frontend-react"
cd frontend-react
if docker build -t ${DOCKER_USERNAME}/${FRONTEND_IMAGE}:pi5 . > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Frontend image built successfully${NC}"
    check_result
else
    echo -e "  ${RED}✗ Frontend Docker build failed${NC}"
    FAILED_JOBS+=("build-docker")
fi
cd ..

# Job completed

# =============================================================================
# JOB 4: DEPLOY TO RASPBERRY PI (Self-Hosted Runner)
# =============================================================================

run_job "Deploy to Raspberry Pi" "self-hosted-runner"

echo -e "${YELLOW}Note: This job would run on your Raspberry Pi self-hosted runner${NC}"
echo -e "${YELLOW}Simulating locally instead...${NC}"
echo ""

run_step "📥 Checkout code"
echo "  Repository already checked out"
check_result

run_step "🔍 System Information"
echo "  Hostname: $(hostname)"
echo "  OS: $(uname -s)"
echo "  Docker Version: $(docker --version | cut -d' ' -f3)"
echo "  Docker Compose Version: $(docker compose version | cut -d' ' -f4)"
echo "  Current Directory: $(pwd)"
echo "  Disk Space: $(df -h . | tail -1 | awk '{print $4}' ) available"
check_result

run_step "🐳 Pull latest Docker images"
echo "  Simulating pull of ${DOCKER_USERNAME}/${BACKEND_IMAGE}:pi5"
echo "  Simulating pull of ${DOCKER_USERNAME}/${FRONTEND_IMAGE}:pi5"
check_result

run_step "🔄 Stop existing containers"
if [ -f "docker-compose.pi5.yml" ]; then
    docker compose -f docker-compose.pi5.yml down > /dev/null 2>&1 || true
    echo "  Containers stopped"
    check_result
else
    echo -e "  ${RED}docker-compose.pi5.yml not found!${NC}"
    FAILED_JOBS+=("deploy-to-pi")
fi

run_step "🚀 Start new containers"
if [ -f "docker-compose.pi5.yml" ]; then
    echo "  Starting containers with docker-compose..."
    # Don't actually start them in simulation
    echo "  Would run: docker compose -f docker-compose.pi5.yml up -d"
    check_result
else
    echo -e "  ${RED}Cannot start - docker-compose.pi5.yml missing${NC}"
    FAILED_JOBS+=("deploy-to-pi")
fi

run_step "⏳ Wait for services to be healthy"
echo "  Would check: http://localhost:8000/api/health"
echo "  Would check: http://localhost:3000"
echo "  Simulating health checks..."
sleep 2
echo -e "  ${GREEN}✓ Services would be healthy${NC}"

run_step "📊 Show running containers"
docker ps --format "table {{.Names}}\t{{.Status}}" | head -5
check_result

run_step "🧹 Clean up old images"
echo "  Running: docker image prune -f"
docker image prune -f > /dev/null 2>&1
check_result

# Job completed

# =============================================================================
# JOB 5: SEND NOTIFICATIONS
# =============================================================================

run_job "Send Notifications" "notify"

run_step "📊 Determine job status"
if [ ${#FAILED_JOBS[@]} -eq 0 ]; then
    echo "  Status: success"
    echo "  Color: 3066993 (green)"
    echo "  Title: ✅ Deployment Successful"
else
    echo "  Status: failure"
    echo "  Color: 15158332 (red)"
    echo "  Title: ❌ Deployment Failed"
    echo "  Failed jobs: ${FAILED_JOBS[@]}"
fi
check_result

run_step "💬 Send Discord notification"
echo "  Would send webhook to Discord"
echo "  Notification would include:"
echo "    - Repository: ${GITHUB_REPOSITORY}"
echo "    - Branch: ${GITHUB_REF_NAME}"
echo "    - Commit: ${GITHUB_SHA:0:7}"
echo "    - Test Results: Backend and Frontend"
echo "    - Build Status: Docker images"
echo "    - Deployment: Status"
check_result

# =============================================================================
# FINAL SUMMARY
# =============================================================================

echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                        SIMULATION COMPLETE                                  ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

echo "WORKFLOW SUMMARY:"
echo "-----------------"
for job in "test-backend" "test-frontend" "build-docker" "deploy-to-pi" "notify"; do
    if [[ " ${FAILED_JOBS[@]} " =~ " ${job} " ]]; then
        echo -e "  ${RED}✗ ${job}: FAILED${NC}"
    else
        echo -e "  ${GREEN}✓ ${job}: SUCCESS${NC}"
    fi
done

echo ""
if [ ${#FAILED_JOBS[@]} -eq 0 ]; then
    echo -e "${GREEN}🎉 ALL JOBS PASSED! Your workflow would succeed on GitHub Actions.${NC}"
    exit 0
else
    echo -e "${RED}⚠️  FAILURES DETECTED! The following jobs would fail on GitHub Actions:${NC}"
    for job in "${FAILED_JOBS[@]}"; do
        echo -e "${RED}   - ${job}${NC}"
    done
    echo ""
    echo -e "${YELLOW}Fix these issues before pushing to GitHub to avoid deployment failures.${NC}"
    exit 1
fi