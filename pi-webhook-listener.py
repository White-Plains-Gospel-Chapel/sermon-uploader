#!/usr/bin/env python3

"""
Webhook listener for automated Docker deployment on Raspberry Pi
Receives deployment triggers from GitHub Actions
"""

import json
import subprocess
import hmac
import hashlib
from flask import Flask, request, jsonify
import os
import logging
from datetime import datetime

app = Flask(__name__)

# Configuration
WEBHOOK_SECRET = os.getenv('WEBHOOK_SECRET', 'your-webhook-secret-here')
PROJECT_DIR = '/opt/sermon-uploader'
COMPOSE_FILE = 'docker-compose.pi5.yml'
LOG_FILE = '/var/log/sermon-webhook.log'

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(LOG_FILE, mode='a'),
        logging.StreamHandler()
    ]
)

def verify_webhook_signature(payload, signature):
    """Verify the webhook signature for security"""
    if not signature:
        return False
    
    expected = hmac.new(
        WEBHOOK_SECRET.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(f"sha256={expected}", signature)

def deploy():
    """Execute the deployment process"""
    try:
        logging.info("Starting deployment...")
        
        # Change to project directory
        os.chdir(PROJECT_DIR)
        
        # Pull latest code
        logging.info("Pulling latest code from GitHub...")
        subprocess.run(['git', 'pull', 'origin', 'master'], check=True)
        
        # Pull latest Docker images
        logging.info("Pulling latest Docker images...")
        subprocess.run([
            'docker', 'compose', '-f', COMPOSE_FILE, 'pull'
        ], check=True)
        
        # Stop existing containers
        logging.info("Stopping existing containers...")
        subprocess.run([
            'docker', 'compose', '-f', COMPOSE_FILE, 'down'
        ], check=True)
        
        # Start new containers
        logging.info("Starting new containers...")
        subprocess.run([
            'docker', 'compose', '-f', COMPOSE_FILE, 'up', '-d'
        ], check=True)
        
        # Clean up old images
        logging.info("Cleaning up old Docker images...")
        subprocess.run(['docker', 'image', 'prune', '-f'], check=True)
        
        logging.info("Deployment completed successfully!")
        return True, "Deployment successful"
        
    except subprocess.CalledProcessError as e:
        error_msg = f"Deployment failed: {str(e)}"
        logging.error(error_msg)
        return False, error_msg
    except Exception as e:
        error_msg = f"Unexpected error: {str(e)}"
        logging.error(error_msg)
        return False, error_msg

@app.route('/webhook', methods=['POST'])
def webhook():
    """Handle incoming webhook requests"""
    
    # Verify signature if secret is configured
    if WEBHOOK_SECRET != 'your-webhook-secret-here':
        signature = request.headers.get('X-Hub-Signature-256')
        if not verify_webhook_signature(request.data, signature):
            logging.warning("Invalid webhook signature")
            return jsonify({'error': 'Invalid signature'}), 401
    
    # Parse payload
    try:
        payload = request.json
    except:
        return jsonify({'error': 'Invalid JSON'}), 400
    
    # Log the deployment request
    logging.info(f"Received deployment request: {payload.get('action', 'unknown')}")
    
    # Trigger deployment
    if payload.get('action') == 'deploy':
        success, message = deploy()
        
        # Send Discord notification if webhook URL is configured
        discord_webhook = os.getenv('DISCORD_WEBHOOK_URL')
        if discord_webhook:
            notify_discord(discord_webhook, success, message, payload)
        
        if success:
            return jsonify({'status': 'success', 'message': message}), 200
        else:
            return jsonify({'status': 'error', 'message': message}), 500
    
    return jsonify({'status': 'ignored', 'message': 'No action taken'}), 200

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        'status': 'healthy',
        'service': 'pi-webhook-listener',
        'timestamp': datetime.utcnow().isoformat()
    }), 200

def notify_discord(webhook_url, success, message, payload):
    """Send notification to Discord"""
    try:
        import requests
        
        color = 3066993 if success else 15158332
        title = "✅ Deployment Successful" if success else "❌ Deployment Failed"
        
        embed = {
            "title": title,
            "description": message,
            "color": color,
            "fields": [
                {
                    "name": "Repository",
                    "value": payload.get('repository', 'Unknown'),
                    "inline": True
                },
                {
                    "name": "Commit",
                    "value": f"`{payload.get('sha', 'Unknown')[:7]}`",
                    "inline": True
                }
            ],
            "timestamp": datetime.utcnow().isoformat()
        }
        
        requests.post(webhook_url, json={"embeds": [embed]})
    except Exception as e:
        logging.error(f"Failed to send Discord notification: {e}")

if __name__ == '__main__':
    # Create log file if it doesn't exist
    os.makedirs(os.path.dirname(LOG_FILE), exist_ok=True)
    
    # Run the webhook listener
    app.run(host='0.0.0.0', port=9001)