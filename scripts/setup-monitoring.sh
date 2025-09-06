#!/bin/bash
#
# Monitoring Setup Script
# This script sets up the complete monitoring and rollback system
#
# Usage: ./setup-monitoring.sh [--install-deps] [--configure-systemd] [--start-monitoring]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="/opt/sermon-uploader"
LOG_FILE="/var/log/sermon-uploader/setup.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Options
INSTALL_DEPS=false
CONFIGURE_SYSTEMD=false
START_MONITORING=false

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
print_step() { echo -e "${BLUE}👉 $1${NC}"; }

# Setup logging
setup_logging() {
    mkdir -p "$(dirname "$LOG_FILE")"
    log "INFO" "Monitoring setup started"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --install-deps)
                INSTALL_DEPS=true
                shift
                ;;
            --configure-systemd)
                CONFIGURE_SYSTEMD=true
                shift
                ;;
            --start-monitoring)
                START_MONITORING=true
                shift
                ;;
            --all)
                INSTALL_DEPS=true
                CONFIGURE_SYSTEMD=true
                START_MONITORING=true
                shift
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
    done
}

# Show help
show_help() {
    cat << EOF
${PURPLE}Monitoring Setup Script${NC}

${YELLOW}USAGE:${NC}
  $0 [OPTIONS]

${YELLOW}OPTIONS:${NC}
  ${GREEN}--install-deps${NC}        Install system dependencies (jq, bc, netcat)
  ${GREEN}--configure-systemd${NC}   Set up systemd services for health monitoring
  ${GREEN}--start-monitoring${NC}    Start monitoring services
  ${GREEN}--all${NC}                 Do everything (install, configure, start)
  ${GREEN}--help${NC}                Show this help

${YELLOW}EXAMPLES:${NC}
  $0 --all                        # Complete setup
  $0 --install-deps               # Just install dependencies  
  $0 --configure-systemd          # Just configure services
  $0 --start-monitoring           # Just start services

${YELLOW}SERVICES CONFIGURED:${NC}
  • Health Monitor (automatic rollback triggers)
  • Recovery Toolkit (manual recovery procedures)
  • Discord Webhook Handler (alert notifications)
  • Prometheus Monitoring Stack (optional)
EOF
}

# Install system dependencies
install_dependencies() {
    print_header "INSTALLING DEPENDENCIES"
    
    print_step "Updating package list..."
    sudo apt update
    
    print_step "Installing required packages..."
    sudo apt install -y \
        jq \
        bc \
        netcat \
        curl \
        wget \
        git \
        docker-compose
    
    # Install Docker if not present
    if ! command -v docker >/dev/null 2>&1; then
        print_step "Installing Docker..."
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        sudo usermod -aG docker $USER
        rm get-docker.sh
        print_warning "You may need to log out and back in for Docker permissions"
    fi
    
    print_success "Dependencies installed successfully"
}

# Configure systemd services
configure_systemd() {
    print_header "CONFIGURING SYSTEMD SERVICES"
    
    print_step "Installing health monitor script..."
    sudo cp "$SCRIPT_DIR/health-monitor.sh" /usr/local/bin/
    sudo chmod +x /usr/local/bin/health-monitor.sh
    
    print_step "Installing recovery toolkit script..."
    sudo cp "$SCRIPT_DIR/recovery-toolkit.sh" /usr/local/bin/
    sudo chmod +x /usr/local/bin/recovery-toolkit.sh
    
    print_step "Creating configuration directory..."
    sudo mkdir -p /etc/sermon-uploader
    
    if [[ ! -f /etc/sermon-uploader/health-monitor.conf ]]; then
        sudo cp "$SCRIPT_DIR/health-monitor.conf" /etc/sermon-uploader/
        print_info "Created default health monitor configuration"
        print_warning "Edit /etc/sermon-uploader/health-monitor.conf with your settings"
    else
        print_info "Health monitor configuration already exists"
    fi
    
    print_step "Creating health monitor systemd service..."
    sudo tee /etc/systemd/system/sermon-uploader-health-monitor.service << EOF >/dev/null
[Unit]
Description=Sermon Uploader Health Monitor with Auto-Rollback
After=network.target docker.service
Requires=docker.service
StartLimitIntervalSec=0

[Service]
Type=forking
ExecStart=/usr/local/bin/health-monitor.sh --daemon --config-file /etc/sermon-uploader/health-monitor.conf
ExecStop=/bin/kill -TERM \$MAINPID
PIDFile=/var/run/sermon-uploader-health-monitor.pid
Restart=always
RestartSec=30
User=pi
Group=pi

# Load environment from config
EnvironmentFile=-/etc/sermon-uploader/health-monitor.conf

# Default fallbacks if not in config
Environment="HEALTH_CHECK_INTERVAL=30"
Environment="ERROR_THRESHOLD=5"
Environment="AUTO_ROLLBACK_ENABLED=true"

[Install]
WantedBy=multi-user.target
EOF
    
    print_step "Creating Discord webhook handler systemd service..."
    sudo tee /etc/systemd/system/discord-webhook-handler.service << EOF >/dev/null
[Unit]
Description=Discord Webhook Handler for Alertmanager
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=/usr/local/bin/discord-webhook-handler.sh --port 5001 --daemon
Restart=always
RestartSec=10
User=pi
Group=pi

# Environment variables (override in /etc/sermon-uploader/webhook.conf)
EnvironmentFile=-/etc/sermon-uploader/webhook.conf
Environment="WEBHOOK_PORT=5001"

[Install]
WantedBy=multi-user.target
EOF
    
    print_step "Reloading systemd daemon..."
    sudo systemctl daemon-reload
    
    print_step "Enabling services..."
    sudo systemctl enable sermon-uploader-health-monitor.service
    sudo systemctl enable discord-webhook-handler.service
    
    print_success "Systemd services configured successfully"
}

