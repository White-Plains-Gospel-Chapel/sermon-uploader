#!/bin/bash

# Deployment Monitoring Script
# This script monitors the health of your deployed services and sends alerts

set -e

# Configuration
BACKEND_URL="http://localhost:8080"
FRONTEND_URL="http://localhost:3000"
HEALTH_CHECK_ENDPOINT="/health"
LOG_FILE="/var/log/sermon-uploader-monitor.log"
DISCORD_WEBHOOK="${DISCORD_WEBHOOK:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Status tracking
OVERALL_STATUS="healthy"
ISSUES=()

# Logging function
log_message() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Send Discord notification
send_discord_alert() {
    local title=$1
    local description=$2
    local color=$3
    
    if [ -n "$DISCORD_WEBHOOK" ]; then
        curl -X POST "$DISCORD_WEBHOOK" \
            -H "Content-Type: application/json" \
            -d "{
                \"embeds\": [{
                    \"title\": \"$title\",
                    \"description\": \"$description\",
                    \"color\": $color,
                    \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%S.000Z)\"
                }]
            }" 2>/dev/null || true
    fi
}

# Check service health
check_service() {
    local service_name=$1
    local url=$2
    local expected_status=${3:-200}
    
    echo -n "Checking $service_name... "
    
    if response=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null); then
        if [ "$response" = "$expected_status" ]; then
            echo -e "${GREEN}✓ Healthy (HTTP $response)${NC}"
            log_message "INFO" "$service_name is healthy (HTTP $response)"
            return 0
        else
            echo -e "${RED}✗ Unhealthy (HTTP $response)${NC}"
            log_message "ERROR" "$service_name returned HTTP $response"
            ISSUES+=("$service_name returned HTTP $response")
            OVERALL_STATUS="unhealthy"
            return 1
        fi
    else
        echo -e "${RED}✗ Unreachable${NC}"
        log_message "ERROR" "$service_name is unreachable"
        ISSUES+=("$service_name is unreachable")
        OVERALL_STATUS="unhealthy"
        return 1
    fi
}

