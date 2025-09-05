#!/bin/bash

# Validate branch naming conventions
# Usage: ./scripts/validate-branch-name.sh [branch-name]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get branch name
if [ -n "$1" ]; then
    BRANCH_NAME="$1"
else
    BRANCH_NAME=$(git rev-parse --abbrev-ref HEAD)
fi

echo "Validating branch name: $BRANCH_NAME"

# Allow main branches
if [[ "$BRANCH_NAME" =~ ^(main|master|develop|staging)$ ]]; then
    echo -e "${GREEN}✅ Main branch, validation passed${NC}"
    exit 0
fi

# Valid branch name patterns:
# feature/description-of-feature
# fix/description-of-fix
# hotfix/description-of-hotfix
# release/version-number
# refactor/description-of-refactor
# docs/description-of-docs
# test/description-of-test
# chore/description-of-chore
# perf/description-of-perf

VALID_PATTERN='^(feature|fix|hotfix|release|refactor|docs|test|chore|perf)\/[a-z0-9\-]+$'

if [[ ! "$BRANCH_NAME" =~ $VALID_PATTERN ]]; then
    echo -e "${RED}❌ Invalid branch name format${NC}"
    echo -e "${YELLOW}Current branch:${NC} $BRANCH_NAME"
    echo ""
    echo -e "${YELLOW}Valid formats:${NC}"
    echo "  feature/description-of-feature"
    echo "  fix/description-of-fix"
    echo "  hotfix/description-of-hotfix"
    echo "  release/version-number"
    echo "  refactor/description-of-refactor"
    echo "  docs/description-of-docs"
    echo "  test/description-of-test"
    echo "  chore/description-of-chore"
    echo "  perf/description-of-perf"
    echo ""
    echo -e "${YELLOW}Examples:${NC}"
    echo "  feature/file-upload-drag-drop"
    echo "  fix/api-timeout-handling"
    echo "  hotfix/security-vulnerability"
    echo "  release/v1.2.0"
    echo "  docs/api-documentation"
    echo ""
    echo -e "${YELLOW}Rules:${NC}"
    echo "  - Use lowercase letters, numbers, and hyphens only"
    echo "  - Start with a valid type followed by a forward slash"
    echo "  - Use descriptive, kebab-case naming"
    echo "  - Avoid special characters except hyphens"
    echo ""
    exit 1
fi

# Additional validation: check length
if [ ${#BRANCH_NAME} -gt 50 ]; then
    echo -e "${YELLOW}⚠️ Branch name is quite long (${#BRANCH_NAME} characters)${NC}"
    echo "Consider shortening for better readability"
fi

# Check for consecutive hyphens
if echo "$BRANCH_NAME" | grep -qE '\-\-'; then
    echo -e "${YELLOW}⚠️ Branch name contains consecutive hyphens${NC}"
    echo "Consider using single hyphens for better readability"
fi

# Check for leading/trailing hyphens in the description part
DESCRIPTION=$(echo "$BRANCH_NAME" | cut -d'/' -f2)
if [[ "$DESCRIPTION" =~ ^- ]] || [[ "$DESCRIPTION" =~ -$ ]]; then
    echo -e "${YELLOW}⚠️ Branch description starts or ends with hyphen${NC}"
    echo "Consider removing leading/trailing hyphens"
fi

echo -e "${GREEN}✅ Branch name validation passed${NC}"
exit 0