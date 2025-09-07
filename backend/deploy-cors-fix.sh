#!/bin/bash

# =============================================================================
# CORS Fix Deployment Script for Sermon Uploader
# =============================================================================
# This script deploys the CORS fixes from PR #55 to production
# Includes automated backup and rollback capabilities
# =============================================================================

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="sermon-uploader-cors-fix"
BINARY_PATH="${SCRIPT_DIR}/bin/${BINARY_NAME}"
SERVICE_NAME="sermon-uploader"
BACKUP_DIR="${SCRIPT_DIR}/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DEPLOYMENT_LOG="${SCRIPT_DIR}/logs/deployment_${TIMESTAMP}.log"
PI_HOST="${PI_HOST:-192.168.1.127}"
PI_USER="${PI_USER:-pi}"
REMOTE_SERVICE_PATH="/opt/sermon-uploader"
REMOTE_BINARY_NAME="sermon-uploader"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    local level=$1
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    echo -e "${timestamp} [${level}] ${message}" | tee -a "${DEPLOYMENT_LOG}"
    
    case $level in
        "ERROR") echo -e "${RED}❌ ${message}${NC}" ;;
        "SUCCESS") echo -e "${GREEN}✅ ${message}${NC}" ;;
        "WARN") echo -e "${YELLOW}⚠️  ${message}${NC}" ;;
        "INFO") echo -e "${BLUE}ℹ️  ${message}${NC}" ;;
    esac
}

# Error handler
error_exit() {
    log "ERROR" "$1"
    echo
    log "INFO" "Deployment failed. Check log: ${DEPLOYMENT_LOG}"
    exit 1
}

# Trap errors
trap 'error_exit "Deployment failed at line $LINENO"' ERR

# Pre-deployment checks
check_prerequisites() {
    log "INFO" "Running pre-deployment checks..."
    
    # Check if binary exists
    if [[ ! -f "${BINARY_PATH}" ]]; then
        error_exit "Binary not found: ${BINARY_PATH}"
    fi
    
    # Check binary architecture
    if ! file "${BINARY_PATH}" | grep -q "ARM aarch64"; then
        error_exit "Binary is not ARM64 architecture"
    fi
    
    # Check Pi connectivity
    if ! ping -c 1 "${PI_HOST}" &> /dev/null; then
        error_exit "Cannot reach Raspberry Pi at ${PI_HOST}"
    fi
    
    # Check SSH connectivity
    if ! ssh -o ConnectTimeout=5 "${PI_USER}@${PI_HOST}" "echo 'SSH connection test'" &> /dev/null; then
        error_exit "Cannot SSH to ${PI_USER}@${PI_HOST}"
    fi
    
    log "SUCCESS" "All prerequisites met"
}

# Create backup of current deployment
create_backup() {
    log "INFO" "Creating backup of current deployment..."
    
    # Create backup directory
    mkdir -p "${BACKUP_DIR}"
    
    # Create local backup info
    local backup_info="${BACKUP_DIR}/backup_${TIMESTAMP}.info"
    cat > "${backup_info}" << EOF
Backup created: $(date)
Git commit: $(git rev-parse HEAD)
Git branch: $(git branch --show-current)
Deployment target: ${PI_USER}@${PI_HOST}
Service: ${SERVICE_NAME}
EOF
    
    # Backup current binary from Pi
    ssh "${PI_USER}@${PI_HOST}" "
        if [[ -f '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}' ]]; then
            cp '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}' '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}.backup.${TIMESTAMP}'
            echo 'Binary backed up on Pi'
        else
            echo 'No existing binary to backup'
        fi
    "
    
    log "SUCCESS" "Backup created: backup_${TIMESTAMP}"
}

# Deploy new binary
deploy_binary() {
    log "INFO" "Deploying new binary to Raspberry Pi..."
    
    # Ensure remote directory exists
    ssh "${PI_USER}@${PI_HOST}" "sudo mkdir -p '${REMOTE_SERVICE_PATH}'"
    
    # Copy new binary
    scp "${BINARY_PATH}" "${PI_USER}@${PI_HOST}:/tmp/${REMOTE_BINARY_NAME}.new"
    
    # Install with proper permissions
    ssh "${PI_USER}@${PI_HOST}" "
        sudo mv '/tmp/${REMOTE_BINARY_NAME}.new' '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}'
        sudo chmod +x '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}'
        sudo chown root:root '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}'
    "
    
    log "SUCCESS" "Binary deployed successfully"
}

# Update configuration for CORS
update_configuration() {
    log "INFO" "Updating configuration for CORS fixes..."
    
    # Copy environment file with CORS settings
    if [[ -f "${SCRIPT_DIR}/.env" ]]; then
        scp "${SCRIPT_DIR}/.env" "${PI_USER}@${PI_HOST}:/tmp/.env.new"
        ssh "${PI_USER}@${PI_HOST}" "
            sudo mv '/tmp/.env.new' '${REMOTE_SERVICE_PATH}/.env'
            sudo chown root:root '${REMOTE_SERVICE_PATH}/.env'
            sudo chmod 600 '${REMOTE_SERVICE_PATH}/.env'
        "
        log "SUCCESS" "Environment configuration updated"
    else
        log "WARN" "No .env file found to deploy"
    fi
}

