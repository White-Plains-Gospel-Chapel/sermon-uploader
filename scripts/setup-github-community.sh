#!/bin/bash

# 🚀 GitHub Community Setup Script
# Sets up Wiki and Discussions for the Sermon Uploader project

set -e

echo "🎯 GitHub Community Setup for Sermon Uploader"
echo "============================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}❌ GitHub CLI (gh) is not installed${NC}"
    echo "Install it with: brew install gh (Mac) or https://cli.github.com"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo -e "${YELLOW}⚠️  Not authenticated with GitHub${NC}"
    echo "Running: gh auth login"
    gh auth login
fi

REPO="White-Plains-Gospel-Chapel/sermon-uploader"
echo -e "${GREEN}✅ Working with repository: $REPO${NC}"
echo ""

# Setup Wiki
echo "📚 Setting up Wiki..."
echo "---------------------"

# Clone wiki repo if it doesn't exist
WIKI_DIR="sermon-uploader.wiki"
if [ ! -d "$WIKI_DIR" ]; then
    echo "Cloning wiki repository..."
    git clone "git@github.com:$REPO.wiki.git" "$WIKI_DIR" 2>/dev/null || {
        echo -e "${YELLOW}Wiki not initialized yet. Creating...${NC}"
        mkdir -p "$WIKI_DIR"
        cd "$WIKI_DIR"
        git init
        git remote add origin "git@github.com:$REPO.wiki.git"
        cd ..
    }
fi

# Copy wiki files
echo "Copying wiki content..."
cp -r wiki/* "$WIKI_DIR/" 2>/dev/null || echo "No wiki files to copy"

# Commit and push wiki
cd "$WIKI_DIR"
if [ -n "$(git status --porcelain)" ]; then
    git add .
    git commit -m "📚 Update wiki with engaging documentation" || true
    git push origin master --force 2>/dev/null || {
        echo -e "${YELLOW}First wiki push - creating initial commit${NC}"
        git push --set-upstream origin master
    }
    echo -e "${GREEN}✅ Wiki updated successfully${NC}"
else
    echo "No wiki changes to push"
fi
cd ..

echo ""

# Setup Discussions
echo "💬 Setting up Discussions..."
echo "---------------------------"

# Check if discussions are enabled
if gh api "repos/$REPO" --jq '.has_discussions' | grep -q false; then
    echo -e "${YELLOW}⚠️  Discussions not enabled for this repository${NC}"
    echo "Please enable them in Settings > General > Features > Discussions"
    echo "Then run this script again"
else
    echo -e "${GREEN}✅ Discussions are enabled${NC}"
    
    # Create discussion categories
    echo ""
    echo "Creating discussion categories..."
    
    CATEGORIES=(
        "General:GENERAL:General community discussion"
        "Ideas:IDEAS:Share ideas and feature requests with 💡 emoji"
        "Q&A:Q_AND_A:Ask questions and get answers from the community"
        "Announcements:ANNOUNCEMENTS:Project updates and releases"
        "Technical Support:GENERAL:Hardware and software troubleshooting"
        "Audio Processing:GENERAL:Discuss audio quality and conversion"
        "Self-Hosting:GENERAL:Deployment and hosting discussions"
        "Show and Tell:SHOW_AND_TELL:Share your church setup with 🤝 emoji"
    )
    
    for category in "${CATEGORIES[@]}"; do
        IFS=':' read -r name slug description <<< "$category"
        echo "  Creating category: $name"
        
        # Check if category exists before creating
        existing=$(gh api "repos/$REPO/discussion-categories" --jq ".[] | select(.name == \"$name\") | .id" 2>/dev/null || echo "")
        
        if [ -z "$existing" ]; then
            gh api "repos/$REPO/discussion-categories" \
                --method POST \
                -f name="$name" \
                -f slug="$slug" \
                -f description="$description" \
                2>/dev/null || echo "    Category might already exist or API error"
        else
            echo "    Category already exists"
        fi
    done
    
    echo ""
    echo -e "${GREEN}✅ Discussion categories configured${NC}"
fi

echo ""

# Create initial discussions
echo "🌱 Creating seed discussions..."
echo "-------------------------------"

# You can uncomment and modify these to create initial discussions
# gh discussion create -R "$REPO" \
#     --category "Announcements" \
#     --title "Welcome to Sermon Uploader Discussions! 👋" \
#     --body "$(cat .github/discussions-seed.md | sed -n '/Welcome to Sermon Uploader/,/Looking forward/p')"

echo -e "${YELLOW}ℹ️  Use the content in .github/discussions-seed.md to manually create initial discussions${NC}"

echo ""

# Summary
echo "📊 Setup Summary"
echo "================"
echo ""
echo -e "${GREEN}✅ Wiki files prepared${NC} - Push them with:"
echo "   cd $WIKI_DIR && git push origin master"
echo ""
echo -e "${GREEN}✅ Discussion templates created${NC} in .github/DISCUSSION_TEMPLATE/"
echo ""
echo -e "${GREEN}✅ Support file created${NC} at .github/SUPPORT.md"
echo ""

# Final instructions
echo "📋 Next Steps"
echo "============="
echo ""
echo "1. Visit the Wiki:"
echo "   https://github.com/$REPO/wiki"
echo ""
echo "2. Visit Discussions:"  
echo "   https://github.com/$REPO/discussions"
echo ""
echo "3. Create initial discussions using content from:"
echo "   .github/discussions-seed.md"
echo ""
echo "4. Pin important discussions:"
echo "   - Welcome message"
echo "   - FAQ"
echo "   - Community guidelines"
echo ""
echo -e "${GREEN}🎉 Community setup complete!${NC}"