#!/bin/bash

# Fix nginx configuration to work with Docker containers
# This updates nginx to proxy to the Docker containers correctly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Fix Nginx for Docker Containers                          ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check if running as root or with sudo
if [ "$EUID" -ne 0 ]; then 
    echo -e "${YELLOW}This script needs sudo privileges. Re-running with sudo...${NC}"
    exec sudo "$0" "$@"
fi

# Step 1: Check if containers are running
echo -e "${BLUE}Step 1: Checking Docker containers...${NC}"
if docker ps | grep -q sermon-uploader-backend; then
    echo -e "${GREEN}✓ Backend container is running${NC}"
    BACKEND_STATUS="running"
else
    echo -e "${YELLOW}⚠ Backend container is not running${NC}"
    BACKEND_STATUS="stopped"
fi

if docker ps | grep -q sermon-uploader-frontend; then
    echo -e "${GREEN}✓ Frontend container is running${NC}"
    FRONTEND_STATUS="running"
else
    echo -e "${YELLOW}⚠ Frontend container is not running${NC}"
    FRONTEND_STATUS="stopped"
fi

# Step 2: Check current nginx configuration
echo -e "\n${BLUE}Step 2: Checking current nginx configuration...${NC}"
NGINX_CONF="/etc/nginx/sites-available/wpgc-platform"
NGINX_DOCKER_CONF="/etc/nginx/sites-available/wpgc-docker"

if [ -f "$NGINX_CONF" ]; then
    echo "Found existing nginx config at $NGINX_CONF"
    if grep -q "proxy_pass http://localhost:8000" "$NGINX_CONF"; then
        echo -e "${YELLOW}⚠ Nginx is configured for local services, needs update for Docker${NC}"
    fi
fi

# Step 3: Create Docker-compatible nginx configuration
echo -e "\n${BLUE}Step 3: Creating Docker-compatible nginx configuration...${NC}"

cat > "$NGINX_DOCKER_CONF" << 'EOF'
# Admin Dashboard - admin.wpgc.church
server {
    listen 80;
    server_name admin.wpgc.church;
    
    # Docker frontend on port 3000
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts for large uploads
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
    
    # API proxy - route /api to backend
    location /api {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Large file uploads (500MB-10GB sermons)
        client_max_body_size 10G;
        proxy_request_buffering off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
        proxy_connect_timeout 600s;
    }
    
    # Health check endpoint
    location /health {
        proxy_pass http://localhost:8000/api/health;
        proxy_http_version 1.1;
    }
}

# API Backend - api.wpgc.church (if you have this domain)
server {
    listen 80;
    server_name api.wpgc.church;
    
    location / {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Large file uploads
        client_max_body_size 10G;
        proxy_request_buffering off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}

# MinIO Console (optional) - minio.wpgc.church
server {
    listen 80;
    server_name minio.wpgc.church;
    
    location / {
        proxy_pass http://localhost:9001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
EOF

echo -e "${GREEN}✓ Docker nginx configuration created${NC}"

# Step 4: Enable the new configuration
echo -e "\n${BLUE}Step 4: Enabling nginx configuration...${NC}"

# Disable old config if exists
if [ -L "/etc/nginx/sites-enabled/wpgc-platform" ]; then
    rm /etc/nginx/sites-enabled/wpgc-platform
    echo "Removed old configuration link"
fi

# Enable new Docker config
ln -sf "$NGINX_DOCKER_CONF" /etc/nginx/sites-enabled/wpgc-docker
echo -e "${GREEN}✓ Docker configuration enabled${NC}"

# Step 5: Test nginx configuration
echo -e "\n${BLUE}Step 5: Testing nginx configuration...${NC}"
if nginx -t; then
    echo -e "${GREEN}✓ Nginx configuration is valid${NC}"
else
    echo -e "${RED}✗ Nginx configuration has errors${NC}"
    exit 1
fi

# Step 6: Reload nginx
echo -e "\n${BLUE}Step 6: Reloading nginx...${NC}"
systemctl reload nginx
echo -e "${GREEN}✓ Nginx reloaded${NC}"

# Step 7: Start containers if not running
if [ "$BACKEND_STATUS" = "stopped" ] || [ "$FRONTEND_STATUS" = "stopped" ]; then
    echo -e "\n${BLUE}Step 7: Starting Docker containers...${NC}"
    cd /opt/sermon-uploader
    docker compose -f docker-compose.pi5.yml up -d
    echo -e "${GREEN}✓ Containers started${NC}"
else
    echo -e "\n${BLUE}Step 7: Containers already running${NC}"
fi

# Step 8: Test the endpoints
echo -e "\n${BLUE}Step 8: Testing endpoints...${NC}"
sleep 5

# Test backend health
if curl -sf http://localhost:8000/api/health > /dev/null; then
    echo -e "${GREEN}✓ Backend is healthy${NC}"
else
    echo -e "${YELLOW}⚠ Backend health check failed${NC}"
    echo "Backend logs:"
    docker logs sermon-uploader-backend --tail 20
fi

# Test frontend
if curl -sf http://localhost:3000 > /dev/null; then
    echo -e "${GREEN}✓ Frontend is responding${NC}"
else
    echo -e "${YELLOW}⚠ Frontend not responding${NC}"
    echo "Frontend logs:"
    docker logs sermon-uploader-frontend --tail 20
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    Nginx Configuration Complete!                            ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Your services should now be accessible at:"
echo "  • Admin Dashboard: http://admin.wpgc.church"
echo "  • Backend API: http://admin.wpgc.church/api"
echo "  • Health Check: http://admin.wpgc.church/health"
echo ""
echo "If you still see 'Bad Gateway', check:"
echo "  1. Container status: docker ps"
echo "  2. Container logs: docker logs sermon-uploader-backend"
echo "  3. Nginx logs: tail -f /var/log/nginx/error.log"