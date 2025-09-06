#!/bin/bash
#
# Health Monitor Script with Automatic Rollback Triggers
# This script continuously monitors system health and triggers rollbacks when thresholds are exceeded
#
# Usage: ./health-monitor.sh [--config-file /path/to/config] [--daemon] [--dry-run]
#

set -euo pipefail

# Default configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="/opt/sermon-uploader"
CONFIG_FILE="${SCRIPT_DIR}/health-monitor.conf"
LOG_FILE="/var/log/sermon-uploader/health-monitor.log"
PID_FILE="/var/run/sermon-uploader-health-monitor.pid"
DAEMON_MODE=false
DRY_RUN=false

# Health check thresholds (can be overridden in config file)
HEALTH_CHECK_INTERVAL=30              # seconds between health checks
ERROR_THRESHOLD=5                     # consecutive failed health checks before action
HIGH_ERROR_RATE_THRESHOLD=0.1         # 10% error rate threshold
HIGH_RESPONSE_TIME_THRESHOLD=5000     # 5 second response time threshold (ms)
MEMORY_USAGE_THRESHOLD=90             # 90% memory usage threshold
CPU_USAGE_THRESHOLD=95                # 95% CPU usage threshold
DISK_USAGE_THRESHOLD=95               # 95% disk usage threshold
CONTAINER_RESTART_THRESHOLD=3         # max restarts in time window
RESTART_TIME_WINDOW=300               # 5 minutes in seconds

# Rollback configuration
AUTO_ROLLBACK_ENABLED=true
ROLLBACK_ON_HEALTH_FAILURE=true
ROLLBACK_ON_PERFORMANCE_DEGRADATION=true
ROLLBACK_ON_RESOURCE_EXHAUSTION=true
EMERGENCY_STOP_ON_CRITICAL=true

# Discord webhook for alerts
DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL:-}"

# GitHub workflow dispatch for automated rollback
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GITHUB_REPO="${GITHUB_REPO:-}"

# Global state variables
consecutive_failures=0
total_checks=0
total_errors=0
last_restart_times=()
startup_time=$(date +%s)

# Logging function
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Load configuration file if it exists
load_config() {
    if [[ -f "$CONFIG_FILE" ]]; then
        log "INFO" "Loading configuration from $CONFIG_FILE"
        source "$CONFIG_FILE"
    else
        log "INFO" "Configuration file not found at $CONFIG_FILE, using defaults"
        create_default_config
    fi
}

# Create default configuration file
create_default_config() {
    cat > "$CONFIG_FILE" << 'EOF'
# Health Monitor Configuration
# Edit these values to customize monitoring behavior

# Check intervals (seconds)
HEALTH_CHECK_INTERVAL=30
RESTART_TIME_WINDOW=300

# Failure thresholds
ERROR_THRESHOLD=5
HIGH_ERROR_RATE_THRESHOLD=0.1
HIGH_RESPONSE_TIME_THRESHOLD=5000
CONTAINER_RESTART_THRESHOLD=3

# Resource usage thresholds (percentage)
MEMORY_USAGE_THRESHOLD=90
CPU_USAGE_THRESHOLD=95
DISK_USAGE_THRESHOLD=95

# Rollback behavior
AUTO_ROLLBACK_ENABLED=true
ROLLBACK_ON_HEALTH_FAILURE=true
ROLLBACK_ON_PERFORMANCE_DEGRADATION=true
ROLLBACK_ON_RESOURCE_EXHAUSTION=true
EMERGENCY_STOP_ON_CRITICAL=true

# Notification settings
DISCORD_WEBHOOK_URL=""
GITHUB_TOKEN=""
GITHUB_REPO=""
EOF
    log "INFO" "Created default configuration file at $CONFIG_FILE"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --config-file)
                CONFIG_FILE="$2"
                shift 2
                ;;
            --daemon)
                DAEMON_MODE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log "ERROR" "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Show help message
show_help() {
    cat << EOF
Health Monitor Script with Automatic Rollback Triggers

Usage: $0 [OPTIONS]

Options:
    --config-file FILE    Use custom configuration file (default: $CONFIG_FILE)
    --daemon             Run as daemon in background
    --dry-run            Don't execute rollbacks, only log what would happen
    --help               Show this help message

Configuration:
    Edit $CONFIG_FILE to customize thresholds and behavior.

Logs:
    Monitor logs at $LOG_FILE

Control:
    Stop daemon: kill \$(cat $PID_FILE)
    Status: systemctl status sermon-uploader-health-monitor
EOF
}