# Restart service
restart_service() {
    log "INFO" "Restarting sermon-uploader service..."
    
    ssh "${PI_USER}@${PI_HOST}" "
        # Stop existing process if running
        sudo pkill -f '${REMOTE_BINARY_NAME}' || true
        sleep 2
        
        # Start new service
        cd '${REMOTE_SERVICE_PATH}'
        nohup sudo ./${REMOTE_BINARY_NAME} > ./service.log 2>&1 &
        
        # Wait for service to start
        sleep 5
        
        # Check if service is running
        if pgrep -f '${REMOTE_BINARY_NAME}' > /dev/null; then
            echo 'Service started successfully'
        else
            echo 'Service failed to start'
            exit 1
        fi
    "
    
    log "SUCCESS" "Service restarted successfully"
}

# Verify deployment
verify_deployment() {
    log "INFO" "Verifying CORS deployment..."
    
    # Wait for service to be fully ready
    sleep 10
    
    # Test basic health check
    local health_url="http://${PI_HOST}:8000/api/health"
    if curl -f -s "${health_url}" > /dev/null; then
        log "SUCCESS" "Health check passed"
    else
        error_exit "Health check failed at ${health_url}"
    fi
    
    # Test CORS headers
    local cors_test=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type" \
        -X OPTIONS \
        "http://${PI_HOST}:8000/api/upload/presigned" || echo "000")
    
    if [[ "${cors_test}" == "200" ]]; then
        log "SUCCESS" "CORS preflight test passed"
    else
        error_exit "CORS preflight test failed (HTTP ${cors_test})"
    fi
    
    log "SUCCESS" "Deployment verification completed"
}

# Rollback function
rollback() {
    local backup_timestamp="${1:-}"
    
    if [[ -z "${backup_timestamp}" ]]; then
        log "ERROR" "No backup timestamp provided for rollback"
        return 1
    fi
    
    log "WARN" "Rolling back to backup: ${backup_timestamp}"
    
    ssh "${PI_USER}@${PI_HOST}" "
        if [[ -f '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}.backup.${backup_timestamp}' ]]; then
            sudo pkill -f '${REMOTE_BINARY_NAME}' || true
            sleep 2
            
            sudo cp '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}.backup.${backup_timestamp}' '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}'
            sudo chmod +x '${REMOTE_SERVICE_PATH}/${REMOTE_BINARY_NAME}'
            
            cd '${REMOTE_SERVICE_PATH}'
            nohup sudo ./${REMOTE_BINARY_NAME} > ./service.log 2>&1 &
            
            sleep 5
            
            if pgrep -f '${REMOTE_BINARY_NAME}' > /dev/null; then
                echo 'Rollback completed successfully'
            else
                echo 'Rollback failed - service not running'
                exit 1
            fi
        else
            echo 'Backup file not found'
            exit 1
        fi
    "
    
    log "SUCCESS" "Rollback completed"
}

# Main deployment function
deploy() {
    log "INFO" "Starting CORS fix deployment..."
    log "INFO" "Target: ${PI_USER}@${PI_HOST}"
    log "INFO" "Binary: ${BINARY_PATH}"
    log "INFO" "Timestamp: ${TIMESTAMP}"
    
    echo
    log "INFO" "=== DEPLOYMENT STEPS ==="
    
    check_prerequisites
    create_backup
    deploy_binary
    update_configuration
    restart_service
    verify_deployment
    
    echo
    log "SUCCESS" "🎉 CORS fix deployment completed successfully!"
    log "INFO" "Deployment log: ${DEPLOYMENT_LOG}"
    log "INFO" "Backup timestamp: ${TIMESTAMP}"
    
    echo
    log "INFO" "=== CORS FEATURES NOW AVAILABLE ==="
    log "INFO" "• Browser-based bulk file uploads"
    log "INFO" "• Proper preflight OPTIONS handling" 
    log "INFO" "• Cross-origin requests from frontend"
    log "INFO" "• All CORS headers correctly configured"
    
    echo
    log "INFO" "To rollback if needed: ${0} rollback ${TIMESTAMP}"
}

# Command line interface
case "${1:-deploy}" in
    "deploy")
        deploy
        ;;
    "rollback")
        if [[ -n "${2:-}" ]]; then
            rollback "$2"
        else
            error_exit "Usage: $0 rollback <timestamp>"
        fi
        ;;
    "verify")
        verify_deployment
        ;;
    *)
        echo "Usage: $0 {deploy|rollback <timestamp>|verify}"
        echo
        echo "Commands:"
        echo "  deploy              - Deploy CORS fixes to production"
        echo "  rollback <timestamp> - Rollback to previous backup"
        echo "  verify              - Verify current deployment"
        echo
        echo "Environment variables:"
        echo "  PI_HOST    - Raspberry Pi IP address (default: 192.168.1.127)"
        echo "  PI_USER    - SSH user for Pi (default: pi)"
        exit 1
        ;;
esac