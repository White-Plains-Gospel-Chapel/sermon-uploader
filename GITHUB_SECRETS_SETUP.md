# GitHub Secrets Setup for Automated Docker Deployment

This guide explains how to configure GitHub Actions to automatically build and deploy Docker images.

## Required GitHub Secrets

Go to your repository settings: 
[https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/secrets/actions](https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/secrets/actions)

Add the following secrets:

### 1. Docker Hub Credentials
- **DOCKER_USERNAME**: Your Docker Hub username (e.g., `wpgcparish`)
- **DOCKER_PASSWORD**: Your Docker Hub password or access token

To create a Docker Hub access token:
1. Log in to [Docker Hub](https://hub.docker.com)
2. Go to Account Settings → Security
3. Click "New Access Token"
4. Give it a name like "GitHub Actions"
5. Copy the token and use it as DOCKER_PASSWORD

### 2. Discord Notification (Optional)
- **DISCORD_WEBHOOK_URL**: Your Discord webhook URL for notifications
  - Already configured in your workflow

### 3. Pi Deployment Webhook (For Automated Deployment)
- **PI_WEBHOOK_URL**: The URL to trigger deployment on your Pi
  - Format: `http://YOUR_PI_PUBLIC_IP:9001/webhook`
  - Requires port forwarding on your router (port 9001)
  - Or use a service like ngrok for secure tunneling

- **WEBHOOK_SECRET**: A secure secret for webhook authentication
  - Generate one: `openssl rand -hex 32`
  - Use the same secret in both GitHub and on your Pi

## Setup Steps

### Step 1: Configure Docker Hub
```bash
# Create a Docker Hub account if you don't have one
# Go to https://hub.docker.com/signup

# Create repositories (optional, will be created automatically):
# - wpgcparish/sermon-uploader-backend
# - wpgcparish/sermon-uploader-frontend
```

### Step 2: Add GitHub Secrets
1. Go to repository Settings → Secrets and variables → Actions
2. Click "New repository secret"
3. Add each secret listed above

### Step 3: Setup Pi Webhook Listener (Optional for Auto-Deploy)
On your Raspberry Pi:
```bash
cd /opt/sermon-uploader
git pull origin master
chmod +x setup-webhook.sh
./setup-webhook.sh

# Configure the webhook secret
sudo nano /etc/systemd/system/webhook-listener.service
# Update WEBHOOK_SECRET with your generated secret

# Start the service
sudo systemctl start webhook-listener
sudo systemctl status webhook-listener
```

### Step 4: Configure Port Forwarding (For Webhook)
1. Access your router admin panel (usually 192.168.1.1)
2. Find Port Forwarding settings
3. Add a rule:
   - External Port: 9001
   - Internal IP: 192.168.1.127 (your Pi's IP)
   - Internal Port: 9001
   - Protocol: TCP

Alternative: Use ngrok for secure tunneling:
```bash
# On Pi:
wget https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-arm64.tgz
tar xvf ngrok-v3-stable-linux-arm64.tgz
./ngrok http 9001
# Use the provided HTTPS URL as PI_WEBHOOK_URL in GitHub
```

## Testing the Workflow

### Manual Trigger
1. Go to Actions tab in your repository
2. Select "Build and Deploy Docker Images"
3. Click "Run workflow"
4. Select branch and click "Run workflow"

### Automatic Trigger
Push any commit to the master branch:
```bash
git commit -m "Test deployment"
git push origin master
```

## Monitoring

### Check GitHub Actions
- Go to the Actions tab to see workflow runs
- Click on a run to see detailed logs

### Check Docker Hub
- Visit https://hub.docker.com/u/wpgcparish
- You should see your images with recent tags

### Check Pi Deployment
```bash
# On Pi:
docker ps  # Check running containers
docker compose -f docker-compose.pi5.yml logs -f  # View logs
sudo journalctl -u webhook-listener -f  # View webhook logs
```

## Troubleshooting

### Images not pushing to Docker Hub
- Verify DOCKER_USERNAME and DOCKER_PASSWORD are correct
- Check if repositories exist on Docker Hub
- Review GitHub Actions logs for errors

### Pi not receiving webhooks
- Check if port 9001 is accessible from internet
- Verify PI_WEBHOOK_URL is correct in GitHub secrets
- Check webhook listener logs: `sudo journalctl -u webhook-listener -f`

### Deployment failing on Pi
- Ensure Docker is running: `sudo systemctl status docker`
- Check disk space: `df -h`
- Review deployment logs: `docker compose -f docker-compose.pi5.yml logs`