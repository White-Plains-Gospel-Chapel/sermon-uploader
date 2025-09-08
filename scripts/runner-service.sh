#!/bin/bash

# GitHub Actions Runner Service Management Script
# This script helps manage the GitHub Actions runner as a systemd service

set -e

RUNNER_DIR="/opt/actions-runner"
SERVICE_NAME="actions.runner.White-Plains-Gospel-Chapel-sermon-uploader.pi-runner"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

show_help() {
    echo "GitHub Actions Runner Service Manager"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  status    - Show runner service status"
    echo "  start     - Start the runner service"
    echo "  stop      - Stop the runner service"
    echo "  restart   - Restart the runner service"
    echo "  logs      - Show runner logs (live)"
    echo "  logs-all  - Show all runner logs"
    echo "  enable    - Enable runner to start at boot"
    echo "  disable   - Disable runner from starting at boot"
    echo "  health    - Check runner health and connectivity"
    echo "  update    - Update the runner to latest version"
    echo "  help      - Show this help message"
}

check_service_exists() {
    if ! systemctl list-units --all | grep -q "$SERVICE_NAME"; then
        echo -e "${RED}Error: Runner service not found!${NC}"
        echo "Service name: $SERVICE_NAME"
        echo "Please run the setup script first: ./setup-github-runner.sh"
        exit 1
    fi
}

