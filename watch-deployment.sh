#!/bin/bash
set -e

echo "👀 GitHub Actions Deployment Monitor"
echo "===================================="

# Function to check if command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

if ! command_exists gh; then
  echo "❌ GitHub CLI not installed"
  echo "💡 Install with: brew install gh && gh auth login"
  exit 1
fi

# Check authentication
if ! gh auth status >/dev/null 2>&1; then
  echo "❌ GitHub CLI not authenticated"
  echo "💡 Run: gh auth login"
  exit 1
fi

# Get current commit
COMMIT_SHA=$(git rev-parse HEAD)
echo "📦 Current commit: ${COMMIT_SHA:0:7}"
echo ""

# Show recent workflow runs
echo "📊 Recent workflow runs:"
gh run list --limit 5 --json status,conclusion,createdAt,headSha,workflowName --jq '.[] | 
  "\(.createdAt | strptime("%Y-%m-%dT%H:%M:%SZ") | strftime("%H:%M:%S")) | " +
  (if .conclusion == "success" then "✅" 
   elif .conclusion == "failure" then "❌" 
   elif .status == "in_progress" then "⏳" 
   else "🟡" end) + " " +
  .workflowName + " | " + 
  (.headSha[:7]) + " | " + 
  (.status // "unknown") +
  (if .conclusion then " (" + .conclusion + ")" else "" end)'

echo ""

# Get the latest workflow run
echo "🔍 Getting latest workflow run..."
LATEST_RUN=$(gh run list --limit 1 --json databaseId,status,conclusion,headSha,workflowName,url)
RUN_ID=$(echo "$LATEST_RUN" | jq -r '.[0].databaseId')
RUN_STATUS=$(echo "$LATEST_RUN" | jq -r '.[0].status')
RUN_CONCLUSION=$(echo "$LATEST_RUN" | jq -r '.[0].conclusion // "pending"')
RUN_COMMIT=$(echo "$LATEST_RUN" | jq -r '.[0].headSha[:7]')
RUN_URL=$(echo "$LATEST_RUN" | jq -r '.[0].url')

echo "🎯 Latest run: $RUN_ID (commit: $RUN_COMMIT)"
echo "📊 Status: $RUN_STATUS ($RUN_CONCLUSION)"
echo "🔗 URL: $RUN_URL"
echo ""

case $RUN_STATUS in
  "completed")
    if [ "$RUN_CONCLUSION" = "success" ]; then
      echo "✅ Latest deployment completed successfully!"
      echo "🎉 Your Pi should be running the latest version"
    else
      echo "❌ Latest deployment failed!"
      echo ""
      echo "🔍 Error details:"
      gh run view "$RUN_ID" --json jobs --jq '.jobs[] | select(.conclusion == "failure") | 
        "❌ Job: " + .name + "\n   Status: " + .conclusion + "\n   Started: " + .startedAt + "\n   URL: " + .url'
      
      echo ""
      echo "📝 Error logs (last 20 lines):"
      FAILED_JOB=$(gh run view "$RUN_ID" --json jobs --jq '.jobs[] | select(.conclusion == "failure") | .name' | head -1)
      if [ -n "$FAILED_JOB" ]; then
        echo "--- From job: '$FAILED_JOB' ---"
        gh run view "$RUN_ID" --log --job "$FAILED_JOB" | tail -20
        echo "--- End of logs ---"
      fi
    fi
    ;;
  "in_progress")
    echo "⏳ Deployment is currently running..."
    echo "📊 Starting live monitoring (Ctrl+C to stop)..."
    echo ""
    
    # Start live monitoring
    gh run watch "$RUN_ID" --exit-status
    ;;
  "queued")
    echo "🏃‍♂️ Deployment is queued and will start soon..."
    echo "⏳ Would you like to wait and watch? (y/n)"
    read -r RESPONSE
    if [ "$RESPONSE" = "y" ] || [ "$RESPONSE" = "Y" ]; then
      gh run watch "$RUN_ID" --exit-status
    fi
    ;;
  *)
    echo "❓ Unknown status: $RUN_STATUS"
    ;;
esac

echo ""
echo "💡 Tips:"
echo "  • Run this script anytime: ./watch-deployment.sh"
echo "  • Auto-monitoring after push: Enabled via post-push hook"
echo "  • Manual check: ./check-deployment.sh"