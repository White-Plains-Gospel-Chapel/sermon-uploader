# Security Model

## Threat Model & Attack Vectors

### External Threats
- **DDoS attacks** → Mitigated by CloudFlare protection
- **SSH brute force** → Eliminated by zero SSH exposure  
- **Web application attacks** → Protected by CloudFlare WAF
- **Network reconnaissance** → Pi not directly accessible

### Internal Threats
- **Malicious file uploads** → Multi-layer validation (MIME, FFmpeg)
- **Privilege escalation** → Non-root container execution
- **Data exfiltration** → VPN-only admin access, audit logging

## Security Architecture Layers

```
┌─────────────────────────────────────────────────────┐
│ Layer 4: CloudFlare Protection (DDoS, WAF, SSL)    │
├─────────────────────────────────────────────────────┤
│ Layer 3: Tunnel Authentication (No direct access)   │
├─────────────────────────────────────────────────────┤
│ Layer 2: VPN Access Control (Admin functions only)  │
├─────────────────────────────────────────────────────┤
│ Layer 1: Application Security (Input validation)    │
└─────────────────────────────────────────────────────┘
```

## Access Control Matrix

| User Type | Web Interface | SSH Access | MinIO Admin | Deployment |
|-----------|---------------|------------|-------------|------------|
| **Public Users** | ✅ (via tunnel) | ❌ | ❌ | ❌ |
| **Administrators** | ✅ (via tunnel) | ✅ (via VPN) | ✅ (via VPN) | ❌ |
| **CI/CD System** | ❌ | ❌ | ❌ | ✅ (via webhook) |

## Authentication Methods

### Public Access
- **Method**: No authentication required for upload interface
- **Protection**: Rate limiting, file validation, CloudFlare protection

### Admin Access  
- **Method**: SSH key authentication through VPN
- **Key Management**: Stored in GitHub Secrets and local .env
- **VPN Access**: Unifi Teleport with invitation-based enrollment

### Deployment Access
- **Method**: HMAC-SHA256 webhook signatures
- **Secret Management**: GitHub Secrets → Environment variables
- **Validation**: Cryptographic proof of request authenticity

## Encryption Standards

### Data in Transit
- **Public Web**: TLS 1.3 via CloudFlare (automatic certificates)
- **Admin VPN**: WireGuard encryption (Unifi Teleport)
- **Deployment**: HTTPS webhook endpoints with signature validation

### Data at Rest
- **File Storage**: MinIO server-side encryption (optional)
- **Configuration**: Environment variable injection (no plaintext secrets)
- **Logs**: Local filesystem (consider encryption for sensitive logs)

## Security Monitoring

### Real-time Monitoring
- Failed authentication attempts
- Unusual upload patterns  
- VPN connection anomalies
- Webhook authentication failures

### Audit Trail
- All admin access via VPN (logged)
- File upload events with SHA256 hashes
- Deployment activities with timestamps
- System access patterns

## Incident Response Procedures

### Security Incident Classification
- **P1**: Active attack or data breach
- **P2**: Failed authentication attempts or suspicious activity
- **P3**: Security configuration issues

### Response Actions
1. **Immediate**: Disable affected services via VPN access
2. **Investigation**: Review logs and audit trail
3. **Containment**: Block malicious IPs via CloudFlare
4. **Recovery**: Restore from known-good backups if needed

## Compliance Considerations

### Data Protection
- Audio files may contain personal information
- Access logging for audit purposes
- Data retention policies (configurable)
- Right to deletion (manual process)

### Security Standards Alignment
- **Zero Trust**: Never trust, always verify
- **Principle of Least Privilege**: Minimum required access
- **Defense in Depth**: Multiple security layers
- **Continuous Monitoring**: Real-time security oversight

## Security Configuration Checklist

### Initial Setup
- [ ] CloudFlare Tunnel configured with HTTPS
- [ ] Unifi Teleport VPN enabled and tested
- [ ] SSH keys generated and added to GitHub Secrets
- [ ] Webhook secrets configured with strong entropy
- [ ] Container running as non-root user
- [ ] Firewall rules reviewed (should be minimal due to tunnel)

### Ongoing Maintenance
- [ ] Regular security updates on Pi
- [ ] SSH key rotation (quarterly recommended)
- [ ] Webhook secret rotation (bi-annually)
- [ ] VPN access review (remove unused invitations)
- [ ] Log review for suspicious activity

### Emergency Procedures
- [ ] VPN access from multiple devices configured
- [ ] Emergency SSH access procedure documented
- [ ] Backup admin accounts configured
- [ ] CloudFlare access credentials secured

---

*Critical: Never expose SSH directly to internet, even with hardening*
*Priority: Maintain VPN access for all administrative functions*