# 🔐 Complete Secret Management Setup Guide

## ✅ **Answer: GitHub Secrets IS Perfect for Your Use Case**

**Why GitHub Secrets works well:**
- ✅ **Automatic injection** during CI/CD
- ✅ **Encrypted storage** (AES-256)
- ✅ **Simple setup** - no additional infrastructure
- ✅ **Perfect for single Pi deployment**
- ✅ **Integrated with GitHub Actions**

**Automatic Access Strategy:**
1. **GitHub Secrets** → Store all secrets securely
2. **CI/CD Pipeline** → Automatically injects secrets during deployment
3. **Pi Environment** → Receives secrets as environment variables
4. **Docker Compose** → Uses environment variables automatically

## 🔧 Complete Setup Process

### Step 1: Generate New Discord Webhook (Replace Exposed One)

```bash
# Your old webhook was exposed, generate a new one:
# 1. Go to Discord Server Settings → Integrations → Webhooks
# 2. Create New Webhook for #sermon-uploads channel
# 3. Copy the new webhook URL
```

### Step 2: GitHub Repository Secrets Configuration

Go to **GitHub.com → Your Repo → Settings → Secrets and variables → Actions**

Add these **Repository Secrets:**

| Secret Name | Value | Example |
|-------------|-------|---------|
| `PI_HOST` | Your Pi's IP address | `192.168.1.100` |
| `PI_USER` | Pi username | `pi` |
| `PI_SSH_KEY` | Private SSH key (see below) | `-----BEGIN RSA PRIVATE KEY-----...` |
| `PI_PORT` | SSH port (optional) | `22` |
| `MINIO_ENDPOINT` | MinIO endpoint | `localhost:9000` |
| `MINIO_ACCESS_KEY` | Strong MinIO username | `sermon_uploader_2024` |
| `MINIO_SECRET_KEY` | Strong MinIO password | `SuperSecurePassword123!` |
| `MINIO_SECURE` | Use HTTPS | `false` |
| `MINIO_BUCKET` | Bucket name | `sermons` |
| `DISCORD_WEBHOOK_URL` | NEW Discord webhook | `https://discord.com/api/webhooks/NEW_ID/NEW_TOKEN` |
| `WAV_SUFFIX` | WAV file suffix | `_raw` |
| `AAC_SUFFIX` | AAC file suffix | `_streamable` |
| `BATCH_THRESHOLD` | Batch size | `2` |
| `PORT` | Application port | `8000` |

### Step 3: SSH Key Generation & Setup

```bash
# On your local machine (Mac)
ssh-keygen -t ed25519 -f ~/.ssh/pi-sermon-deploy -C "github-actions-deploy"

# Copy public key to Pi
ssh-copy-id -i ~/.ssh/pi-sermon-deploy.pub pi@YOUR_PI_IP

# Test connection
ssh -i ~/.ssh/pi-sermon-deploy pi@YOUR_PI_IP

# Get private key content for GitHub secret
cat ~/.ssh/pi-sermon-deploy
# Copy the ENTIRE output (including -----BEGIN and -----END lines) to PI_SSH_KEY secret
```

### Step 4: Automatic Secret Injection Flow

```mermaid
graph TD
    A[Push to main] --> B[GitHub Actions Triggered]
    B --> C[Security Scan]
    C --> D[Build Container Image]
    D --> E[SSH into Pi]
    E --> F[Create .env with GitHub Secrets]
    F --> G[Deploy Updated Container]
    G --> H[Health Check]
    H --> I[Discord Notification]
```

**How it works:**
1. **GitHub Actions** reads secrets from GitHub Secrets Manager
2. **SSH connection** established using PI_SSH_KEY
3. **Environment file** created on Pi with all secrets
4. **Docker Compose** reads .env file automatically
5. **Application** gets secrets as environment variables

### Step 5: Initial Pi Setup

