# Docker Deployment

This directory contains Docker configuration for running x-extract in containers.

## Quick Start

### 1. Copy environment file
```bash
cp .env.example .env
```

### 2. Create required directories
```bash
mkdir -p config downloads
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

### Directory Structure

The application uses two main directories:

| Directory | Container Path | Purpose |
|-----------|----------------|---------|
| config | /app/config | Configuration file (config.yaml) and database (queue.db) |
| downloads | /downloads | Downloaded files, cookies, logs |

### Environment Variables (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| `EXPOSE_PORT` | 9090 | Host port to expose |
| `CONFIG_DIR` | ./config | Config volume (config.yaml, queue.db) |
| `DOWNLOADS_DIR` | ./downloads | Downloads volume |

### Custom Configuration

To customize settings, create a `config.yaml` in the config directory:

1. Create config directory:
   ```bash
   mkdir -p config
   ```

2. The application will auto-create `config/config.yaml` with defaults on first run

3. Edit `config/config.yaml` to customize settings

4. Restart: `docker-compose restart`

### User Override Config

You can also place a `config.yaml` in `downloads/config/` to override specific settings. This is useful for keeping your custom settings separate from the main config.

## Volumes

| Volume | Mount Point | Purpose |
|--------|-------------|---------|
| config | /app/config | Config file and database |
| downloads | /downloads | Downloaded files, cookies, logs |

## Docker Mode

When running in Docker, the application automatically detects the environment and:

- Uses `/app/config` for configuration (XDG_CONFIG_HOME)
- Uses `/downloads` as the default base_dir for data
- Creates all necessary subdirectories automatically

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
