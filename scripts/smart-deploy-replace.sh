#!/bin/bash

# =============================================================================
# SMART DEPLOYMENT WITH SERVICE REPLACEMENT
# This script intelligently handles port conflicts by:
# 1. Detecting what's using our required ports
# 2. Stopping conflicting containers if they're ours
# 3. Using existing services when appropriate (e.g., existing MinIO)
# 4. Auto-killing our orphaned processes
# 5. Failing with clear errors only for truly external services
# 
# Usage:
#   ./smart-deploy-replace.sh [compose-file] [--auto]
#   --auto: Automatically kill conflicting processes without prompting
# =============================================================================

set -e

# Check for auto mode
AUTO_MODE=false
if [[ "$2" == "--auto" ]] || [[ "$GITHUB_ACTIONS" == "true" ]] || [[ "$CI" == "true" ]]; then
    AUTO_MODE=true
    echo "Running in AUTO mode (CI/CD environment detected or --auto flag set)"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

# Required ports for our services
REQUIRED_PORTS=(
    "9000:MinIO"
    "9001:MinIO Console"
    "8000:Backend API"
    "3000:Frontend"
)

# Function to get process using a port
get_port_user() {
    local port=$1
    
    # Try lsof first
    if command -v lsof > /dev/null 2>&1; then
        local result=$(lsof -i :$port 2>/dev/null | grep LISTEN | head -1)
        if [ -n "$result" ]; then
            local process=$(echo "$result" | awk '{print $1}')
            local pid=$(echo "$result" | awk '{print $2}')
            echo "$process (PID: $pid)"
            return 0
        fi
    fi
    
    # Try netstat as fallback
    if command -v netstat > /dev/null 2>&1; then
        local result=$(netstat -tlnp 2>/dev/null | grep ":$port " | head -1)
        if [ -n "$result" ]; then
            echo "Unknown process"
            return 0
        fi
    fi
    
    echo ""
    return 1
}

# Function to check if a Docker container is using a port
get_container_using_port() {
    local port=$1
    docker ps --format "{{.Names}} {{.Ports}}" | grep "0.0.0.0:$port->" | awk '{print $1}' | head -1
}

# Function to check if MinIO is already running
check_existing_minio() {
    local port=$1
    
    # Check if MinIO health endpoint responds
    if curl -sf http://localhost:$port/minio/health/live > /dev/null 2>&1; then
        return 0  # MinIO is running
    fi
    return 1  # Not MinIO or not responding
}

