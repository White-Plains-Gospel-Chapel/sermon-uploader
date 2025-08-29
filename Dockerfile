# Multi-stage build for Pi deployment
FROM node:18-alpine AS frontend-builder

# Build React frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/. ./
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

# Install runtime dependencies for audio processing and Pi
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    curl

# Create app directory
WORKDIR /app

# Copy Go backend binary
COPY --from=backend-builder /app/backend/sermon-uploader .

# Copy React frontend build
COPY --from=frontend-builder /app/frontend/out ./frontend/out

# Copy configuration

# Create uploads directory
RUN mkdir -p uploads temp

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/api/health || exit 1

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN chown -R appuser:appgroup /app
USER appuser

CMD ["./sermon-uploader"]