#!/bin/bash

# Test MinIO connectivity from Docker container
# This script tests different ways to connect to MinIO from within Docker

echo "🔍 Testing MinIO Connectivity from Docker Container"
echo "=================================================="

# Test different endpoints
ENDPOINTS=(
    "host.docker.internal:9000"
    "172.17.0.1:9000"  # Docker bridge gateway
    "192.168.1.127:9000"  # Direct IP
    "localhost:9000"  # Won't work from container
)

echo "Testing from within a Docker container..."
for endpoint in "${ENDPOINTS[@]}"; do
    echo -n "Testing $endpoint... "
    
    # Run a test container to check connectivity
    result=$(docker run --rm --add-host=host.docker.internal:host-gateway alpine:latest \
        sh -c "wget -q -O /dev/null -T 5 http://$endpoint/minio/health/live && echo 'SUCCESS' || echo 'FAILED'" 2>/dev/null)
    
    if [ "$result" = "SUCCESS" ]; then
        echo "✅ SUCCESS - Can connect to MinIO at $endpoint"
        WORKING_ENDPOINT=$endpoint
    else
        echo "❌ FAILED - Cannot connect to MinIO at $endpoint"
    fi
done

echo ""
if [ -n "$WORKING_ENDPOINT" ]; then
    echo "✅ Found working endpoint: $WORKING_ENDPOINT"
    echo "Use this in your docker-compose.yml:"
    echo "  MINIO_ENDPOINT=$WORKING_ENDPOINT"
else
    echo "❌ No working endpoint found. MinIO may not be accessible from Docker."
    echo "Checking MinIO status on host..."
    systemctl status minio --no-pager | head -10
fi