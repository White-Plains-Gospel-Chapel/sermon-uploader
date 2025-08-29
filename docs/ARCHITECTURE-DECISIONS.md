# Architecture Decision Records (ADRs)

## ADR-001: Single Container vs Microservices

**Decision**: Embed MinIO within application container
**Status**: Accepted
**Date**: 2025-01-29

**Context**: Need object storage for audio files with Pi deployment constraints

**Options Considered**:
- External MinIO service
- Cloud storage (S3)
- Embedded MinIO

**Decision**: Embedded MinIO
**Rationale**: 
- Eliminates service orchestration complexity
- Atomic deployments and backups
- Reduced network overhead
- Better resource utilization on Pi

**Consequences**:
- Positive: Simplified deployment, better performance
- Negative: Less flexible scaling options

---

## ADR-002: SSH vs Webhook Deployment  

**Decision**: Replace SSH deployment with webhook-triggered updates
**Status**: Accepted  
**Date**: 2025-01-29

**Context**: GitHub Actions SSH deployment exposed security risks and failed frequently

**Options Considered**:
- Hardened SSH with restricted keys
- Self-hosted GitHub runner
- Webhook-based deployment

**Decision**: Webhook deployment
**Rationale**:
- Eliminates SSH exposure to internet
- Better error handling and logging
- Cryptographic authentication via HMAC
- Works through CloudFlare tunnel

**Consequences**:
- Positive: Enhanced security, better reliability
- Negative: Additional webhook server complexity

---

## ADR-003: CloudFlare Tunnel vs Port Forwarding

**Decision**: Use CloudFlare Tunnel for public access
**Status**: Accepted
**Date**: 2025-01-29

**Context**: Need public web access without exposing Pi directly to internet

**Options Considered**:
- Router port forwarding + security hardening
- VPN-only access (no public web)
- CloudFlare Tunnel
- ngrok/similar tunneling services

**Decision**: CloudFlare Tunnel
**Rationale**:
- Zero Pi exposure to internet
- Built-in DDoS protection
- Automatic SSL/TLS certificates
- Free tier suitable for project scale
- Better performance with global CDN

**Consequences**:
- Positive: Maximum security with public accessibility
- Negative: Dependency on CloudFlare service

---

## ADR-004: Go Fiber vs Express.js Backend

**Decision**: Use Go with Fiber framework for backend
**Status**: Accepted
**Date**: 2025-01-29

**Context**: Need performant backend for Pi hardware constraints

**Options Considered**:
- Node.js with Express
- Python with FastAPI
- Go with Gin
- Go with Fiber

**Decision**: Go with Fiber
**Rationale**:
- Better performance on ARM64 architecture
- Lower memory footprint than Node.js
- Native concurrency for file uploads
- Fast JSON serialization
- Express-like API familiar to developers

**Consequences**:
- Positive: Excellent Pi performance, familiar API
- Negative: Smaller ecosystem than Node.js

---

## ADR-005: Pre-deployment Verification Strategy

**Decision**: Implement comprehensive local verification before GitHub Actions
**Status**: Accepted
**Date**: 2025-01-29

**Context**: Frequent deployment failures wasting GitHub Actions minutes and time

**Options Considered**:
- Debug failures in GitHub Actions
- Basic connectivity checking
- Comprehensive local verification
- Self-hosted runners

**Decision**: Comprehensive local verification
**Rationale**:
- Catches 90% of deployment issues locally
- Significant cost savings on Actions minutes
- Faster feedback loop for developers
- Better developer experience

**Consequences**:
- Positive: Reduced costs, faster development cycle
- Negative: Additional tooling complexity

---

## ADR-006: Unifi Teleport vs OpenVPN for Admin Access

**Decision**: Use existing Unifi Teleport VPN for admin access
**Status**: Accepted
**Date**: 2025-01-29

**Context**: Need secure admin access to Pi without exposing SSH

**Options Considered**:
- OpenVPN setup
- WireGuard configuration  
- Tailscale mesh network
- Existing Unifi Teleport

**Decision**: Unifi Teleport
**Rationale**:
- Already available with UDM Pro
- Zero additional cost
- Simple setup and management
- Integrates with existing network infrastructure
- WireGuard-based performance

**Consequences**:
- Positive: No additional software/cost, easy management
- Negative: Vendor lock-in to Unifi ecosystem

---

## Decision Impact Matrix

| Decision | Security Impact | Cost Impact | Complexity Impact | Performance Impact |
|----------|----------------|-------------|------------------|-------------------|
| Single Container | Medium | Low | Low | High |
| Webhook Deployment | High | Medium | Medium | Medium |
| CloudFlare Tunnel | High | Low | Low | High |
| Go/Fiber Backend | Low | Low | Low | High |
| Pre-deployment Verification | Medium | High | Medium | Medium |
| Unifi Teleport | High | High | Low | Medium |

---

*These decisions can be revisited as project requirements evolve*