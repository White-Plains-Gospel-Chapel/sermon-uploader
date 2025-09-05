#!/bin/bash
set -e

# API Upload Test Suite Setup Script
# ==================================
# 
# This script sets up the comprehensive API upload testing environment
# for testing with real 40GB WAV files from ridgepoint Pi.

echo "🚀 Setting up API Upload Test Suite"
echo "===================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Check if we're in the right directory
if [[ ! -f "api_upload_tests.py" ]]; then
    print_error "Please run this script from the test_automation directory"
    exit 1
fi

print_info "Checking system requirements..."

# Check Python version
if command -v python3 &> /dev/null; then
    PYTHON_VERSION=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:2])))')
    if [[ $(echo "$PYTHON_VERSION >= 3.8" | bc -l) -eq 1 ]]; then
        print_status "Python $PYTHON_VERSION found"
    else
        print_error "Python 3.8+ required, found $PYTHON_VERSION"
        exit 1
    fi
else
    print_error "Python 3 not found. Please install Python 3.8+"
    exit 1
fi

# Check if pip is available
if ! command -v pip &> /dev/null; then
    if ! command -v pip3 &> /dev/null; then
        print_error "pip not found. Please install pip"
        exit 1
    else
        alias pip=pip3
    fi
fi

print_status "pip found"

# Install Python dependencies
print_info "Installing Python dependencies..."
if pip install -r requirements.txt; then
    print_status "Python dependencies installed"
else
    print_error "Failed to install Python dependencies"
    exit 1
fi

# Check SSH connectivity to ridgepoint Pi
print_info "Testing ridgepoint Pi connectivity..."
RIDGEPOINT_HOST="ridgepoint.local"
RIDGEPOINT_USER="gaius"

if timeout 10 ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$RIDGEPOINT_USER@$RIDGEPOINT_HOST" "echo 'SSH connection test successful'" &> /dev/null; then
    print_status "ridgepoint Pi SSH connection verified"
else
    print_warning "Could not connect to ridgepoint Pi via SSH"
    print_info "You may need to:"
    echo "  1. Ensure ridgepoint Pi is on the network"
    echo "  2. Add SSH key: ssh-copy-id $RIDGEPOINT_USER@$RIDGEPOINT_HOST"
    echo "  3. Or configure password-less SSH access"
fi

# Check WAV files availability
print_info "Checking WAV files on ridgepoint Pi..."
if timeout 15 ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$RIDGEPOINT_USER@$RIDGEPOINT_HOST" "find /home/gaius/data -name '*.wav' -type f | wc -l" &> /dev/null; then
    WAV_COUNT=$(timeout 15 ssh -o ConnectTimeout=5 "$RIDGEPOINT_USER@$RIDGEPOINT_HOST" "find /home/gaius/data -name '*.wav' -type f | wc -l" 2>/dev/null || echo "0")
    if [[ "$WAV_COUNT" -gt 0 ]]; then
        print_status "$WAV_COUNT WAV files found on ridgepoint Pi"
    else
        print_warning "No WAV files found on ridgepoint Pi"
    fi
else
    print_warning "Could not check WAV files on ridgepoint Pi"
fi

# Check if Go is installed (for running the API server)
print_info "Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | head -1)
    print_status "Go $GO_VERSION found"
    
    # Check if we can build the server
    print_info "Testing API server build..."
    cd ..
    if go build -o test_server main.go; then
        print_status "API server builds successfully"
        rm -f test_server
    else
        print_warning "Could not build API server"
    fi
    cd test_automation
else
    print_warning "Go not found. You'll need Go to run the API server"
    print_info "Install Go from: https://golang.org/dl/"
fi

