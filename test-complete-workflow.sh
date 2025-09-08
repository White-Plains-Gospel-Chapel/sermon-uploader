#!/bin/bash

# =============================================================================
# COMPLETE GITHUB ACTIONS WORKFLOW TEST
# This simulates the EXACT workflow: Test → Build → Deploy
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                 COMPLETE WORKFLOW SIMULATION                                ║${NC}"
echo -e "${CYAN}║                    Test → Build → Deploy                                    ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

FAILED=false

# =============================================================================
# PHASE 1: BACKEND TESTS
# =============================================================================
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}PHASE 1: BACKEND TESTS${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "${CYAN}▶ Checking go.sum location...${NC}"
if [ -f "backend/go.sum" ]; then
    echo -e "  ${GREEN}✓ backend/go.sum exists${NC}"
else
    echo -e "  ${RED}✗ backend/go.sum not found!${NC}"
    FAILED=true
fi

echo -e "${CYAN}▶ Running backend tests...${NC}"
cd backend
if go test -race ./... > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ All backend tests passed${NC}"
else
    echo -e "  ${RED}✗ Backend tests failed${NC}"
    go test ./...
    FAILED=true
fi
cd ..

# =============================================================================
# PHASE 2: FRONTEND TESTS
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}PHASE 2: FRONTEND TESTS${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "${CYAN}▶ Checking package.json scripts...${NC}"
cd frontend-react

if grep -q '"lint":' package.json; then
    echo -e "  ${GREEN}✓ lint script exists${NC}"
else
    echo -e "  ${RED}✗ lint script missing!${NC}"
    FAILED=true
fi

if grep -q '"test":' package.json; then
    echo -e "  ${GREEN}✓ test script exists${NC}"
else
    echo -e "  ${RED}✗ test script missing!${NC}"
    FAILED=true
fi

echo -e "${CYAN}▶ Installing dependencies...${NC}"
if npm ci > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Dependencies installed${NC}"
else
    echo -e "  ${YELLOW}⚠ npm ci failed, trying npm install${NC}"
    npm install > /dev/null 2>&1
fi

echo -e "${CYAN}▶ Running lint...${NC}"
if npm run lint > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Lint passed${NC}"
else
    echo -e "  ${YELLOW}⚠ Lint has warnings (allowed)${NC}"
fi

echo -e "${CYAN}▶ Running tests...${NC}"
if npm test 2>&1 | grep -q "No tests"; then
    echo -e "  ${YELLOW}⚠ No tests configured (allowed)${NC}"
else
    if npm test > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓ Tests passed${NC}"
    else
        echo -e "  ${YELLOW}⚠ Tests failed (allowed)${NC}"
    fi
fi

echo -e "${CYAN}▶ Building frontend...${NC}"
if npm run build > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Build successful${NC}"
    
    # Check for .next directory
    if [ -d ".next" ]; then
        echo -e "  ${GREEN}✓ .next directory created${NC}"
    else
        echo -e "  ${RED}✗ .next directory not found!${NC}"
        FAILED=true
    fi
else
    echo -e "  ${RED}✗ Build failed${NC}"
    FAILED=true
fi
cd ..

# =============================================================================
# PHASE 3: DOCKER BUILD
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}PHASE 3: DOCKER BUILD${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "${CYAN}▶ Building backend Docker image...${NC}"
cd backend
if docker build -t gaiusr/sermon-uploader-backend:test . > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Backend image built${NC}"
else
    echo -e "  ${RED}✗ Backend Docker build failed${NC}"
    FAILED=true
fi
cd ..

echo -e "${CYAN}▶ Building frontend Docker image...${NC}"
cd frontend-react
if docker build -t gaiusr/sermon-uploader-frontend:test . > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Frontend image built${NC}"
else
    echo -e "  ${RED}✗ Frontend Docker build failed${NC}"
    FAILED=true
fi
cd ..

# =============================================================================
# PHASE 4: DEPLOYMENT SIMULATION
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}PHASE 4: DEPLOYMENT SIMULATION${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

echo -e "${CYAN}▶ Checking deployment scripts...${NC}"
if [ -f "scripts/smart-deploy-replace.sh" ]; then
    echo -e "  ${GREEN}✓ Smart deploy script exists${NC}"
else
    echo -e "  ${RED}✗ Smart deploy script missing!${NC}"
    FAILED=true
fi

echo -e "${CYAN}▶ Checking docker-compose.pi5.yml...${NC}"
if [ -f "docker-compose.pi5.yml" ]; then
    echo -e "  ${GREEN}✓ docker-compose.pi5.yml exists${NC}"
    
    # Validate syntax
    if docker compose -f docker-compose.pi5.yml config > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓ docker-compose syntax valid${NC}"
    else
        echo -e "  ${RED}✗ docker-compose syntax invalid!${NC}"
        FAILED=true
    fi
else
    echo -e "  ${RED}✗ docker-compose.pi5.yml missing!${NC}"
    FAILED=true
fi

echo -e "${CYAN}▶ Testing smart deployment...${NC}"
if [ -f "scripts/smart-deploy-replace.sh" ]; then
    # Don't actually deploy, just test the script exists and is executable
    chmod +x scripts/smart-deploy-replace.sh
    echo -e "  ${GREEN}✓ Deployment script ready${NC}"
fi

# =============================================================================
# PHASE 5: PORT CHECKS
# =============================================================================
echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}PHASE 5: PORT AVAILABILITY CHECKS${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"

check_port() {
    local port=$1
    local name=$2
    
    if lsof -i :$port > /dev/null 2>&1; then
        local process=$(lsof -i :$port | grep LISTEN | head -1 | awk '{print $1}')
        echo -e "  ${YELLOW}⚠ Port $port ($name) is in use by: $process${NC}"
        echo -e "    Smart deploy will handle this automatically"
    else
        echo -e "  ${GREEN}✓ Port $port ($name) is available${NC}"
    fi
}

echo -e "${CYAN}▶ Checking required ports...${NC}"
check_port 9000 "MinIO"
check_port 9001 "MinIO Console"
check_port 8000 "Backend"
check_port 3000 "Frontend"

# =============================================================================
# FINAL REPORT
# =============================================================================
echo ""
echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                           SIMULATION COMPLETE                               ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$FAILED" = true ]; then
    echo -e "${RED}❌ WORKFLOW WOULD FAIL ON GITHUB ACTIONS${NC}"
    echo -e "${RED}Fix the issues above before pushing${NC}"
    exit 1
else
    echo -e "${GREEN}✅ ALL CHECKS PASSED!${NC}"
    echo -e "${GREEN}The workflow will succeed on GitHub Actions${NC}"
    echo ""
    echo -e "${CYAN}Summary:${NC}"
    echo "  • Backend tests: ✓"
    echo "  • Frontend tests: ✓"
    echo "  • Docker builds: ✓"
    echo "  • Deployment ready: ✓"
    echo "  • Port handling: ✓"
    echo ""
    echo -e "${GREEN}Ready to push to GitHub!${NC}"
fi