#!/bin/bash

# Validate commit message format
# Usage: ./scripts/validate-commit-msg.sh <commit-msg-file>

set -e

COMMIT_MSG_FILE="$1"
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Skip merge commits and revert commits
if echo "$COMMIT_MSG" | grep -qE "^(Merge|Revert)"; then
    echo -e "${GREEN}✅ Merge/Revert commit, skipping validation${NC}"
    exit 0
fi

# Skip empty commits
if [ -z "$(echo "$COMMIT_MSG" | grep -v '^#' | head -n1 | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')" ]; then
    echo -e "${RED}❌ Empty commit message${NC}"
    exit 1
fi

# Extract the first line (subject)
SUBJECT=$(echo "$COMMIT_MSG" | head -n1)

# Conventional commit format: type(scope): description
# type: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
# scope: optional, can be anything
# description: lowercase, no period at the end

PATTERN='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?: .{1,50}$'

if ! echo "$SUBJECT" | grep -qE "$PATTERN"; then
    echo -e "${RED}❌ Invalid commit message format${NC}"
    echo -e "${YELLOW}Current message:${NC} $SUBJECT"
    echo ""
    echo -e "${YELLOW}Expected format:${NC} type(scope): description"
    echo ""
    echo -e "${YELLOW}Valid types:${NC}"
    echo "  - feat: A new feature"
    echo "  - fix: A bug fix"
    echo "  - docs: Documentation only changes"
    echo "  - style: Changes that do not affect the meaning of the code"
    echo "  - refactor: A code change that neither fixes a bug nor adds a feature"
    echo "  - perf: A code change that improves performance"
    echo "  - test: Adding missing tests"
    echo "  - build: Changes that affect the build system or dependencies"
    echo "  - ci: Changes to CI configuration files and scripts"
    echo "  - chore: Other changes that don't modify src or test files"
    echo "  - revert: Reverts a previous commit"
    echo ""
    echo -e "${YELLOW}Examples:${NC}"
    echo "  feat(upload): add drag and drop file upload"
    echo "  fix(api): handle file size validation errors"
    echo "  docs: update API documentation"
    echo "  test(upload): add unit tests for file validation"
    echo ""
    exit 1
fi

# Check subject length (should be <= 50 characters)
SUBJECT_LENGTH=$(echo "$SUBJECT" | wc -c)
if [ "$SUBJECT_LENGTH" -gt 51 ]; then
    echo -e "${YELLOW}⚠️ Subject line is longer than 50 characters (${SUBJECT_LENGTH})${NC}"
    echo "Consider shortening: $SUBJECT"
fi

# Check if subject starts with uppercase (should be lowercase after type)
TYPE_AND_SCOPE=$(echo "$SUBJECT" | cut -d':' -f1)
DESCRIPTION=$(echo "$SUBJECT" | cut -d':' -f2- | sed 's/^[[:space:]]*//')

if echo "$DESCRIPTION" | grep -qE '^[A-Z]'; then
    echo -e "${YELLOW}⚠️ Description should start with lowercase letter${NC}"
    echo "Current: $DESCRIPTION"
fi

# Check if subject ends with period
if echo "$SUBJECT" | grep -qE '\.$'; then
    echo -e "${YELLOW}⚠️ Subject line should not end with a period${NC}"
    echo "Current: $SUBJECT"
fi

# Check for empty lines after subject (if body exists)
LINES=$(echo "$COMMIT_MSG" | grep -v '^#' | wc -l)
if [ "$LINES" -gt 1 ]; then
    SECOND_LINE=$(echo "$COMMIT_MSG" | sed -n '2p')
    if [ -n "$SECOND_LINE" ]; then
        echo -e "${YELLOW}⚠️ Second line should be empty (separate subject from body)${NC}"
    fi
fi

# Check body line length (should be <= 72 characters)
BODY_LINES=$(echo "$COMMIT_MSG" | tail -n +3 | grep -v '^#')
if [ -n "$BODY_LINES" ]; then
    while IFS= read -r line; do
        if [ -n "$line" ] && [ ${#line} -gt 72 ]; then
            echo -e "${YELLOW}⚠️ Body line exceeds 72 characters: ${#line}${NC}"
            echo "Line: $line"
        fi
    done <<< "$BODY_LINES"
fi

# Check for breaking changes format
if echo "$COMMIT_MSG" | grep -qi "BREAKING CHANGE"; then
    if ! echo "$COMMIT_MSG" | grep -qE "BREAKING CHANGE: .+"; then
        echo -e "${YELLOW}⚠️ Breaking change should be formatted as 'BREAKING CHANGE: description'${NC}"
    fi
fi

# Check for issue references
if echo "$COMMIT_MSG" | grep -qE "#[0-9]+"; then
    echo -e "${GREEN}✅ Issue reference found${NC}"
fi

echo -e "${GREEN}✅ Commit message validation passed${NC}"
exit 0