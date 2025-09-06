# Comprehensive Deployment Workflow Guide

This guide documents the complete deployment workflow system designed to prevent all types of failures in the sermon-uploader project.

## 🏗️ Architecture Overview

The deployment system consists of **7 phases** with **multiple safety layers** to ensure zero-failure deployments:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PRE-FLIGHT    │───▶│  BUILD & TEST   │───▶│  QUALITY GATES  │
│    CHECKS       │    │   VALIDATION    │    │   & SECURITY    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   DEPLOYMENT    │◀───│   INTEGRATION   │◀───│     DOCKER      │
│   & ROLLBACK    │    │     TESTING     │    │     BUILD       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                       ┌─────────────────┐
                       │ POST-DEPLOYMENT │
                       │   VALIDATION    │
                       └─────────────────┘
```

## 🚀 Workflow Files

### Primary Workflows

1. **`comprehensive-deployment.yml`** - Main deployment pipeline
2. **`runner-optimization.yml`** - Self-hosted runner maintenance
3. **`emergency-rollback.yml`** - Emergency recovery system

### Supporting Infrastructure

- **Quality Gates:** Coverage thresholds, security scanning, performance benchmarks
- **Blue-Green Deployment:** Zero-downtime deployments with automatic rollback
- **Health Monitoring:** Multi-layer health checks and performance validation

## 📋 Phase-by-Phase Breakdown

### Phase 1: Pre-flight Checks (FAIL FAST)

**Purpose:** Detect issues before wasting compute resources

**Jobs:**
- `detect-changes` - Smart change detection for targeted testing
- `pre-flight-security` - Secret scanning, credential validation
- `syntax-validation` - Go, TypeScript, Python syntax validation

**Failure Prevention:**
```yaml
# Example quality check
if grep -r -E "(password|token|key|secret)\s*[:=]\s*['\"][^'\"]{8,}" .; then
  echo "❌ Found hardcoded credentials"
  exit 1
fi
```

**Thresholds:**
- Zero hardcoded secrets allowed
- All syntax must be valid
- Environment configuration must be complete

### Phase 2: Build Verification

**Purpose:** Ensure all components build correctly with comprehensive testing

**Jobs:**
- `build-go-backend` - Go compilation with race detection
- `build-frontend` - TypeScript compilation and bundling
- `build-python-processor` - Python dependency validation

**Quality Metrics:**
- **Go:** Minimum 70% test coverage, race detection enabled
- **Frontend:** Bundle size monitoring, type checking
- **Python:** Compilation validation, dependency scanning

**Build Safety Features:**
```yaml
# Go race detection
CGO_ENABLED=1 go build -race -v ./...

# Multi-architecture validation
GOOS=linux GOARCH=amd64 go build -v ./...
GOOS=linux GOARCH=arm64 go build -v ./...
```

### Phase 3: Quality Gates

**Purpose:** Enforce quality standards before deployment

**Quality Thresholds:**
```yaml
MIN_GO_COVERAGE: 70      # Minimum Go test coverage
MIN_JS_COVERAGE: 60      # Minimum JavaScript coverage  
MAX_CRITICAL_VULNS: 0    # Zero critical vulnerabilities
MAX_HIGH_VULNS: 2        # Maximum 2 high-severity vulns
MAX_BUILD_TIME: 600      # 10-minute build timeout
```

**Security Scanning:**
- Trivy filesystem scanning
- Dependency vulnerability assessment
- Container security validation

### Phase 4: Docker Build & Validation

**Purpose:** Validate containerized deployment artifacts

**Features:**
- Multi-architecture builds (AMD64/ARM64)
- Container security scanning
- Runtime validation testing
- Resource usage monitoring

**Safety Checks:**
```yaml
# Test container startup
docker run --rm -d --name sermon-test sermon-uploader:test
sleep 10
curl -f http://localhost:8080/health || exit 1
```

### Phase 5: Integration Testing

**Purpose:** Validate end-to-end system integration

**Test Environment:**
- MinIO service container
- Network connectivity testing
- API endpoint validation
- Performance baseline testing

**Validation Strategy:**
```yaml
# MinIO connectivity test
for i in {1..12}; do
  if curl -f http://localhost:9000/minio/health/live; then
    break
  fi
  sleep 10