case "$1" in
    status)
        check_service_exists
        echo -e "${BLUE}Runner Service Status:${NC}"
        sudo systemctl status "$SERVICE_NAME" --no-pager
        ;;
    
    start)
        check_service_exists
        echo -e "${GREEN}Starting runner service...${NC}"
        sudo systemctl start "$SERVICE_NAME"
        sleep 2
        sudo systemctl status "$SERVICE_NAME" --no-pager | head -n 10
        ;;
    
    stop)
        check_service_exists
        echo -e "${YELLOW}Stopping runner service...${NC}"
        sudo systemctl stop "$SERVICE_NAME"
        echo -e "${GREEN}Service stopped.${NC}"
        ;;
    
    restart)
        check_service_exists
        echo -e "${YELLOW}Restarting runner service...${NC}"
        sudo systemctl restart "$SERVICE_NAME"
        sleep 2
        sudo systemctl status "$SERVICE_NAME" --no-pager | head -n 10
        ;;
    
    logs)
        check_service_exists
        echo -e "${BLUE}Showing live runner logs (Ctrl+C to exit):${NC}"
        sudo journalctl -u "$SERVICE_NAME" -f
        ;;
    
    logs-all)
        check_service_exists
        echo -e "${BLUE}Showing all runner logs:${NC}"
        sudo journalctl -u "$SERVICE_NAME" --no-pager
        ;;
    
    enable)
        check_service_exists
        echo -e "${GREEN}Enabling runner to start at boot...${NC}"
        sudo systemctl enable "$SERVICE_NAME"
        echo -e "${GREEN}Runner will now start automatically at boot.${NC}"
        ;;
    
    disable)
        check_service_exists
        echo -e "${YELLOW}Disabling runner from starting at boot...${NC}"
        sudo systemctl disable "$SERVICE_NAME"
        echo -e "${YELLOW}Runner will not start automatically at boot.${NC}"
        ;;
    
    health)
        echo -e "${BLUE}Checking runner health...${NC}"
        echo ""
        
        # Check if service exists and is running
        if systemctl list-units --all | grep -q "$SERVICE_NAME"; then
            if systemctl is-active --quiet "$SERVICE_NAME"; then
                echo -e "${GREEN}✓ Runner service is active${NC}"
            else
                echo -e "${RED}✗ Runner service is not active${NC}"
            fi
        else
            echo -e "${RED}✗ Runner service not installed${NC}"
        fi
        
        # Check GitHub connectivity
        echo -n "Checking GitHub API connectivity... "
        if curl -s https://api.github.com > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Connected${NC}"
        else
            echo -e "${RED}✗ Cannot reach GitHub${NC}"
        fi
        
        # Check Docker
        echo -n "Checking Docker... "
        if command -v docker &> /dev/null; then
            if docker ps > /dev/null 2>&1; then
                echo -e "${GREEN}✓ Docker is running${NC}"
            else
                echo -e "${YELLOW}⚠ Docker installed but not accessible (check permissions)${NC}"
            fi
        else
            echo -e "${YELLOW}⚠ Docker not installed${NC}"
        fi
        
        # Check disk space
        echo -n "Checking disk space... "
        DISK_USAGE=$(df / | awk 'NR==2 {print $5}' | sed 's/%//')
        if [ "$DISK_USAGE" -lt 80 ]; then
            echo -e "${GREEN}✓ Disk usage: ${DISK_USAGE}%${NC}"
        elif [ "$DISK_USAGE" -lt 90 ]; then
            echo -e "${YELLOW}⚠ Disk usage: ${DISK_USAGE}% (getting full)${NC}"
        else
            echo -e "${RED}✗ Disk usage: ${DISK_USAGE}% (critically full)${NC}"
        fi
        
        # Show runner info if available
        if [ -f "$RUNNER_DIR/.runner" ]; then
            echo ""
            echo -e "${BLUE}Runner Configuration:${NC}"
            cat "$RUNNER_DIR/.runner" | jq -r '
                "Repository: \(.gitHubUrl)",
                "Runner Name: \(.agentName)",
                "Runner ID: \(.agentId)",
                "Pool ID: \(.poolId)"
            '
        fi
        ;;
    
    update)
        echo -e "${BLUE}Updating GitHub Actions runner...${NC}"
        cd "$RUNNER_DIR"
        
        # Stop the service
        echo "Stopping runner service..."
        sudo ./svc.sh stop
        
        # Get latest version
        LATEST_VERSION=$(curl -s https://api.github.com/repos/actions/runner/releases/latest | jq -r '.tag_name' | sed 's/v//')
        CURRENT_VERSION=$(./config.sh --version 2>&1 | grep -oP '\d+\.\d+\.\d+' | head -1)
        
        if [ "$LATEST_VERSION" == "$CURRENT_VERSION" ]; then
            echo -e "${GREEN}Runner is already up to date (version $CURRENT_VERSION)${NC}"
        else
            echo "Updating from $CURRENT_VERSION to $LATEST_VERSION..."
            
            # Detect architecture
            ARCH=$(uname -m)
            if [[ "$ARCH" == "aarch64" ]] || [[ "$ARCH" == "arm64" ]]; then
                RUNNER_ARCH="arm64"
            elif [[ "$ARCH" == "armv7l" ]]; then
                RUNNER_ARCH="arm"
            else
                echo -e "${RED}Unsupported architecture: $ARCH${NC}"
                exit 1
            fi
            
            # Download new version
            RUNNER_FILE="actions-runner-linux-${RUNNER_ARCH}-${LATEST_VERSION}.tar.gz"
            RUNNER_URL="https://github.com/actions/runner/releases/download/v${LATEST_VERSION}/${RUNNER_FILE}"
            
            echo "Downloading runner version ${LATEST_VERSION}..."
            curl -L -o runner-update.tar.gz "$RUNNER_URL"
            
            # Backup config
            cp .runner .runner.backup
            cp .credentials .credentials.backup
            cp .credentials_rsaparams .credentials_rsaparams.backup 2>/dev/null || true
            
            # Extract update
            tar xzf runner-update.tar.gz
            rm runner-update.tar.gz
            
            echo -e "${GREEN}✓ Runner updated to version $LATEST_VERSION${NC}"
        fi
        
        # Start the service
        echo "Starting runner service..."
        sudo ./svc.sh start
        ;;
    
    help|"")
        show_help
        ;;
    
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac