#!/bin/bash

# =============================================================================
# SMART DEPLOYMENT SCRIPT WITH DYNAMIC PORT CONFLICT RESOLUTION
# This script automatically handles ANY port conflicts by:
# 1. Detecting which ports are in use
# 2. Finding available alternative ports
# 3. Dynamically reconfiguring docker-compose
# 4. Updating environment variables
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Default ports
DEFAULT_MINIO_PORT=9000
DEFAULT_MINIO_CONSOLE_PORT=9001
DEFAULT_BACKEND_PORT=8000
DEFAULT_FRONTEND_PORT=3000

# Track port mappings
declare -a PORT_MAPPINGS

# Function to check if port is available
is_port_available() {
    local port=$1
    if lsof -i :$port > /dev/null 2>&1; then
        return 1  # Port is in use
    else
        return 0  # Port is available
    fi
}

# Function to find next available port
find_available_port() {
    local start_port=$1
    local max_attempts=100
    
    for ((i=0; i<$max_attempts; i++)); do
        local port=$((start_port + i))
        if is_port_available $port; then
            echo $port
            return 0
        fi
    done
    
    echo -e "${RED}Could not find available port starting from $start_port${NC}" >&2
    return 1
}

# Function to detect and resolve port conflicts
resolve_port_conflicts() {
    local compose_file=$1
    local output_file=$2
    
    echo -e "${CYAN}🔍 Detecting and resolving port conflicts...${NC}"
    
    # Copy original file
    cp $compose_file $output_file
    
    # Check MinIO ports
    local minio_port=$DEFAULT_MINIO_PORT
    local minio_console_port=$DEFAULT_MINIO_CONSOLE_PORT
    local backend_port=$DEFAULT_BACKEND_PORT
    local frontend_port=$DEFAULT_FRONTEND_PORT
    
    local changes_made=false
    
    # Check if MinIO service exists in compose file
    if grep -q "^\s*minio:" $compose_file; then
        # MinIO main port
        if ! is_port_available $DEFAULT_MINIO_PORT; then
            echo -e "${YELLOW}⚠ Port $DEFAULT_MINIO_PORT is in use${NC}"
            
            # Check if it's MinIO already running
            if curl -s http://localhost:$DEFAULT_MINIO_PORT/minio/health/live > /dev/null 2>&1; then
                echo -e "${GREEN}✓ Existing MinIO detected on port $DEFAULT_MINIO_PORT${NC}"
                echo -e "${BLUE}→ Removing MinIO service from deployment and using existing instance${NC}"
                
                # Remove MinIO service from compose file
                sed -i.bak '/^  minio:/,/^  [a-z]/{ /^  [a-z]/!d; }' $output_file
                sed -i.bak '/^  minio:/d' $output_file
                
                # Update backend to use host MinIO
                sed -i.bak 's/MINIO_ENDPOINT=minio:9000/MINIO_ENDPOINT=host.docker.internal:9000/g' $output_file
                
                # Add extra_hosts if not present
                if ! grep -q "extra_hosts:" $output_file; then
                    sed -i.bak '/^\s*backend:/,/^\s*[a-z]*:/ { /networks:/i\
    extra_hosts:\
      - "host.docker.internal:host-gateway"
                    }' $output_file
                fi
                
                # Remove MinIO dependency from backend
                sed -i.bak '/depends_on:/,/minio:/ { /minio:/,/condition:/d; }' $output_file
                
                changes_made=true
                PORT_MAPPINGS+=("MinIO: Using existing on port $DEFAULT_MINIO_PORT")
            else
                # Port is in use by something else, find alternative
                minio_port=$(find_available_port $DEFAULT_MINIO_PORT)
                echo -e "${GREEN}✓ Using alternative MinIO port: $minio_port${NC}"
                sed -i.bak "s/\"$DEFAULT_MINIO_PORT:9000\"/\"$minio_port:9000\"/g" $output_file
                sed -i.bak "s/MINIO_ENDPOINT=minio:9000/MINIO_ENDPOINT=minio:9000/g" $output_file
                changes_made=true
                PORT_MAPPINGS+=("MinIO: $DEFAULT_MINIO_PORT → $minio_port")
            fi
        else
            PORT_MAPPINGS+=("MinIO: $DEFAULT_MINIO_PORT (unchanged)")
        fi
        
        # MinIO console port
        if ! is_port_available $DEFAULT_MINIO_CONSOLE_PORT; then
            minio_console_port=$(find_available_port $DEFAULT_MINIO_CONSOLE_PORT)
            echo -e "${YELLOW}⚠ Port $DEFAULT_MINIO_CONSOLE_PORT is in use${NC}"
            echo -e "${GREEN}✓ Using alternative MinIO console port: $minio_console_port${NC}"
            sed -i.bak "s/\"$DEFAULT_MINIO_CONSOLE_PORT:9001\"/\"$minio_console_port:9001\"/g" $output_file
            changes_made=true
            PORT_MAPPINGS+=("MinIO Console: $DEFAULT_MINIO_CONSOLE_PORT → $minio_console_port")
        else
            PORT_MAPPINGS+=("MinIO Console: $DEFAULT_MINIO_CONSOLE_PORT (unchanged)")
        fi
    fi
    
    # Backend port
    if ! is_port_available $DEFAULT_BACKEND_PORT; then
        backend_port=$(find_available_port $DEFAULT_BACKEND_PORT)
        echo -e "${YELLOW}⚠ Port $DEFAULT_BACKEND_PORT is in use${NC}"
        echo -e "${GREEN}✓ Using alternative backend port: $backend_port${NC}"
        sed -i.bak "s/\"$DEFAULT_BACKEND_PORT:8000\"/\"$backend_port:8000\"/g" $output_file
        
        # Update frontend's backend URL
        sed -i.bak "s/NEXT_PUBLIC_API_URL=http:\/\/localhost:$DEFAULT_BACKEND_PORT/NEXT_PUBLIC_API_URL=http:\/\/localhost:$backend_port/g" $output_file
        
        changes_made=true
        PORT_MAPPINGS+=("Backend: $DEFAULT_BACKEND_PORT → $backend_port")
    else
        PORT_MAPPINGS+=("Backend: $DEFAULT_BACKEND_PORT (unchanged)")
    fi
    
    # Frontend port
    if ! is_port_available $DEFAULT_FRONTEND_PORT; then
        frontend_port=$(find_available_port $DEFAULT_FRONTEND_PORT)
        echo -e "${YELLOW}⚠ Port $DEFAULT_FRONTEND_PORT is in use${NC}"
        echo -e "${GREEN}✓ Using alternative frontend port: $frontend_port${NC}"
        sed -i.bak "s/\"$DEFAULT_FRONTEND_PORT:3000\"/\"$frontend_port:3000\"/g" $output_file
        changes_made=true
        PORT_MAPPINGS+=("Frontend: $DEFAULT_FRONTEND_PORT → $frontend_port")
    else
        PORT_MAPPINGS+=("Frontend: $DEFAULT_FRONTEND_PORT (unchanged)")
    fi
    
    # Clean up backup files
    rm -f ${output_file}.bak
    
    # Report changes
    if [ "$changes_made" = true ]; then
        echo -e "${CYAN}📝 Dynamic configuration created: $output_file${NC}"
        return 0
    else
        echo -e "${GREEN}✓ No port conflicts detected${NC}"
        return 1
    fi
}

