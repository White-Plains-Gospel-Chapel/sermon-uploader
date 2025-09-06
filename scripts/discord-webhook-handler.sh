#!/bin/bash
#
# Discord Webhook Handler for Alertmanager
# Converts Prometheus/Alertmanager webhooks to Discord-formatted messages
# This script acts as a webhook receiver and formats alerts for Discord
#
# Usage: ./discord-webhook-handler.sh [--port 5001] [--webhook-url URL]
#

set -euo pipefail

# Configuration
PORT="${WEBHOOK_PORT:-5001}"
DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL:-}"
LOG_FILE="/var/log/sermon-uploader/webhook-handler.log"
PID_FILE="/var/run/discord-webhook-handler.pid"

# Colors for Discord embeds
COLOR_CRITICAL="16711680"  # Red
COLOR_WARNING="16776960"   # Orange/Yellow
COLOR_INFO="65280"         # Green
COLOR_RESOLVED="65280"     # Green

# Create log directory
mkdir -p "$(dirname "$LOG_FILE")"

# Logging function
log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --port)
                PORT="$2"
                shift 2
                ;;
            --webhook-url)
                DISCORD_WEBHOOK_URL="$2"
                shift 2
                ;;
            --daemon)
                DAEMON=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log "ERROR" "Unknown option: $1"
                exit 1
                ;;
        esac
    done
}

# Show help
show_help() {
    cat << EOF
Discord Webhook Handler for Alertmanager

Usage: $0 [OPTIONS]

Options:
    --port PORT           Listen on port (default: 5001)
    --webhook-url URL     Discord webhook URL
    --daemon              Run as daemon
    --help                Show this help

Environment Variables:
    DISCORD_WEBHOOK_URL   Discord webhook URL
    WEBHOOK_PORT         Port to listen on

Examples:
    $0 --port 5001
    DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..." $0 --daemon
EOF
}

# Convert alert to Discord embed
alert_to_discord_embed() {
    local alert_json="$1"
    local severity="$2"
    
    # Extract alert information using jq
    local status=$(echo "$alert_json" | jq -r '.status // "unknown"')
    local alertname=$(echo "$alert_json" | jq -r '.labels.alertname // "Unknown Alert"')
    local service=$(echo "$alert_json" | jq -r '.labels.service // "unknown"')
    local summary=$(echo "$alert_json" | jq -r '.annotations.summary // "No summary"')
    local description=$(echo "$alert_json" | jq -r '.annotations.description // "No description"')
    local runbook_url=$(echo "$alert_json" | jq -r '.annotations.runbook_url // ""')
    local starts_at=$(echo "$alert_json" | jq -r '.startsAt // ""')
    local ends_at=$(echo "$alert_json" | jq -r '.endsAt // ""')
    
    # Determine color based on severity and status
    local color="$COLOR_INFO"
    local title_prefix="ℹ️"
    
    case "$severity" in
        critical)
            color="$COLOR_CRITICAL"
            title_prefix="🚨"
            ;;
        warning)
            color="$COLOR_WARNING"
            title_prefix="⚠️"
            ;;
        info)
            color="$COLOR_INFO"
            title_prefix="ℹ️"
            ;;
    esac
    
    if [[ "$status" == "resolved" ]]; then
        color="$COLOR_RESOLVED"
        title_prefix="✅"
    fi
    
    # Format timestamp
    local formatted_time=""
    if [[ -n "$starts_at" && "$starts_at" != "null" ]]; then
        formatted_time=$(date -d "$starts_at" '+%Y-%m-%d %H:%M:%S %Z' 2>/dev/null || echo "$starts_at")
    fi
    
    # Build Discord embed
    local embed_fields="["
    
    # Add service field
    embed_fields+="{\"name\": \"Service\", \"value\": \"$service\", \"inline\": true},"
    
    # Add severity field
    embed_fields+="{\"name\": \"Severity\", \"value\": \"$severity\", \"inline\": true},"
    
    # Add status field
    embed_fields+="{\"name\": \"Status\", \"value\": \"$status\", \"inline\": true},"
    
    # Add time field if available
    if [[ -n "$formatted_time" ]]; then
        embed_fields+="{\"name\": \"Time\", \"value\": \"$formatted_time\", \"inline\": false},"
    fi
    
    # Add runbook field if available
    if [[ -n "$runbook_url" && "$runbook_url" != "null" ]]; then
        embed_fields+="{\"name\": \"Runbook\", \"value\": \"[View Runbook]($runbook_url)\", \"inline\": false},"
    fi
    
    # Remove trailing comma and close array
    embed_fields="${embed_fields%,}]"
    
    # Create Discord webhook payload
    local webhook_payload=$(cat << EOF
{
    "embeds": [{
        "title": "$title_prefix $alertname",
        "description": "$description",
        "color": $color,
        "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)",
        "fields": $embed_fields,
        "footer": {
            "text": "Sermon Uploader Monitoring"
        }
    }]
}
EOF
)
    
    echo "$webhook_payload"
}

