# Secure Pi Deployment Architecture - 2025

## Overview
This document outlines a secure deployment architecture for the Sermon Uploader application using:
- Unifi Dream Machine Pro with Teleport VPN
- CloudFlare Tunnel for public access
- Webhook-based deployment
- Zero direct Pi exposure to internet

## Current Status
- ✅ Local development and testing working
- ✅ Pre-deployment verification system implemented
- ❌ GitHub Actions failing due to network isolation (192.168.1.127 not accessible from internet)
- 🎯 **Goal**: Public web app with maximum security

## Architecture Layers

### Layer 1: Public Web Access (CloudFlare Tunnel)
```
Internet Users → CloudFlare → Tunnel → Pi:8000 (Web App)
```
- **Purpose**: Public access to sermon uploader website
- **Security**: Pi never directly exposed to internet
- **Benefits**: DDoS protection, SSL termination, global CDN

### Layer 2: Admin Access (Unifi Teleport VPN)
```
Admin Device → Teleport VPN → UDM Pro → Local Network → Pi:22 (SSH)
```
- **Purpose**: SSH access, MinIO admin, system maintenance
- **Security**: VPN-only access, no open ports
- **Benefits**: Uses existing UDM Pro infrastructure

### Layer 3: Deployment (Webhook-based)
```
GitHub Actions → CloudFlare Tunnel → Pi Webhook → Local Deployment
```
- **Purpose**: Automated deployment from GitHub
- **Security**: No SSH exposure, pull-based updates
- **Benefits**: Secure automation, audit trail

## 🔗 Implementation Resources (2025)

