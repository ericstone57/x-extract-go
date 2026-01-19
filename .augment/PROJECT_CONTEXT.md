# X-Extract Go - Project Context

**Last Updated**: 2026-01-19

## Project Overview

**X-Extract Go** is a modern, high-performance download manager for X/Twitter and Telegram media, built with Go. It replaces a legacy bash script system with a robust, scalable solution.

### Purpose
Download and manage media content from:
- **X/Twitter** (using `yt-dlp` binary)
- **Telegram** (using `tdl` binary)

### Key Capabilities
- Concurrent downloads with configurable limits
- Intelligent queue management with auto-start/stop
- Web UI, REST API, and CLI interfaces
- macOS notifications
- Real-time statistics and monitoring
- Automatic retry with exponential backoff
- Docker deployment support
- Structured logging (JSON/console)

---

## Architecture

### Design Pattern: Clean Architecture (3-Layer)

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

### Layer Details

#### 1. Domain Layer (`internal/domain/`)
**Purpose**: Core business entities and rules

**Key Files**:
- `download.go` - Download entity with state management
- `config.go` - Configuration models with validation
- `downloader.go` - Downloader interface
- `repository.go` - Repository interface

**Key Concepts**:
- **Platforms**: `x` (Twitter), `telegram`
- **Download Modes**: `default`, `single`, `group`
- **Status Lifecycle**: `queued` → `processing` → `completed`/`failed`
- **Retry Logic**: Exponential backoff with configurable max retries

#### 2. Application Layer (`internal/app/`)
**Purpose**: Orchestrate business logic

**Key Services**:
- `DownloadManager` (`download_manager.go`)
  - Manages download lifecycle
  - Handles concurrency with semaphore
  - Coordinates with downloaders and notifier
  - Implements retry logic

- `QueueManager` (`queue_manager.go`)
  - Processes download queue
  - Auto-start/stop workers
  - Periodic queue checking
  - CRUD operations for downloads

#### 3. Infrastructure Layer (`internal/infrastructure/`)
**Purpose**: External system integrations

**Components**:
- `downloaders/twitter.go` - yt-dlp wrapper for X/Twitter
- `downloaders/telegram.go` - tdl wrapper for Telegram
- `persistence/sqlite.go` - SQLite repository implementation
- `notifier/macos.go` - macOS notification service

---

## Tech Stack

- **Language**: Go 1.21+
- **Database**: SQLite3 with GORM
- **Web Framework**: Gin
- **CLI Framework**: Cobra
- **Configuration**: Viper (YAML)
- **Logging**: Zap (structured logging)
- **Testing**: testify
- **External Tools**: yt-dlp, tdl

---

## Project Structure

```
x-extract-go/
├── cmd/                    # Application entry points
│   ├── server/            # HTTP server (main application)
│   └── cli/               # CLI tool for queue management
├── internal/              # Private application code
│   ├── domain/           # Domain models and business logic
│   ├── app/              # Application services
│   └── infrastructure/   # External integrations
├── pkg/                   # Public libraries
│   ├── logger/           # Logging utilities
│   └── validator/        # Validation utilities
├── api/                   # HTTP API layer
│   ├── handlers/         # HTTP request handlers
│   ├── middleware/       # HTTP middleware
│   └── router.go         # Route definitions
├── web/                   # Web interface
│   ├── static/           # CSS, JS assets
│   └── templates/        # HTML templates
├── configs/               # Configuration files
│   └── config.yaml       # Main configuration
├── deployments/           # Deployment configurations
│   ├── docker/           # Docker files
│   └── k8s/              # Kubernetes manifests
├── test/                  # Integration tests
│   ├── fixtures/         # Test data
│   └── integration/      # Integration test suites
├── docs/                  # Documentation
│   ├── API.md            # API documentation
│   ├── PROJECT_SUMMARY.md
│   ├── QUICKSTART.md
│   └── TROUBLESHOOTING.md
├── scripts/               # Build and utility scripts
├── bin/                   # Compiled binaries
│   ├── x-extract-server
│   └── x-extract-cli
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile              # Build automation
└── README.md             # Project README
```

---

## Configuration

**Location**: `configs/config.yaml`

