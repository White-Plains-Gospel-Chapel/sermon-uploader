#!/bin/bash

# Fix VPN routing to access remote network where Pi is located
# The Pi (192.168.1.127) is on the remote network, accessible through VPN

echo "🌐 Setting up VPN Route to Remote Network"
echo "=========================================="
echo ""

# Configuration
PI_IP="192.168.1.127"
REMOTE_NETWORK="192.168.1.0/24"  # The entire remote network
VPN_GATEWAY="10.23.100.1"        # Your VPN gateway
VPN_INTERFACE="utun6"            # Your VPN interface

echo "Current routing table for 192.168.1.x:"
netstat -rn | grep "192.168.1" | head -5
echo ""

echo "Step 1: Removing conflicting local network route..."
# Remove the route that's pointing to local interface (en0)
sudo route delete -net 192.168.1.0/24 2>/dev/null || true
sudo route delete -host 192.168.1.127 2>/dev/null || true

echo "Step 2: Adding route through VPN..."
# Add route to remote network through VPN gateway
sudo route add -net 192.168.1.0/24 10.23.100.1

echo ""
echo "New routing table for 192.168.1.x:"
netstat -rn | grep "192.168.1" | head -5

echo ""
echo "Step 3: Testing connection to Pi..."
if ping -c 2 -t 3 192.168.1.127 > /dev/null 2>&1; then
    echo "✅ Success! Pi is now reachable through VPN"
    echo ""
    echo "Step 4: Testing SSH connection..."
    
    if ssh -o ConnectTimeout=5 gaius@192.168.1.127 "echo 'SSH working!' && hostname" 2>/dev/null; then
        echo "✅ SSH connection successful!"
        echo ""
        echo "=========================================="
        echo "Ready to deploy! Run:"
        echo "  ssh gaius@192.168.1.127"
        echo "  cd /home/gaius/sermon-uploader"
        echo "  git pull && ./deploy-from-pi.sh"
        echo "=========================================="
    else
        echo "⚠️  Pi is reachable but SSH failed. Possible issues:"
        echo "  - SSH service not running on Pi"
        echo "  - Wrong username (not 'gaius')"
        echo "  - SSH key not set up"
        echo ""
        echo "Try with password:"
        echo "  ssh gaius@192.168.1.127"
    fi
else
    echo "⚠️  Pi still not reachable. Trying alternate approach..."
    
    # Try adding specific host route
    sudo route delete -net 192.168.1.0/24 2>/dev/null || true
    sudo route add -host 192.168.1.127 -interface utun6
    
    if ping -c 2 -t 3 192.168.1.127 > /dev/null 2>&1; then
        echo "✅ Success with interface route!"
    else
        echo "❌ Cannot reach Pi. Possible issues:"
        echo "  1. VPN is not fully connected"
        echo "  2. Pi is offline"
        echo "  3. Firewall blocking traffic"
        echo "  4. Remote network is not 192.168.1.0/24"
        echo ""
        echo "Debug info:"
        echo "VPN Interface (utun6):"
        ifconfig utun6 | grep inet
        echo ""
        echo "Try manually:"
        echo "  sudo route add -net 192.168.1.0/24 10.23.100.1"
        echo "  ping 192.168.1.127"
    fi
fi

echo ""
echo "To reverse these changes (go back to local network):"
echo "  sudo route delete -net 192.168.1.0/24"
echo "  sudo route add -net 192.168.1.0/24 -interface en0"