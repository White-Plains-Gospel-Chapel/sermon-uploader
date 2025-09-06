# Rollback and Recovery Guide

This comprehensive guide covers rollback and recovery procedures for the Sermon Uploader deployment system. It includes automated rollback triggers, manual recovery procedures, monitoring setup, and emergency protocols.

## Table of Contents

1. [Overview](#overview)
2. [Automatic Rollback System](#automatic-rollback-system)
3. [Manual Recovery Procedures](#manual-recovery-procedures)
4. [Monitoring and Alerting](#monitoring-and-alerting)
5. [Emergency Procedures](#emergency-procedures)
6. [Troubleshooting](#troubleshooting)
7. [Recovery Toolkit](#recovery-toolkit)
8. [Best Practices](#best-practices)

## Overview

The rollback and recovery system provides multiple layers of protection:

- **Automatic Rollback**: Triggered by health check failures, performance degradation, or resource exhaustion
- **Manual Recovery**: Step-by-step procedures for various failure scenarios  
- **Monitoring**: Comprehensive metrics collection and alerting via Discord
- **Emergency Protocols**: Last-resort procedures for critical failures

## Automatic Rollback System

### Rollback Triggers

The system automatically triggers rollbacks when:

1. **Health Check Failures**
   - Service unavailable for >1 minute
   - MinIO connection failures for >2 minutes
   - API health endpoint returning errors

2. **Performance Degradation**  
   - Error rate >10% for 3 minutes
   - Response time >5 seconds (95th percentile) for 5 minutes
   - Upload failure rate >10% for 2 minutes

3. **Resource Exhaustion**
   - Memory usage >90% for 2 minutes
   - CPU usage >95% for 3 minutes
   - Disk usage >90%

4. **Container Issues**
   - Container restarts >3 times in 5 minutes
   - Container not running
   - Docker daemon failures

### GitHub Actions Rollback Workflow

#### Triggering a Rollback

**Automatic Trigger** (via health monitor):
```bash
# The health monitor automatically triggers rollbacks via GitHub API
curl -X POST \
  -H "Authorization: token $GITHUB_TOKEN" \
  -H "Accept: application/vnd.github.v3+json" \
  "https://api.github.com/repos/$REPO/actions/workflows/rollback.yml/dispatches" \
  -d '{"ref": "main", "inputs": {"rollback_type": "previous_version", "reason": "Health check failures", "force_rollback": "true"}}'
```

**Manual Trigger** (via GitHub UI):
1. Go to Actions tab in GitHub repository
2. Select "Automated Rollback" workflow
3. Click "Run workflow"
4. Choose rollback type:
   - `previous_version`: Roll back to last successful deployment
   - `specific_commit`: Roll back to specific commit SHA
   - `emergency_stop`: Stop all services immediately
5. Provide reason and confirm

#### Rollback Types

**Previous Version Rollback**:
```yaml
rollback_type: "previous_version"
reason: "High error rate detected"
force_rollback: "false"  # Checks system health first
```

**Specific Commit Rollback**:
```yaml
rollback_type: "specific_commit"
target_commit: "abc123def456"
reason: "Rolling back to known good state"
force_rollback: "true"
```

**Emergency Stop**:
```yaml
rollback_type: "emergency_stop"
reason: "Critical system failure"
force_rollback: "true"
```

### Health Monitoring Service

#### Setup Health Monitor

```bash
# Install and configure health monitor
sudo cp scripts/health-monitor.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/health-monitor.sh

# Create configuration
sudo mkdir -p /etc/sermon-uploader
sudo cp scripts/health-monitor.conf /etc/sermon-uploader/

# Create systemd service
sudo tee /etc/systemd/system/sermon-uploader-health-monitor.service << EOF
[Unit]
Description=Sermon Uploader Health Monitor
After=network.target docker.service
Requires=docker.service

[Service]
Type=forking
ExecStart=/usr/local/bin/health-monitor.sh --daemon
ExecStop=/bin/kill -TERM \$MAINPID
PIDFile=/var/run/sermon-uploader-health-monitor.pid
Restart=always
RestartSec=10
User=pi
Environment=DISCORD_WEBHOOK_URL=your-webhook-url
Environment=GITHUB_TOKEN=your-token
Environment=GITHUB_REPO=your-org/sermon-uploader

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl enable sermon-uploader-health-monitor
sudo systemctl start sermon-uploader-health-monitor
```

#### Health Monitor Configuration

Edit `/etc/sermon-uploader/health-monitor.conf`:

```bash
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
```

## Manual Recovery Procedures

### Recovery Toolkit Usage

The recovery toolkit provides comprehensive manual recovery options:

```bash
# Make executable
chmod +x scripts/recovery-toolkit.sh

# Run comprehensive diagnostics
./scripts/recovery-toolkit.sh diagnose

# Perform health check
./scripts/recovery-toolkit.sh health-check

# Try automatic fixes
./scripts/recovery-toolkit.sh quick-fix

# Restart services with verification
./scripts/recovery-toolkit.sh restart-services

# Emergency stop
./scripts/recovery-toolkit.sh emergency-stop

# Create backup
./scripts/recovery-toolkit.sh backup-create

# View logs
./scripts/recovery-toolkit.sh logs --tail 50
```

### Step-by-Step Recovery Procedures

#### 1. Service Not Responding

```bash
# Check service status
./scripts/recovery-toolkit.sh health-check

# Check container status
cd /opt/sermon-uploader
docker compose -f docker-compose.single.yml ps

# Check logs for errors
docker compose -f docker-compose.single.yml logs --tail 50

# Try quick fix
./scripts/recovery-toolkit.sh quick-fix

# If quick fix fails, restart services
./scripts/recovery-toolkit.sh restart-services
```

#### 2. High Memory Usage

```bash
# Check memory usage
free -h
docker stats --no-stream

# Check for memory leaks
ps aux --sort=-%mem | head -20

# Clean up Docker resources
docker system prune -af
docker volume prune -f

# Restart services with memory limits
./scripts/recovery-toolkit.sh restart-services
```

#### 3. Disk Space Issues

```bash
# Check disk usage
df -h
du -sh /opt/sermon-uploader/* | sort -hr

# Clean up old logs
find /var/log -name "*.log" -mtime +7 -delete
journalctl --vacuum-time=7d

# Clean up Docker
docker system prune -af --volumes
docker image prune -af --filter="until=48h"

# Clean up MinIO temp files
rm -rf /opt/sermon-uploader/temp/*
rm -rf /opt/sermon-uploader/uploads/temp/*
```

#### 4. Database/MinIO Corruption

```bash
# Stop services
./scripts/recovery-toolkit.sh emergency-stop

# Check MinIO data integrity
cd /opt/sermon-uploader
docker run --rm -v sermon_data:/data minio/mc ls local/

# If MinIO data is corrupted, restore from backup
# (Backup procedures should be implemented separately)

# Restart with fresh MinIO instance if needed
docker volume rm sermon_data
./scripts/recovery-toolkit.sh restart-services
```

#### 5. Container Won't Start

```bash
# Check Docker daemon status
sudo systemctl status docker

# Check for port conflicts
netstat -tlpn | grep -E ":(8000|9000|9001)"

# Kill conflicting processes
sudo fuser -k 8000/tcp
sudo fuser -k 9000/tcp
sudo fuser -k 9001/tcp

# Remove problematic containers
docker compose -f docker-compose.single.yml down --remove-orphans
docker system prune -f

# Start fresh
./scripts/recovery-toolkit.sh restart-services
```

## Monitoring and Alerting

### Discord Notifications

#### Alert Severity Levels

**Critical Alerts** (Immediate notification):
- Service completely down
- MinIO unavailable
- High error rates (>10%)
- Disk usage >90%
- System completely unresponsive

**Warning Alerts** (Grouped notifications):
- High response times
- Memory usage >90%
- CPU usage >95%
- Container restarts
- Performance degradation

**Info Alerts** (Daily digest):
- Successful deployments
- System statistics
- Maintenance notifications

#### Discord Webhook Setup

```bash
# Configure Discord webhook in environment
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR/WEBHOOK/URL"

# Test webhook
curl -X POST "$DISCORD_WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d '{
    "embeds": [{
      "title": "🧪 Test Alert",
      "description": "Testing rollback and recovery notification system",
      "color": 65280,
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%S.000Z)'",
      "footer": {"text": "Sermon Uploader Monitoring"}
    }]
  }'
```

### Prometheus Monitoring Setup

#### Deploy Monitoring Stack

```bash
# Navigate to monitoring directory
cd monitoring/

# Configure environment
cp .env.example .env
# Edit .env with your Discord webhook URL and Grafana password

# Start monitoring stack
docker compose -f docker-compose.monitoring.yml up -d

# Verify services
curl http://localhost:9090  # Prometheus
curl http://localhost:9093  # Alertmanager
curl http://localhost:3000  # Grafana (admin/admin123)
curl http://localhost:9100/metrics  # Node Exporter
curl http://localhost:8080/metrics  # cAdvisor
```

#### Key Metrics to Monitor

**Service Health Metrics**:
```promql
# Service availability
up{job="sermon-uploader"}

# Request rate
rate(sermon_uploader_requests_total[5m])

# Error rate
rate(sermon_uploader_errors_total[5m]) / rate(sermon_uploader_requests_total[5m])

# Response time
histogram_quantile(0.95, rate(sermon_uploader_request_duration_seconds_bucket[5m]))
```

**System Metrics**:
```promql
# Memory usage
(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes) / node_memory_MemTotal_bytes

# CPU usage
100 - (avg by(instance)(irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# Disk usage
(node_filesystem_size_bytes - node_filesystem_free_bytes) / node_filesystem_size_bytes
```

**Container Metrics**:
```promql
# Container restarts
rate(container_start_time_seconds[10m])

# Container memory usage
container_memory_usage_bytes{name=~".*sermon-uploader.*"}

# Container CPU usage
rate(container_cpu_usage_seconds_total{name=~".*sermon-uploader.*"}[5m])
```

### Alert Thresholds

Configure these thresholds in `monitoring/alerts.yml`:

| Alert | Threshold | Duration | Action |
|-------|-----------|----------|--------|
| ServiceDown | up == 0 | 1m | Immediate Discord alert + Auto rollback |
| HighErrorRate | >10% | 3m | Discord alert + Auto rollback |
| HighResponseTime | >5s (95th) | 5m | Discord warning + Consider rollback |
| HighMemoryUsage | >90% | 2m | Discord warning + Monitor |
| HighCPUUsage | >95% | 3m | Discord warning + Monitor |
| HighDiskUsage | >90% | 1m | Discord alert + Clean up |
| ContainerRestarting | >3 restarts/10m | 1m | Discord warning + Investigate |

## Emergency Procedures

### Emergency Contact Information

**Primary Contacts**:
- System Administrator: [contact info]
- Discord Channel: #sermons-uploading-notif
- GitHub Repository: [repo URL]

**Escalation Path**:
1. Automated systems attempt recovery (0-5 minutes)
2. Discord alerts sent to team (immediate)
3. Health monitor attempts rollback (2-10 minutes)
4. Manual intervention required alert (10+ minutes)

### Emergency Stop Procedure

When system is completely unresponsive:

```bash
# Option 1: Use recovery toolkit
./scripts/recovery-toolkit.sh emergency-stop

# Option 2: Manual emergency stop
cd /opt/sermon-uploader
docker compose -f docker-compose.single.yml down --timeout 5
docker compose -f docker-compose.prod.yml down --timeout 5
pkill -f "sermon-uploader"
pkill -f "minio"
sudo systemctl stop docker  # If needed
```

### Emergency Restart Procedure

Complete system restart as last resort:

```bash
# Create emergency backup
./scripts/recovery-toolkit.sh backup-create emergency-$(date +%Y%m%d-%H%M%S)

# Stop all services
./scripts/recovery-toolkit.sh emergency-stop

# Wait for complete shutdown
sleep 30

# Clean up corrupted state
docker system prune -af --volumes
docker network prune -f

# Restart Docker daemon
sudo systemctl restart docker

# Restart services
cd /opt/sermon-uploader
./scripts/recovery-toolkit.sh restart-services

# Verify recovery
./scripts/recovery-toolkit.sh health-check
```

## Troubleshooting

### Common Issues and Solutions

#### Issue: "Container exits immediately"

**Symptoms**: Container starts then exits with code 1 or 125

**Diagnosis**:
```bash
docker compose -f docker-compose.single.yml logs sermon-uploader --tail 50
docker inspect $(docker compose -f docker-compose.single.yml ps -q sermon-uploader)
```

**Solutions**:
1. Check environment variables in `.env`
2. Verify image integrity: `docker pull ghcr.io/white-plains-gospel-chapel/sermon-uploader:latest`
3. Check file permissions: `ls -la /opt/sermon-uploader/`
4. Verify MinIO volume: `docker volume ls | grep sermon`

#### Issue: "High memory usage leading to OOM kills"

**Symptoms**: Containers randomly stopping, `dmesg` shows OOM killer

**Diagnosis**:
```bash
dmesg | grep -i "killed process"
docker stats --no-stream
free -h
```

**Solutions**:
1. Add memory limits to compose file
2. Increase swap space: `sudo dphys-swapfile setup`
3. Optimize Pi settings in backend config
4. Clean up memory leaks via restart

#### Issue: "MinIO connection timeouts"

**Symptoms**: "connection timeout" errors in logs

**Diagnosis**:
```bash
curl -v http://localhost:9000/minio/health/live
netstat -tlpn | grep 9000
docker compose -f docker-compose.single.yml exec sermon-uploader ping minio
```

**Solutions**:
1. Check MinIO container status
2. Verify internal Docker networking
3. Restart containers with network reset
4. Check firewall settings

### Log Analysis

#### Important Log Locations

```bash
# Application logs
docker compose -f docker-compose.single.yml logs sermon-uploader

# System logs
journalctl -u docker
journalctl -u sermon-uploader-health-monitor

# Recovery toolkit logs
tail -f /var/log/sermon-uploader/recovery.log

# Health monitor logs
tail -f /var/log/sermon-uploader/health-monitor.log
```

#### Log Analysis Commands

```bash
# Find errors in last 1000 lines
docker compose logs --tail 1000 | grep -i "error\|fail\|panic\|fatal"

# Count error types
docker compose logs --since 1h | grep -i error | sort | uniq -c | sort -nr

# Monitor logs in real-time
docker compose logs -f --tail 10

# Extract performance metrics from logs
grep "request completed" docker.log | awk '{print $NF}' | sort -n
```

## Best Practices

### Preventive Measures

1. **Regular Health Checks**
   - Run `./scripts/recovery-toolkit.sh health-check` daily
   - Monitor Discord alerts channel
   - Review weekly system metrics

2. **Backup Strategy**
   - Create backups before major changes
   - Automate daily configuration backups
   - Test backup restoration procedures monthly

3. **Resource Management**
   - Monitor disk space weekly
   - Clean up Docker resources regularly
   - Update containers monthly during maintenance windows

4. **Documentation**
   - Keep runbooks updated
   - Document all manual interventions
   - Maintain contact information current

### Deployment Safety

1. **Pre-deployment Checks**
   ```bash
   # Create pre-deployment backup
   ./scripts/recovery-toolkit.sh backup-create pre-deploy-$(date +%Y%m%d)
   
   # Verify system health
   ./scripts/recovery-toolkit.sh health-check
   
   # Check resource availability
   df -h && free -h
   ```

2. **Post-deployment Verification**
   ```bash
   # Wait for services to stabilize
   sleep 60
   
   # Run health checks
   ./scripts/recovery-toolkit.sh health-check
   
   # Test key functionality
   curl -f http://localhost:8000/api/health
   curl -f http://localhost:9000/minio/health/live
   ```

3. **Rollback Decision Matrix**

   | Condition | Action | Urgency |
   |-----------|---------|---------|
   | Health check fails | Automatic rollback | Immediate |
   | Error rate >10% | Alert + consider rollback | 3 minutes |
   | Response time >5s | Monitor + alert | 5 minutes |
   | Resource exhaustion | Clean up + alert | Immediate |
   | Container crashes | Automatic restart | Immediate |

### Monitoring Best Practices

1. **Alert Fatigue Prevention**
   - Use appropriate thresholds
   - Group related alerts
   - Implement alert suppression during maintenance

2. **Escalation Procedures**
   - Define clear escalation paths
   - Set maximum auto-retry attempts
   - Require human confirmation for destructive actions

3. **Documentation Standards**
   - Document all alert responses
   - Maintain runbook accuracy
   - Update procedures after incidents

This comprehensive guide ensures reliable rollback and recovery capabilities for the Sermon Uploader system. Regular testing and updates of these procedures are essential for maintaining system reliability.