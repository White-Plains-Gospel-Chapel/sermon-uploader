# Self-Hosted GitHub Actions Runner Setup Guide

This guide will help you set up a self-hosted GitHub Actions runner on your Raspberry Pi for automated deployment with comprehensive monitoring.

## 📋 Prerequisites

- Raspberry Pi 4/5 with at least 4GB RAM
- Ubuntu Server or Raspberry Pi OS (64-bit recommended)
- Docker and Docker Compose installed
- SSH access to your Pi
- GitHub account with repository access

## 🚀 Quick Setup

### Step 1: Copy Scripts to Your Raspberry Pi

First, copy the setup scripts to your Pi:

```bash
# From your local machine
scp scripts/setup-github-runner.sh gaius@192.168.1.127:~/
scp scripts/runner-service.sh gaius@192.168.1.127:~/
scp scripts/monitor-deployment.sh gaius@192.168.1.127:~/
```

### Step 2: Run the Setup Script on Your Pi

SSH into your Raspberry Pi and run the setup:

```bash
ssh gaius@192.168.1.127

# Make scripts executable
chmod +x setup-github-runner.sh runner-service.sh monitor-deployment.sh

# Run the setup script
./setup-github-runner.sh
```

When prompted:
1. Go to: https://github.com/White-Plains-Gospel-Chapel/sermon-uploader/settings/actions/runners/new
2. Select **Linux** and **ARM64** (for Pi 4/5 with 64-bit OS)
3. Copy the registration token (starts with 'A' followed by many characters)
4. Paste it into the script when prompted

### Step 3: Verify Installation

After the script completes, verify the runner is working:

```bash
# Check runner status
./runner-service.sh status

# Check runner health
./runner-service.sh health

# View live logs
./runner-service.sh logs
```

### Step 4: Update GitHub Workflow

The repository now has a new workflow file `.github/workflows/self-hosted-deploy.yml` that:
- Runs tests on GitHub's cloud runners (for speed)
- Builds and pushes Docker images to Docker Hub
- Deploys to your Pi using the self-hosted runner
- Provides detailed monitoring at each stage

### Step 5: Set Up Monitoring (Optional)

To enable continuous monitoring of your deployment:

```bash
# Move monitoring script to appropriate location
sudo mv monitor-deployment.sh /usr/local/bin/
sudo chmod +x /usr/local/bin/monitor-deployment.sh

# Add to crontab for regular monitoring (every 5 minutes)
(crontab -l 2>/dev/null; echo "*/5 * * * * /usr/local/bin/monitor-deployment.sh --cron") | crontab -

# Or run as a daemon
/usr/local/bin/monitor-deployment.sh --daemon &
```

## 📊 Monitoring Features

The new workflow provides comprehensive monitoring:

### GitHub Actions UI
- **Detailed job status**: Each stage (test, build, deploy) is tracked separately
- **Step summaries**: Markdown summaries in the Actions UI showing:
  - Test results and coverage
  - Docker image tags
  - Deployment environment info
  - Container status
  - Access URLs

### Health Checks
- Automatic health checks after deployment
- Verifies backend API is responding
- Confirms frontend is accessible
- Shows running container status

### Notifications
- Discord webhook notifications (if configured)
- Detailed status for each job
- Direct links to workflow runs

## 🔧 Runner Management Commands

Use the `runner-service.sh` script for easy management:

```bash
# Service management
./runner-service.sh status    # Check runner status
./runner-service.sh start     # Start the runner
./runner-service.sh stop      # Stop the runner
./runner-service.sh restart   # Restart the runner

# Monitoring
./runner-service.sh logs      # View live logs
./runner-service.sh health    # Check runner health

# Maintenance
./runner-service.sh update    # Update runner to latest version
./runner-service.sh enable    # Enable auto-start at boot
./runner-service.sh disable   # Disable auto-start
```

## 🛠️ Troubleshooting

### Runner Not Connecting
1. Check network connectivity: `ping github.com`
2. Verify runner status: `./runner-service.sh status`
3. Check logs: `./runner-service.sh logs`

### Deployment Failing
1. Ensure Docker is running: `docker ps`
2. Check disk space: `df -h`
3. Verify Docker permissions: `docker run hello-world`

### High Resource Usage
1. Check memory: `free -m`
2. Check CPU: `top`
3. Clean up Docker: `docker system prune -a`

## 🔄 Workflow Stages

The self-hosted deployment workflow includes:

1. **Test Backend** (Cloud runner)
   - Sets up Go environment
   - Runs tests with coverage
   - Uploads coverage reports

2. **Test Frontend** (Cloud runner)
   - Sets up Node.js environment
   - Runs linting and tests
   - Builds production bundle

3. **Build Docker Images** (Cloud runner)
   - Builds multi-architecture images (ARM64 + AMD64)
   - Pushes to Docker Hub
   - Tags with branch and commit SHA

4. **Deploy to Pi** (Self-hosted runner)
   - Pulls latest images
   - Stops old containers
   - Starts new containers
   - Performs health checks
   - Cleans up old images

5. **Send Notifications** (Cloud runner)
   - Aggregates job statuses
   - Sends Discord webhook
   - Provides workflow links

## 🔐 Security Notes

- The runner runs as a non-root user
- Docker permissions are granted via group membership
- The runner only accepts jobs from your specific repository
- All secrets are stored encrypted in GitHub

## 📝 Next Steps

1. **Test the deployment**: Make a small change and push to master
2. **Monitor the workflow**: Watch it run in the Actions tab
3. **Check your Pi**: Verify services are running with `docker ps`
4. **Set up monitoring**: Configure the monitoring script for alerts

## 🆘 Support

If you encounter issues:
1. Check the runner logs: `./runner-service.sh logs`
2. Review the GitHub Actions run in the web UI
3. Check Docker logs: `docker logs sermon-uploader-backend`
4. Use the monitoring script: `./monitor-deployment.sh`

---

**Note**: Remember to keep your Docker Hub credentials up to date in GitHub Secrets:
- `DOCKER_USERNAME`: Your Docker Hub username
- `DOCKER_PASSWORD`: Your Docker Hub password or access token
- `DISCORD_WEBHOOK`: (Optional) Discord webhook URL for notifications