# Start monitoring services
start_monitoring() {
    print_header "STARTING MONITORING SERVICES"
    
    print_step "Starting health monitor service..."
    if sudo systemctl start sermon-uploader-health-monitor.service; then
        print_success "Health monitor started"
    else
        print_error "Failed to start health monitor"
        print_info "Check logs: sudo journalctl -u sermon-uploader-health-monitor -f"
    fi
    
    print_step "Starting Discord webhook handler..."
    if sudo systemctl start discord-webhook-handler.service; then
        print_success "Discord webhook handler started"
    else
        print_error "Failed to start Discord webhook handler"
        print_info "Check logs: sudo journalctl -u discord-webhook-handler -f"
    fi
    
    print_step "Checking service status..."
    echo
    sudo systemctl status sermon-uploader-health-monitor.service --no-pager -l
    echo
    sudo systemctl status discord-webhook-handler.service --no-pager -l
    echo
    
    print_success "Monitoring services started"
}

# Test monitoring setup
test_monitoring() {
    print_header "TESTING MONITORING SETUP"
    
    print_step "Testing health monitor..."
    if pgrep -f "health-monitor.sh" >/dev/null; then
        print_success "Health monitor is running"
    else
        print_error "Health monitor is not running"
    fi
    
    print_step "Testing webhook handler..."
    if curl -s http://localhost:5001/health | grep -q "healthy"; then
        print_success "Discord webhook handler is responding"
    else
        print_warning "Discord webhook handler may not be responding"
    fi
    
    print_step "Testing recovery toolkit..."
    if /usr/local/bin/recovery-toolkit.sh health-check >/dev/null 2>&1; then
        print_success "Recovery toolkit is functional"
    else
        print_warning "Recovery toolkit may have issues"
    fi
    
    print_step "Checking log files..."
    local log_files=(
        "/var/log/sermon-uploader/health-monitor.log"
        "/var/log/sermon-uploader/recovery.log"
        "/var/log/sermon-uploader/webhook-handler.log"
    )
    
    for log_file in "${log_files[@]}"; do
        if [[ -f "$log_file" ]]; then
            print_success "Log file exists: $log_file"
        else
            print_info "Log file will be created: $log_file"
        fi
    done
    
    print_success "Monitoring setup test completed"
}

