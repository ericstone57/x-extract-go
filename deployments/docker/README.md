# Docker Deployment

This directory contains Docker configuration for running x-extract in containers.

## Quick Start

### 1. Copy environment file
```bash
cp .env.example .env
```

### 2. Create required directories
```bash
mkdir -p downloads data logs
```

### 3. Build and start
```bash
docker-compose up -d
```

### 4. Verify
```bash
docker-compose ps
curl http://localhost:8080/health
```

## CLI Commands (xc wrapper)

The `xc` wrapper script provides convenient CLI access to the x-extract server running in Docker.

### Installation
```bash
# Option 1: Copy to your PATH
cp xc /usr/local/bin/xc
chmod +x /usr/local/bin/xc

# Option 2: Add docker directory to PATH
export PATH="$(pwd):$PATH"
```

### Usage

| Command | Description |
|---------|-------------|
| `xc add [url]` | Add a download task |
| `xc list` | List all downloads |
| `xc list --status queued` | List downloads filtered by status |
| `xc stats` | Show download statistics |
| `xc get [id]` | Get download details |
| `xc cancel [id]` | Cancel a download |
| `xc retry [id]` | Retry a failed download |
| `xc logs [id]` | View download process logs |
| `xc --help` | Show help message |

### Examples
```bash
# Add a download
xc add https://x.com/user/status/1234567890

# Add with platform flag
xc add https://t.me/channel/post/123 --platform telegram

# Check status
xc list

# View logs
xc logs abc12345
```

## Commands

| Command | Description |
|---------|-------------|
| `docker-compose up -d` | Start services |
| `docker-compose up -d --build` | Rebuild and start |
| `docker-compose down` | Stop services |
| `docker-compose logs -f` | View logs |
| `docker-compose logs -f --tail=100` | View last 100 log lines |
| `docker-compose down -v` | Stop and remove volumes |
| `docker-compose ps` | Show status |

## Configuration

### Environment Variables (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| `EXPOSE_PORT` | 8080 | Host port to expose |
| `LOGGING_LEVEL` | info | Log level |
| `DOWNLOADS_DIR` | ./downloads | Downloads volume |
| `DATA_DIR` | ./data | Data volume (queue.db) |
| `LOGS_DIR` | ./logs | Logs volume |
| `TELEGRAM_PROFILE` | rogan | Telegram profile |

### Custom Configuration

To use a custom config.yaml:

1. Edit `config.yaml` (copy from `config.example.yaml`)
2. Uncomment the volume mount in `docker-compose.yml`:
   ```yaml
   volumes:
     - ./config.yaml:/app/configs/config.yaml:ro
   ```
3. Restart: `docker-compose down && docker-compose up -d`

## Volumes

| Volume | Mount Point | Purpose |
|--------|-------------|---------|
| downloads | /app/downloads | Downloaded files |
| data | /app/data | SQLite database |
| logs | /app/logs | Application logs |

## Docker Mode

When running in Docker mode:

- **Server**: Always runs (never auto-exits even when queue is empty)
- **CLI**: Available via `docker exec` or the `xc` wrapper script
- **API**: Accessible at `http://localhost:9090` (or your configured port)

## Health Check

The container includes a health check that verifies:
- `/health` endpoint returns HTTP 200

Unhealthy containers will be restarted automatically (unless stopped).

## Multi-Platform Build

Build for multiple platforms (requires Docker Buildx):

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t x-extract:latest \
  -f deployments/docker/Dockerfile \
  --load .
```
