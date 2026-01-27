# Quick Start Guide

Get X-Extract Go up and running in 5 minutes!

## Prerequisites

Before you begin, ensure you have:

- **Go 1.21+** installed ([download](https://go.dev/dl/))
- **yt-dlp** for X/Twitter downloads ([install](https://github.com/yt-dlp/yt-dlp#installation))
- **tdl** for Telegram downloads ([install](https://github.com/iyear/tdl))
- **SQLite3** (usually pre-installed on macOS/Linux)

### Quick Install External Tools

```bash
# macOS
brew install yt-dlp

# Linux
pip install yt-dlp

# tdl (all platforms)
# Download from https://github.com/iyear/tdl/releases
```

## Installation

### Option 1: From Source (Recommended for Development)

```bash
# Clone the repository
git clone https://github.com/yourusername/x-extract-go.git
cd x-extract-go

# Download dependencies
make deps

# Build
make build

# Binaries will be in bin/
ls bin/
# x-extract-server
# x-extract-cli
```

### Option 2: Using Docker

```bash
# Clone the repository
git clone https://github.com/yourusername/x-extract-go.git
cd x-extract-go/deployments/docker

# Start with Docker Compose
docker-compose up -d

# Check logs
docker-compose logs -f
```

## Configuration

### 1. Create Configuration File

```bash
# Create config directory
mkdir -p ~/.x-extract

# Copy default config
cp configs/config.yaml ~/.x-extract/config.yaml

# Edit configuration
nano ~/.x-extract/config.yaml
```

### 2. Essential Configuration

Edit `~/.x-extract/config.yaml`:

```yaml
server:
  host: localhost
  port: 8080

download:
  base_dir: $HOME/Downloads/x-download
  completed_dir: $HOME/Downloads/x-download/completed
  incoming_dir: $HOME/Downloads/x-download/incoming
  cookies_dir: $HOME/Downloads/x-download/cookies
  logs_dir: $HOME/Downloads/x-download/logs
  config_dir: $HOME/Downloads/x-download/config
  max_retries: 3
  concurrent_limit: 1
  auto_start_workers: true

queue:
  database_path: $HOME/Downloads/x-download/config/queue.db
  check_interval: 10s

telegram:
  profile: rogan  # Your tdl profile name
  storage_path: $HOME/Downloads/x-download/cookies/telegram/rogan
  use_group: true
  rewrite_ext: true

twitter:
  cookie_file: $HOME/Downloads/x-download/cookies/x.com/default.cookie

notification:
  enabled: true
  method: osascript

logging:
  level: info
  format: console
  output_path: auto  # Creates date-based logs (YYYYMMDD.log)
```

**Note**: All subdirectories are created automatically on first run. If you have an existing installation, files will be automatically migrated to the new structure.

### 3. Setup Telegram (if using)

```bash
# Login to Telegram with tdl
tdl login -n rogan

# Follow the prompts to authenticate
```

### 4. Setup Twitter Cookies (if needed)

Export cookies from your browser:
1. Install a cookie export extension
2. Export cookies for x.com
3. Save to `~/Downloads/x-download/cookies/x.com/default.cookie`

## Running the Application

### Start the Server

```bash
# Using make
make run-server

# Or directly
./bin/x-extract-server

# With custom config
CONFIG_PATH=/path/to/config.yaml ./bin/x-extract-server
```

You should see:
```
INFO  Starting X-Extract server...
INFO  Server listening on localhost:8080
INFO  Queue manager started
```

### Access the Web Interface

Open your browser and navigate to:
```
http://localhost:8080
```

You should see the X-Extract dashboard with:
- Statistics panel
- Add download form
- Downloads list

## Your First Download

### Using the Web Interface

1. Open http://localhost:8080
2. Enter a URL in the input field:
   - X/Twitter: `https://x.com/username/status/123456789`
   - Telegram: `https://t.me/channel/123`
3. Select download mode (default, single, or group)
4. Click "Add to Queue"
5. Watch the download progress in the list below

### Using the CLI

```bash
# Add a download
./bin/x-extract-cli add "https://x.com/username/status/123456789"

# List downloads
./bin/x-extract-cli list

# View statistics
./bin/x-extract-cli stats

# Get download details
./bin/x-extract-cli get <download-id>
```

### Using the API

```bash
# Add a download
curl -X POST http://localhost:8080/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{"url": "https://x.com/username/status/123456789"}'

# List downloads
curl http://localhost:8080/api/v1/downloads

# Get statistics
curl http://localhost:8080/api/v1/downloads/stats
```

## Verify Everything Works

### 1. Check Server Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "queue": {
    "running": true
  }
}
```

### 2. Check Statistics

```bash
./bin/x-extract-cli stats
```

Expected output:
```
Download Statistics:
  Total:      0
  Queued:     0
  Processing: 0
  Completed:  0
  Failed:     0
  Cancelled:  0
```

### 3. Test a Download

```bash
# Add a test download
./bin/x-extract-cli add "https://x.com/test/status/123" --mode single

# Check status
./bin/x-extract-cli list
```

## Common Issues

### Port Already in Use

```bash
# Change port in config
server:
  port: 8081
```

### yt-dlp Not Found

```bash
# Install yt-dlp
pip install yt-dlp

# Or specify path in config
twitter:
  ytdlp_binary: /path/to/yt-dlp
```

### Database Locked

```bash
# Ensure no other instance is running
pkill x-extract-server

# Remove lock files
rm ~/Downloads/x-download/queue.db-shm
rm ~/Downloads/x-download/queue.db-wal
```

## Next Steps

- Read the [full documentation](../README.md)
- Check the [API documentation](API.md)
- Review [troubleshooting guide](TROUBLESHOOTING.md)
- Explore [configuration options](../configs/config.yaml)

## Development

### Run Tests

```bash
# Unit tests
make test

# Integration tests
go test -tags=integration ./test/integration/...

# Coverage report
make test-coverage
open coverage.html
```

### Build for Production

```bash
# Build release binaries
make release

# Binaries will be in bin/ for multiple platforms
```

## Getting Help

- Check [Troubleshooting Guide](TROUBLESHOOTING.md)
- Review [GitHub Issues](https://github.com/yourusername/x-extract-go/issues)
- Read [API Documentation](API.md)

---

**Congratulations!** 🎉 You now have X-Extract Go running!

