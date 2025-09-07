#!/bin/bash
# Remote deployment script - runs commands via curl to trigger deployment

echo "🚀 Triggering remote deployment on Pi..."

# First, let's build and push to a local registry or use the binary directly
echo "📦 Building Docker image locally..."
docker build -t sermon-uploader:cors-fix .

# Save the image to a tar file
echo "💾 Saving Docker image..."
docker save sermon-uploader:cors-fix | gzip > sermon-uploader-cors-fix.tar.gz

echo "📤 Image saved ($(du -h sermon-uploader-cors-fix.tar.gz | cut -f1))"

# Now we need to get this to the Pi
# Since SSH isn't working, let's try a different approach

# Option 1: Start a temporary HTTP server to serve the image
echo "🌐 Starting temporary HTTP server..."
python3 -m http.server 8888 &
SERVER_PID=$!

echo "📡 Server started on port 8888"
echo ""
echo "📋 Manual steps needed on the Pi:"
echo "1. SSH to Pi: ssh pi@192.168.1.127"
echo "2. Download image:"
echo "   wget http://$(ipconfig getifaddr en0):8888/sermon-uploader-cors-fix.tar.gz"
echo "3. Load image:"
echo "   docker load < sermon-uploader-cors-fix.tar.gz"
echo "4. Update docker-compose.prod.yml to use sermon-uploader:cors-fix"
echo "5. Restart services:"
echo "   docker-compose -f docker-compose.prod.yml down"
echo "   docker-compose -f docker-compose.prod.yml up -d"
echo ""
echo "Press Ctrl+C when done to stop the server"

# Wait for user to complete manual steps
wait

# Cleanup
kill $SERVER_PID 2>/dev/null