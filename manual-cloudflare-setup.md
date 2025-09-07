# Manual CloudFlare DNS Setup Instructions

Since we need your CloudFlare credentials, please follow these steps manually:

## Step 1: Get Your CloudFlare API Token
1. Go to https://dash.cloudflare.com/profile/api-tokens
2. Click "Create Token"
3. Use "Custom token" template
4. Permissions: `Zone:Zone:Read`, `Zone:DNS:Edit`
5. Zone Resources: `Include - All zones` (or your specific domain)
6. Copy the generated token

## Step 2: Identify Your Domain
What is your domain? (e.g., wpgcservices.com, yourdomain.com)

## Step 3: Get Your Pi's Public IP
Run this to find your public IP:
```bash
curl -s ifconfig.me
```

## Step 4: Manual DNS Configuration
In your CloudFlare dashboard:

1. **For the main app** (keep this as is if already exists):
   - Type: A
   - Name: sermon-uploader
   - Content: [Your Pi's Public IP]
   - Proxy: 🟠 ON (Proxied)

2. **For MinIO** (create this new record):
   - Type: A  
   - Name: minio
   - Content: [Your Pi's Public IP]
   - Proxy: ⚪ OFF (DNS Only) ← **CRITICAL: Must be DNS Only**

The key is that the MinIO subdomain must have the proxy disabled (grey cloud) to bypass CloudFlare's upload limits.