# Check Docker containers
check_docker_containers() {
    echo -n "Checking Docker containers... "
    
    local expected_containers=("sermon-uploader-backend" "sermon-uploader-frontend")
    local missing_containers=()
    local unhealthy_containers=()
    
    for container in "${expected_containers[@]}"; do
        if docker ps --format "{{.Names}}" | grep -q "^$container$"; then
            # Container is running, check if it's healthy
            status=$(docker inspect --format='{{.State.Status}}' "$container" 2>/dev/null || echo "unknown")
            if [ "$status" != "running" ]; then
                unhealthy_containers+=("$container ($status)")
            fi
        else
            missing_containers+=("$container")
        fi
    done
    
    if [ ${#missing_containers[@]} -eq 0 ] && [ ${#unhealthy_containers[@]} -eq 0 ]; then
        echo -e "${GREEN}✓ All containers running${NC}"
        log_message "INFO" "All Docker containers are running"
    else
        echo -e "${RED}✗ Container issues detected${NC}"
        if [ ${#missing_containers[@]} -gt 0 ]; then
            log_message "ERROR" "Missing containers: ${missing_containers[*]}"
            ISSUES+=("Missing containers: ${missing_containers[*]}")
        fi
        if [ ${#unhealthy_containers[@]} -gt 0 ]; then
            log_message "ERROR" "Unhealthy containers: ${unhealthy_containers[*]}"
            ISSUES+=("Unhealthy containers: ${unhealthy_containers[*]}")
        fi
        OVERALL_STATUS="unhealthy"
    fi
}

# Check disk space
check_disk_space() {
    echo -n "Checking disk space... "
    
    local threshold=80
    local usage=$(df / | awk 'NR==2 {print $5}' | sed 's/%//')
    
    if [ "$usage" -lt "$threshold" ]; then
        echo -e "${GREEN}✓ ${usage}% used${NC}"
        log_message "INFO" "Disk usage: ${usage}%"
    else
        echo -e "${RED}✗ ${usage}% used (threshold: ${threshold}%)${NC}"
        log_message "WARNING" "High disk usage: ${usage}%"
        ISSUES+=("High disk usage: ${usage}%")
        if [ "$usage" -ge 90 ]; then
            OVERALL_STATUS="unhealthy"
        fi
    fi
}

# Check memory usage
check_memory() {
    echo -n "Checking memory usage... "
    
    local mem_info=$(free -m | awk 'NR==2')
    local total=$(echo "$mem_info" | awk '{print $2}')
    local used=$(echo "$mem_info" | awk '{print $3}')
    local percent=$((used * 100 / total))
    
    if [ "$percent" -lt 80 ]; then
        echo -e "${GREEN}✓ ${percent}% used (${used}MB/${total}MB)${NC}"
        log_message "INFO" "Memory usage: ${percent}% (${used}MB/${total}MB)"
    else
        echo -e "${YELLOW}⚠ ${percent}% used (${used}MB/${total}MB)${NC}"
        log_message "WARNING" "High memory usage: ${percent}% (${used}MB/${total}MB)"
        if [ "$percent" -ge 90 ]; then
            ISSUES+=("High memory usage: ${percent}%")
            OVERALL_STATUS="unhealthy"
        fi
    fi
}

# Check CPU load
check_cpu_load() {
    echo -n "Checking CPU load... "
    
    local load_avg=$(uptime | awk -F'load average:' '{print $2}')
    local load_1min=$(echo "$load_avg" | cut -d, -f1 | xargs)
    local cpu_count=$(nproc)
    
    # Compare load average to CPU count
    if (( $(echo "$load_1min < $cpu_count" | bc -l) )); then
        echo -e "${GREEN}✓ Load average: $load_1min (${cpu_count} CPUs)${NC}"
        log_message "INFO" "Load average: $load_1min"
    else
        echo -e "${YELLOW}⚠ Load average: $load_1min (${cpu_count} CPUs)${NC}"
        log_message "WARNING" "High load average: $load_1min"
        if (( $(echo "$load_1min > $cpu_count * 2" | bc -l) )); then
            ISSUES+=("High CPU load: $load_1min")
            OVERALL_STATUS="unhealthy"
        fi
    fi
}

# Check GitHub runner status
check_github_runner() {
    echo -n "Checking GitHub Actions runner... "
    
    local service_name="actions.runner.White-Plains-Gospel-Chapel-sermon-uploader.pi-runner"
    
    if systemctl is-active --quiet "$service_name"; then
        echo -e "${GREEN}✓ Running${NC}"
        log_message "INFO" "GitHub Actions runner is active"
    else
        echo -e "${RED}✗ Not running${NC}"
        log_message "ERROR" "GitHub Actions runner is not active"
        ISSUES+=("GitHub Actions runner is not active")
        OVERALL_STATUS="unhealthy"
    fi
}

# Check recent logs for errors
check_logs() {
    echo -n "Checking recent logs for errors... "
    
    local error_count=0
    local log_files=(
        "/var/log/docker/sermon-uploader-backend.log"
        "/var/log/docker/sermon-uploader-frontend.log"
    )
    
    for log_file in "${log_files[@]}"; do
        if [ -f "$log_file" ]; then
            # Check for errors in last 100 lines
            errors=$(tail -n 100 "$log_file" | grep -iE "error|exception|fatal|panic" | wc -l)
            error_count=$((error_count + errors))
        fi
    done
    
    # Also check Docker logs
    for container in sermon-uploader-backend sermon-uploader-frontend; do
        if docker ps --format "{{.Names}}" | grep -q "^$container$"; then
            errors=$(docker logs --tail 100 "$container" 2>&1 | grep -iE "error|exception|fatal|panic" | wc -l)
            error_count=$((error_count + errors))
        fi
    done
    
    if [ "$error_count" -eq 0 ]; then
        echo -e "${GREEN}✓ No recent errors${NC}"
        log_message "INFO" "No errors found in recent logs"
    else
        echo -e "${YELLOW}⚠ Found $error_count error(s) in recent logs${NC}"
        log_message "WARNING" "Found $error_count error(s) in recent logs"
    fi
}

# Main monitoring function
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Sermon Uploader Deployment Monitor${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo "Time: $(date '+%Y-%m-%d %H:%M:%S')"
    echo ""
    
    # Run all checks
    check_service "Backend API" "${BACKEND_URL}${HEALTH_CHECK_ENDPOINT}"
    check_service "Frontend" "$FRONTEND_URL"
    check_docker_containers
    check_disk_space
    check_memory
    check_cpu_load
    check_github_runner
    check_logs
    
    echo ""
    echo -e "${BLUE}========================================${NC}"
    
    # Summary
    if [ "$OVERALL_STATUS" = "healthy" ]; then
        echo -e "${GREEN}✓ Overall Status: HEALTHY${NC}"
        log_message "INFO" "Overall status: HEALTHY"
    else
        echo -e "${RED}✗ Overall Status: UNHEALTHY${NC}"
        log_message "ERROR" "Overall status: UNHEALTHY"
        
        # List issues
        if [ ${#ISSUES[@]} -gt 0 ]; then
            echo ""
            echo "Issues detected:"
            for issue in "${ISSUES[@]}"; do
                echo "  - $issue"
            done
            
            # Send Discord alert
            issue_list=$(printf "\n- %s" "${ISSUES[@]}")
            send_discord_alert "⚠️ Deployment Health Alert" "Issues detected:$issue_list" 15158332
        fi
    fi
    
    echo -e "${BLUE}========================================${NC}"
}

# Handle script arguments
case "${1:-}" in
    --daemon)
        # Run in daemon mode (continuous monitoring)
        echo "Starting monitoring daemon (checking every 5 minutes)..."
        while true; do
            main
            sleep 300  # Check every 5 minutes
        done
        ;;
    --cron)
        # Run once for cron job (suppress colored output)
        RED=""
        GREEN=""
        YELLOW=""
        BLUE=""
        NC=""
        main
        ;;
    *)
        # Run once interactively
        main
        ;;
esac