# Create configuration file if it doesn't exist
if [[ ! -f "api_test_config.json" ]]; then
    print_info "Creating default configuration file..."
    cat > api_test_config.json << 'EOF'
{
  "api": {
    "base_url": "http://localhost:8000",
    "timeout": 300,
    "max_retries": 3,
    "retry_backoff": 1.0
  },
  "ridgepoint": {
    "hostname": "ridgepoint.local",
    "username": "gaius",
    "private_key_path": null,
    "ssh_timeout": 30
  },
  "testing": {
    "max_concurrent_uploads": 5,
    "chunk_size": 1048576,
    "retry_attempts": 3,
    "test_timeout": 600,
    "progress_interval": 10
  },
  "file_selection": {
    "small_files_limit": 3,
    "medium_files_limit": 3,
    "large_files_limit": 2,
    "xlarge_files_limit": 2
  },
  "performance_targets": {
    "min_throughput_mbps": 5.0,
    "max_api_response_time": 2.0,
    "target_success_rate": 95.0
  },
  "reporting": {
    "detailed_logs": true,
    "save_failed_requests": true,
    "generate_charts": false
  }
}
EOF
    print_status "Configuration file created: api_test_config.json"
else
    print_status "Configuration file already exists"
fi

# Create reports directory
if [[ ! -d "reports" ]]; then
    mkdir -p reports
    print_status "Reports directory created"
fi

# Make scripts executable
chmod +x setup_tests.sh
chmod +x api_upload_tests.py
chmod +x sunday_morning_stress_test.py
chmod +x run_api_tests.py
print_status "Scripts made executable"

# Test basic Python imports
print_info "Testing Python module imports..."
if python3 -c "
import requests, paramiko, json, time, hashlib, logging, threading
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple, Any
from dataclasses import dataclass, asdict
from concurrent.futures import ThreadPoolExecutor, as_completed
print('All required modules imported successfully')
" 2>/dev/null; then
    print_status "Python module imports successful"
else
    print_error "Some Python modules failed to import"
    print_info "Try running: pip install -r requirements.txt"
fi

echo ""
echo "📋 Setup Summary"
echo "================"
echo ""

# Final status check
SETUP_SUCCESS=true

# Check critical requirements
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 not available"
    SETUP_SUCCESS=false
fi

if ! pip show requests &> /dev/null; then
    print_error "Required Python packages not installed"
    SETUP_SUCCESS=false
fi

if [[ "$SETUP_SUCCESS" == true ]]; then
    print_status "Setup completed successfully!"
    echo ""
    echo "🚀 Quick Start Commands:"
    echo ""
    echo "1. Start the API server (in another terminal):"
    echo "   cd .. && go run main.go"
    echo ""
    echo "2. Run quick validation:"
    echo "   python3 run_api_tests.py --quick-validation"
    echo ""
    echo "3. Run full test suite:"
    echo "   python3 run_api_tests.py --full-suite"
    echo ""
    echo "4. Run stress tests only:"
    echo "   python3 run_api_tests.py --stress-only"
    echo ""
    echo "📁 Configuration file: api_test_config.json"
    echo "📊 Reports will be saved to: reports/"
    echo "📚 Documentation: README.md"
else
    print_error "Setup completed with errors"
    echo ""
    echo "Please resolve the issues above before running tests."
fi

echo ""
echo "🔧 Advanced Usage:"
echo ""
echo "• Test specific file sizes:"
echo "  python3 api_upload_tests.py --test single --size large --files 3"
echo ""
echo "• Test batch uploads:"
echo "  python3 api_upload_tests.py --test batch --files 10 --batch-size 5"
echo ""
echo "• Run specific stress scenario:"
echo "  python3 sunday_morning_stress_test.py --scenario Sunday_Immediate_Rush"
echo ""
echo "• Enable debug logging:"
echo "  export LOG_LEVEL=DEBUG && python3 run_api_tests.py --quick-validation"
echo ""

if [[ -f "../main.go" ]]; then
    echo "💡 To start testing immediately:"
    echo ""
    echo "Terminal 1: cd .. && go run main.go"
    echo "Terminal 2: python3 run_api_tests.py --quick-validation"
    echo ""
fi

print_info "Setup complete! Check README.md for detailed usage instructions."