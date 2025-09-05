# Multi-stage build for Pi deployment
FROM node:18-alpine AS frontend-builder

# Build React frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Go backend stage
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sermon-uploader .

# Final Pi-optimized image
FROM alpine:latest

# Install runtime dependencies for audio processing, Pi, and MinIO
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    curl \
    wget \
    bash

# Create app directory
WORKDIR /app

# Copy Go backend binary
COPY --from=backend-builder /app/backend/sermon-uploader .

# Copy React frontend build
COPY --from=frontend-builder /app/frontend/out ./frontend/out

# Copy configuration

# Download and install MinIO server and client
RUN if [ "$(uname -m)" = "aarch64" ]; then \
      ARCH="arm64"; \
    else \
      ARCH="amd64"; \
    fi && \
    wget https://dl.min.io/server/minio/release/linux-${ARCH}/minio -O /usr/local/bin/minio && \
    wget https://dl.min.io/client/mc/release/linux-${ARCH}/mc -O /usr/local/bin/mc && \
    chmod +x /usr/local/bin/minio /usr/local/bin/mc

# Create uploads and MinIO data directories
RUN mkdir -p uploads temp data/minio

# Create startup script
RUN cat > /app/start.sh << 'EOF'
#!/bin/bash
set -e

echo "🚀 Starting sermon uploader container..."

# Verify binaries exist
if [ ! -f "/usr/local/bin/minio" ]; then
  echo "❌ MinIO binary not found"
  exit 1
fi

if [ ! -f "./sermon-uploader" ]; then
  echo "❌ Sermon uploader binary not found"
  exit 1
fi

# Start MinIO in background
echo "🗄️ Starting MinIO server..."
MINIO_ROOT_USER="${MINIO_ACCESS_KEY:-admin}" \
MINIO_ROOT_PASSWORD="${MINIO_SECRET_KEY:-password}" \
minio server /app/data/minio --console-address ":9001" --address ":9000" &
MINIO_PID=$!

echo "⏳ Waiting for MinIO to start..."
sleep 10

# Wait for MinIO to be ready with more attempts
for i in {1..60}; do
  if curl -f http://localhost:9000/minio/health/live >/dev/null 2>&1; then
    echo "✅ MinIO is ready! (attempt $i)"
    break
  elif [ $i -eq 60 ]; then
    echo "❌ MinIO failed to start after 60 attempts"
    echo "MinIO logs:"
    jobs -p | xargs -I {} kill -0 {} 2>/dev/null || echo "MinIO process not running"
    exit 1
  else
    echo "⏳ MinIO not ready yet... (attempt $i/60)"
    sleep 2
  fi
done

# Create bucket if it doesn't exist
echo "📦 Setting up MinIO bucket..."
if command -v mc >/dev/null 2>&1; then
  echo "Configuring mc client..."
  mc alias set local http://localhost:9000 "${MINIO_ACCESS_KEY:-admin}" "${MINIO_SECRET_KEY:-password}" || {
    echo "❌ Failed to configure mc client"
    exit 1
  }
  
  echo "Creating bucket: ${MINIO_BUCKET:-sermons}"
  mc mb local/"${MINIO_BUCKET:-sermons}" --ignore-existing || {
    echo "⚠️ Warning: Could not create bucket (may already exist)"
  }
  
  echo "Setting bucket policy..."
  mc anonymous set public local/"${MINIO_BUCKET:-sermons}" || {
    echo "⚠️ Warning: Could not set bucket policy"
  }
  
  echo "✅ MinIO setup complete!"
else
  echo "⚠️ mc command not available, skipping bucket setup"
fi

# Verify environment variables
echo "🔧 Environment check:"
echo "  PORT: ${PORT:-8000}"
echo "  MINIO_ENDPOINT: ${MINIO_ENDPOINT:-http://localhost:9000}"
echo "  MINIO_BUCKET: ${MINIO_BUCKET:-sermons}"

# Start the main application
echo "🚀 Starting sermon uploader on port ${PORT:-8000}..."
exec ./sermon-uploader
EOF

RUN chmod +x /app/start.sh

# Expose ports (8000 for app, 9000 for MinIO, 9001 for MinIO console)
EXPOSE 8000 9000 9001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/api/health || exit 1

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app
USER appuser

CMD ["/app/start.sh"]