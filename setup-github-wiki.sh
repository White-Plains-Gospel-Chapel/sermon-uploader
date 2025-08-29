#!/bin/bash
set -e

echo "📚 Setting up GitHub Wiki for Sermon Uploader..."

# Check if wiki is enabled
echo "🔍 Checking wiki status..."
WIKI_ENABLED=$(gh api repos/White-Plains-Gospel-Chapel/sermon-uploader --jq '.has_wiki')

if [ "$WIKI_ENABLED" = "false" ]; then
  echo "⚠️ Wiki is disabled. Enabling it..."
  gh api repos/White-Plains-Gospel-Chapel/sermon-uploader -X PATCH -f has_wiki=true
  echo "✅ Wiki enabled"
fi

# Create wiki directory structure
echo "📁 Creating local wiki structure..."
mkdir -p wiki-content

# Create Home page
cat > wiki-content/Home.md << 'EOF'
# Sermon Uploader - Secure Pi Deployment

Welcome to the Sermon Uploader project wiki. This documentation covers the complete secure deployment architecture.

## 🏗️ Architecture Overview

The system uses a **zero-exposure architecture**:
- **CloudFlare Tunnel** for public web access (no Pi exposure)
- **Unifi Teleport VPN** for secure admin access  
- **Webhook deployment** for automated CI/CD (no SSH exposure)

## 📋 Project Status

**Current Milestone**: [Secure Pi Deployment v1.0](../milestone/1)