**Current Settings**:
```yaml
server:
  host: localhost
  port: 8080

download:
  base_dir: $HOME/Downloads/x-download
  temp_dir: $HOME/Downloads/x-download/temp
  max_retries: 3
  retry_delay: 30s
  concurrent_limit: 1
  auto_start_workers: true

queue:
  database_path: $HOME/Downloads/x-download/queue.db
  check_interval: 10s
  auto_exit_on_empty: false
  empty_wait_time: 5m

telegram:
  profile: rogan
  storage_type: bolt
  storage_path: $HOME/Downloads/x-download/tdl-rogan
  use_group: true
  rewrite_ext: true
  tdl_binary: tdl

twitter:
  cookie_file: $HOME/Downloads/x-download/x.com.cookie
  ytdlp_binary: yt-dlp
  write_metadata: true

notification:
  enabled: true
  sound: true
  method: osascript

logging:
  level: info
  format: console
  output_path: stdout
```

---

## Entry Points

### 1. Server (`cmd/server/main.go`)
**Purpose**: HTTP server with REST API and Web UI

**Startup Flow**:
1. Load configuration from `configs/config.yaml`
2. Initialize logger
3. Initialize SQLite repository
4. Initialize platform downloaders (Twitter, Telegram)
5. Initialize notification service
6. Create DownloadManager
7. Create QueueManager
8. Start queue workers (if `auto_start_workers: true`)
9. Setup HTTP router with Gin
10. Start HTTP server on configured port

**Default URL**: `http://localhost:8080`

### 2. CLI (`cmd/cli/main.go`)
**Purpose**: Command-line interface for queue management

**Available Commands**:
- `add <url>` - Add download to queue
- `list` - List all downloads
- `stats` - View statistics
- `get <id>` - Get download details
- `cancel <id>` - Cancel download
- `retry <id>` - Retry failed download

**Usage**: `./bin/x-extract-cli --server http://localhost:8080 <command>`

---

## API Endpoints

**Base URL**: `http://localhost:8080/api/v1`

### Health Checks
- `GET /health` - Health status
- `GET /ready` - Readiness check

### Downloads
- `POST /api/v1/downloads` - Add download
- `GET /api/v1/downloads` - List downloads
- `GET /api/v1/downloads/stats` - Get statistics
- `GET /api/v1/downloads/:id` - Get download details
- `POST /api/v1/downloads/:id/cancel` - Cancel download
- `POST /api/v1/downloads/:id/retry` - Retry download

### Web UI
- `GET /` - Web interface
- `GET /static/*` - Static assets

**API Documentation**: See `docs/API.md`

---

## Key Workflows

### Download Lifecycle

1. **Add to Queue**
   - Client submits URL via API/CLI/Web
   - `QueueManager.AddDownload()` validates platform and mode
   - Creates `Download` entity with status `queued`
   - Saves to SQLite repository

2. **Queue Processing**
   - `QueueManager` runs periodic check (every 10s)
   - Fetches pending downloads from repository
   - Passes to `DownloadManager.ProcessDownload()`

3. **Download Execution**
   - Acquires semaphore slot (concurrency control)
   - Updates status to `processing`
   - Sends notification (download started)
   - Selects appropriate downloader (Twitter/Telegram)
   - Executes download with retry logic
   - Updates status to `completed` or `failed`
   - Sends notification (success/failure)

4. **Retry Logic**
   - On failure, increments retry count
   - Waits for `retry_delay` (30s default)
   - Retries up to `max_retries` (3 default)
   - Uses exponential backoff

### Platform Detection

**X/Twitter**:
- URL patterns: `x.com/*`, `twitter.com/*`
- Downloader: `TwitterDownloader` (uses yt-dlp)
- Cookie authentication via `cookie_file`

**Telegram**:
- URL patterns: `t.me/*`, `telegram.me/*`
- Downloader: `TelegramDownloader` (uses tdl)
- Profile-based authentication
- Supports group downloads

---

## Build & Run

### Build
```bash
make build              # Build both server and CLI
make run-server         # Build and run server
make run-cli            # Build and run CLI
```

### Test
```bash
make test               # Run all tests
make test-coverage      # Generate coverage report
make lint               # Run linters
```

