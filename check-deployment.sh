#!/bin/bash
set -e

echo "🔍 Checking deployment status..."

# Get repository info
REPO="White-Plains-Gospel-Chapel/sermon-uploader"
COMMIT_SHA=$(git rev-parse HEAD)

echo "📦 Latest commit: ${COMMIT_SHA:0:7}"
echo "🔗 Actions page: https://github.com/$REPO/actions"

# Simple status check using curl and GitHub API
echo "📡 Fetching latest workflow status..."

# Check latest workflow run status
WORKFLOW_STATUS=$(curl -s -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/$REPO/actions/runs?per_page=1" | \
  grep -o '"status":"[^"]*' | head -1 | cut -d'"' -f4)

WORKFLOW_CONCLUSION=$(curl -s -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/$REPO/actions/runs?per_page=1" | \
  grep -o '"conclusion":"[^"]*' | head -1 | cut -d'"' -f4)

echo "📊 Current status: $WORKFLOW_STATUS"

case $WORKFLOW_STATUS in
  "completed")
    if [ "$WORKFLOW_CONCLUSION" = "success" ]; then
      echo "✅ Deployment completed successfully!"
      echo "🎉 Your Pi is now running the latest version"
    else
      echo "❌ Deployment failed"
      echo "🔍 Check the actions page for details"
    fi
    ;;
  "in_progress")
    echo "⏳ Deployment is currently running..."
    echo "💡 Run this script again in a few minutes to check progress"
    ;;
  "queued")
    echo "🏃‍♂️ Deployment is queued and will start soon..."
    ;;
  *)
    echo "❓ Unknown status: $WORKFLOW_STATUS"
    ;;
esac

echo ""
echo "💡 For real-time monitoring, install GitHub CLI:"
echo "   brew install gh"
echo "   gh auth login" 
echo "   gh run watch"