done
```

### Phase 6: Blue-Green Deployment

**Purpose:** Zero-downtime deployment with automatic rollback

**Deployment Strategy:**
1. **Backup current state** - Full system snapshot
2. **Deploy new version** - Parallel deployment
3. **Health validation** - Comprehensive health checks
4. **Traffic cutover** - Seamless transition
5. **Automatic rollback** - On any failure

**Health Check System:**
```yaml
HEALTH_CHECK_TIMEOUT=300  # 5-minute timeout
for i in {1..10}; do
  if curl -f http://localhost:8000/health; then
    echo "✅ Health check passed"
    break
  fi
  sleep 20
done
```

**Rollback Triggers:**
- Health check failures
- Performance degradation
- Service unavailability
- Container crashes

### Phase 7: Post-Deployment Validation

**Purpose:** Verify deployment success and system stability

**Validation Tests:**
- External connectivity testing
- Performance baseline validation
- Resource usage monitoring
- End-to-end functionality testing

## 🎯 Self-Hosted Runner Optimization

### Job Allocation Strategy

**GitHub Cloud Runners:**
- Build and compilation tasks (high CPU/memory)
- Security scanning and vulnerability assessment
- Multi-architecture Docker builds
- Artifact management and caching
- Integration testing with external services

**Self-Hosted Pi Runner:**
- Direct deployment to target environment
- System health monitoring and optimization
- Emergency rollback operations
- Local network connectivity testing
- Resource cleanup and maintenance

### Resource Management

**Memory Optimization:**
```yaml
MAX_MEMORY_USAGE_MB: 1024  # 1GB limit for Pi
MAX_DISK_USAGE_GB: 8       # 8GB limit for CI artifacts
```

**Cleanup Strategy:**
- Daily automated cleanup at 2 AM UTC
- Docker resource pruning (3-day retention)
- System cache clearing
- Performance metric collection

## 🚨 Emergency Response System

### Automatic Rollback Triggers

**Critical Scenarios:**
- Security incidents
- Data corruption detection
- Service unavailability
- Health check failures

**Rollback Strategies:**
- **Immediate** - Stop everything, restore from backup
- **Fast** - Quick service replacement with minimal checks
- **Graceful** - Safe transition with full validation

### Emergency Communication

**Notification Channels:**
- Discord webhook alerts
- GitHub Actions summaries
- System log entries
- Performance metric updates

## 📊 Quality Metrics and Monitoring

### Build Quality Indicators

```yaml
Build Success Rate: >99%
Average Build Time: <8 minutes
Test Coverage: >70% (Go), >60% (JS)
Security Vulnerabilities: 0 critical, <3 high
```

### Deployment Health Metrics

```yaml
Deployment Success Rate: >99.5%
Average Deployment Time: <5 minutes
Rollback Frequency: <1% of deployments
Health Check Response Time: <2 seconds
```

### System Performance Baselines

```yaml
Memory Usage: <80% of available
CPU Temperature: <70°C
Disk Usage: <75% of available
Docker Resource Usage: <8GB
```

## 🔧 Configuration and Setup

### Required GitHub Secrets

```yaml
# Pi Connection
PI_HOST: "192.168.1.127"
PI_USER: "pi"
PI_SSH_KEY: "-----BEGIN OPENSSH PRIVATE KEY-----..."
PI_PORT: "22"

# MinIO Configuration
MINIO_ENDPOINT: "http://localhost:9000"
MINIO_ACCESS_KEY: "gaius"
MINIO_SECRET_KEY: "John 3:16"
MINIO_SECURE: "false"
MINIO_BUCKET: "sermons"

