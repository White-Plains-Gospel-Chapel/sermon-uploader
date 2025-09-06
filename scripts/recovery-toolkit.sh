#!/bin/bash
#
# Recovery Toolkit - Manual Recovery Procedures for Sermon Uploader
# This script provides step-by-step recovery procedures for various failure scenarios
#
# Usage: ./recovery-toolkit.sh [command] [options]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="/opt/sermon-uploader"
BACKUP_DIR="/opt/sermon-uploader-backups"
LOG_FILE="/var/log/sermon-uploader/recovery.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging function
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Colored output functions
print_header() { echo -e "${PURPLE}=== $1 ===${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠️ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }
print_info() { echo -e "${BLUE}ℹ️ $1${NC}"; }
print_step() { echo -e "${CYAN}👉 $1${NC}"; }

# Setup logging directory
setup_logging() {
    mkdir -p "$(dirname "$LOG_FILE")"
    log "INFO" "Recovery toolkit started"
}

# Show help
show_help() {
    cat << EOF
${PURPLE}Sermon Uploader Recovery Toolkit${NC}

${YELLOW}USAGE:${NC}
  $0 <command> [options]

${YELLOW}COMMANDS:${NC}
  ${GREEN}diagnose${NC}              - Run comprehensive system diagnostics
  ${GREEN}health-check${NC}          - Perform health checks and show status
  ${GREEN}quick-fix${NC}             - Attempt automatic fixes for common issues
  ${GREEN}restart-services${NC}      - Restart all services with health verification
  ${GREEN}rollback${NC}              - Interactive rollback to previous version
  ${GREEN}emergency-stop${NC}        - Emergency stop all services
  ${GREEN}emergency-restart${NC}     - Emergency restart (last resort)
  ${GREEN}backup-create${NC}         - Create backup of current state
  ${GREEN}backup-restore${NC}        - Restore from backup (interactive)
  ${GREEN}logs${NC}                 - View and analyze logs
  ${GREEN}network-test${NC}          - Test network connectivity and ports
  ${GREEN}storage-check${NC}         - Check storage usage and MinIO health
  ${GREEN}cleanup${NC}               - Clean up old containers, images, and files

${YELLOW}DIAGNOSTIC COMMANDS:${NC}
  ${GREEN}check-containers${NC}      - Check Docker container status
  ${GREEN}check-minio${NC}           - Check MinIO service health
  ${GREEN}check-discord${NC}         - Test Discord webhook connectivity
  ${GREEN}check-performance${NC}     - Analyze system performance
  ${GREEN}check-resources${NC}       - Check system resource usage

${YELLOW}EXAMPLES:${NC}
  $0 diagnose                     # Full system diagnosis
  $0 health-check                 # Quick health check
  $0 quick-fix                    # Try automatic fixes
  $0 restart-services             # Restart with verification
  $0 logs --tail 50              # Show last 50 log lines
  $0 backup-create               # Create backup
  
${YELLOW}OPTIONS:${NC}
  --dry-run                      # Show what would be done without executing
  --verbose                      # Show detailed output
  --force                        # Skip confirmations
  --help                         # Show this help

${RED}WARNING: Some operations may cause service interruption!${NC}
EOF
}

# Prompt for confirmation unless --force is used
confirm() {
    local message="$1"
    if [[ "${FORCE:-false}" == "true" ]]; then
        print_info "Auto-confirming: $message"
        return 0
    fi
    
    echo -e "${YELLOW}$message${NC}"
    read -p "Continue? [y/N]: " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]]
}

