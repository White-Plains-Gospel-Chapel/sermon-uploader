# Pi Processor Dockerfile - completely fixed version
FROM python:3.11-slim

# Install system dependencies including FFmpeg
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    ca-certificates \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Verify FFmpeg installation
RUN ffmpeg -version

# Set working directory
WORKDIR /app

# Copy requirements first for better caching
COPY requirements.txt ./

# Install Python dependencies with pip
RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

# Copy application files
COPY sermon_queue_gui.py ./
# Copy .env if it exists, otherwise it will be provided at runtime
COPY .en[v] ./.env* ./

# Create necessary directories with proper permissions
RUN mkdir -p /var/log /tmp/sermon_processing && \
    chmod 755 /var/log /tmp/sermon_processing

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Test imports and FFmpeg before running
RUN python -c "import minio; import requests; print('Python imports successful')" && \
    ffmpeg -version > /dev/null && echo "FFmpeg working"

# Health check
HEALTHCHECK --interval=60s --timeout=10s --start-period=30s --retries=3 \
    CMD python -c "import sys; sys.exit(0)"

# Run the processor
CMD ["python", "sermon_queue_gui.py"]