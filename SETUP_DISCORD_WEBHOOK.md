# Setting Up Discord Webhook for Testing

## Quick Setup Guide

### 1. Get Your Discord Webhook URL

1. **Open Discord** and go to your server
2. Click the **gear icon** next to a channel (or create a test channel)
3. Go to **Integrations** → **Webhooks**
4. Click **New Webhook** or **Create Webhook**
5. Give it a name like "Sermon Uploader Test"
6. Click **Copy Webhook URL**

### 2. Add the Webhook URL to Your Environment

Option A - Update backend/.env:
```bash
# Edit the file
nano backend/.env

# Replace the line with your actual webhook URL:
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN
```

Option B - Set temporarily for testing:
```bash
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN"
```

### 3. Run the Test

```bash
# Test with the simple script (uses standard library only)
python3 test_discord_simple.py
```

## What You Should See

1. **In Terminal:**
   - Message creation confirmation
   - Progress updates (20%, 40%, 60%, 80%, 100%)
   - Completion message

2. **In Discord:**
   - A single message appears (not multiple)
   - The message updates every 2 seconds
   - Progress bar fills from empty to full
   - Color changes from orange to green when complete
   - Shows file counts, elapsed time, and status

## Example Discord Message Evolution

**Initial State:**
```
🧪 Discord Live Update Test
Testing live message updates...

Status
Initializing test...

Progress
░░░░░░░░░░ 0%
```

**During Updates:**
```
🧪 Discord Live Update Test
Testing live message updates... (Update #3)

Status
Converting to AAC...

Progress
██████░░░░ 60%

Files Processed: 3/5
Time Elapsed: 6s
```

**Final State:**
```
🧪 Discord Live Update Test
Testing live message updates... (Update #5)

Status
Complete!

Progress
██████████ 100%

Files Processed: 5/5
Time Elapsed: 10s
```

## Troubleshooting

**"Invalid webhook URL" error:**
- Make sure the URL includes both the webhook ID and token
- Format: `https://discord.com/api/webhooks/[18-digit-ID]/[68-character-token]`

**No updates appearing:**
- Check the Discord channel permissions
- Ensure the webhook is active (not deleted)
- Try creating a new webhook

**Rate limiting:**
- The test includes 2-second delays to avoid rate limits
- If you see 429 errors, wait a minute and try again