# Discord Notifications
DISCORD_WEBHOOK_URL: "https://discord.com/api/webhooks/..."

# Application Settings
PORT: "8000"
WAV_SUFFIX: "_raw"
AAC_SUFFIX: "_streamable"
BATCH_THRESHOLD: "2"
```

### Environment Files

**Backend (.env.example):**
```env
MINIO_ENDPOINT=http://localhost:9000
MINIO_ACCESS_KEY=your_access_key
MINIO_SECRET_KEY=your_secret_key
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/your_webhook
PORT=8000
```

**Frontend (.env.example):**
```env
NEXT_PUBLIC_API_URL=http://localhost:8000
```

**Pi Processor (.env.example):**
```env
MINIO_ENDPOINT=http://localhost:9000
MINIO_ACCESS_KEY=your_access_key
MINIO_SECRET_KEY=your_secret_key
PROCESSING_INTERVAL=30
```

## 🛡️ Security Considerations

### Secret Management
- All secrets managed through GitHub Secrets
- No hardcoded credentials in codebase
- Regular secret rotation reminders
- Environment-based configuration

### Access Control
- Self-hosted runner isolated to deployment tasks only
- SSH key-based authentication
- Network access restrictions
- Audit logging enabled

### Vulnerability Management
- Automated security scanning in every build
- Zero-tolerance for critical vulnerabilities
- Regular dependency updates
- Container image security validation

## 📈 Performance Optimization

### Caching Strategy
```yaml
# Go module caching
uses: actions/setup-go@v5
with:
  cache: true
  cache-dependency-path: backend/go.sum

# Node.js dependency caching
uses: actions/setup-node@v4
with:
  cache: 'npm'
  cache-dependency-path: frontend/package-lock.json

# Docker layer caching
uses: docker/build-push-action@v5
with:
  cache-from: type=gha
  cache-to: type=gha,mode=max
```

### Resource Limits
- Build timeout: 10 minutes maximum
- Test timeout: 5 minutes maximum
- Deployment timeout: 30 minutes maximum
- Health check timeout: 5 minutes maximum

## 🔄 Workflow Triggers

### Automatic Triggers
```yaml
on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
  schedule:
    - cron: '0 2 * * *'  # Daily maintenance
```

### Manual Triggers
```yaml
workflow_dispatch:
  inputs:
    skip_tests:
      description: 'Skip tests (emergency)'
      type: boolean
    force_deploy:
      description: 'Force deploy despite quality gates'
      type: boolean
    emergency_mode:
      description: 'Emergency deployment mode'
      type: boolean
```

## 📚 Troubleshooting Guide

### Common Issues

**Build Failures:**
1. Check syntax validation logs
2. Verify dependency versions
3. Review test coverage reports
4. Check resource usage limits

**Deployment Failures:**
1. Verify Pi connectivity
2. Check disk space availability
3. Review Docker container logs
4. Validate environment configuration

**Health Check Failures:**
1. Test service endpoints manually
2. Review application logs
3. Check MinIO connectivity
4. Verify network configuration

### Emergency Procedures

**Complete System Failure:**
1. Trigger emergency rollback workflow
2. Review system state capture
3. Analyze rollback logs
4. Execute manual recovery if needed

**Security Incident:**
1. Immediately revoke all credentials
2. Trigger emergency rollback
3. Audit access logs
4. Update security configuration

## 🚀 Getting Started

### Initial Setup
1. Configure all required GitHub Secrets
2. Set up self-hosted runner on Pi
3. Test connectivity and permissions
4. Run initial deployment workflow
5. Verify all health checks pass

### Regular Operations
1. Monitor workflow execution status
2. Review quality metrics weekly
3. Update dependencies monthly
4. Test emergency procedures quarterly

### Continuous Improvement
1. Analyze failure patterns
2. Update quality thresholds
3. Optimize build performance
4. Enhance monitoring capabilities

This comprehensive deployment workflow provides multiple layers of protection against failures while maintaining high deployment velocity and system reliability.