### Docker
```bash
make docker-build       # Build Docker image
make docker-up          # Start with Docker Compose
make docker-down        # Stop services
make docker-logs        # View logs
```

### Clean
```bash
make clean              # Remove build artifacts
```

---

## Development Guidelines

### Code Organization
- Follow Clean Architecture principles
- Domain layer has NO external dependencies
- Application layer depends only on domain
- Infrastructure layer implements domain interfaces

### Testing
- Unit tests for domain models: `internal/domain/*_test.go`
- Unit tests for services: `internal/app/*_test.go`
- Integration tests: `test/integration/`
- Coverage target: >80%

### Logging
- Use structured logging with Zap
- Log levels: debug, info, warn, error, fatal
- Include context: download ID, URL, platform

### Error Handling
- Return errors, don't panic
- Wrap errors with context
- Log errors before returning

### Git Commit Messages
Follow conventional commit standards with simplified, concentrated messages:

**Format**:
```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, no logic change)
- `refactor`: Code refactoring (no feature/fix)
- `perf`: Performance improvements
- `test`: Adding/updating tests
- `chore`: Maintenance tasks (deps, build, etc.)
- `ci`: CI/CD changes

**Rules**:
- Keep subject line ≤ 50 characters
- Use imperative mood ("add" not "added")
- No period at end of subject
- Capitalize first letter
- Use body for "what" and "why" (not "how")
- Keep commits focused and atomic

**Examples**:
```
feat(api): add pause download endpoint
fix(queue): prevent duplicate processing
docs(readme): update installation steps
refactor(downloader): simplify retry logic
test(domain): add download state tests
chore(deps): update gin to v1.9.1
```

**Bad Examples** (avoid):
```
❌ Updated stuff
❌ Fixed bug
❌ WIP
❌ asdfasdf
❌ Fixed the issue where downloads were failing sometimes
```

---

## Dependencies

### External Binaries (Required)
- **yt-dlp**: X/Twitter media download
- **tdl**: Telegram media download
- **SQLite3**: Database

### Go Modules (Key)
- `github.com/gin-gonic/gin` - Web framework
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `go.uber.org/zap` - Logging
- `gorm.io/gorm` - ORM
- `gorm.io/driver/sqlite` - SQLite driver
- `github.com/google/uuid` - UUID generation
- `github.com/stretchr/testify` - Testing

---

## Common Tasks

### Add New Platform Support
1. Add platform constant to `internal/domain/download.go`
2. Implement `domain.Downloader` interface in `internal/infrastructure/downloaders/`
3. Register downloader in `cmd/server/main.go`
4. Update platform detection logic

### Modify Configuration
1. Update `internal/domain/config.go` struct
2. Update `configs/config.yaml` example
3. Update validation logic
4. Update documentation

### Add New API Endpoint
1. Add handler method in `api/handlers/`
2. Register route in `api/router.go`
3. Update `docs/API.md`

---

## Troubleshooting

### Common Issues
- **Queue not processing**: Check `auto_start_workers: true` in config
- **Download fails**: Verify yt-dlp/tdl binaries are installed and in PATH
- **Cookie errors**: Update Twitter cookie file
- **Permission errors**: Check download directory permissions

**Full Troubleshooting Guide**: See `docs/TROUBLESHOOTING.md`

---

## Project Status

✅ **Production Ready**
- Core features fully implemented
- Unit tests for critical paths
- Documentation complete
- Docker deployment ready
- Makefile automation

🔄 **Active Development**
- Additional platform support (future)
- Enhanced monitoring/metrics (future)
- Authentication/authorization (future)

---

## Quick Reference

### File Locations
- **Config**: `configs/config.yaml`
- **Database**: `$HOME/Downloads/x-download/queue.db`
- **Downloads**: `$HOME/Downloads/x-download/`
- **Logs**: stdout (configurable)
- **Binaries**: `bin/x-extract-server`, `bin/x-extract-cli`

### Important Interfaces
- `domain.Downloader` - Platform downloader contract
- `domain.DownloadRepository` - Persistence contract
- `domain.NotificationService` - Notification contract

### Key Constants
- Default server port: `8080`
- Queue check interval: `10s`
- Max retries: `3`
- Retry delay: `30s`
- Concurrent limit: `1`

---

**End of Project Context**

