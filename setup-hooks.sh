#!/bin/bash
set -e

echo "🔧 Setting up Git hooks for sermon-uploader project..."

# Check if we're in the right directory
if [ ! -f "backend/go.mod" ]; then
  echo "❌ Please run this script from the project root directory"
  exit 1
fi

# Configure Git to use our hooks directory
echo "📁 Configuring Git hooks path..."
git config core.hooksPath .githooks

# Make all hooks executable
echo "🔒 Making hooks executable..."
chmod +x .githooks/*

# Test the pre-commit hook
echo "🧪 Testing pre-commit hook..."
if [ -f ".githooks/pre-commit" ]; then
  echo "✅ Pre-commit hook found and executable"
else
  echo "❌ Pre-commit hook not found"
  exit 1
fi

echo ""
echo "🎉 Git hooks setup complete!"
echo ""
echo "📋 What happens now:"
echo "  • Every commit will run automatic checks"
echo "  • Go build, TypeScript, ESLint, and Docker validation"
echo "  • Commits are blocked if checks fail"
echo "  • This prevents failed GitHub Actions runs"
echo ""
echo "💡 To bypass hooks in emergency: git commit --no-verify"
echo "🔧 To run checks manually: ./.githooks/pre-commit"