```bash
# SSH into your Pi
ssh pi@YOUR_PI_IP

# Install Docker (if not installed)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
newgrp docker

# Create project directory
sudo mkdir -p /opt/sermon-uploader
sudo chown $USER:$USER /opt/sermon-uploader
cd /opt/sermon-uploader

# Clone repository (replace with your actual repo)
git clone https://github.com/yourusername/sermon-uploader.git .

# Create initial .env (will be overwritten by CI/CD)
cp .env.example .env
nano .env  # Fill in your actual secrets temporarily

# Start services for the first time
docker compose -f docker-compose.prod.yml up -d

# Check status
docker compose -f docker-compose.prod.yml ps
```

## 🔄 How Automatic Secret Management Works

### During Deployment:
1. **GitHub Actions** triggers on push to main
2. **Security scan** ensures no secrets in code
3. **Build phase** creates multi-architecture container
4. **Deploy phase** SSH into Pi and:
   ```bash
   # This happens automatically:
   cat > .env << EOL
   MINIO_ACCESS_KEY=actual_secret_from_github
   MINIO_SECRET_KEY=actual_secret_from_github
   DISCORD_WEBHOOK_URL=actual_webhook_from_github
   # ... all other secrets
   EOL
   chmod 600 .env  # Secure permissions
   ```
5. **Container restart** picks up new environment variables
6. **Health check** validates deployment
7. **Discord notification** confirms success

### Security Benefits:
- ✅ **Secrets never in code**
- ✅ **Encrypted in transit** (SSH)
- ✅ **Encrypted at rest** (GitHub AES-256)
- ✅ **Minimal exposure** (.env file only on Pi)
- ✅ **Automatic rotation** (just update GitHub secret)

## 🧪 Testing Secret Management

### Test GitHub Actions Locally:
```bash
# Install act (GitHub Actions local runner)
brew install act

# Test the workflow
act -s GITHUB_TOKEN=your_token push
```

### Test Secret Injection:
```bash
# On Pi, check if secrets are properly injected
cd /opt/sermon-uploader
cat .env  # Should show actual values, not placeholders

# Test container can read secrets
docker compose -f docker-compose.prod.yml exec sermon-uploader env | grep MINIO
```

### Test Discord Integration:
```bash
# Test Discord webhook
curl -X POST "$DISCORD_WEBHOOK_URL" -H "Content-Type: application/json" -d '{"content": "Test message from secret setup"}'
```

## 🔍 Alternative Secret Managers (For Future)

If you ever outgrow GitHub Secrets:

### **HashiCorp Vault** (Enterprise-grade)
- ✅ Dynamic secret rotation
- ✅ Fine-grained access control  
- ✅ Audit logging
- ❌ Complex setup
- ❌ Additional infrastructure

### **AWS Secrets Manager** (Cloud-based)
- ✅ Automatic rotation
- ✅ Integration with AWS services
- ✅ Cross-service sharing
- ❌ Monthly costs
- ❌ Requires AWS account

### **Azure Key Vault** (Microsoft)
- ✅ Hardware security modules
- ✅ Certificate management
- ✅ RBAC integration
- ❌ Azure dependency
- ❌ Additional complexity

### **Recommendation: Stick with GitHub Secrets**
For a single Pi deployment, GitHub Secrets provides:
- Perfect security level
- Zero additional cost
- Simple management
- Automatic integration

## 🚨 Security Best Practices

1. **Rotate secrets regularly**
   - Discord webhook: Every 6 months
   - MinIO credentials: Every 3 months
   - SSH keys: Annually

2. **Monitor access**
   - Check GitHub Actions logs
   - Monitor Pi logs for unusual activity
   - Set up Discord alerts for deployments

3. **Backup strategy**
   - Export GitHub secrets list (not values)
   - Document secret rotation dates
   - Keep encrypted backup of SSH keys

4. **Incident response**
   - If secrets compromised: Immediately rotate in GitHub
   - Next deployment will auto-update Pi
   - No manual Pi access needed