# Function to deploy with smart port handling
smart_deploy() {
    local compose_file=${1:-"docker-compose.pi5.yml"}
    
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║                     SMART DEPLOYMENT WITH DYNAMIC PORTS                     ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Check if compose file exists
    if [ ! -f "$compose_file" ]; then
        echo -e "${RED}✗ $compose_file not found!${NC}"
        exit 1
    fi
    
    # Stop existing containers
    echo -e "${BLUE}🔄 Stopping existing containers...${NC}"
    docker compose -f $compose_file down 2>/dev/null || true
    
    # Resolve port conflicts and create dynamic config
    local dynamic_compose="docker-compose.dynamic.yml"
    if resolve_port_conflicts $compose_file $dynamic_compose; then
        echo -e "${CYAN}Using dynamic configuration with resolved ports${NC}"
        compose_file=$dynamic_compose
    fi
    
    # Deploy with the appropriate configuration
    echo -e "${BLUE}🚀 Starting containers...${NC}"
    if docker compose -f $compose_file up -d; then
        echo -e "${GREEN}✓ Containers started successfully!${NC}"
        
        # Wait for health checks
        echo -e "${BLUE}⏳ Waiting for services to be healthy...${NC}"
        sleep 5
        
        # Display deployment summary
        echo ""
        echo -e "${CYAN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${CYAN}║                         DEPLOYMENT SUCCESSFUL                               ║${NC}"
        echo -e "${CYAN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "${GREEN}📊 Port Mappings:${NC}"
        for mapping in "${PORT_MAPPINGS[@]}"; do
            echo "  • $mapping"
        done
        echo ""
        
        # Show actual running containers
        echo -e "${GREEN}🐳 Running Containers:${NC}"
        docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep sermon || true
        echo ""
        
        # Display access URLs with actual ports
        local actual_backend_port=$(docker ps --format "{{.Ports}}" | grep -oE "0.0.0.0:([0-9]+)->8000" | cut -d: -f2 | cut -d- -f1 | head -1)
        local actual_frontend_port=$(docker ps --format "{{.Ports}}" | grep -oE "0.0.0.0:([0-9]+)->3000" | cut -d: -f2 | cut -d- -f1 | head -1)
        
        echo -e "${GREEN}🌐 Access URLs:${NC}"
        echo "  • Frontend: http://localhost:${actual_frontend_port:-3000}"
        echo "  • Backend API: http://localhost:${actual_backend_port:-8000}/api/health"
        
        # If MinIO was deployed, show its URL too
        local actual_minio_console=$(docker ps --format "{{.Ports}}" | grep -oE "0.0.0.0:([0-9]+)->9001" | cut -d: -f2 | cut -d- -f1 | head -1)
        if [ -n "$actual_minio_console" ]; then
            echo "  • MinIO Console: http://localhost:${actual_minio_console}"
        fi
        
        # Save port configuration for future reference
        echo ""
        echo -e "${BLUE}💾 Saving port configuration to .ports.env${NC}"
        cat > .ports.env <<EOF
# Dynamically assigned ports from last deployment
FRONTEND_PORT=${actual_frontend_port:-3000}
BACKEND_PORT=${actual_backend_port:-8000}
MINIO_CONSOLE_PORT=${actual_minio_console:-9001}
# Generated at: $(date)
EOF
        
        return 0
    else
        echo -e "${RED}✗ Deployment failed!${NC}"
        return 1
    fi
}

# Main execution
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    smart_deploy "$@"
fi