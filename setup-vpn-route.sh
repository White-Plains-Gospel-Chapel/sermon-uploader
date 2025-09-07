#!/bin/bash

# Script to set up routing to Pi through VPN and deploy

echo "🌐 Setting up VPN route to Raspberry Pi"
echo "========================================"
echo ""

PI_IP="192.168.1.127"
VPN_INTERFACE="utun6"

# Check if we can already reach the Pi
echo "Testing current connection to Pi..."
if ping -c 1 -t 2 $PI_IP > /dev/null 2>&1; then
    echo "✅ Pi is already reachable!"
else
    echo "❌ Pi not reachable, setting up route..."
    
    # Remove existing route if it exists
    echo "Removing existing route (if any)..."
    sudo route delete -host $PI_IP 2>/dev/null || true
    
    # Add route through VPN
    echo "Adding route through VPN interface ($VPN_INTERFACE)..."
    sudo route add -host $PI_IP -interface $VPN_INTERFACE
    
    # Test connection again
    echo "Testing connection..."
    if ping -c 1 -t 2 $PI_IP > /dev/null 2>&1; then
        echo "✅ Route added successfully!"
    else
        echo "⚠️  Route added but Pi still not responding. Trying gateway route..."
        # Try with VPN gateway
        sudo route delete -host $PI_IP 2>/dev/null || true
        sudo route add -host $PI_IP 10.23.100.1
        
        if ping -c 1 -t 2 $PI_IP > /dev/null 2>&1; then
            echo "✅ Route through gateway successful!"
        else
            echo "❌ Still can't reach Pi. Check VPN connection."
            exit 1
        fi
    fi
fi

echo ""
echo "========================================"
echo "🚀 Deploying to Raspberry Pi"
echo "========================================"
echo ""

# Now deploy to the Pi
echo "Connecting to Pi and deploying..."
ssh gaius@$PI_IP << 'DEPLOY_SCRIPT'
echo "Connected to Pi successfully!"
echo ""

# Navigate to project
cd /home/gaius/sermon-uploader || exit 1

# Pull latest code
echo "📦 Pulling latest code..."
git pull origin master

# Make deployment script executable
chmod +x deploy-from-pi.sh

# Run deployment
echo "🚀 Running deployment script..."
./deploy-from-pi.sh

DEPLOY_SCRIPT

if [ $? -eq 0 ]; then
    echo ""
    echo "========================================"
    echo "✅ Deployment Successful!"
    echo "========================================"
    echo ""
    echo "Services should now be running at:"
    echo "  MinIO HTTPS: https://$PI_IP:9000"
    echo "  MinIO Console: https://$PI_IP:9001"
    echo "  Backend API: http://$PI_IP:8000"
    echo ""
    echo "Next steps:"
    echo "1. Open https://$PI_IP:9000 in your browser"
    echo "2. Accept the security certificate"
    echo "3. Test multipart upload endpoint:"
    echo "   curl -k http://$PI_IP:8000/api/upload/multipart/init \\"
    echo "     -H 'Content-Type: application/json' \\"
    echo "     -d '{\"filename\":\"test.wav\",\"fileSize\":734003200,\"fileHash\":\"test123\"}'"
else
    echo ""
    echo "❌ Deployment failed. Check the output above for errors."
    echo ""
    echo "Try running manually:"
    echo "  ssh gaius@$PI_IP"
    echo "  cd /home/gaius/sermon-uploader"
    echo "  ./deploy-from-pi.sh"
fi