# Build stage - Multi-arch support for Pi 5
FROM --platform=$BUILDPLATFORM node:18-bookworm-slim AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies with platform-specific optimizations
RUN npm ci --no-audit --no-fund

# Copy source code
COPY . .

# Build the Next.js app
RUN npm run build

# Runtime stage - Optimized for ARM64
FROM node:18-bookworm-slim

WORKDIR /app

# Install dumb-init for proper signal handling
RUN apt-get update && \
    apt-get install -y dumb-init && \
    rm -rf /var/lib/apt/lists/*

# Copy built application
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package*.json ./
COPY --from=builder /app/next.config.js ./

# Install production dependencies only
RUN npm ci --production --no-audit --no-fund && \
    npm cache clean --force

# Create non-root user
RUN useradd -m -u 1001 nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000

# Use dumb-init to handle signals properly
ENTRYPOINT ["dumb-init", "--"]
CMD ["npm", "start"]