### 🔒 Unifi Teleport VPN Resources:
✅ **Primary Tutorial**: [UniFi Gateway - Teleport VPN (Official Ubiquiti)](https://help.ui.com/hc/en-us/articles/5246403561495-UniFi-Gateway-Teleport-VPN)
✅ **Step-by-Step Guide**: [UniFi Teleport - How to set up and use the one-click VPN (LazyAdmin)](https://lazyadmin.nl/home-network/unifi-teleport/)
✅ **Video Tutorial**: [How to Set Up Teleport VPN on UniFi (WunderTech)](https://www.wundertech.net/how-to-set-up-teleport-vpn-on-unifi/)
✅ **Comprehensive Guide**: [How to setup UniFi Teleport VPN (UniHosted)](https://www.unihosted.com/blog/unifi-teleport-secure-remote-access-setup)

**Key Requirements for 2025**:
- Dream Machine/UDM Pro running firmware 1.12.0+ (or Dream Router 2.4.0+)
- UniFi Network version 7.1 or later
- Remote access enabled in UniFi Console
- WiFiman app for client connections

### 🌐 CloudFlare Tunnel Resources:
✅ **Latest Pi Tutorial**: [Setting up a Cloudflare Tunnel on the Raspberry Pi (Pi My Life Up - Feb 2025)](https://pimylifeup.com/raspberry-pi-cloudflare-tunnel/)
✅ **ARM64 Specific**: [Securely Expose Your Raspberry Pi 5 with Cloudflare Tunnel (Ladvien - May 2025)](https://ladvien.com/cloud-flare-raspberry-pi-home-server/)
✅ **Comprehensive Setup**: [Setting Up Cloudflare Tunnel on Your Raspberry Pi (FleetStack)](https://fleetstack.io/blog/cloudflare-tunnel-raspberry-pi-setup)
✅ **Quick Setup Guide**: [How to setup cloudflare tunnels on a pi (GitHub Gist)](https://gist.github.com/Zoobdude/55be8e623eee3151feaecb0d07049061)

**ARM64 Installation Commands**:
```bash
# Download ARM64 version
curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.deb -o cloudflared.deb
sudo dpkg -i cloudflared.deb

# Alternative method
wget -O cloudflared https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64
sudo mv cloudflared /usr/local/bin
sudo chmod +x /usr/local/bin/cloudflared
```

### 🔐 Webhook Deployment Security Resources:
✅ **Security Architecture**: [My Automated Deployment System Using GitHub Webhook (Blog - 2025)](https://blog.mikihands.com/en/whitedec/2025/7/21/github-webhook-auto-deploy-architecture/)
✅ **Docker Integration**: [GitHub Webhooks, Docker, and Python for Automatic Deployments (Better Programming)](https://medium.com/better-programming/github-webhooks-docker-and-python-for-automatic-app-deployments-a7f18d23d5b7)
✅ **Lightweight Solution**: [webhook - lightweight incoming webhook server (GitHub)](https://github.com/adnanh/webhook)
✅ **Docker-specific**: [docker-hook - Automatic Docker Deployment via Webhooks](https://github.com/schickling/docker-hook)

**Security Best Practices**:
- Use X-Hub-Signature-256 header for request validation
- Implement reverse proxy (Nginx/Apache) with HTTPS
- Generate secure auth tokens with `uuidgen` or `openssl`
- Never expose webhook endpoints directly to internet
- Use Let's Encrypt certificates for HTTPS communication

## Step-by-Step Implementation Plan

### Phase 1: Unifi Teleport VPN Setup (20 minutes)
**Prerequisites**: UDM Pro with firmware 1.12.0+, UniFi Network 7.1+

**Steps**:
1. **Enable Remote Access**: 
   - Go to UniFi Console → Settings → Console Settings → Remote Access
   - Sign in with UI account and enable remote access
   
2. **Enable Teleport VPN**:
   - Navigate to Settings → Teleport & VPN → Teleport
   - Toggle on "Enable Teleport VPN"
   
3. **Generate Invitation Links**:
   - Click "Create invite" 
   - Copy the link (valid for 24 hours)
   - Each device needs its own invitation
   
4. **Install WiFiman App**:
   - Download from app store or UniFi download page
   - Available for mobile and desktop
   
5. **Connect Devices**:
   - Use invitation link or sign in with site-admin account
   - Test VPN connectivity

### Phase 2: CloudFlare Tunnel Setup (30 minutes)
**Prerequisites**: CloudFlare account, domain name, Pi with internet access

**Steps**:
1. **Install cloudflared on Pi**:
   ```bash
   curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.deb -o cloudflared.deb
   sudo dpkg -i cloudflared.deb
   ```

2. **Authenticate with CloudFlare**:
   ```bash
   cloudflared tunnel login
   ```
   - Opens browser to select domain

3. **Create Tunnel**:
   ```bash
   cloudflared tunnel create sermon-uploader
   cloudflared tunnel route dns sermon-uploader yourdomain.com
   ```

4. **Configure Service**:
   ```bash
   sudo cloudflared service install
   sudo systemctl start cloudflared
   sudo systemctl enable cloudflared
   ```

5. **Test Public Access**:
   - Visit your domain
   - Verify SSL certificate
   - Test from external network

### Phase 3: Webhook Deployment (45 minutes)
**Prerequisites**: CloudFlare tunnel operational, GitHub repository access

**Steps**:
1. **Create Webhook Endpoint** on Pi:
   ```bash
   # Install webhook server
   wget https://github.com/adnanh/webhook/releases/latest/download/webhook-linux-arm64.tar.gz
   tar -xzf webhook-linux-arm64.tar.gz
   sudo mv webhook /usr/local/bin/
   ```

2. **Configure Webhook Security**:
   - Generate secure token: `uuidgen`
   - Create webhook configuration file
   - Set up HTTPS with certificates

3. **Modify GitHub Actions Workflow**:
   - Replace SSH deployment with webhook POST
   - Add signature validation
   - Configure secrets in GitHub

4. **Test Deployment Flow**:
   - Push to trigger webhook
   - Verify container deployment
   - Check monitoring and logs

## Security Analysis

### Current Issues:
- ❌ GitHub Actions can't reach Pi (192.168.1.127)
- ❌ SSH would be exposed to internet (security risk)
- ❌ Manual deployment monitoring required

### Security Benefits After Implementation:
- ✅ **Zero Pi Exposure**: No direct internet access to Pi
- ✅ **VPN-Only Admin**: All admin access through encrypted VPN
- ✅ **Encrypted Public Access**: CloudFlare handles SSL/TLS
- ✅ **DDoS Protection**: CloudFlare shields against attacks
- ✅ **Audit Trail**: All deployments logged and monitored
- ✅ **Zero Port Forwarding**: No router configuration needed

## Network Configuration Changes

### Remove (Current Insecure Setup):
```
Internet → Router Port Forward → Pi:8000 (Public)
Internet → Router Port Forward → Pi:22 (SSH - RISKY)
```

### Add (Secure Setup):
```
Internet → CloudFlare → Tunnel → Pi:8000 (Public Access)
Admin → Teleport VPN → UDM Pro → Pi:22 (Admin Only)
GitHub → CloudFlare → Webhook → Pi (Deployment)
```

## Cost Analysis

### CloudFlare Tunnel:
- **Free Tier**: Unlimited bandwidth, basic DDoS protection
- **Pro Tier**: $20/month for advanced security features

### Unifi Teleport:
- **Included**: With UDM Pro (no additional cost)
- **Zero Licensing Fees**

### Total Monthly Cost:
- **Recommended Setup**: $0/month (free tiers)
- **Enhanced Security**: $20/month (CloudFlare Pro)

## Testing and Validation

### Security Testing Checklist:
- [ ] Pi not reachable via direct IP scan
- [ ] VPN required for SSH access
- [ ] Public website accessible globally
- [ ] HTTPS certificate valid and auto-renewing
- [ ] Webhook signature validation working
- [ ] DDoS protection active

### Performance Testing:
- [ ] Website response time < 2 seconds
- [ ] VPN connection time < 5 seconds  
- [ ] Deployment time < 3 minutes
- [ ] Uptime monitoring configured

### Emergency Access Plan:
- [ ] Physical Pi access procedure
- [ ] Backup admin account configured
- [ ] Emergency DNS failover ready
- [ ] Rollback procedure documented

## Success Metrics

### Security Goals:
- Zero failed external SSH attempts (no exposure)
- All admin access authenticated via VPN
- 100% HTTPS traffic with valid certificates
- Automated security updates and monitoring

### Performance Goals:
- Website uptime > 99.9%
- Global response time < 3 seconds
- VPN connection reliability > 99.5%
- Deployment success rate > 95%

### Operational Goals:
- Zero manual deployment interventions
- Automated monitoring and alerting
- Self-healing deployment pipeline
- Complete audit trail for all changes

---
*Document created: January 2025*
*Version: 2.0*
*Status: Ready for Implementation*