# Setup logging directory
setup_logging() {
    local log_dir=$(dirname "$LOG_FILE")
    mkdir -p "$log_dir"
    
    # Rotate logs if they get too large
    if [[ -f "$LOG_FILE" ]] && [[ $(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null || echo 0) -gt 10485760 ]]; then
        mv "$LOG_FILE" "$LOG_FILE.old"
        log "INFO" "Rotated log file"
    fi
}

# Check if running as daemon
setup_daemon() {
    if [[ "$DAEMON_MODE" == "true" ]]; then
        # Check if already running
        if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
            log "ERROR" "Health monitor daemon is already running (PID: $(cat "$PID_FILE"))"
            exit 1
        fi
        
        # Start as daemon
        log "INFO" "Starting health monitor daemon..."
        echo $$ > "$PID_FILE"
        
        # Cleanup PID file on exit
        trap 'rm -f "$PID_FILE"' EXIT
        
        # Handle signals
        trap 'log "INFO" "Received SIGTERM, shutting down..."; exit 0' TERM
        trap 'log "INFO" "Received SIGINT, shutting down..."; exit 0' INT
    fi
}

# Perform basic health check
check_service_health() {
    local health_url="http://localhost:8000/api/health"
    local start_time=$(date +%s%3N)
    
    # Test basic connectivity
    if ! curl -f -s --connect-timeout 5 --max-time 10 "$health_url" >/dev/null 2>&1; then
        return 1
    fi
    
    local end_time=$(date +%s%3N)
    local response_time=$((end_time - start_time))
    
    # Check response time threshold
    if [[ $response_time -gt $HIGH_RESPONSE_TIME_THRESHOLD ]]; then
        log "WARN" "High response time: ${response_time}ms (threshold: ${HIGH_RESPONSE_TIME_THRESHOLD}ms)"
        return 2
    fi
    
    return 0
}

# Check detailed service metrics
check_detailed_health() {
    local metrics_url="http://localhost:8000/api/health/detailed"
    local metrics_response
    
    # Get detailed health metrics if available
    if metrics_response=$(curl -f -s --connect-timeout 5 --max-time 10 "$metrics_url" 2>/dev/null); then
        # Parse metrics (simplified - would need jq in real implementation)
        local error_rate=$(echo "$metrics_response" | grep -o '"error_rate":[0-9.]*' | cut -d: -f2 || echo "0")
        
        if [[ $(echo "$error_rate > $HIGH_ERROR_RATE_THRESHOLD" | bc -l) -eq 1 ]]; then
            log "WARN" "High error rate: $error_rate (threshold: $HIGH_ERROR_RATE_THRESHOLD)"
            return 3
        fi
    fi
    
    return 0
}

