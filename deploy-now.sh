#!/bin/bash

# Quick deployment script
# This commits and pushes all changes to trigger GitHub Actions deployment

echo "🚀 Deploying Multipart Upload System to Raspberry Pi"
echo "===================================================="
echo ""

# Make scripts executable
chmod +x setup-github-secrets.sh
chmod +x setup-on-pi.sh
chmod +x scripts/*.sh

# Check GitHub secrets are configured
if ! gh secret list | grep -q PI_HOST; then
    echo "⚠️  GitHub secrets not configured. Running setup..."
    ./setup-github-secrets.sh
fi

# Add all changes
echo "📦 Preparing changes for deployment..."
git add -A

# Show what's being deployed
echo ""
echo "Files to deploy:"
git status --short

echo ""
read -p "Commit message [feat: implement multipart upload system]: " COMMIT_MSG
COMMIT_MSG=${COMMIT_MSG:-"feat: implement multipart upload system"}

# Commit changes
git commit -m "$COMMIT_MSG

- Add multipart upload backend with 5MB chunks
- Implement server-controlled queue (1 file at a time)
- Add MinIO native TLS support
- Create chunked uploader frontend
- Add resumability with session management
- Configure GitHub Actions deployment

🤖 Generated with Claude Code

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push to trigger deployment
echo ""
echo "🚀 Pushing to GitHub (this will trigger deployment)..."
git push

echo ""
echo "===================================================="
echo "✅ Deployment triggered!"
echo "===================================================="
echo ""
echo "Monitor deployment:"
echo "  GitHub: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/actions"
echo "  Discord: Check #sermons-uploading-notif channel"
echo ""
echo "Once deployed, test with:"
echo "  curl -k https://192.168.1.127:9000/minio/health/live"
echo "  curl -k https://192.168.1.127:8000/api/health"
echo ""
echo "To manually trigger deployment:"
echo "  gh workflow run deploy-to-pi.yml -f deploy_type=both"