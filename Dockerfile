# Multi-stage build for Pi deployment
FROM node:18-alpine AS frontend-builder

# Build React frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Go backend stage
FROM golang:1.21-alpine AS backend-builder

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

# Start MinIO in background
echo "Starting MinIO server..."
MINIO_ROOT_USER="${MINIO_ACCESS_KEY:-admin}" \
MINIO_ROOT_PASSWORD="${MINIO_SECRET_KEY:-password}" \
minio server /app/data/minio --console-address ":9001" &

# Wait for MinIO to be ready
echo "Waiting for MinIO to start..."
sleep 5
for i in {1..30}; do
  if curl -f http://localhost:9000/minio/health/live >/dev/null 2>&1; then
    echo "MinIO is ready!"
    break
  fi
  sleep 1
done

# Create bucket if it doesn't exist
if command -v mc >/dev/null 2>&1; then
  mc alias set local http://localhost:9000 "${MINIO_ACCESS_KEY:-admin}" "${MINIO_SECRET_KEY:-password}" || true
  mc mb local/"${MINIO_BUCKET:-sermons}" --ignore-existing || true
  mc anonymous set public local/"${MINIO_BUCKET:-sermons}" || true
fi

# Start the main application
echo "Starting sermon uploader..."
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