# Setup monitoring stack with Docker Compose
setup_monitoring_stack() {
    print_header "SETTING UP MONITORING STACK"
    
    local monitoring_dir="$PROJECT_DIR/monitoring"
    
    if [[ ! -d "$monitoring_dir" ]]; then
        print_warning "Monitoring directory not found at $monitoring_dir"
        print_info "Creating monitoring directory..."
        mkdir -p "$monitoring_dir"
        
        # Copy monitoring files if they exist in the script directory
        if [[ -f "$SCRIPT_DIR/../monitoring/docker-compose.monitoring.yml" ]]; then
            cp -r "$SCRIPT_DIR/../monitoring/"* "$monitoring_dir/"
            print_success "Copied monitoring configuration files"
        fi
    fi
    
    cd "$monitoring_dir"
    
    if [[ -f "docker-compose.monitoring.yml" ]]; then
        print_step "Starting monitoring stack..."
        
        # Create .env file if it doesn't exist
        if [[ ! -f ".env" ]]; then
            cat > .env << EOF
DISCORD_WEBHOOK_URL=${DISCORD_WEBHOOK_URL:-}
GRAFANA_ADMIN_PASSWORD=admin123
EOF
            print_info "Created monitoring .env file"
            print_warning "Edit $monitoring_dir/.env with your Discord webhook URL"
        fi
        
        # Start monitoring stack
        docker compose -f docker-compose.monitoring.yml up -d
        
        print_success "Monitoring stack started"
        print_info "Access Grafana at http://$(hostname -I | awk '{print $1}'):3000"
        print_info "Access Prometheus at http://$(hostname -I | awk '{print $1}'):9090"
        print_info "Access Alertmanager at http://$(hostname -I | awk '{print $1}'):9093"
    else
        print_warning "Monitoring stack configuration not found"
        print_info "Skipping monitoring stack setup"
    fi
}

# Show final configuration summary
show_summary() {
    print_header "SETUP SUMMARY"
    
    local ip_address=$(hostname -I | awk '{print $1}')
    
    echo -e "${GREEN}✅ Monitoring system setup completed!${NC}"
    echo
    echo -e "${YELLOW}Services Status:${NC}"
    echo "  • Health Monitor: $(systemctl is-active sermon-uploader-health-monitor.service || echo "inactive")"
    echo "  • Webhook Handler: $(systemctl is-active discord-webhook-handler.service || echo "inactive")"
    echo
    echo -e "${YELLOW}Configuration Files:${NC}"
    echo "  • Health Monitor: /etc/sermon-uploader/health-monitor.conf"
    echo "  • Recovery Toolkit: /usr/local/bin/recovery-toolkit.sh"
    echo "  • Health Monitor Script: /usr/local/bin/health-monitor.sh"
    echo
    echo -e "${YELLOW}Log Files:${NC}"
    echo "  • Health Monitor: /var/log/sermon-uploader/health-monitor.log"
    echo "  • Recovery: /var/log/sermon-uploader/recovery.log"
    echo "  • Setup: /var/log/sermon-uploader/setup.log"
    echo
    echo -e "${YELLOW}Useful Commands:${NC}"
    echo "  • Check health: recovery-toolkit.sh health-check"
    echo "  • View logs: sudo journalctl -u sermon-uploader-health-monitor -f"
    echo "  • Service status: sudo systemctl status sermon-uploader-health-monitor"
    echo "  • Manual rollback: Go to GitHub Actions → Automated Rollback"
    echo
    echo -e "${YELLOW}Web Interfaces:${NC}"
    echo "  • Grafana: http://$ip_address:3000 (admin/admin123)"
    echo "  • Prometheus: http://$ip_address:9090"
    echo "  • Alertmanager: http://$ip_address:9093"
    echo
    echo -e "${YELLOW}Next Steps:${NC}"
    echo "  1. Edit /etc/sermon-uploader/health-monitor.conf with your Discord webhook and GitHub credentials"
    echo "  2. Test the system: recovery-toolkit.sh health-check"
    echo "  3. Configure Discord channel for alerts"
    echo "  4. Test rollback procedure in non-production environment"
    echo
    echo -e "${GREEN}Documentation: docs/ROLLBACK_RECOVERY_GUIDE.md${NC}"
}

# Main execution
main() {
    setup_logging
    parse_args "$@"
    
    if [[ "$INSTALL_DEPS" != "true" && "$CONFIGURE_SYSTEMD" != "true" && "$START_MONITORING" != "true" ]]; then
        print_error "No actions specified. Use --help for usage information."
        exit 1
    fi
    
    if [[ "$INSTALL_DEPS" == "true" ]]; then
        install_dependencies
    fi
    
    if [[ "$CONFIGURE_SYSTEMD" == "true" ]]; then
        configure_systemd
    fi
    
    if [[ "$START_MONITORING" == "true" ]]; then
        start_monitoring
        
        # Also set up monitoring stack if available
        setup_monitoring_stack || true
        
        # Test the setup
        test_monitoring
    fi
    
    show_summary
    
    log "INFO" "Monitoring setup completed successfully"
}

# Run main function with all arguments
main "$@"