#!/bin/bash

# Deploy admin dashboard to admin.wpgc.church
# This can be deployed to a separate server or the same Pi

set -e

echo "🚀 Building Admin Dashboard for Production"
echo "========================================="

# Build the Next.js app
echo "📦 Building Next.js application..."
npm run build

# The built app will be in .next directory
# You can deploy this to:
# 1. Vercel (easiest for Next.js)
# 2. Your Pi with Node.js
# 3. A VPS with Node.js

echo ""
echo "✅ Build complete!"
echo ""
echo "Deployment Options:"
echo ""
echo "Option 1: Deploy to Vercel (Recommended for Next.js)"
echo "  1. Install Vercel CLI: npm i -g vercel"
echo "  2. Run: vercel --prod"
echo "  3. Set custom domain to admin.wpgc.church"
echo ""
echo "Option 2: Deploy to your Pi"
echo "  1. Copy files to Pi:"
echo "     rsync -avz .next package.json node_modules gaius@192.168.1.127:/opt/admin-dashboard/"
echo "  2. On Pi, run:"
echo "     cd /opt/admin-dashboard"
echo "     npm start"
echo "  3. Setup nginx to proxy port 3000 to admin.wpgc.church"
echo ""
echo "Option 3: Docker deployment"
echo "  See Dockerfile.admin for containerized deployment"