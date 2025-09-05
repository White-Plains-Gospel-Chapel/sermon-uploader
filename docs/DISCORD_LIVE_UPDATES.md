# Discord Live Updates Documentation

## Overview

The sermon uploader now supports **live Discord message updates** instead of sending multiple notification messages. This provides real-time progress tracking without spamming your Discord channel.

### Message Categories

The system uses **ONE message per category** that updates live:

1. **🚀 Server/System Messages** - One message for all server status updates
   - Server startup → initializing → ready
   - Service health checks
   - System status changes

2. **📤 Upload Messages** - One message per upload batch
   - Upload start → progress (0-100%) → complete
   - Live progress bars for each file
   - Real-time status updates

3. **🔧 Admin Action Messages** - One message per admin operation
   - Bucket cleanup progress
   - Maintenance task status
   - System operations

4. **❌ Error Messages** - Static (don't update)
   - Upload errors
   - System failures
   - Critical alerts

## How It Works

### Single Message Updates
- Creates **one message per batch** of sermon uploads
- Updates that same message in real-time as files progress through the pipeline
- No more message spam - just clean, live progress tracking

### Visual Progress Indicators
Each file shows a progress bar that updates through these stages:
1. **File Detected** (░░░░░░░░░░)
2. **Uploading** (██░░░░░░░░)  
3. **Uploaded** (████░░░░░░)
4. **Processing** (██████░░░░)
5. **Complete** (██████████)

### Color-Coded Status
Messages automatically change color based on overall status:
- 🟠 **Orange**: Processing in progress
- 🟢 **Green**: All files completed successfully
- 🔴 **Red**: One or more files encountered errors

## Configuration

### Environment Variables
Set your Discord webhook URL in the `.env` file:
```bash
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN
```

### Getting a Discord Webhook URL
1. Open Discord and go to your server
2. Go to Server Settings → Integrations → Webhooks
3. Click "New Webhook"
4. Name it (e.g., "Sermon Uploader")
5. Select the channel for notifications
6. Copy the Webhook URL

## Implementation Details

### Architecture
The system uses Discord's webhook API with message editing capabilities:
- Initial message created with `?wait=true` to get message ID
- Subsequent updates use the webhook's message editing endpoint
- Thread-safe message tracking for concurrent updates

### File Structure
```
shared/
├── discord_live_notifier.py   # Core notification system with live updates
    ├── DiscordLiveNotifier     # Low-level Discord API wrapper
    ├── SermonPipelineTracker   # High-level pipeline tracking
    └── ProcessingStage         # Enum for tracking stages

pi-processor/
├── sermon_processor_live.py    # Enhanced processor with live updates
    └── SermonProcessorLive     # Main processor with Discord integration
```

### Message Format Example
```
📤 Uploading 3 Sermons
━━━━━━━━━━━━━━━━━━━━

📄 sermon_2024_01_01.wav
██████░░░░ 
Status: Processing
Size: 45.2 MB
Duration: 4.5s

📄 sermon_2024_01_08.wav  
██████████
Status: Complete
Size: 52.1 MB
Size Reduction: 72.5%

📊 Summary
Total: 3 files
Completed: 2
Errors: 0
```

## Testing

### Manual Test Script
Run the test script to verify Discord live updates are working:
```bash
python3 test_discord_live.py
```

This will:
1. Create a test batch message
2. Simulate upload progress (20% → 100%)
3. Simulate processing stages
4. Complete with mixed success/error results

### What to Look For
- Message should update in-place (not create new messages)
- Progress bars should animate smoothly
- Color should change based on status
- Final message should show completion summary

## API Reference

### SermonPipelineTracker Methods

#### `start_batch_upload(files: List[str]) -> Optional[str]`
Starts tracking a new batch of files. Returns batch ID for updates.

#### `update_file_upload_progress(batch_id: str, filename: str, progress_percent: int, size: Optional[int])`
Updates upload progress for a specific file (0-100%).

#### `mark_file_processing(batch_id: str, filename: str)`
Marks a file as being converted/processed.

#### `mark_file_complete(batch_id: str, filename: str, duration: float, size_reduction: float)`
Marks a file as successfully completed.

#### `mark_file_error(batch_id: str, filename: str, error: str)`
Marks a file as having encountered an error.

#### `complete_batch(batch_id: str)`
Finalizes the batch and updates the title to show completion status.

## Troubleshooting

### Message Not Updating
- Verify webhook URL is correct and includes both ID and token
- Check Discord API status at https://discordstatus.com
- Ensure the webhook has permissions in the target channel

### Rate Limiting
Discord webhooks have rate limits:
- 30 requests per minute per webhook
- The system automatically handles this with built-in delays

### Old Messages
The system automatically cleans up tracked messages older than 48 hours to prevent memory buildup.

## Migration from Old System

If upgrading from the old multi-message system:
1. Update to the new `sermon_processor_live.py`
2. Import from `shared.discord_live_notifier`
3. Replace old `DiscordNotifier` with new `DiscordLiveNotifier`
4. Use `SermonPipelineTracker` for high-level tracking

## Benefits

- **Cleaner Discord Channel**: One message per batch instead of multiple
- **Real-time Progress**: See exactly where each file is in the pipeline
- **Better UX**: No notification spam, just useful updates
- **Error Visibility**: Clearly see which files succeeded or failed
- **Professional Look**: Color-coded, well-formatted progress tracking