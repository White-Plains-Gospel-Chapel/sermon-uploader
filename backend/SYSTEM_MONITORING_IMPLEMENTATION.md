# System Resource Monitoring Implementation

## Overview

Comprehensive system resource monitoring for the sermon-uploader service running on Raspberry Pi 5. Monitors only the resources actually used by our application with real-time Discord notifications.

## Architecture

### Components

1. **SystemResourceMonitor** (`services/system_monitor.go`)
   - Core monitoring service with Discord integration
   - Focuses only on resources used by sermon-uploader
   - Real-time metrics collection with historical trends

2. **System Helpers** (`services/system_monitor_helpers.go`)
   - Linux system file readers (/proc/stat, /proc/meminfo, etc.)
   - Pi 5 specific thermal and hardware monitoring
   - Network interface statistics and health checks

3. **Discord Integration** (`services/system_monitor_discord.go`)
   - Live-updating Discord messages (no spam!)
   - Concise resource status with status icons
   - EST timezone formatting and session tracking

## Monitored Resources

### CPU Usage
- **Purpose**: HTTP request processing, file handling, Go runtime
- **Metrics**: Usage percentage, goroutine count, load average
- **Thresholds**: 🟢 <70%, 🟡 70-90%, 🔴 >90%

### Memory Usage
- **Purpose**: File uploads, streaming, Go memory management
- **Metrics**: System RAM usage, Go allocation, available memory
- **Thresholds**: 🟢 <60%, 🟡 60-80%, 🔴 >80%

### Temperature Monitoring
- **Purpose**: Pi 5 thermal management during file processing
- **Metrics**: CPU temperature, throttling status
- **Thresholds**: 🟢 <70°C, 🟡 70-80°C, 🔴 >80°C
- **Critical**: 85°C (Pi 5 thermal limit)

### Disk Usage
- **Purpose**: MinIO storage, log files, temporary uploads
- **Metrics**: Free space, usage percentage, inode usage
- **Thresholds**: 🟢 <70%, 🟡 70-90%, 🔴 >90%

### Network Statistics
- **Purpose**: File uploads, Discord webhooks, MinIO communication
- **Metrics**: Interface status, error counts, traffic stats
- **Monitoring**: eth0 (primary), wlan0 (fallback)

## Features

### Real-Time Discord Updates
- Single live-updating message (prevents spam)
- Session duration tracking
- EST timezone formatting
- Status icons for quick visual assessment

### Historical Trends
- CPU, memory, and temperature trends over 1 hour
- Trend indicators: 🔺 Increasing, 🔻 Decreasing, ➡️ Stable, 📊 Collecting

### Resource Status Icons
- 🟢 Normal operation
- 🟡 Warning threshold reached
- 🔴 Critical threshold or error condition
- 🔍 Detecting/initializing

## Configuration

### Environment Variables
```bash
# System monitoring interval (default: 60s)
SYSTEM_MONITOR_INTERVAL=60s

# Discord webhook for system notifications
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
```

### Integration Points
```go
// Initialize system monitor
systemMonitor := services.NewSystemResourceMonitor(
    logger,
    discordLiveService, 
    60*time.Second, // monitoring interval
)

// Start monitoring
if err := systemMonitor.Start(); err != nil {
    log.Printf("Failed to start system monitoring: %v", err)
}
```

## Discord Message Format

```
🖥️ Raspberry Pi 5 - System Monitor
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 Session Started: 3:45 PM EST
⏱️ Runtime: 2h 15m
🔄 Last Updated: 5:59:45 PM EST

📊 Resource Usage (Sermon Uploader)
├─ 🟢 CPU: 15.2% | 45 goroutines | Load: 0.8
├─ 🟢 Memory: 32.1% (2.1/8.0 GB) | Go: 45MB
├─ 🟢 Temperature: 42.5°C
├─ 🟢 Disk: 15.2 GB free (68% used)
└─ 🟢 Network: eth0 (UP)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🏷️ Raspberry Pi 5 - ARM64 | Sermon Uploader v1.1.0
📊 Monitoring CPU, Memory, Thermal, Disk & Network
```

## Implementation Details

### Linux System Integration
- Reads `/proc/stat` for CPU usage calculation
- Parses `/proc/meminfo` for memory statistics  
- Monitors `/sys/class/thermal/thermal_zone0/temp` for CPU temperature
- Checks `/proc/net/dev` for network interface statistics
- Uses `syscall.Statfs()` for disk usage information

### Pi 5 Specific Features
- Thermal throttling detection via `vcgencmd get_throttled`
- Temperature monitoring with 85°C critical threshold
- ARM64 architecture optimizations
- Network interface detection (eth0/wlan0)

### Error Handling
- Graceful fallbacks when system files are unavailable
- Non-blocking Discord notifications
- Continues monitoring even if individual metrics fail
- Detailed error logging with context

### Performance Considerations
- Minimal system overhead (~1% CPU usage)
- 60-second default monitoring interval
- Historical data limited to 1 hour (60 readings)
- Async Discord updates to prevent blocking

## Testing

### Manual Testing
```bash
# Test system monitoring
curl http://localhost:8001/api/system/metrics

# Check Discord integration
curl -X POST http://localhost:8001/api/test/system/monitor
```

### Integration Testing
- System monitoring initialization
- Metric collection accuracy
- Discord message formatting
- Resource threshold alerts
- Historical trend calculation

## Production Deployment

### Requirements
- Raspberry Pi 5 with ARM64 Linux
- Discord webhook URL configured
- System access to /proc and /sys filesystems
- Network connectivity for Discord notifications

### Monitoring Alerts
- CPU usage >90% for >5 minutes
- Memory usage >80% for >5 minutes  
- Temperature >80°C
- Disk space <2GB free
- Network interface errors >10 per minute

## Benefits

1. **Focused Monitoring**: Only tracks resources actually used by sermon-uploader
2. **Real-Time Visibility**: Immediate Discord notifications for issues
3. **Proactive Alerts**: Warning thresholds before critical issues
4. **Historical Context**: Trend analysis for performance optimization
5. **Pi 5 Optimized**: Tailored for Raspberry Pi 5 hardware characteristics

## Future Enhancements

- Integration with production logger for correlation
- Custom alert thresholds per environment
- Metrics export to time-series database
- Advanced trend analysis and predictions
- Integration with GitHub Actions for deployment health checks

## Related Documentation

- [Production Logger Implementation](./PRODUCTION_LOGGING.md)
- [Discord Integration Guide](./GITHUB_DISCORD_INTEGRATION.md)
- [TDD Implementation Guide](./TDD_CYCLES_DOCUMENTATION.md)
- [Pi 5 Optimization Settings](./.env.example)