### Implementation Progress:
- [ ] **Phase 1**: CloudFlare Tunnel Setup (Issues #1, #4, #5)
- [ ] **Phase 2**: Unifi Teleport VPN (Issues #2, #6, #7)  
- [ ] **Phase 3**: Webhook Deployment (Issues #3, #8, #9, #10)
- [ ] **Phase 4**: Integration Testing (Issue #11)

## 🔗 Quick Links

### Documentation:
- [[Architecture Design]] - Complete technical architecture
- [[Security Model]] - Zero-trust security implementation  
- [[Deployment Guide]] - Step-by-step setup instructions
- [[Troubleshooting]] - Common issues and solutions

### Implementation:
- [[CloudFlare Tunnel Setup]] - Public access configuration
- [[Unifi Teleport Guide]] - VPN setup for UDM Pro
- [[Webhook Deployment]] - CI/CD automation setup

### Operations:
- [[Monitoring]] - System health and performance  
- [[Backup Procedures]] - Data protection strategies
- [[Incident Response]] - Emergency procedures

## 🔄 Development Workflow

1. **Issues** → Feature branches (auto-created)
2. **Implementation** → Testing → PR creation
3. **Code Review** → @claude-code verification  
4. **Approval** → Merge → Auto-deployment
5. **Verification** → Issue closure

## 🛠️ Getting Started

New to the project? Start here:
1. [[Project Setup]] - Initial development environment
2. [[Local Testing]] - Running and testing locally
3. [[Contributing]] - How to contribute code
4. [[Code Standards]] - Coding guidelines and practices

---
*Last updated: {{date}}*
EOF

# Create Architecture Design page
cat > wiki-content/Architecture-Design.md << 'EOF'
# Architecture Design

## System Overview

The Sermon Uploader uses a **layered security architecture** that eliminates direct Pi exposure while providing public access and secure administration.

```mermaid
graph TB
    subgraph "Public Access Layer"
        Internet[Internet Users] --> CF[CloudFlare]
        CF --> Tunnel[CloudFlare Tunnel]
        Tunnel --> Pi[Raspberry Pi:8000]
    end
    
    subgraph "Admin Access Layer"  
        Admin[Admin Device] --> VPN[Teleport VPN]
        VPN --> UDM[UDM Pro]
        UDM --> PiSSH[Pi:22 SSH]
        UDM --> PiMinio[Pi:9001 MinIO]
    end
    
    subgraph "Deployment Layer"
        GHA[GitHub Actions] --> Webhook[Webhook Endpoint]
        Webhook --> Deploy[Local Deployment]
        Deploy --> Docker[Docker Containers]
    end
```

## Components

### 1. Application Stack
- **Backend**: Go with Fiber framework
- **Frontend**: React/Next.js with shadcn/ui
- **Storage**: MinIO (embedded in single container)
- **Database**: File-based storage

### 2. Infrastructure
- **Host**: Raspberry Pi (ARM64)
- **Container**: Docker single-container architecture  
- **Networking**: Unifi Dream Machine Pro
- **DNS**: CloudFlare managed

### 3. Security Layers
- **Layer 1**: CloudFlare Tunnel (zero Pi exposure)
- **Layer 2**: Teleport VPN (admin access only)
- **Layer 3**: HMAC webhook authentication
- **Layer 4**: Container isolation and non-root execution

## Data Flow

### Public Access Flow:
1. User accesses `https://yourdomain.com`
2. CloudFlare handles SSL termination and DDoS protection
3. Tunnel routes to Pi port 8000 (outbound only)
4. Application serves content securely

### Admin Access Flow:
1. Admin connects via Teleport VPN
2. UDM Pro authenticates and routes traffic
3. SSH/MinIO accessible only through VPN tunnel
4. No external ports exposed on Pi

### Deployment Flow:
1. Code pushed to GitHub main branch
2. GitHub Actions builds and pushes Docker image
3. Webhook triggered through CloudFlare tunnel
4. Pi pulls new image and restarts containers
5. Health checks verify deployment success

## Security Benefits

✅ **Zero Direct Exposure** - Pi never directly accessible from internet
✅ **DDoS Protection** - CloudFlare absorbs attacks  
✅ **SSL/TLS Everywhere** - End-to-end encryption
✅ **VPN-Only Admin** - Administrative functions require VPN
✅ **Authenticated Webhooks** - HMAC signature validation
✅ **Container Isolation** - Application runs in isolated environment
✅ **Audit Trail** - All deployments and access logged

## Performance Characteristics

- **Response Time**: <2 seconds via CloudFlare CDN
- **Deployment Time**: <5 minutes end-to-end
- **Uptime Target**: 99.5% (excluding Pi maintenance)
- **VPN Connection**: <10 seconds establishment time

---
*Related: [[Security Model]], [[Deployment Guide]]*
EOF

# Create Security Model page
cat > wiki-content/Security-Model.md << 'EOF'
# Security Model

## Zero-Trust Architecture

The Sermon Uploader implements a **zero-trust security model** where no component trusts any other by default.

## Threat Model

### Threats Mitigated:
- ✅ **Direct Pi attacks** - No exposed ports or services
- ✅ **DDoS attacks** - CloudFlare protection layer
- ✅ **SSH brute force** - No external SSH access
- ✅ **Man-in-the-middle** - End-to-end TLS encryption
- ✅ **Unauthorized deployment** - HMAC webhook authentication
- ✅ **Privilege escalation** - Non-root container execution

### Residual Risks:
- ⚠️ **VPN compromise** - Mitigated by limited VPN access
- ⚠️ **CloudFlare dependency** - Mitigated by fallback procedures
- ⚠️ **Physical Pi access** - Mitigated by physical security

## Security Controls

### Network Security:
- **Ingress**: CloudFlare Tunnel only (outbound connection from Pi)
- **Admin Access**: Teleport VPN required for SSH/MinIO
- **Firewall**: All external ports blocked except VPN
- **Monitoring**: Connection logging and anomaly detection

### Application Security:
- **Authentication**: Multi-layer (VPN + application + webhook)
- **Authorization**: Role-based access control
- **Encryption**: TLS 1.3 for all communications
- **Input Validation**: Sanitized file uploads and form data

### Infrastructure Security:
- **Container**: Non-root execution, minimal base image
- **Secrets**: Environment variable injection, no hardcoded secrets
- **Updates**: Automated security patching via CI/CD
- **Backup**: Encrypted, versioned backups

## Compliance

### Standards Alignment:
- **NIST Cybersecurity Framework** - Identify, Protect, Detect, Respond, Recover
- **Zero Trust Principles** - Never trust, always verify
- **Defense in Depth** - Multiple security layers

### Audit Requirements:
- **Access Logs** - All VPN and application access logged
- **Deployment Logs** - Complete CI/CD audit trail  
- **Change Management** - All changes via PR process
- **Incident Response** - Documented procedures and contacts

## Security Procedures

### Regular Tasks:
- **Weekly**: Security patch review and application
- **Monthly**: Access review and cleanup
- **Quarterly**: Penetration testing and vulnerability assessment
- **Annually**: Security architecture review

### Incident Response:
1. **Detect** - Automated monitoring alerts
2. **Contain** - VPN disconnect and traffic blocking
3. **Investigate** - Log analysis and impact assessment  
4. **Recover** - System restoration and hardening
5. **Learn** - Post-incident review and improvements

---
*Related: [[Architecture Design]], [[Incident Response]]*
EOF

echo "📝 Creating remaining wiki pages..."

# Create quick deployment guide
cat > wiki-content/Deployment-Guide.md << 'EOF'
# Deployment Guide

Quick reference for deploying the Sermon Uploader with secure architecture.

## Prerequisites

- Raspberry Pi with Docker installed
- Unifi Dream Machine Pro
- CloudFlare account with domain
- GitHub repository access

## Phase 1: CloudFlare Tunnel (30 minutes)

### 1. Install cloudflared
```bash
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.deb -o cloudflared.deb
sudo dpkg -i cloudflared.deb
```

### 2. Authenticate and Create Tunnel
```bash
cloudflared tunnel login
cloudflared tunnel create sermon-uploader
cloudflared tunnel route dns sermon-uploader yourdomain.com
```

### 3. Configure Service
```bash
sudo mkdir -p /etc/cloudflared
sudo tee /etc/cloudflared/config.yml << EOF
tunnel: sermon-uploader
credentials-file: /home/pi/.cloudflared/$(cloudflared tunnel list | grep sermon-uploader | awk '{print $1}').json
ingress:
  - hostname: yourdomain.com
    service: http://localhost:8000
  - service: http_status:404
EOF

sudo cloudflared service install
sudo systemctl start cloudflared
sudo systemctl enable cloudflared
```

## Phase 2: Unifi Teleport VPN (20 minutes)

### 1. Enable Teleport
- Go to UniFi Console → Settings → Teleport & VPN → Teleport
- Toggle on "Enable Teleport VPN"

### 2. Create Device Invitations
- Click "Create invite" for each admin device
- Install WiFiman app and connect using invitation links

### 3. Test VPN Access
```bash
# After connecting via VPN:
ssh pi@192.168.1.127
# MinIO admin: http://192.168.1.127:9001
```

## Phase 3: Webhook Deployment (45 minutes)

### 1. Install Webhook Server
```bash
wget https://github.com/adnanh/webhook/releases/latest/download/webhook-linux-arm64.tar.gz
tar -xzf webhook-linux-arm64.tar.gz
sudo mv webhook /usr/local/bin/
```

### 2. Create Deployment Script
```bash
sudo tee /opt/deploy.sh << 'EOF'
#!/bin/bash
set -e
cd /opt/sermon-uploader
git pull origin main
docker compose -f docker-compose.single.yml pull
docker compose -f docker-compose.single.yml up -d --force-recreate
EOF
sudo chmod +x /opt/deploy.sh
```

### 3. Configure Webhook Security
```bash
WEBHOOK_SECRET=$(uuidgen)
echo "Add this to GitHub Secrets: $WEBHOOK_SECRET"

sudo tee /etc/webhook.json << EOF
[{
  "id": "deploy-sermon-uploader",
  "execute-command": "/opt/deploy.sh",
  "command-working-directory": "/opt/sermon-uploader",
  "trigger-rule": {
    "match": {
      "type": "payload-hmac-sha256",
      "secret": "$WEBHOOK_SECRET",
      "parameter": {
        "source": "header",
        "name": "X-Hub-Signature-256"
      }
    }
  }
}]
EOF
```

### 4. Start Webhook Service
```bash
sudo tee /etc/systemd/system/webhook.service << EOF
[Unit]
Description=Webhook Server
After=network.target

[Service]
Type=simple
User=pi
ExecStart=/usr/local/bin/webhook -hooks /etc/webhook.json -verbose -port 9000
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable webhook
sudo systemctl start webhook
```

## Phase 4: Update GitHub Actions

Replace SSH deployment with webhook in `.github/workflows/deploy.yml`:

```yaml
- name: Trigger Pi Deployment via Webhook
  run: |
    SIGNATURE=$(echo -n '${{ github.event.after }}' | openssl dgst -sha256 -hmac '${{ secrets.WEBHOOK_SECRET }}' | sed 's/(stdin)= //')
    curl -X POST \
      -H "Content-Type: application/json" \
      -H "X-Hub-Signature-256: sha256=$SIGNATURE" \
      "https://yourdomain.com/hooks/deploy-sermon-uploader" \
      -d '{"ref": "${{ github.ref }}", "after": "${{ github.event.after }}"}'
```

## Verification

### Test Complete Flow:
1. Push code to GitHub main branch
2. Verify GitHub Actions completes without SSH errors
3. Check webhook logs: `sudo journalctl -u webhook -n 20`
4. Verify containers updated: `docker ps`
5. Test public access: Visit `https://yourdomain.com`
6. Test admin access: Connect via VPN and SSH

---
*For troubleshooting, see [[Troubleshooting]] page*
EOF

echo "🔗 Setting up wiki repository..."

# Initialize wiki as git repository
cd wiki-content
git init
git remote add origin https://github.com/White-Plains-Gospel-Chapel/sermon-uploader.wiki.git 2>/dev/null || true

echo "📤 Wiki content created in wiki-content/ directory"
echo ""
echo "🌐 MANUAL WIKI SETUP:"
echo "1. Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/wiki"
echo "2. Click 'Create the first page' or 'New Page'"
echo "3. Copy content from wiki-content/ files to create pages"
echo ""
echo "Or clone and push the wiki repository:"
echo "  cd wiki-content"
echo "  git add ."
echo "  git commit -m 'Initial wiki documentation'"
echo "  git push origin master"
echo ""
echo "✅ Wiki setup complete!"