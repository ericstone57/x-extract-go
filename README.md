# X-Extract Go

A modern, high-performance download manager for X/Twitter and Telegram media, built with Go. This application replaces the legacy bash script system with a robust, scalable solution featuring a REST API, web interface, and CLI.

## Features

- 🚀 **High Performance**: Concurrent downloads with configurable limits
- 🔄 **Queue Management**: Intelligent queue system with auto-start/stop
- 🌐 **Web Interface**: Modern web UI for monitoring and management
- 🔌 **REST API**: Full-featured API for programmatic access
- 💻 **CLI Tool**: Command-line interface for power users
- 🔔 **Notifications**: macOS notification support
- 📊 **Statistics**: Real-time download statistics and monitoring
- 🔁 **Retry Logic**: Automatic retry with exponential backoff
- 🐳 **Docker Support**: Containerized deployment ready
- 📝 **Structured Logging**: JSON and console logging with levels
- ⚙️ **Flexible Configuration**: YAML-based configuration with environment variable support

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │   CLI    │  │  Web UI  │  │ REST API │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────────┐
│                   Application Layer                          │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐       │
│  │Queue Manager │  │Download Mgr │  │Config Manager│       │
│  └──────────────┘  └─────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
                           │
┌─────────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ yt-dlp   │  │   tdl    │  │ SQLite   │  │  Logger  │   │
│  │(Twitter) │  │(Telegram)│  │  (Queue) │  │          │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- yt-dlp (for X/Twitter downloads)
- tdl (for Telegram downloads)
- SQLite3

### Installation

#### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/x-extract-go.git
cd x-extract-go

# Download dependencies
go mod download

# Build
make build

# Run server
./bin/x-extract-server
```

#### Using Docker

```bash
# Build and run with Docker Compose
cd deployments/docker
docker-compose up -d
```

### Configuration

The default configuration file is at `configs/config.yaml`. The application uses an organized directory structure:

```
$HOME/Downloads/x-download/
├── cookies/              # Authentication files
│   ├── x.com/           # Twitter/X cookies
│   └── telegram/        # Telegram storage
├── completed/           # Successfully downloaded files
├── incoming/            # Files being downloaded
├── logs/                # Date-based log files
└── config/              # Configuration and database
    ├── queue.db         # SQLite database
    ├── config.yaml      # Runtime config (overrides defaults)
    └── local.yaml       # Optional local overrides (highest priority)
```

**Basic configuration**:

```yaml
server:
  host: localhost
  port: 8080

download:
  base_dir: $HOME/Downloads/x-download
  max_retries: 3
  concurrent_limit: 1

telegram:
  profile: rogan
  cookie_file: $HOME/Downloads/x-download/cookies/x.com/default.cookie

notification:
  enabled: true
  method: osascript

logging:
  level: info
  format: json
  output_path: auto  # Creates date-based logs (YYYYMMDD.log) in logs/ directory
```

**Logging Modes**:
1. **Console** (`output_path: stdout`): Logs to console only (default)
2. **Date-based** (`output_path: auto`): Single log file per day (`YYYYMMDD.log`)
3. **Multi-logger** (`--multi-logger` flag): Topic-based logs:
   - `general-YYYYMMDD.log` - General application logs
   - `web-access-YYYYMMDD.log` - HTTP request/response logs
   - `queue-YYYYMMDD.log` - Queue management logs
   - `download-progress-YYYYMMDD.log` - Download progress logs
   - `error-YYYYMMDD.log` - All error-level logs

See `configs/config.yaml` for full configuration options.

**Configuration Priority** (highest to lowest):
1. Environment variables (prefix: `XEXTRACT_`)
2. `$base_dir/config/local.yaml` (local overrides)
3. `$base_dir/config/config.yaml` (runtime config)
4. `configs/config.yaml` (default config)

## Usage

### Web Interface

Access the web interface at `http://localhost:8080`

Features:
- Add downloads with URL
- Monitor download progress
- View statistics
- Filter by status
- Retry failed downloads

### CLI

```bash
# Add a download
x-extract-cli add "https://x.com/user/status/123"

# Add with specific mode
x-extract-cli add "https://t.me/channel/123" --mode single

# List downloads
x-extract-cli list

# Filter by status
x-extract-cli list --status completed

# View statistics
x-extract-cli stats

# Get download details
x-extract-cli get <download-id>

# Retry failed download
x-extract-cli retry <download-id>

# Cancel download
x-extract-cli cancel <download-id>
```

### REST API

#### Add Download
```bash
curl -X POST http://localhost:8080/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{"url": "https://x.com/user/status/123"}'
```

#### List Downloads
```bash
curl http://localhost:8080/api/v1/downloads
```

#### Get Statistics
```bash
curl http://localhost:8080/api/v1/downloads/stats
```

See [API Documentation](docs/API.md) for complete API reference.

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# View coverage report
open coverage.html
```

### Building

```bash
# Build both server and CLI
make build

# Build server only
go build -o bin/x-extract-server ./cmd/server

# Build CLI only
go build -o bin/x-extract-cli ./cmd/cli
```

### Docker Development

```bash
# Build Docker image
make docker-build

# Run with Docker Compose
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

## Project Structure

```
x-extract-go/
├── cmd/                    # Application entry points
│   ├── server/            # HTTP server
│   └── cli/               # CLI tool
├── internal/              # Private application code
│   ├── domain/           # Domain models and interfaces
│   ├── app/              # Application services
│   └── infrastructure/   # External integrations
├── pkg/                   # Public libraries
│   ├── logger/           # Logging utilities
│   └── validator/        # Validation utilities
├── api/                   # API handlers and routing
│   ├── handlers/         # HTTP handlers
│   └── middleware/       # HTTP middleware
├── web/                   # Web interface
│   ├── static/           # CSS, JS assets
│   └── templates/        # HTML templates
├── configs/               # Configuration files
├── deployments/           # Deployment configurations
│   └── docker/           # Docker files
├── test/                  # Integration tests
├── docs/                  # Documentation
└── scripts/               # Build and utility scripts
```

## Performance

- **Concurrent Downloads**: Process multiple downloads simultaneously
- **Efficient Queue**: SQLite-based persistent queue
- **Low Memory**: Optimized for minimal resource usage
- **Fast Startup**: Sub-second startup time

## Monitoring

### Health Checks

```bash
# Health check
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready
```

### Metrics

The application exposes download statistics via the API:
- Total downloads
- Downloads by status
- Success/failure rates
- Processing times

## Acknowledgments

- Built to replace the legacy bash script system
- Uses yt-dlp for X/Twitter downloads
- Uses tdl for Telegram downloads
