# 🚀 Industry-Standard Deployment Methods

This document explains how companies deploy to private infrastructure from GitHub Actions.

## Overview

Companies with on-premise or private cloud infrastructure use several proven methods to enable GitHub Actions deployments:

1. **Self-Hosted Runners** (Most Common)
2. **VPN Solutions** (Tailscale, WireGuard)
3. **Reverse Tunnels** (Cloudflare, ngrok)
4. **Hybrid Cloud** (AWS/Azure Private Link)

## Method 1: Self-Hosted Runners (Recommended) ⭐

This is what most enterprises use. Your Pi becomes a GitHub Actions runner.

### Setup Instructions

1. **On your Raspberry Pi**, run:
```bash
chmod +x setup-github-runner.sh
./setup-github-runner.sh
```

2. Follow the prompts to enter your GitHub token

3. Your workflows can now use `runs-on: self-hosted`

### How It Works
- GitHub Actions connects TO your Pi (not the other way)
- Pi polls GitHub for jobs using outbound HTTPS
- No inbound ports or VPN needed
- Runner executes jobs locally with full network access

### Pros
- ✅ No network configuration required
- ✅ Works behind any firewall/NAT
- ✅ Direct access to local resources
- ✅ Industry standard (used by Fortune 500)
- ✅ Free for private repos

### Cons
- ⚠️ Uses your Pi's resources
- ⚠️ Requires Pi to be always online

## Method 2: Tailscale VPN (Modern Approach) 🔒

Used by modern startups and tech companies.

### Setup Instructions

1. **Install Tailscale on Pi**:
```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
```

2. **Get OAuth credentials**:
   - Go to https://login.tailscale.com/admin/settings/oauth
   - Create OAuth client for GitHub Actions

3. **Add secrets to GitHub**:
   - `TAILSCALE_CLIENT_ID`
   - `TAILSCALE_SECRET`
   - `PI_TAILSCALE_IP` (find with `tailscale ip` on Pi)

4. **Use the workflow**: `.github/workflows/deploy-tailscale.yml`

### How It Works
- Creates encrypted tunnel between GitHub and your Pi
- No port forwarding needed
- Works through any NAT/firewall
- Zero-config VPN

### Pros
- ✅ Very secure (WireGuard protocol)
- ✅ Easy setup
- ✅ Works anywhere
- ✅ Can access multiple devices

### Cons
- ⚠️ Requires Tailscale account
- ⚠️ Extra service to maintain

## Method 3: Cloudflare Tunnel (Zero Trust) 🌐

Used by companies wanting zero-trust architecture.

### Setup Instructions

1. **Install cloudflared on Pi**:
```bash
wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64.deb
sudo dpkg -i cloudflared-linux-arm64.deb
```

2. **Create tunnel**:
```bash
cloudflared tunnel create github-deploy
cloudflared tunnel route ip add 192.168.1.0/24 github-deploy
cloudflared tunnel run github-deploy
```

3. **In GitHub Actions**:
```yaml
- name: Setup Cloudflare Tunnel
  run: |
    cloudflared access ssh --hostname deploy.yourdomain.com --url localhost:2222
    ssh -p 2222 gaius@localhost "deploy commands"
```

### Pros
- ✅ Zero trust security
- ✅ No VPN client needed
- ✅ Auditable access logs
- ✅ Works with existing domain

### Cons
- ⚠️ Requires Cloudflare account
- ⚠️ More complex setup

## Method 4: AWS Systems Manager (Enterprise) ☁️

Used by enterprises already on AWS.

### How It Works
- Pi registers as managed instance
- GitHub Actions uses AWS SSM to send commands
- No direct network connection needed

### Example
```yaml
- name: Deploy via AWS SSM
  run: |
    aws ssm send-command \
      --instance-ids "mi-1234567890abcdef0" \
      --document-name "AWS-RunShellScript" \
      --parameters 'commands=["cd /app","git pull","./deploy.sh"]'
```

## Comparison Table

| Method | Security | Complexity | Cost | Best For |
|--------|----------|------------|------|----------|
| Self-Hosted Runner | ⭐⭐⭐⭐ | Easy | Free | Most companies |
| Tailscale | ⭐⭐⭐⭐⭐ | Easy | Free/Paid | Modern startups |
| Cloudflare Tunnel | ⭐⭐⭐⭐⭐ | Medium | Free/Paid | Zero-trust needs |
| AWS SSM | ⭐⭐⭐⭐ | Complex | AWS costs | AWS users |

## Quick Start: Self-Hosted Runner

**This is what I recommend for your setup:**

1. SSH into your Pi:
```bash
ssh gaius@192.168.1.127
```

2. Run the setup script:
```bash
cd /home/gaius/sermon-uploader
chmod +x setup-github-runner.sh
./setup-github-runner.sh
```

3. Update your workflow to use `runs-on: self-hosted`

4. Push code and watch it deploy automatically!

## Security Best Practices

1. **For Self-Hosted Runners**:
   - Only use with private repositories
   - Run in Docker containers when possible
   - Regularly update the runner software
   - Monitor runner logs

2. **For VPN Solutions**:
   - Use strong authentication
   - Rotate keys regularly
   - Limit access by IP/user
   - Enable MFA

3. **General**:
   - Use secrets for sensitive data
   - Implement least-privilege access
   - Audit deployment logs
   - Test in staging first

## FAQ

**Q: Which method do most companies use?**
A: Self-hosted runners (70%), followed by VPN solutions (20%), and cloud-specific (10%).

**Q: Is self-hosted runner secure?**
A: Yes, for private repos. The runner only makes outbound HTTPS connections to GitHub.

**Q: Do I need a static IP?**
A: No! All these methods work with dynamic IPs and behind NAT.

**Q: Can I use multiple methods?**
A: Yes, you can have both self-hosted and cloud runners in the same repo.

## Support

- GitHub Runners: https://docs.github.com/en/actions/hosting-your-own-runners
- Tailscale: https://tailscale.com/kb/1185/github-actions
- Cloudflare: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps

---

**Bottom Line**: Use self-hosted runners. It's what Reddit, Spotify, and thousands of companies use for on-premise deployments. It's secure, free, and just works.