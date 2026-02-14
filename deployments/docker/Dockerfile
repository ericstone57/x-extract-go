# Multi-stage build for optimized image size

# =============================================================================
# Stage 1: Build Next.js Dashboard
# =============================================================================
FROM node:20-alpine AS dashboard-builder

WORKDIR /app/web-dashboard
COPY web-dashboard/ ./
RUN npm install && npm run build

# =============================================================================
# Stage 2: Go Builder - Compile Go binaries
# =============================================================================
FROM golang:1.23-bookworm AS builder

# Set working directory
WORKDIR /app

# Copy Go module files first for better cache utilization
COPY go.mod go.sum ./
RUN go mod download && \
    go mod verify

# Copy source code
COPY . .

# Copy built dashboard from previous stage
COPY --from=dashboard-builder /app/web-dashboard/build ./web-dashboard/build

# Build Go application (using glibc-based image for sqlite3 compatibility)
RUN CGO_ENABLED=1 \
    go build -ldflags "-s -w" \
    -o /app/bin/x-extract-server ./cmd/server && \
    CGO_ENABLED=1 \
    go build -ldflags "-s -w" \
    -o /app/bin/x-extract-cli ./cmd/cli && \
    chmod +x /app/bin/*

# =============================================================================
# Stage 3: Runtime - Minimal image for running the application
# =============================================================================
FROM debian:12-slim AS runtime

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv \
    ffmpeg \
    wget \
    && python3 -m venv /opt/venv \
    && /opt/venv/bin/pip install --no-cache-dir yt-dlp \
    && rm -rf /var/lib/apt/lists/*

# Install tdl (Telegram downloader)
RUN TDL_VERSION=$(wget -qO- https://api.github.com/repos/iyear/tdl/releases/latest | grep '"tag_name"' | sed 's/.*"v\([0-9.]*\)".*/\1/') && \
    wget -qO /usr/local/bin/tdl "https://github.com/iyear/tdl/releases/download/v${TDL_VERSION}/tdl_Linux_64bit" && \
    chmod +x /usr/local/bin/tdl || \
    echo "Warning: tdl installation failed, will use system tdl if available"

# Create non-root user
RUN groupadd -g 1000 app && \
    useradd -u 1000 -g app -m -s /bin/bash app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder --chown=app:app /app/bin/ /app/bin/

# Copy embedded dashboard from builder
COPY --from=builder --chown=app:app /app/web-dashboard/build/ /app/web-dashboard/build/

# Create necessary directories with correct permissions
# Config directory: /app/config (for config.yaml and queue.db)
# Data directory: /downloads (for downloads, cookies, logs)
RUN mkdir -p /app/config /downloads /downloads/completed /downloads/incoming /downloads/cookies /downloads/logs /downloads/config && \
    chown -R app:app /app /downloads

# Switch to non-root user
USER app

# Set environment variables
# XDG_CONFIG_HOME tells the app to look for config in /app/config
ENV XDG_CONFIG_HOME=/app/config
# Add CLI to PATH for docker exec access
ENV PATH=/app/bin:${PATH}

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"] || \
    ["wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/ready"]

# Run the application
ENTRYPOINT ["/app/bin/x-extract-server"]
