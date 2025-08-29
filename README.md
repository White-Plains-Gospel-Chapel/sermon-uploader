# Sermon Uploader Pi

A complete sermon audio file management system designed for Raspberry Pi deployment. Upload WAV files through a web interface, automatically convert to streaming-ready AAC format, and get Discord notifications.

## Features

- **Web-based drag & drop interface** - Modern React frontend with shadcn/ui
- **Audio processing** - WAV to AAC conversion at 320kbps using FFmpeg  
- **Duplicate detection** - SHA256 hashing prevents duplicate uploads
- **MinIO object storage** - Reliable file storage and retrieval
- **Discord notifications** - Real-time upload status via webhooks
- **WebSocket updates** - Live progress feedback during uploads
- **Single Pi deployment** - Everything runs in one Docker container
- **Pre-commit hooks** - Automatic build validation before commits

## Project Structure

```
sermon-uploader/
├── backend/                 # Go backend with Fiber framework
│   ├── main.go             # Main application entry point
│   ├── handlers/           # HTTP request handlers
│   ├── services/           # Core business logic services
│   ├── config/             # Configuration management
│   ├── go.mod              # Go dependencies
│   └── .env.example        # Environment template
├── frontend/               # React frontend with Next.js
│   ├── components/         # UI components (shadcn/ui)
│   ├── app/                # Next.js app router
│   ├── package.json        # Node.js dependencies
│   └── tailwind.config.js  # Styling configuration
├── Dockerfile              # Single container build
├── docker-compose.yml      # Pi deployment config
└── README.md              # This file
```

## Quick Start

1. **Clone and configure:**
   ```bash
   git clone <repo-url>
   cd sermon-uploader
   cp backend/.env.example backend/.env
   # Edit backend/.env with your settings
   ```

2. **Deploy on Pi:**
   ```bash
   docker-compose up -d
   ```

3. **Access the interface:**
   Open `http://your-pi-ip:8000` in your browser

## Configuration

Edit `backend/.env` with your settings:

```bash
# MinIO Configuration
MINIO_ENDPOINT=192.168.1.127:9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_SECURE=false
MINIO_BUCKET=sermons

# Discord Configuration  
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...

# File Processing
WAV_SUFFIX=_raw
AAC_SUFFIX=_streamable
BATCH_THRESHOLD=2

# Server
PORT=8000
```

## Testing

Test the connections using Postman:

- **Discord webhook:** `POST http://your-pi-ip:8000/api/test/discord`
- **MinIO connection:** `GET http://your-pi-ip:8000/api/test/minio`

## API Endpoints

- `GET /api/health` - Health check
- `GET /api/status` - System status (MinIO, bucket, file count)
- `POST /api/upload` - Upload WAV files
- `GET /api/files` - List stored files
- `POST /api/test/discord` - Test Discord webhook
- `GET /api/test/minio` - Test MinIO connection
- `GET /ws` - WebSocket for real-time updates

## Architecture

- **Go backend** - Fiber web framework for Pi performance
- **React frontend** - Next.js with shadcn/ui components  
- **MinIO storage** - S3-compatible object storage
- **FFmpeg processing** - Audio format conversion
- **Docker deployment** - Single container with everything

## File Processing

1. Upload WAV files via web interface
2. System calculates SHA256 hash for duplicate detection
3. Files stored in MinIO with `_raw` suffix
4. FFmpeg converts to AAC format with `_streamable` suffix
5. Discord notifications sent for batch operations
6. Real-time progress via WebSocket

## Development Setup

### First-time Setup

1. **Install Git hooks** (required for all developers):
   ```bash
   ./setup-hooks.sh
   ```

2. **What the hooks do**:
   - **Pre-commit**: Go build, TypeScript, ESLint, Docker validation
   - **Pre-push**: Full Docker build test to prevent deployment failures
   - Automatically prevents broken code from reaching GitHub Actions

3. **Emergency bypass** (use sparingly):
   ```bash
   git commit --no-verify  # Skip pre-commit checks
   git push --no-verify    # Skip pre-push checks
   ```

### Manual Testing
```bash
# Run pre-commit checks manually
./.githooks/pre-commit

# Run pre-push checks manually  
./.githooks/pre-push
```

## Requirements

- Raspberry Pi with Docker installed
- MinIO server accessible on network (or embedded in container)
- Discord webhook URL for notifications
- Go 1.21+ and Node.js 18+ for development

