#!/bin/bash

echo "🔧 Fixing Multiple GitHub Runners Issue"
echo "======================================="
echo ""
echo "This will stop all runners and ensure only one is running"
echo ""

# Stop all runner services
echo "Stopping all runner services..."
sudo systemctl stop 'actions.runner.*' 2>/dev/null || true
cd ~/actions-runner && sudo ./svc.sh stop 2>/dev/null || true

# Kill any remaining runner processes
echo "Cleaning up runner processes..."
pkill -f "Runner.Listener" || true
pkill -f "Runner.Worker" || true

# Check for multiple runner directories
echo ""
echo "Looking for runner installations..."
find ~ -maxdepth 3 -name "actions-runner*" -type d 2>/dev/null | while read dir; do
    echo "Found: $dir"
done

echo ""
echo "To properly fix this:"
echo "1. SSH into your Pi: ssh gaius@192.168.1.127"
echo "2. Stop all runners: sudo ./actions-runner/svc.sh stop"
echo "3. Uninstall extras: sudo ./actions-runner/svc.sh uninstall"
echo "4. Start just one: sudo ./actions-runner/svc.sh start"
echo ""
echo "Or remove all and reinstall:"
echo "  cd ~/actions-runner"
echo "  ./config.sh remove --token YOUR_REMOVE_TOKEN"
echo "  Then re-run setup-github-runner.sh"