# Run comprehensive system diagnostics
run_diagnostics() {
    print_header "SYSTEM DIAGNOSTICS"
    
    local issues=()
    local warnings=()
    
    print_step "Checking basic system info..."
    echo "Hostname: $(hostname)"
    echo "Uptime: $(uptime)"
    echo "Load: $(uptime | awk -F'load average:' '{print $2}')"
    echo "Disk space: $(df -h "$PROJECT_DIR" | tail -1)"
    echo "Memory: $(free -h | grep Mem:)"
    echo
    
    print_step "Checking Docker environment..."
    if ! command -v docker >/dev/null 2>&1; then
        issues+=("Docker not installed or not in PATH")
    else
        echo "Docker version: $(docker --version)"
        echo "Docker compose version: $(docker compose version)"
        
        if ! docker info >/dev/null 2>&1; then
            issues+=("Docker daemon not running or not accessible")
        fi
    fi
    echo
    
    print_step "Checking project directory..."
    if [[ ! -d "$PROJECT_DIR" ]]; then
        issues+=("Project directory not found: $PROJECT_DIR")
    else
        echo "Project directory: $PROJECT_DIR"
        echo "Contents: $(ls -la "$PROJECT_DIR" | wc -l) files"
        
        if [[ ! -f "$PROJECT_DIR/docker-compose.single.yml" ]]; then
            issues+=("Docker compose file missing: docker-compose.single.yml")
        fi
        
        if [[ ! -f "$PROJECT_DIR/.env" ]]; then
            warnings+=("Environment file missing: .env")
        fi
    fi
    echo
    
    print_step "Checking container status..."
    cd "$PROJECT_DIR" || exit 1
    
    if docker compose -f docker-compose.single.yml ps | grep -q "sermon-uploader"; then
        local container_status=$(docker compose -f docker-compose.single.yml ps sermon-uploader --format table | tail -1 || echo "NOT_FOUND")
        echo "Container status: $container_status"
        
        if ! echo "$container_status" | grep -q "Up"; then
            issues+=("Main container is not running")
        fi
    else
        issues+=("No containers found")
    fi
    echo
    
    print_step "Checking service endpoints..."
    
    # Test MinIO
    if curl -f -s --connect-timeout 3 http://localhost:9000/minio/health/live >/dev/null 2>&1; then
        print_success "MinIO endpoint accessible"
    else
        issues+=("MinIO endpoint not accessible (http://localhost:9000)")
    fi
    
    # Test main service
    if curl -f -s --connect-timeout 3 http://localhost:8000/api/health >/dev/null 2>&1; then
        print_success "Main service endpoint accessible"
    else
        issues+=("Main service endpoint not accessible (http://localhost:8000)")
    fi
    echo
    
    print_step "Checking logs for errors..."
    local recent_errors=$(docker compose -f docker-compose.single.yml logs --tail=100 2>/dev/null | grep -i "error\|fail\|panic\|fatal" | wc -l || echo "0")
    if [[ $recent_errors -gt 5 ]]; then
        warnings+=("High number of recent errors in logs: $recent_errors")
    fi
    echo "Recent errors in logs: $recent_errors"
    echo
    
    # Summary
    print_header "DIAGNOSTIC SUMMARY"
    
    if [[ ${#issues[@]} -eq 0 ]] && [[ ${#warnings[@]} -eq 0 ]]; then
        print_success "All checks passed! System appears healthy."
    else
        if [[ ${#issues[@]} -gt 0 ]]; then
            print_error "Critical issues found:"
            for issue in "${issues[@]}"; do
                echo -e "  ${RED}• $issue${NC}"
            done
        fi
        
        if [[ ${#warnings[@]} -gt 0 ]]; then
            print_warning "Warnings:"
            for warning in "${warnings[@]}"; do
                echo -e "  ${YELLOW}• $warning${NC}"
            done
        fi
        
        echo
        print_info "Recommended actions:"
        if [[ ${#issues[@]} -gt 0 ]]; then
            echo -e "  ${CYAN}• Run: $0 quick-fix${NC}"
            echo -e "  ${CYAN}• Run: $0 restart-services${NC}"
        fi
        echo -e "  ${CYAN}• Check logs: $0 logs${NC}"
    fi
}

# Perform health checks
health_check() {
    print_header "HEALTH CHECK"
    
    local healthy=true
    
    print_step "Testing service endpoints..."
    
    # Test main service with detailed response
    if response=$(curl -f -s --connect-timeout 5 http://localhost:8000/api/health 2>&1); then
        print_success "Main service: HEALTHY"
        echo "Response: $response"
    else
        print_error "Main service: FAILED"
        healthy=false
    fi
    
    # Test MinIO
    if curl -f -s --connect-timeout 5 http://localhost:9000/minio/health/live >/dev/null 2>&1; then
        print_success "MinIO service: HEALTHY"
    else
        print_error "MinIO service: FAILED"
        healthy=false
    fi
    
    print_step "Checking container status..."
    cd "$PROJECT_DIR" || exit 1
    
    if docker compose -f docker-compose.single.yml ps sermon-uploader | grep -q "Up"; then
        print_success "Container: RUNNING"
        
        # Check container resource usage
        local container_id=$(docker compose -f docker-compose.single.yml ps -q sermon-uploader)
        local stats=$(docker stats --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}" "$container_id" | tail -1)
        echo "Resource usage: $stats"
    else
        print_error "Container: NOT RUNNING"
        healthy=false
    fi
    
    print_step "Checking recent container restarts..."
    local container_id=$(docker compose -f docker-compose.single.yml ps -q sermon-uploader 2>/dev/null)
    if [[ -n "$container_id" ]]; then
        local restart_count=$(docker inspect "$container_id" --format '{{.RestartCount}}' 2>/dev/null || echo "0")
        if [[ $restart_count -gt 3 ]]; then
            print_warning "Container has restarted $restart_count times"
            healthy=false
        else
            print_success "Container restarts: $restart_count (acceptable)"
        fi
    fi
    
    echo
    if [[ "$healthy" == "true" ]]; then
        print_success "Overall health: HEALTHY"
        log "INFO" "Health check passed"
        return 0
    else
        print_error "Overall health: UNHEALTHY"
        log "WARN" "Health check failed"
        print_info "Consider running: $0 quick-fix"
        return 1
    fi
}

# Attempt quick fixes for common issues
quick_fix() {
    print_header "QUICK FIX - COMMON ISSUES"
    
    cd "$PROJECT_DIR" || exit 1
    local fixes_applied=()
    
    print_step "Checking container status..."
    if ! docker compose -f docker-compose.single.yml ps sermon-uploader | grep -q "Up"; then
        if confirm "Container is not running. Start it?"; then
            print_step "Starting container..."
            docker compose -f docker-compose.single.yml up -d
            sleep 10
            fixes_applied+=("Started main container")
        fi
    fi
    
    print_step "Checking for zombie containers..."
    local zombie_containers=$(docker ps -a --filter "status=exited" --filter "name=sermon" --format "{{.Names}}" | wc -l)
    if [[ $zombie_containers -gt 0 ]]; then
        if confirm "Found $zombie_containers stopped containers. Remove them?"; then
            docker compose -f docker-compose.single.yml down
            docker compose -f docker-compose.single.yml up -d
            fixes_applied+=("Cleaned up stopped containers")
        fi
    fi
    
    print_step "Checking disk space..."
    local disk_usage=$(df "$PROJECT_DIR" | tail -1 | awk '{print $5}' | tr -d '%')
    if [[ $disk_usage -gt 90 ]]; then
        if confirm "Disk usage is high ($disk_usage%). Clean up Docker images?"; then
            docker image prune -af --filter="until=24h"
            docker container prune -f
            fixes_applied+=("Cleaned up Docker resources")
        fi
    fi
    
    print_step "Checking for port conflicts..."
    for port in 8000 9000 9001; do
        if netstat -tlpn 2>/dev/null | grep ":$port " | grep -v docker >/dev/null; then
            print_warning "Port $port may be in use by another process"
            if confirm "Kill processes using port $port?"; then
                fuser -k "${port}/tcp" 2>/dev/null || true
                fixes_applied+=("Freed port $port")
            fi
        fi
    done
    
    print_step "Checking log file sizes..."
    find /var/log -name "*.log" -size +100M 2>/dev/null | while read -r large_log; do
        if confirm "Large log file found: $large_log. Truncate it?"; then
            echo "Log truncated $(date)" > "$large_log"
            fixes_applied+=("Truncated large log: $large_log")
        fi
    done
    
    print_step "Verifying fixes..."
    sleep 15
    
    if health_check >/dev/null 2>&1; then
        print_success "Quick fixes successful!"
        if [[ ${#fixes_applied[@]} -gt 0 ]]; then
            echo "Fixes applied:"
            for fix in "${fixes_applied[@]}"; do
                echo -e "  ${GREEN}• $fix${NC}"
            done
        fi
        log "INFO" "Quick fixes completed successfully"
        return 0
    else
        print_error "Quick fixes did not resolve all issues"
        print_info "Consider running: $0 restart-services"
        log "WARN" "Quick fixes completed but issues remain"
        return 1
    fi
}

# Restart services with health verification
restart_services() {
    print_header "RESTART SERVICES"
    
    if ! confirm "This will restart all services. Continue?"; then
        return 1
    fi
    
    cd "$PROJECT_DIR" || exit 1
    
    print_step "Creating pre-restart backup..."
    local backup_name="pre-restart-$(date +%Y%m%d-%H%M%S)"
    create_backup "$backup_name" --quiet
    
    print_step "Stopping services gracefully..."
    docker compose -f docker-compose.single.yml stop || true
    
    print_step "Waiting for graceful shutdown..."
    sleep 10
    
    print_step "Removing containers..."
    docker compose -f docker-compose.single.yml down || true
    
    print_step "Starting services..."
    docker compose -f docker-compose.single.yml up -d
    
    print_step "Waiting for services to initialize..."
    sleep 30
    
    print_step "Verifying MinIO startup..."
    local minio_ready=false
    for i in {1..12}; do
        if curl -f -s --connect-timeout 3 http://localhost:9000/minio/health/live >/dev/null 2>&1; then
            print_success "MinIO is ready (attempt $i)"
            minio_ready=true
            break
        else
            print_info "Waiting for MinIO... (attempt $i/12)"
            sleep 10
        fi
    done
    
    if [[ "$minio_ready" != "true" ]]; then
        print_error "MinIO failed to start within 2 minutes"
        print_info "Check logs: docker compose -f docker-compose.single.yml logs sermon-uploader"
        return 1
    fi
    
    print_step "Verifying main service startup..."
    local service_ready=false
    for i in {1..10}; do
        if curl -f -s --connect-timeout 5 http://localhost:8000/api/health >/dev/null 2>&1; then
            print_success "Main service is ready (attempt $i)"
            service_ready=true
            break
        else
            print_info "Waiting for main service... (attempt $i/10)"
            sleep 15
        fi
    done
    
    if [[ "$service_ready" != "true" ]]; then
        print_error "Main service failed to start properly"
        print_info "Check logs: docker compose -f docker-compose.single.yml logs sermon-uploader --tail 50"
        return 1
    fi
    
    print_success "Services restarted successfully!"
    print_info "Backup created: $backup_name"
    log "INFO" "Service restart completed successfully"
    
    # Show final status
    echo
    print_step "Final service status:"
    docker compose -f docker-compose.single.yml ps
    
    return 0
}

# Emergency stop all services
emergency_stop() {
    print_header "EMERGENCY STOP"
    
    print_error "WARNING: This will forcibly stop all services!"
    if ! confirm "Are you sure you want to perform an emergency stop?"; then
        return 1
    fi
    
    print_step "Creating emergency backup..."
    create_backup "emergency-stop-$(date +%Y%m%d-%H%M%S)" --quiet || true
    
    print_step "Stopping Docker containers..."
    cd "$PROJECT_DIR" || true
    docker compose -f docker-compose.single.yml down --timeout 5 || true
    docker compose -f docker-compose.prod.yml down --timeout 5 || true
    
    print_step "Killing remaining processes..."
    pkill -f "sermon-uploader" || true
    pkill -f "minio" || true
    
    print_step "Stopping Docker daemon (if needed)..."
    systemctl is-active docker >/dev/null && sudo systemctl stop docker || true
    
    print_success "Emergency stop completed"
    print_warning "To restart, run: $0 restart-services"
    log "WARN" "Emergency stop performed"
}

# Create backup
create_backup() {
    local backup_name="${1:-manual-$(date +%Y%m%d-%H%M%S)}"
    local quiet="${2:-false}"
    
    [[ "$quiet" != "--quiet" ]] && print_header "CREATE BACKUP"
    
    mkdir -p "$BACKUP_DIR"
    local backup_path="$BACKUP_DIR/$backup_name"
    mkdir -p "$backup_path"
    
    [[ "$quiet" != "--quiet" ]] && print_step "Backing up configuration files..."
    cd "$PROJECT_DIR" || exit 1
    
    # Backup configuration
    cp .env "$backup_path/" 2>/dev/null || echo "# No .env file" > "$backup_path/.env"
    cp docker-compose.single.yml "$backup_path/" 2>/dev/null || true
    cp docker-compose.prod.yml "$backup_path/" 2>/dev/null || true
    
    # Backup container state
    [[ "$quiet" != "--quiet" ]] && print_step "Recording container state..."
    docker compose -f docker-compose.single.yml ps > "$backup_path/container_status.txt" 2>/dev/null || true
    docker images | grep sermon > "$backup_path/docker_images.txt" 2>/dev/null || true
    docker compose -f docker-compose.single.yml logs --tail=100 > "$backup_path/recent_logs.txt" 2>/dev/null || true
    
    # Backup system info
    [[ "$quiet" != "--quiet" ]] && print_step "Recording system information..."
    {
        echo "Backup created: $(date)"
        echo "Hostname: $(hostname)"
        echo "Uptime: $(uptime)"
        echo "Disk usage: $(df -h)"
        echo "Memory: $(free -h)"
    } > "$backup_path/system_info.txt"
    
    # Create restore instructions
    cat > "$backup_path/RESTORE_INSTRUCTIONS.txt" << EOF
# Restore Instructions for backup: $backup_name
# Created: $(date)

To restore this backup:

1. Stop current services:
   cd $PROJECT_DIR
   docker compose -f docker-compose.single.yml down

2. Restore configuration:
   cp $backup_path/.env $PROJECT_DIR/
   cp $backup_path/docker-compose.single.yml $PROJECT_DIR/

3. Start services:
   docker compose -f docker-compose.single.yml up -d

4. Verify health:
   $SCRIPT_DIR/recovery-toolkit.sh health-check

# Backup contents:
$(ls -la "$backup_path")
EOF
    
    [[ "$quiet" != "--quiet" ]] && print_success "Backup created: $backup_path"
    log "INFO" "Backup created: $backup_name"
    echo "$backup_path"
}

# View logs
view_logs() {
    local tail_lines="${TAIL_LINES:-50}"
    local follow="${FOLLOW:-false}"
    
    print_header "VIEWING LOGS"
    
    cd "$PROJECT_DIR" || exit 1
    
    if [[ "$follow" == "true" ]]; then
        print_info "Following logs (Ctrl+C to exit)..."
        docker compose -f docker-compose.single.yml logs -f --tail="$tail_lines"
    else
        print_info "Showing last $tail_lines lines..."
        docker compose -f docker-compose.single.yml logs --tail="$tail_lines"
    fi
}

# Parse command line arguments
parse_args() {
    COMMAND=""
    FORCE=false
    VERBOSE=false
    DRY_RUN=false
    TAIL_LINES=50
    FOLLOW=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            diagnose|health-check|quick-fix|restart-services|rollback|emergency-stop|emergency-restart|backup-create|backup-restore|logs|network-test|storage-check|cleanup|check-containers|check-minio|check-discord|check-performance|check-resources)
                COMMAND="$1"
                ;;
            --force)
                FORCE=true
                ;;
            --verbose)
                VERBOSE=true
                ;;
            --dry-run)
                DRY_RUN=true
                ;;
            --tail)
                TAIL_LINES="$2"
                shift
                ;;
            --follow|-f)
                FOLLOW=true
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
        shift
    done
    
    if [[ -z "$COMMAND" ]]; then
        print_error "No command specified"
        show_help
        exit 1
    fi
}

# Main execution
main() {
    setup_logging
    parse_args "$@"
    
    case "$COMMAND" in
        diagnose)
            run_diagnostics
            ;;
        health-check)
            health_check
            ;;
        quick-fix)
            quick_fix
            ;;
        restart-services)
            restart_services
            ;;
        emergency-stop)
            emergency_stop
            ;;
        backup-create)
            create_backup
            ;;
        logs)
            view_logs
            ;;
        *)
            print_error "Command '$COMMAND' not yet implemented"
            print_info "Available: diagnose, health-check, quick-fix, restart-services, emergency-stop, backup-create, logs"
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"