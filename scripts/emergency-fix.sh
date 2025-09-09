#!/bin/bash

# Emergency fix script to get everything working
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Emergency Fix - Get Everything Working                   ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Step 1: Stop any conflicting services
echo -e "${BLUE}Step 1: Stopping conflicting services...${NC}"
sudo systemctl stop minio 2>/dev/null || true
sudo systemctl disable minio 2>/dev/null || true
sudo pkill minio 2>/dev/null || true
echo -e "${GREEN}✓ Cleared port conflicts${NC}"

# Step 2: Create necessary directories
echo -e "\n${BLUE}Step 2: Creating directories...${NC}"
sudo mkdir -p /opt/minio-data
sudo mkdir -p /opt/sermon-uploader
sudo chown -R $(whoami):$(whoami) /opt/minio-data
echo -e "${GREEN}✓ Directories created${NC}"

# Step 3: Pull latest code
echo -e "\n${BLUE}Step 3: Pulling latest code...${NC}"
cd /opt/sermon-uploader
git pull origin master || {
    echo -e "${YELLOW}Cloning fresh repository...${NC}"
    cd /opt
    sudo rm -rf sermon-uploader
    git clone https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.git
    cd sermon-uploader
}
echo -e "${GREEN}✓ Code updated${NC}"

# Step 4: Stop existing containers
echo -e "\n${BLUE}Step 4: Stopping existing containers...${NC}"
docker compose -f docker-compose.pi5.yml down 2>/dev/null || true
docker stop sermon-uploader-backend sermon-uploader-frontend sermon-uploader-minio 2>/dev/null || true
docker rm sermon-uploader-backend sermon-uploader-frontend sermon-uploader-minio 2>/dev/null || true
echo -e "${GREEN}✓ Containers stopped${NC}"

# Step 5: Pull latest images
echo -e "\n${BLUE}Step 5: Pulling latest Docker images...${NC}"
docker compose -f docker-compose.pi5.yml pull
echo -e "${GREEN}✓ Images updated${NC}"

# Step 6: Start containers
echo -e "\n${BLUE}Step 6: Starting containers...${NC}"
docker compose -f docker-compose.pi5.yml up -d
echo -e "${GREEN}✓ Containers started${NC}"

# Step 7: Wait for services to be ready
echo -e "\n${BLUE}Step 7: Waiting for services to be ready...${NC}"
for i in {1..30}; do
    if curl -sf http://localhost:8000/api/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Backend is healthy${NC}"
        break
    fi
    echo "Waiting for backend... ($i/30)"
    sleep 2
done

# Step 8: Fix nginx if installed
if command -v nginx > /dev/null 2>&1; then
    echo -e "\n${BLUE}Step 8: Configuring nginx...${NC}"
    
    # Create simple nginx config
    sudo tee /etc/nginx/sites-available/sermon-uploader << 'EOF' > /dev/null
server {
    listen 80;
    server_name admin.wpgc.church sermon-uploader.local;
    
    # Frontend
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
    
    # Backend API
    location /api {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Large uploads
        client_max_body_size 10G;
        proxy_request_buffering off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}

# MinIO Console access
server {
    listen 9090;
    server_name _;
    
    location / {
        proxy_pass http://127.0.0.1:9001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
EOF
    
    # Enable the config
    sudo ln -sf /etc/nginx/sites-available/sermon-uploader /etc/nginx/sites-enabled/
    sudo rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
    
    # Test and reload
    if sudo nginx -t 2>/dev/null; then
        sudo systemctl reload nginx
        echo -e "${GREEN}✓ Nginx configured${NC}"
    else
        echo -e "${YELLOW}⚠ Nginx configuration failed, but containers are running${NC}"
    fi
else
    echo -e "${YELLOW}Nginx not installed, skipping...${NC}"
fi

# Step 9: Show status
echo -e "\n${BLUE}Step 9: Checking status...${NC}"
echo ""
echo "Container Status:"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "NAME|sermon" || true

echo ""
echo "Service Health:"
if curl -sf http://localhost:8000/api/health > /dev/null 2>&1; then
    echo -e "  Backend API: ${GREEN}✓ Healthy${NC}"
else
    echo -e "  Backend API: ${RED}✗ Not responding${NC}"
fi

if curl -sf http://localhost:3000 > /dev/null 2>&1; then
    echo -e "  Frontend: ${GREEN}✓ Healthy${NC}"
else
    echo -e "  Frontend: ${RED}✗ Not responding${NC}"
fi

if curl -sf http://localhost:9000/minio/health/live > /dev/null 2>&1; then
    echo -e "  MinIO: ${GREEN}✓ Healthy${NC}"
else
    echo -e "  MinIO: ${RED}✗ Not responding${NC}"
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    Emergency Fix Complete!                                  ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Access your services at:"
echo "  • Frontend: http://$(hostname -I | awk '{print $1}'):3000"
echo "  • Backend API: http://$(hostname -I | awk '{print $1}'):8000/api/health"
echo "  • MinIO Console: http://$(hostname -I | awk '{print $1}'):9001"
echo ""
echo "If using nginx proxy:"
echo "  • Admin Dashboard: http://admin.wpgc.church"
echo "  • API: http://admin.wpgc.church/api"
echo ""
echo "MinIO Login:"
echo "  • Username: gaius"
echo "  • Password: John 3:16"
echo ""

# Show logs if something failed
if ! curl -sf http://localhost:8000/api/health > /dev/null 2>&1; then
    echo -e "${YELLOW}Backend logs:${NC}"
    docker logs sermon-uploader-backend --tail 20 2>&1 || true
fi