# Check container status and restarts
check_container_health() {
    cd "$PROJECT_DIR" || return 1
    
    # Check if containers are running
    if ! docker compose -f docker-compose.single.yml ps sermon-uploader | grep -q "Up"; then
        log "ERROR" "Main container is not running"
        return 1
    fi
    
    # Check container restart count
    local container_id=$(docker compose -f docker-compose.single.yml ps -q sermon-uploader)
    local restart_count=$(docker inspect "$container_id" --format '{{.RestartCount}}' 2>/dev/null || echo "0")
    
    # Track restart times
    local current_time=$(date +%s)
    if [[ $restart_count -gt 0 ]]; then
        # Remove old restart times outside the window
        local filtered_times=()
        for restart_time in "${last_restart_times[@]}"; do
            if [[ $((current_time - restart_time)) -lt $RESTART_TIME_WINDOW ]]; then
                filtered_times+=("$restart_time")
            fi
        done
        last_restart_times=("${filtered_times[@]}")
        
        # Add current restart if it's new
        if [[ ${#last_restart_times[@]} -eq 0 ]] || [[ ${last_restart_times[-1]} -ne $current_time ]]; then
            last_restart_times+=("$current_time")
        fi
        
        # Check restart threshold
        if [[ ${#last_restart_times[@]} -gt $CONTAINER_RESTART_THRESHOLD ]]; then
            log "ERROR" "Container restart threshold exceeded: ${#last_restart_times[@]} restarts in ${RESTART_TIME_WINDOW}s"
            return 2
        fi
    fi
    
    return 0
}

# Check system resource usage
check_system_resources() {
    local issues=()
    
    # Check memory usage
    local memory_usage
    if command -v free >/dev/null 2>&1; then
        memory_usage=$(free | grep Mem: | awk '{print int($3/$2 * 100)}')
        if [[ $memory_usage -gt $MEMORY_USAGE_THRESHOLD ]]; then
            issues+=("Memory usage: ${memory_usage}% (threshold: ${MEMORY_USAGE_THRESHOLD}%)")
        fi
    fi
    
    # Check CPU usage (5-minute average)
    local cpu_usage
    if command -v uptime >/dev/null 2>&1; then
        cpu_usage=$(uptime | awk -F'load average:' '{print $2}' | awk '{print int($2 * 100)}')
        if [[ $cpu_usage -gt $CPU_USAGE_THRESHOLD ]]; then
            issues+=("CPU usage: ${cpu_usage}% (threshold: ${CPU_USAGE_THRESHOLD}%)")
        fi
    fi
    
    # Check disk usage
    local disk_usage
    disk_usage=$(df "$PROJECT_DIR" | tail -1 | awk '{print int($5)}' | tr -d '%')
    if [[ $disk_usage -gt $DISK_USAGE_THRESHOLD ]]; then
        issues+=("Disk usage: ${disk_usage}% (threshold: ${DISK_USAGE_THRESHOLD}%)")
    fi
    
    if [[ ${#issues[@]} -gt 0 ]]; then
        for issue in "${issues[@]}"; do
            log "WARN" "Resource threshold exceeded: $issue"
        done
        return 1
    fi
    
    return 0
}

# Send Discord notification
send_discord_notification() {
    local title="$1"
    local description="$2"
    local color="$3"  # Green=65280, Yellow=16776960, Red=16711680
    local severity="$4"
    
    if [[ -z "$DISCORD_WEBHOOK_URL" ]]; then
        return 0
    fi
    
    local hostname=$(hostname)
    local timestamp=$(date -u +%Y-%m-%dT%H:%M:%S.000Z)
    
    curl -X POST "$DISCORD_WEBHOOK_URL" \
        -H "Content-Type: application/json" \
        -d "{
            \"embeds\": [{
                \"title\": \"$title\",
                \"description\": \"$description\",
                \"color\": $color,
                \"timestamp\": \"$timestamp\",
                \"fields\": [
                    {\"name\": \"Host\", \"value\": \"$hostname\", \"inline\": true},
                    {\"name\": \"Severity\", \"value\": \"$severity\", \"inline\": true},
                    {\"name\": \"Consecutive Failures\", \"value\": \"$consecutive_failures\", \"inline\": true},
                    {\"name\": \"Total Checks\", \"value\": \"$total_checks\", \"inline\": true},
                    {\"name\": \"Error Rate\", \"value\": \"$(echo "scale=2; $total_errors * 100 / $total_checks" | bc -l 2>/dev/null || echo "0")%\", \"inline\": true},
                    {\"name\": \"Uptime\", \"value\": \"$(($(date +%s) - startup_time))s\", \"inline\": true}
                ],
                \"footer\": {
                    \"text\": \"Health Monitor - Sermon Uploader\"
                }
            }]
        }" >/dev/null 2>&1 || log "WARN" "Failed to send Discord notification"
}

# Trigger GitHub Actions rollback
trigger_rollback() {
    local rollback_type="$1"
    local reason="$2"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "INFO" "[DRY RUN] Would trigger rollback: type=$rollback_type, reason=$reason"
        return 0
    fi
    
    if [[ -z "$GITHUB_TOKEN" ]] || [[ -z "$GITHUB_REPO" ]]; then
        log "WARN" "GitHub credentials not configured, cannot trigger automatic rollback"
        return 1
    fi
    
    log "INFO" "Triggering automatic rollback via GitHub Actions..."
    
    curl -X POST \
        -H "Authorization: token $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$GITHUB_REPO/actions/workflows/rollback.yml/dispatches" \
        -d "{
            \"ref\": \"main\",
            \"inputs\": {
                \"rollback_type\": \"$rollback_type\",
                \"reason\": \"Automatic rollback triggered by health monitor: $reason\",
                \"force_rollback\": \"true\"
            }
        }" >/dev/null 2>&1
    
    if [[ $? -eq 0 ]]; then
        log "INFO" "Rollback workflow triggered successfully"
        send_discord_notification \
            "🔄 Automatic Rollback Triggered" \
            "Health monitor has triggered an automatic rollback due to: $reason" \
            "16776960" \
            "WARNING"
        return 0
    else
        log "ERROR" "Failed to trigger rollback workflow"
        return 1
    fi
}

# Perform emergency stop
emergency_stop() {
    local reason="$1"
    
    log "ERROR" "EMERGENCY STOP triggered: $reason"
    
    send_discord_notification \
        "🚨 EMERGENCY STOP" \
        "Critical system failure detected. All services stopped. Reason: $reason" \
        "16711680" \
        "CRITICAL"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log "INFO" "[DRY RUN] Would perform emergency stop"
        return 0
    fi
    
    trigger_rollback "emergency_stop" "$reason"
}

# Main monitoring loop
run_health_checks() {
    log "INFO" "Starting health monitoring (interval: ${HEALTH_CHECK_INTERVAL}s, error threshold: $ERROR_THRESHOLD)"
    
    while true; do
        total_checks=$((total_checks + 1))
        local health_issues=()
        local critical_issues=()
        
        # Basic service health check
        check_service_health
        local health_status=$?
        
        case $health_status in
            0)
                # Service is healthy
                consecutive_failures=0
                ;;
            1)
                health_issues+=("Service health check failed")
                ;;
            2)
                health_issues+=("High response time detected")
                ;;
        esac
        
        # Detailed health metrics
        if [[ $health_status -eq 0 ]]; then
            check_detailed_health
            if [[ $? -ne 0 ]]; then
                health_issues+=("Performance degradation detected")
            fi
        fi
        
        # Container health
        check_container_health
        local container_status=$?
        if [[ $container_status -eq 1 ]]; then
            critical_issues+=("Container not running")
        elif [[ $container_status -eq 2 ]]; then
            health_issues+=("Excessive container restarts")
        fi
        
        # System resources
        check_system_resources
        if [[ $? -ne 0 ]]; then
            health_issues+=("Resource threshold exceeded")
        fi
        
        # Process health check results
        if [[ ${#critical_issues[@]} -gt 0 ]] && [[ "$EMERGENCY_STOP_ON_CRITICAL" == "true" ]]; then
            emergency_stop "Critical issues: ${critical_issues[*]}"
            break
        elif [[ ${#health_issues[@]} -gt 0 ]]; then
            consecutive_failures=$((consecutive_failures + 1))
            total_errors=$((total_errors + 1))
            
            log "WARN" "Health check failed ($consecutive_failures/$ERROR_THRESHOLD): ${health_issues[*]}"
            
            # Check if we should trigger rollback
            if [[ $consecutive_failures -ge $ERROR_THRESHOLD ]] && [[ "$AUTO_ROLLBACK_ENABLED" == "true" ]]; then
                local rollback_reason="Health check failures: ${health_issues[*]}"
                trigger_rollback "previous_version" "$rollback_reason"
                
                # Reset counter after triggering rollback
                consecutive_failures=0
            fi
        else
            # All checks passed
            if [[ $consecutive_failures -gt 0 ]]; then
                log "INFO" "Health checks recovered after $consecutive_failures failures"
                send_discord_notification \
                    "✅ System Recovery" \
                    "Health checks have recovered. System is now stable." \
                    "65280" \
                    "INFO"
            fi
            consecutive_failures=0
        fi
        
        # Wait for next check
        sleep "$HEALTH_CHECK_INTERVAL"
    done
}

# Main execution
main() {
    parse_args "$@"
    setup_logging
    load_config
    setup_daemon
    
    log "INFO" "Health monitor started (PID: $$)"
    log "INFO" "Configuration: auto_rollback=$AUTO_ROLLBACK_ENABLED, error_threshold=$ERROR_THRESHOLD, check_interval=${HEALTH_CHECK_INTERVAL}s"
    
    run_health_checks
}

# Run main function with all arguments
main "$@"