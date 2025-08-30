# Critical Project Insights

## 🔑 Key Technical Decisions

### Single Container Architecture
**Why**: Embedding MinIO eliminates external dependencies and simplifies deployment
**Impact**: Atomic deployments, easier backup/restore, reduced failure modes

### Zero-Exposure Security Model  
**Problem**: Need public web access without exposing Pi to internet
**Solution**: CloudFlare Tunnel + Unifi Teleport VPN
**Result**: Public website with admin-only VPN access

### Pre-Deployment Verification
**Discovery**: 90% of deployment failures are environmental, not code
**Solution**: Local SSH/network testing before GitHub Actions
**Savings**: ~$50-100/month in wasted Actions minutes

## 🛡️ Security Evolution

```
Phase 1: SSH + Port Forwarding (HIGH RISK)
Phase 2: Hardened SSH + Firewall (MEDIUM RISK)  
Phase 3: Zero Exposure + Tunnels (LOW RISK)
```

**Critical Realization**: Public web apps don't require Pi exposure

## 🚀 Deployment Pipeline Insights

### Cost Optimization Pattern
1. **Pre-commit**: Go build, TypeScript, ESLint validation
2. **Pre-push**: Pi connectivity + Docker build verification  
3. **Deploy**: Webhook-triggered (no SSH exposure)
4. **Monitor**: Automatic deployment tracking

### Failure Prevention Strategy
- **Network Check**: Ping + SSH port accessibility
- **Auth Test**: SSH key format validation
- **Service Check**: Docker + project directory verification  
- **Build Test**: Full Docker build before push

## 🔧 Performance Optimizations

### ARM64 Specific
- Native Go binaries (cross-compilation)
- ARM64-optimized Docker images
- Hardware-specific FFmpeg builds

### Resource Management
- **Memory**: 512MB container limit for Pi stability
- **Storage**: SSD for containers, SD for bulk data
- **CPU**: Reserve cores for system operations

## 📊 Monitoring Strategy

### Health Check Layers
1. **Container**: Docker health checks
2. **Application**: HTTP endpoint monitoring
3. **Service**: MinIO + FFmpeg availability  
4. **System**: Pi resource monitoring

### Log Aggregation
- Application logs (structured JSON)
- System logs (Pi events)  
- Docker logs (container lifecycle)
- Webhook logs (deployment events)

## 🔗 Critical Connections

### Security ↔ Usability
VPN-only admin access maintains security while preserving functionality

### Performance ↔ Cost
Edge computing (Pi) reduces cloud costs while improving local performance

### Reliability ↔ Simplicity  
Single container reduces complexity and improves reliability

### Development ↔ Operations
Git hooks bridge development practices with operational concerns

---
*For detailed implementation: See [[DEPLOYMENT-ARCHITECTURE.md]]*
*For setup procedures: See [[README.md]] and [[SETUP_INSTRUCTIONS.md]]*