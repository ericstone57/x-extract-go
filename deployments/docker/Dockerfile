# Multi-stage build for optimized image size

# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make curl unzip

# Install Bun
RUN curl -fsSL https://bun.sh/install | bash
ENV PATH="/root/.bun/bin:${PATH}"

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build Next.js dashboard
WORKDIR /app/web-dashboard
RUN bun install
RUN bun run build

# Return to app directory
WORKDIR /app

# Build the Go application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /app/bin/x-extract-server ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /app/bin/x-extract-cli ./cmd/cli

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    python3 \
    py3-pip \
    ffmpeg \
    && pip3 install --no-cache-dir yt-dlp

# Install tdl (Telegram downloader)
# Note: You may need to adjust this based on how tdl is distributed
RUN wget -O /usr/local/bin/tdl https://github.com/iyear/tdl/releases/latest/download/tdl_Linux_64bit \
    && chmod +x /usr/local/bin/tdl || echo "tdl installation skipped"

# Create app user
RUN addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/bin/x-extract-server /app/bin/x-extract-cli /app/bin/

# Copy default config
COPY --from=builder /app/configs /app/configs

# Create necessary directories
RUN mkdir -p /app/downloads /app/data && \
    chown -R app:app /app

# Switch to app user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["/app/bin/x-extract-server"]

