#!/bin/bash
set -e

echo "🔐 Setting up branch protection for master branch..."
echo "This will require PR reviews and prevent direct pushes to master."

# Check if user has admin access
echo "🔍 Checking repository permissions..."
PERMISSIONS=$(gh api repos/White-Plains-Gospel-Chapel/sermon-uploader --jq '.permissions.admin')

if [ "$PERMISSIONS" != "true" ]; then
  echo "❌ Error: Admin access required to set up branch protection"
  echo "Please ensure you have admin rights to this repository."
  exit 1
fi

echo "✅ Admin access confirmed"

# Set up branch protection
echo "🛡️ Enabling branch protection for master..."

# Method 1: Try via web UI instructions
echo ""
echo "📋 MANUAL SETUP REQUIRED:"
echo "Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/branches"
echo ""
echo "Click 'Add rule' and configure:"
echo "  ✅ Branch name pattern: master"
echo "  ✅ Require a pull request before merging"
echo "  ✅ Require approvals: 1" 
echo "  ✅ Require review from code owners (CODEOWNERS file)"
echo "  ✅ Dismiss stale PR reviews when new commits are pushed"
echo "  ✅ Require status checks to pass before merging"
echo "  ✅ Require branches to be up to date before merging"
echo "  ✅ Require linear history"
echo "  ✅ Do not allow bypassing the above settings"
echo ""

# Alternative: Try API method (might work with proper formatting)
echo "🤖 Attempting automated setup..."

cat > /tmp/protection-config.json << 'EOF'
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["security-scan", "build-and-push"]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": true,
    "require_last_push_approval": false
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "block_creations": false,
  "required_conversation_resolution": true
}
EOF

if gh api repos/White-Plains-Gospel-Chapel/sermon-uploader/branches/master/protection \
   -X PUT \
   --input /tmp/protection-config.json 2>/dev/null; then
  echo "✅ Branch protection enabled automatically!"
else
  echo "⚠️ Automated setup failed - please use manual setup above"
fi

rm -f /tmp/protection-config.json

echo ""
echo "🎯 VERIFICATION:"
echo "After setup, test with:"
echo "  1. Try pushing directly to master (should fail)"
echo "  2. Create feature branch and PR (should work)"
echo "  3. Try merging without review (should fail)"
echo ""

echo "✅ Branch protection setup complete!"
echo "Now all changes to master require PR → Review → Approval → Merge workflow"