# Function to handle port conflicts intelligently
handle_port_conflict() {
    local port=$1
    local service_name=$2
    local compose_file=$3
    
    echo -e "${BLUE}Checking port $port for $service_name...${NC}"
    
    local port_user=$(get_port_user $port)
    
    if [ -z "$port_user" ]; then
        echo -e "  ${GREEN}✓ Port $port is available${NC}"
        return 0
    fi
    
    echo -e "  ${YELLOW}⚠ Port $port is in use by: $port_user${NC}"
    
    # Check if it's a Docker container
    local container=$(get_container_using_port $port)
    
    if [ -n "$container" ]; then
        echo -e "  ${CYAN}Container using port: $container${NC}"
        
        # Check if it's one of our sermon-uploader containers
        if [[ "$container" == sermon-uploader-* ]]; then
            echo -e "  ${BLUE}→ Stopping our old container: $container${NC}"
            docker stop $container > /dev/null 2>&1
            docker rm $container > /dev/null 2>&1
            echo -e "  ${GREEN}✓ Old container removed, port $port is now free${NC}"
            return 0
        else
            echo -e "  ${YELLOW}→ External container detected: $container${NC}"
            
            # Special handling for MinIO
            if [ "$port" = "9000" ] && check_existing_minio $port; then
                echo -e "  ${GREEN}✓ Existing MinIO detected, will use it${NC}"
                return 2  # Special return code for existing MinIO
            fi
            
            echo -e "  ${RED}✗ Cannot proceed: Port $port is used by external container${NC}"
            echo -e "  ${YELLOW}Options:${NC}"
            echo -e "    1. Stop the container: docker stop $container"
            echo -e "    2. Remove the container: docker rm $container"
            return 1
        fi
    else
        # Port is used by non-Docker process
        
        # Special handling for MinIO
        if [ "$port" = "9000" ] && check_existing_minio $port; then
            echo -e "  ${GREEN}✓ Existing MinIO service detected (non-Docker), will use it${NC}"
            return 2  # Special return code for existing MinIO
        fi
        
        # Check if it's one of our own processes (sermon-uploader related)
        if [[ "$port_user" == *"sermon"* ]] || [[ "$port_user" == *"sermon-up"* ]]; then
            echo -e "  ${YELLOW}→ Found our orphaned process: $port_user${NC}"
            
            # Extract PID from the string
            local pid=$(echo "$port_user" | grep -oE 'PID: [0-9]+' | cut -d' ' -f2)
            
            if [ -n "$pid" ]; then
                echo -e "  ${BLUE}→ Attempting to stop process (PID: $pid)...${NC}"
                
                # Try graceful kill first
                if kill -TERM $pid 2>/dev/null; then
                    sleep 2
                    
                    # Check if process is gone
                    if ! kill -0 $pid 2>/dev/null; then
                        echo -e "  ${GREEN}✓ Process stopped successfully, port $port is now free${NC}"
                        return 0
                    else
                        # Force kill if still running
                        echo -e "  ${YELLOW}→ Process didn't stop gracefully, force killing...${NC}"
                        kill -KILL $pid 2>/dev/null || true
                        sleep 1
                        echo -e "  ${GREEN}✓ Process killed, port $port should be free${NC}"
                        return 0
                    fi
                else
                    echo -e "  ${YELLOW}⚠ Could not kill process (may require sudo)${NC}"
                    echo -e "  ${CYAN}Try: sudo kill $pid${NC}"
                fi
            fi
        fi
        
        # Check for other common development servers
        if [[ "$port_user" == *"node"* ]] && [ "$port" = "3000" ]; then
            echo -e "  ${YELLOW}→ Found Node.js process on port $port${NC}"
            
            # Extract PID
            local pid=$(echo "$port_user" | grep -oE 'PID: [0-9]+' | cut -d' ' -f2)
            
            if [ -n "$pid" ]; then
                echo -e "  ${BLUE}→ This appears to be a development server${NC}"
                
                local should_kill=false
                if [ "$AUTO_MODE" = true ]; then
                    echo -e "  ${BLUE}→ Auto-killing Node.js process (AUTO mode)${NC}"
                    should_kill=true
                else
                    echo -e "  ${YELLOW}Kill it? (y/n):${NC} "
                    read -r answer
                    if [ "$answer" = "y" ] || [ "$answer" = "Y" ]; then
                        should_kill=true
                    fi
                fi
                
                if [ "$should_kill" = true ]; then
                    if kill -TERM $pid 2>/dev/null; then
                        sleep 1
                        echo -e "  ${GREEN}✓ Node process stopped, port $port is now free${NC}"
                        return 0
                    else
                        echo -e "  ${YELLOW}⚠ Could not kill process (may require sudo)${NC}"
                    fi
                fi
            fi
        fi
        
        echo -e "  ${RED}✗ Cannot proceed: Port $port is used by external process${NC}"
        echo -e "  ${YELLOW}Process: $port_user${NC}"
        
        # Provide helpful suggestions
        if [[ "$port_user" == *"minio"* ]]; then
            echo -e "  ${YELLOW}Tip: MinIO is already running. Consider using the existing instance.${NC}"
        fi
        
        return 1
    fi
}

# Function to create modified docker-compose for external MinIO
create_external_minio_config() {
    local input_file=$1
    local output_file=$2
    
    echo -e "${CYAN}Creating configuration for external MinIO...${NC}"
    
    # Copy the original file
    cp $input_file $output_file
    
    # Remove MinIO service block
    # This removes from 'minio:' to the next service definition
    awk '
    /^  minio:$/ { skip = 1; next }
    /^  [a-z]+:$/ && skip { skip = 0 }
    !skip { print }
    ' $input_file > $output_file.tmp && mv $output_file.tmp $output_file
    
    # Update backend environment to use host MinIO
    sed -i.bak 's/MINIO_ENDPOINT=minio:9000/MINIO_ENDPOINT=host.docker.internal:9000/g' $output_file
    
    # Add extra_hosts to backend if not present
    if ! grep -q "extra_hosts:" $output_file; then
        awk '
        /^  backend:$/ { backend = 1 }
        backend && /^    networks:$/ {
            print "    extra_hosts:"
            print "      - \"host.docker.internal:host-gateway\""
        }
        { print }
        ' $output_file > $output_file.tmp && mv $output_file.tmp $output_file
    fi
    
    # Remove MinIO dependency from backend
    sed -i.bak '/depends_on:/,/minio:/ {
        /minio:/,/condition:/ d
    }' $output_file
    
    # Remove MinIO volume definition if no longer needed
    sed -i.bak '/^volumes:$/,/^[a-z]*:$/ {
        /minio_data:/ d
    }' $output_file
    
    # Clean up backup files
    rm -f ${output_file}.bak
    
    echo -e "${GREEN}✓ Configuration created for external MinIO${NC}"
}