# Process webhook payload and send to Discord
process_webhook() {
    local payload="$1"
    local endpoint="$2"
    
    log "INFO" "Processing webhook for endpoint: $endpoint"
    
    # Parse webhook payload
    local receiver=$(echo "$payload" | jq -r '.receiver // "unknown"')
    local status=$(echo "$payload" | jq -r '.status // "unknown"')
    local alerts=$(echo "$payload" | jq -r '.alerts[]')
    
    # Determine severity from endpoint
    local severity="info"
    case "$endpoint" in
        */critical) severity="critical" ;;
        */warning) severity="warning" ;;
        */info) severity="info" ;;
    esac
    
    log "INFO" "Received $status alert(s) for receiver: $receiver, severity: $severity"
    
    # Process each alert
    echo "$payload" | jq -c '.alerts[]' | while read -r alert; do
        log "DEBUG" "Processing alert: $(echo "$alert" | jq -r '.labels.alertname // "unknown"')"
        
        # Convert to Discord format
        local discord_payload=$(alert_to_discord_embed "$alert" "$severity")
        
        # Send to Discord
        if [[ -n "$DISCORD_WEBHOOK_URL" ]]; then
            local response=$(curl -s -w "%{http_code}" -X POST "$DISCORD_WEBHOOK_URL" \
                -H "Content-Type: application/json" \
                -d "$discord_payload" \
                --max-time 10 || echo "000")
            
            local http_code="${response: -3}"
            if [[ "$http_code" =~ ^20[0-9]$ ]]; then
                log "INFO" "Successfully sent alert to Discord (HTTP $http_code)"
            else
                log "ERROR" "Failed to send alert to Discord (HTTP $http_code)"
                log "DEBUG" "Discord payload: $discord_payload"
            fi
        else
            log "WARN" "Discord webhook URL not configured, skipping notification"
            log "DEBUG" "Would send: $discord_payload"
        fi
    done
}

# HTTP server handler
handle_request() {
    local method="$1"
    local path="$2"
    local content_length="$3"
    
    case "$method" in
        POST)
            case "$path" in
                /api/webhooks/discord/critical|/api/webhooks/discord/warning|/api/webhooks/discord/info)
                    # Read request body
                    local body=""
                    if [[ "$content_length" -gt 0 ]]; then
                        body=$(head -c "$content_length")
                    fi
                    
                    # Process the webhook
                    if [[ -n "$body" ]]; then
                        process_webhook "$body" "$path"
                        echo "HTTP/1.1 200 OK"
                        echo "Content-Type: application/json"
                        echo "Content-Length: 25"
                        echo ""
                        echo '{"status": "success"}'
                    else
                        echo "HTTP/1.1 400 Bad Request"
                        echo "Content-Type: application/json"
                        echo "Content-Length: 27"
                        echo ""
                        echo '{"error": "Empty body"}'
                    fi
                    ;;
                /health)
                    echo "HTTP/1.1 200 OK"
                    echo "Content-Type: application/json"
                    echo "Content-Length: 22"
                    echo ""
                    echo '{"status": "healthy"}'
                    ;;
                *)
                    echo "HTTP/1.1 404 Not Found"
                    echo "Content-Type: application/json"
                    echo "Content-Length: 26"
                    echo ""
                    echo '{"error": "Not found"}'
                    ;;
            esac
            ;;
        GET)
            case "$path" in
                /health|/)
                    echo "HTTP/1.1 200 OK"
                    echo "Content-Type: application/json"
                    echo "Content-Length: 22"
                    echo ""
                    echo '{"status": "healthy"}'
                    ;;
                *)
                    echo "HTTP/1.1 404 Not Found"
                    echo "Content-Type: application/json"
                    echo "Content-Length: 26"
                    echo ""
                    echo '{"error": "Not found"}'
                    ;;
            esac
            ;;
        *)
            echo "HTTP/1.1 405 Method Not Allowed"
            echo "Content-Type: application/json"
            echo "Content-Length: 35"
            echo ""
            echo '{"error": "Method not allowed"}'
            ;;
    esac
}

# Simple HTTP server using netcat
start_server() {
    log "INFO" "Starting Discord webhook handler on port $PORT"
    
    if [[ -n "$DISCORD_WEBHOOK_URL" ]]; then
        log "INFO" "Discord webhook URL configured"
    else
        log "WARN" "Discord webhook URL not configured - alerts will be logged only"
    fi
    
    while true; do
        {
            # Read HTTP request line
            read -r request_line
            method=$(echo "$request_line" | cut -d' ' -f1)
            path=$(echo "$request_line" | cut -d' ' -f2)
            
            # Read headers
            local content_length=0
            while read -r header; do
                # Remove carriage return
                header=$(echo "$header" | tr -d '\r')
                
                # Break on empty line (end of headers)
                [[ -z "$header" ]] && break
                
                # Extract Content-Length
                if [[ "$header" =~ ^[Cc]ontent-[Ll]ength:\ ([0-9]+) ]]; then
                    content_length="${BASH_REMATCH[1]}"
                fi
            done
            
            # Handle the request
            handle_request "$method" "$path" "$content_length"
            
        } | nc -l -p "$PORT" -q 1
        
        # Small delay to prevent rapid respawn
        sleep 0.1
    done
}

# Main execution
main() {
    parse_args "$@"
    
    # Check dependencies
    if ! command -v jq >/dev/null 2>&1; then
        log "ERROR" "jq is required but not installed"
        exit 1
    fi
    
    if ! command -v nc >/dev/null 2>&1; then
        log "ERROR" "netcat (nc) is required but not installed"
        exit 1
    fi
    
    # Start server
    start_server
}

# Run main function
main "$@"