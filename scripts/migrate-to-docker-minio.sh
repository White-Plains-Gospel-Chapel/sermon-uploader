#!/bin/bash

# Migration script: Move from system MinIO to Docker MinIO
# This script helps migrate your existing MinIO data to the containerized version

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    MinIO Migration to Docker                                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Configuration
SSD_MOUNT="/mnt/ssd"
NEW_MINIO_DATA="$SSD_MOUNT/minio-data"
OLD_MINIO_DATA="/var/lib/minio/data"  # Update this if your MinIO data is elsewhere

echo -e "${YELLOW}This script will:${NC}"
echo "1. Stop the system MinIO service"
echo "2. Create directory structure on SSD"
echo "3. Copy existing MinIO data to SSD"
echo "4. Start MinIO in Docker"
echo "5. Verify the migration"
echo ""
echo -e "${YELLOW}Current configuration:${NC}"
echo "  Old MinIO data: $OLD_MINIO_DATA"
echo "  New MinIO data: $NEW_MINIO_DATA"
echo ""

read -p "Continue with migration? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Migration cancelled"
    exit 1
fi

# Step 1: Check if system MinIO is running
echo -e "\n${BLUE}Step 1: Checking MinIO status...${NC}"
if systemctl is-active --quiet minio; then
    echo "MinIO service is running"
    echo "Stopping MinIO service..."
    sudo systemctl stop minio
    echo -e "${GREEN}✓ MinIO service stopped${NC}"
    
    # Disable it from starting on boot
    sudo systemctl disable minio
    echo -e "${GREEN}✓ MinIO service disabled${NC}"
else
    echo "MinIO service is not running (or not using systemd)"
    # Check if MinIO is running as a process
    if pgrep -x "minio" > /dev/null; then
        echo "MinIO is running as a process"
        echo "Stopping MinIO process..."
        sudo pkill minio
        echo -e "${GREEN}✓ MinIO process stopped${NC}"
    fi
fi

# Step 2: Check SSD mount
echo -e "\n${BLUE}Step 2: Checking SSD mount...${NC}"
if [ ! -d "$SSD_MOUNT" ]; then
    echo -e "${RED}✗ SSD mount directory not found at $SSD_MOUNT${NC}"
    echo "Please update the SSD_MOUNT variable in this script"
    exit 1
fi

# Check if it's actually a mount point
if mountpoint -q "$SSD_MOUNT"; then
    echo -e "${GREEN}✓ SSD is mounted at $SSD_MOUNT${NC}"
    df -h "$SSD_MOUNT"
else
    echo -e "${YELLOW}⚠ $SSD_MOUNT exists but might not be a mount point${NC}"
    echo "Current disk usage:"
    df -h "$SSD_MOUNT"
    read -p "Continue anyway? (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Step 3: Create MinIO data directory on SSD
echo -e "\n${BLUE}Step 3: Creating MinIO data directory...${NC}"
if [ -d "$NEW_MINIO_DATA" ]; then
    echo -e "${YELLOW}⚠ Directory $NEW_MINIO_DATA already exists${NC}"
    ls -la "$NEW_MINIO_DATA"
else
    sudo mkdir -p "$NEW_MINIO_DATA"
    echo -e "${GREEN}✓ Created $NEW_MINIO_DATA${NC}"
fi

# Step 4: Copy existing MinIO data
echo -e "\n${BLUE}Step 4: Migrating MinIO data...${NC}"
if [ -d "$OLD_MINIO_DATA" ]; then
    SIZE=$(du -sh "$OLD_MINIO_DATA" | cut -f1)
    echo "Found existing MinIO data: $SIZE"
    echo "This may take a while for large datasets..."
    
    # Use rsync for reliable copy with progress
    sudo rsync -av --progress "$OLD_MINIO_DATA/" "$NEW_MINIO_DATA/"
    
    echo -e "${GREEN}✓ Data migration complete${NC}"
else
    echo -e "${YELLOW}No existing MinIO data found at $OLD_MINIO_DATA${NC}"
    echo "Starting with fresh MinIO installation"
fi

# Step 5: Set proper permissions
echo -e "\n${BLUE}Step 5: Setting permissions...${NC}"
# MinIO in Docker runs as UID 1000 by default
sudo chown -R 1000:1000 "$NEW_MINIO_DATA"
echo -e "${GREEN}✓ Permissions set${NC}"

# Step 6: Pull Docker images
echo -e "\n${BLUE}Step 6: Pulling Docker images...${NC}"
cd /opt/sermon-uploader
docker compose -f docker-compose.pi5.yml pull
echo -e "${GREEN}✓ Docker images updated${NC}"

# Step 7: Start Docker containers
echo -e "\n${BLUE}Step 7: Starting Docker containers...${NC}"
docker compose -f docker-compose.pi5.yml up -d
echo -e "${GREEN}✓ Containers started${NC}"

# Step 8: Wait for MinIO to be ready
echo -e "\n${BLUE}Step 8: Waiting for MinIO to be ready...${NC}"
for i in {1..30}; do
    if curl -sf http://localhost:9000/minio/health/live > /dev/null; then
        echo -e "${GREEN}✓ MinIO is healthy${NC}"
        break
    fi
    echo "Waiting for MinIO... ($i/30)"
    sleep 2
done

# Step 9: Verify deployment
echo -e "\n${BLUE}Step 9: Verifying deployment...${NC}"
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                    Migration Complete!                                      ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Access URLs:"
echo "  • MinIO Console: http://192.168.1.127:9001"
echo "  • MinIO API: http://192.168.1.127:9000"
echo "  • Frontend: http://192.168.1.127:3000"
echo ""
echo "Login to MinIO console with:"
echo "  Username: gaius"
echo "  Password: John 3:16"
echo ""
echo -e "${YELLOW}Note: Your old MinIO data is still at $OLD_MINIO_DATA${NC}"
echo "Once you verify everything works, you can remove it with:"
echo "  sudo rm -rf $OLD_MINIO_DATA"