# Main deployment function
smart_deploy_with_replacement() {
    local compose_file=${1:-"docker-compose.pi5.yml"}
    
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║              SMART DEPLOYMENT WITH INTELLIGENT SERVICE REPLACEMENT          ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Check if compose file exists
    if [ ! -f "$compose_file" ]; then
        echo -e "${RED}✗ $compose_file not found!${NC}"
        exit 1
    fi
    
    # First, stop any existing sermon-uploader containers
    echo -e "${BLUE}🔄 Checking for existing sermon-uploader containers...${NC}"
    local existing_containers=$(docker ps -a --format "{{.Names}}" | grep "^sermon-uploader-" || true)
    if [ -n "$existing_containers" ]; then
        echo -e "${YELLOW}Found existing containers:${NC}"
        echo "$existing_containers"
        echo -e "${BLUE}Stopping and removing old containers...${NC}"
        for container in $existing_containers; do
            docker stop $container > /dev/null 2>&1 || true
            docker rm $container > /dev/null 2>&1 || true
            echo -e "  ${GREEN}✓ Removed: $container${NC}"
        done
    fi
    
    # Check each required port
    echo ""
    echo -e "${CYAN}🔍 Checking required ports...${NC}"
    
    local use_external_minio=false
    local all_ports_available=true
    
    # Check MinIO port specifically
    handle_port_conflict 9000 "MinIO" $compose_file
    local minio_result=$?
    
    if [ $minio_result -eq 2 ]; then
        # External MinIO detected
        use_external_minio=true
        echo -e "${CYAN}→ Will use existing MinIO instance${NC}"
    elif [ $minio_result -ne 0 ]; then
        all_ports_available=false
    fi
    
    # Check other ports
    for port_info in "${REQUIRED_PORTS[@]}"; do
        local port="${port_info%%:*}"
        local service="${port_info##*:}"
        
        # Skip MinIO ports if using external MinIO
        if [ "$use_external_minio" = true ] && [[ "$port" == "900"* ]]; then
            continue
        fi
        
        # Skip if we already checked MinIO main port
        if [ "$port" = "9000" ]; then
            continue
        fi
        
        handle_port_conflict $port "$service" $compose_file
        if [ $? -ne 0 ] && [ $? -ne 2 ]; then
            all_ports_available=false
        fi
    done
    
    # Check if we can proceed
    if [ "$all_ports_available" = false ]; then
        echo ""
        echo -e "${RED}❌ Cannot proceed with deployment${NC}"
        echo -e "${RED}Some required ports are blocked by external services${NC}"
        echo -e "${YELLOW}Please resolve the conflicts and try again${NC}"
        exit 1
    fi
    
    # Prepare the appropriate docker-compose file
    local deploy_compose=$compose_file
    
    if [ "$use_external_minio" = true ]; then
        deploy_compose="docker-compose.external-minio.yml"
        create_external_minio_config $compose_file $deploy_compose
    fi
    
    # Deploy
    echo ""
    echo -e "${BLUE}🚀 Starting deployment...${NC}"
    
    # Remove any existing network
    docker network rm sermon-uploader_sermon-network > /dev/null 2>&1 || true
    
    # Deploy with docker-compose
    if docker compose -f $deploy_compose up -d; then
        echo -e "${GREEN}✓ Containers started successfully!${NC}"
        
        # Wait for services
        echo -e "${BLUE}⏳ Waiting for services to be healthy...${NC}"
        sleep 5
        
        # Check backend health
        if curl -sf http://localhost:8000/api/health > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Backend is healthy${NC}"
        else
            echo -e "${YELLOW}⚠ Backend health check pending...${NC}"
        fi
        
        # Display summary
        echo ""
        echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${CYAN}║                         DEPLOYMENT SUCCESSFUL                               ║${NC}"
        echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        
        echo -e "${GREEN}🐳 Running Containers:${NC}"
        docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep sermon || true
        echo ""
        
        echo -e "${GREEN}🌐 Access URLs:${NC}"
        echo "  • Frontend: http://localhost:3000"
        echo "  • Backend API: http://localhost:8000/api/health"
        
        if [ "$use_external_minio" = false ]; then
            echo "  • MinIO Console: http://localhost:9001"
        else
            echo "  • MinIO: Using existing instance on port 9000"
        fi
        
        # Save deployment info
        cat > .deployment-info <<EOF
# Deployment Information
# Generated: $(date)
DEPLOYMENT_TYPE=$([ "$use_external_minio" = true ] && echo "external-minio" || echo "full-stack")
COMPOSE_FILE=$deploy_compose
FRONTEND_URL=http://localhost:3000
BACKEND_URL=http://localhost:8000
EOF
        
        echo ""
        echo -e "${BLUE}💾 Deployment info saved to .deployment-info${NC}"
        
        return 0
    else
        echo -e "${RED}✗ Deployment failed!${NC}"
        echo -e "${YELLOW}Check docker logs for details${NC}"
        return 1
    fi
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    smart_deploy_with_replacement "$@"
fi