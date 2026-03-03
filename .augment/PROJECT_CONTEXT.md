# X-Extract Go - Project Context

**Last Updated**: 2026-02-23

## Project Overview

**X-Extract Go** is a modern, high-performance download manager for X/Twitter and Telegram media, built with Go. It replaces a legacy bash script system with a robust, scalable solution.

### Purpose
Download and manage media content from:
- **X/Twitter** (using `yt-dlp` binary)
- **Telegram** (using `tdl` binary)

### Key Capabilities
- Per-platform parallel downloads (different platforms download simultaneously, same-platform serialized)
- Intelligent queue management with auto-start/stop and auto-exit
- Next.js web dashboard, REST API, and CLI interfaces
- Desktop notifications (macOS osascript, Linux notify-send)
- Real-time statistics and monitoring
- WebSocket-based real-time log streaming
- Automatic retry with exponential backoff
- Daemon mode server (auto-forks to background)
- CLI auto-starts server if not running
- Docker deployment support
- XDG Base Directory compliant configuration
- Topic-based structured logging (queue, error, download, stderr)
- Telegram channel name caching and message metadata caching

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
- `download.go` - Download entity with state management, priority, platform detection
- `config.go` - Configuration models with XDG paths, `DefaultConfigDir()`, `DefaultQueueDBPath()`, `IsDocker()`
- `downloader.go` - Downloader interface with `DownloadProgressCallback`
- `repository.go` - Repository interface with `FindByURL`, `CountActive`, `ResetOrphanedProcessing`
- `telegram_channel.go` - Telegram channel ID/name mapping and `TelegramChannelRepository` interface
- `telegram_message_cache.go` - Message metadata caching and `TelegramMessageCacheRepository` interface

**Key Concepts**:
- **Platforms**: `x` (Twitter), `telegram`
- **Download Modes**: `default`, `single`, `group`
- **Status Lifecycle**: `queued` → `processing` → `completed`/`failed`/`cancelled`
- **Priority**: Integer field for download ordering (higher = processed first)
- **Retry Logic**: Exponential backoff with configurable max retries
- **Helper Methods**: `IsPending()`, `IsProcessing()`, `IsTerminal()`, `DetectPlatform()`, `ValidatePlatform()`, `ValidateMode()`

#### 2. Application Layer (`internal/app/`)
**Purpose**: Orchestrate business logic

**Key Services**:
- `DownloadManager` (`download_manager.go`)
  - Manages download lifecycle
  - Per-platform semaphores (limit=1 each, allowing cross-platform parallelism)
  - Coordinates with downloaders and notifier
  - Implements retry logic with cancellation checks
  - Supports cancel and retry operations

- `QueueManager` (`queue_manager.go`)
  - Processes download queue with `*logger.MultiLogger`
  - Auto-start/stop workers
  - Periodic queue checking with parallel download spawning
  - CRUD operations for downloads (including delete)
  - Auto-exit support when queue empties (`WaitForExit()` channel)
  - Resets orphaned processing downloads on startup

- `ConfigLoader` (`config_loader.go`)
  - XDG-based config loading (`~/.config/x-extract-go/config.yaml`)
  - Config cascade: hardcoded defaults → system config → user override
  - Path expansion (`$HOME`, `~`)
  - Auto-creates default config file if missing
  - Old directory structure migration support

#### 3. Infrastructure Layer (`internal/infrastructure/`)
**Purpose**: External system integrations

**Components**:
- `downloader_twitter.go` - yt-dlp wrapper for X/Twitter (accepts `logsDir`, `*logger.MultiLogger`)
- `downloader_telegram.go` - tdl wrapper for Telegram (channel caching, message caching, takeout mode)
- `repository_sqlite.go` - SQLite repository implementing `DownloadRepository`, `TelegramChannelRepository`, `TelegramMessageCacheRepository`
- `notification.go` - Desktop notifications (macOS osascript + Linux notify-send)
- `shell_utils.go` - Shell command escaping utilities for safe logging

---

## Tech Stack

- **Language**: Go 1.21+
- **Database**: SQLite3 with GORM
- **Web Framework**: Gin
- **Web Dashboard**: Next.js (static export, embedded in Go binary via `go:embed`)
- **CLI Framework**: Cobra
- **Configuration**: Viper (YAML) with XDG Base Directory Specification
- **Logging**: Zap (topic-based structured logging via MultiLogger)
- **WebSocket**: gorilla/websocket (real-time log streaming)
- **Testing**: testify
- **External Tools**: yt-dlp, tdl

---

## Project Structure

```
x-extract-go/
├── cmd/                    # Application entry points
│   ├── server/            # HTTP server with daemon mode
│   │   └── main.go       # Server entrypoint (fork/daemon support)
│   └── cli/               # CLI tool for queue management
│       ├── main.go        # CLI commands (add, list, stats, logs, regenerate-metadata)
│       ├── server.go      # Auto-start server logic
│       ├── server_unix.go # Unix process detaching
│       └── server_windows.go # Windows process detaching
├── internal/              # Private application code
│   ├── domain/           # Domain models and business logic
│   │   ├── download.go   # Download entity with priority, state management
│   │   ├── config.go     # Config models, XDG defaults, IsDocker()
│   │   ├── downloader.go # Downloader interface, DownloadProgressCallback
│   │   ├── repository.go # Repository interfaces
│   │   ├── telegram_channel.go      # Channel ID/name mapping
│   │   └── telegram_message_cache.go # Message metadata caching
│   ├── app/              # Application services
│   │   ├── download_manager.go  # Per-platform semaphores, retry logic
│   │   ├── queue_manager.go     # Queue processing, auto-exit
│   │   └── config_loader.go     # XDG config loading, migration
│   └── infrastructure/   # External integrations
│       ├── downloader_twitter.go    # yt-dlp wrapper
│       ├── downloader_telegram.go   # tdl wrapper with channel/message caching
│       ├── repository_sqlite.go     # SQLite: downloads, channels, message cache
│       ├── notification.go          # Desktop notifications (osascript/notify-send)
│       └── shell_utils.go          # Shell escaping utilities
├── pkg/                   # Public libraries
│   └── logger/           # Logging utilities
│       ├── multi_logger.go  # Topic-based multi-logger (queue, error)
│       ├── adapter.go       # LoggerAdapter for backward compatibility
│       ├── log_reader.go    # Log file reading and tailing
│       └── logger.go        # Base logger utilities
├── api/                   # HTTP API layer
│   ├── handlers/         # HTTP request handlers
│   │   ├── download_handler.go  # Download CRUD + delete
│   │   ├── health_handler.go    # Health and readiness checks
│   │   ├── log_handler.go       # Log viewing, search, export
│   │   └── log_websocket.go     # WebSocket log streaming
│   ├── middleware/       # HTTP middleware (logger, recovery, CORS)
│   └── router.go         # Route definitions with embedded dashboard
├── web-dashboard/         # Next.js frontend (static export)
│   ├── src/              # Next.js source code
│   ├── embed.go          # go:embed directive for static files
│   └── package.json      # Node.js dependencies (uses bun)
├── configs/               # Reference configuration file
│   └── config.yaml       # Reference config (actual config at ~/.config/x-extract-go/)
├── deployments/           # Deployment configurations
│   ├── docker/           # Dockerfile, docker-compose, config example
│   └── k8s/              # Kubernetes manifests
├── test/                  # Integration tests
│   ├── fixtures/         # Test data
│   └── integration/      # Integration test suites
├── docs/                  # Documentation
│   ├── API.md, PROJECT_SUMMARY.md, QUICKSTART.md, TROUBLESHOOTING.md
├── bin/                   # Compiled binaries
│   ├── x-extract-server  # Server binary
│   ├── x-extract-cli     # CLI binary
│   └── x-extract         # CLI alias binary
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── Makefile              # Build automation
├── AGENTS.md             # AI agent guidelines
└── README.md             # Project README
```

---

## Configuration

**Primary Location**: `~/.config/x-extract-go/config.yaml` (XDG Base Directory Specification)
- **Docker**: `/app/config/config.yaml`
- **Custom XDG**: `$XDG_CONFIG_HOME/x-extract-go/config.yaml`
- **Reference**: `configs/config.yaml` (in repository, not used directly)

**Current Settings** (defaults from `domain.DefaultConfig()`):
```yaml
server:
  host: localhost
  port: 8080    # Note: reference config uses 9091

download:
  # Base directory - empty means auto-resolve to $HOME/Downloads/x-download (or /downloads in Docker)
  # Subdirectories auto-created: completed/, incoming/, cookies/, logs/, config/
  base_dir: ""
  max_retries: 3
  retry_delay: 30s
  # DEPRECATED: concurrent_limit is no longer used for global concurrency control.
  # Downloads use per-platform semaphores (limit=1 per platform).
  concurrent_limit: 3
  auto_start_workers: true

queue:
  # Empty means auto-resolve to ~/.config/x-extract-go/queue.db
  database_path: ""
  check_interval: 10s
  auto_exit_on_empty: true
  empty_wait_time: 30s

telegram:
  profile: default
  storage_type: bolt
  storage_path: ""    # Auto-resolves to $base_dir/cookies/telegram/default
  use_group: true
  rewrite_ext: true
  extra_params: ""    # Extra parameters for tdl command
  tdl_binary: tdl
  takeout: false      # Use takeout mode for Telegram

twitter:
  cookie_file: ""     # Auto-resolves to $base_dir/cookies/x.com/default.cookie
  ytdlp_binary: yt-dlp
  write_metadata: true

notification:
  enabled: true
  sound: true
  method: osascript   # osascript (macOS), notify-send (Linux)

logging:
  level: info
  format: console
  # Options: stdout, stderr, auto (topic-based logs in base_dir/logs/), or file path
  output_path: stdout
```

**Data Directory Structure** (`$HOME/Downloads/x-download/` by default):
```
$HOME/Downloads/x-download/
├── cookies/              # Authentication files
│   ├── x.com/           # Twitter/X cookies
│   │   └── default.cookie
│   └── telegram/        # Telegram storage
│       └── default/     # Profile-specific storage
├── completed/           # Successfully downloaded files
│   └── [media files + .info.json metadata]
├── incoming/            # Files being downloaded
│   └── [temp files]
├── logs/                # Topic-based log files (always multi-logger)
│   ├── queue-YYYYMMDD.log     # Queue lifecycle events (JSON format)
│   ├── error-YYYYMMDD.log     # Application errors (JSON format)
│   ├── download-YYYYMMDD.log  # Raw download output from yt-dlp/tdl
│   └── stderr-YYYYMMDD.log    # Raw stderr from yt-dlp/tdl
└── config/              # User override configuration
    └── config.yaml      # Optional overrides (merged on top of system config)
```

**Configuration Cascade** (priority low → high):
1. Hardcoded defaults (`domain.DefaultConfig()`)
2. System config (`~/.config/x-extract-go/config.yaml` or `/app/config/config.yaml` in Docker)
3. User override (`$base_dir/config/config.yaml` if exists)

**Config Database Location**: `~/.config/x-extract-go/queue.db` (separate from data dir)

---

## Entry Points

### 1. Server (`cmd/server/main.go`)
**Purpose**: HTTP server with REST API and embedded Next.js dashboard

**Daemon Mode**:
- Server runs as a daemon by default (forks to background with `-server-mode` flag)
- Parent process starts child with `-server-mode` and exits immediately
- Child process detaches with `Setsid: true` and redirects I/O to `/dev/null`
- Logs go to topic-based log files in `$base_dir/logs/`

**Startup Flow**:
1. Parse flags; if no `-server-mode`, fork as daemon and exit
2. Load configuration via `app.LoadConfig()` (XDG-based)
3. Create logs directory, initialize `MultiLogger` (queue + error categories)
4. Create `LoggerAdapter` for backward compatibility
5. Migrate old directory structure if needed
6. Initialize `SQLiteDownloadRepository` (downloads, channels, message cache tables)
7. Initialize notification service
8. Initialize platform downloaders (Twitter with logsDir/multiLog, Telegram with channel+message cache repos)
9. Create `DownloadManager` (per-platform semaphores)
10. Create `QueueManager` (with MultiLogger)
11. Start queue workers (if `auto_start_workers: true`)
12. Setup HTTP router with embedded Next.js dashboard
13. Start HTTP server on configured port
14. Wait for shutdown signal OR auto-exit from queue manager

**Default URL**: `http://localhost:8080` (reference config uses `9091`)

### 2. CLI (`cmd/cli/main.go`)
**Purpose**: Command-line interface for queue management

**Auto-Start Server**: CLI auto-detects if server is running (via `/health` check). If not running, it locates and starts the server binary in the background, then waits for it to become ready. Use `--no-auto-start` to disable.

**Available Commands**:
- `add <url>` - Add download to queue (`-m` mode, `-p` platform)
- `list` - List all downloads (`-s` status filter)
- `stats` - View statistics
- `get <id>` - Get download details
- `cancel <id>` - Cancel download
- `retry <id>` - Retry failed download
- `logs <id>` - View download process logs (`-j` for JSON output)
- `regenerate-metadata` - Regenerate metadata JSON files for Telegram downloads with missing descriptions (`-n` dry-run, `-d` completed-dir)

**Usage**: `./bin/x-extract-cli --server http://localhost:9091 <command>`

---

## API Endpoints

**Base URL**: `http://localhost:8080/api/v1` (or `:9091` if using reference config)

### Health Checks
- `GET /health` - Health status
- `GET /ready` - Readiness check

### Downloads
- `POST /api/v1/downloads` - Add download (body: `{url, platform?, mode?}`)
- `GET /api/v1/downloads` - List downloads (query: `?status=`)
- `GET /api/v1/downloads/stats` - Get statistics
- `GET /api/v1/downloads/:id` - Get download details
- `POST /api/v1/downloads/:id/cancel` - Cancel download
- `POST /api/v1/downloads/:id/retry` - Retry download
- `DELETE /api/v1/downloads/:id` - Delete download record

### Logs
- `GET /api/v1/logs/categories` - List log categories (download, stderr, queue, error)
- `GET /api/v1/logs/:category` - Read logs (query: `?limit=100&date=YYYY-MM-DD`)
- `GET /api/v1/logs/:category/search` - Search logs (query: `?q=&limit=100&date=`)
- `GET /api/v1/logs/:category/export` - Export log file (query: `?date=`)

### Web Dashboard (Embedded Next.js)
- `GET /` - Dashboard (Next.js SPA with client-side routing)
- `GET /_next/*` - Next.js static assets

**API Documentation**: See `docs/API.md`

---

## Key Workflows

### Download Lifecycle

1. **Add to Queue**
   - Client submits URL via API/CLI/Web dashboard
   - `QueueManager.AddDownload()` validates platform (via `DetectPlatform()`) and mode (via `ValidateMode()`)
   - Creates `Download` entity with status `queued` and optional priority
   - Saves to SQLite repository
   - Sends desktop notification (download queued)

2. **Queue Processing**
   - `QueueManager` runs periodic check (every `check_interval`, default 10s)
   - Fetches pending downloads from repository (ordered by priority, then creation time)
   - For each pending download, spawns goroutine calling `DownloadManager.ProcessDownload()`
   - Checks auto-exit condition: if queue empty for `empty_wait_time` (30s) and `auto_exit_on_empty` is true

3. **Download Execution**
   - Acquires **per-platform semaphore** (limit=1 per platform; different platforms download in parallel)
   - Checks for cancellation before proceeding
   - Updates status to `processing` via `MarkProcessing()`
   - Sends notification (download started)
   - Selects appropriate downloader based on platform
   - Executes download with progress callback (`DownloadProgressCallback`)
   - Checks for cancellation after download completes
   - Updates status to `completed` or `failed`
   - Sends notification (success/failure)
   - Releases platform semaphore

4. **Retry Logic**
   - On failure, increments retry count
   - Waits for `retry_delay` (30s default)
   - Retries up to `max_retries` (3 default)
   - Uses exponential backoff

5. **Cancellation**
   - `DownloadManager.CancelDownload()` sets status to `cancelled`
   - Processing goroutine checks cancellation at multiple points (before semaphore, after semaphore, after download)

6. **Auto-Exit**
   - `QueueManager.WaitForExit()` returns a channel
   - Server listens on this channel alongside OS signals
   - When queue is empty for `empty_wait_time`, signals exit

### Platform Detection

**X/Twitter** (`DetectPlatform()` in `domain/download.go`):
- URL patterns: `x.com/*`, `twitter.com/*`
- Downloader: `TwitterDownloader` (uses yt-dlp)
- Cookie authentication via `cookie_file`
- Logs: download output → `download-YYYYMMDD.log`, stderr → `stderr-YYYYMMDD.log`

**Telegram** (`DetectPlatform()` in `domain/download.go`):
- URL patterns: `t.me/*`, `telegram.me/*`
- Downloader: `TelegramDownloader` (uses tdl)
- Profile-based authentication via `telegram.profile`
- Supports group downloads and takeout mode
- Channel name caching (7-day refresh via `TelegramChannelRepository`)
- Message metadata caching for smart incremental exports (`TelegramMessageCacheRepository`)

---

## Build & Run

### Build
```bash
make build              # Build dashboard + server + CLI (3 binaries)
make build-dashboard    # Build Next.js dashboard only (bun run build)
make run-server         # Build and run server
make run-cli            # Build and run CLI
make deploy             # Build and copy binaries to ~/bin/
```

### Test & Lint
```bash
make test               # Run all tests with race detection + coverage
make test-coverage      # Run tests + open HTML coverage report
make lint               # Run go vet + check formatting
make fmt                # Format all Go code
```

### Docker
```bash
make docker-build       # Build multi-platform Docker image (amd64+arm64)
make docker-build-local # Build Docker image for local platform only
make docker-up          # Start Docker Compose services
make docker-up-build    # Rebuild and start Docker Compose services
make docker-down        # Stop services
make docker-logs        # View logs
make docker-clean       # Remove Docker resources
make docker-status      # Show container status
```

### Server Management
```bash
make kill-server        # Kill running server (pkill)
make restart-server     # Kill, rebuild, and restart server
```

### Other
```bash
make clean              # Remove build artifacts
make deps               # Download and tidy Go modules
make install-tools      # Install dev tools (golangci-lint)
make help               # Show all available targets
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
- Run: `go test -v ./internal/domain/ -run TestFunctionName`

### Logging
- **MultiLogger** (always enabled): Topic-based structured logging via `pkg/logger/multi_logger.go`
- **Log Categories**: `queue` (queue lifecycle), `error` (application errors), `download` (raw downloader output), `stderr` (raw downloader stderr)
- **Log Format**: JSON for queue/error logs, raw text for download/stderr logs
- **Log Files**: `{category}-YYYYMMDD.log` in `$base_dir/logs/`
- **LoggerAdapter**: Wraps MultiLogger for backward compatibility with code expecting `*zap.Logger`
- Use structured logging with context: download ID, URL, platform

### Error Handling
- Return errors, don't panic
- Wrap errors with context using `fmt.Errorf("action: %w", err)`
- Check errors immediately with early return

### Git Commit Messages
Follow conventional commit standards. See `.augment/GIT_COMMIT_GUIDE.md` for full details.

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
- **bun**: Node.js runtime for building Next.js dashboard

### Go Modules (Key)
- `github.com/gin-gonic/gin` - Web framework
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration (YAML)
- `go.uber.org/zap` - Structured logging
- `gorm.io/gorm` - ORM (SQLite)
- `gorm.io/driver/sqlite` - SQLite driver (CGo)
- `github.com/google/uuid` - UUID generation
- `github.com/gorilla/websocket` - WebSocket (real-time log streaming)
- `github.com/stretchr/testify` - Testing assertions

---

## Common Tasks

### Add New Platform Support
1. Add platform constant to `internal/domain/download.go`
2. Update `DetectPlatform()` URL matching in `internal/domain/download.go`
3. Implement `domain.Downloader` interface in `internal/infrastructure/downloader_<platform>.go`
4. Add platform semaphore in `internal/app/download_manager.go` `NewDownloadManager()`
5. Register downloader in `cmd/server/main.go`

### Modify Configuration
1. Update `internal/domain/config.go` struct and `DefaultConfig()`
2. Update `configs/config.yaml` reference file
3. Update `internal/app/config_loader.go` if loading logic changes
4. Update documentation

### Add New API Endpoint
1. Add handler method in `api/handlers/`
2. Register route in `api/router.go`
3. Update `docs/API.md`

### Add New Log Category
1. Add category constant in `pkg/logger/multi_logger.go`
2. Initialize writer in `MultiLogger.NewMultiLogger()`
3. Add to valid categories in `api/handlers/log_handler.go`

---

## Troubleshooting

### Common Issues
- **Queue not processing**: Check `auto_start_workers: true` in config
- **Download fails**: Verify yt-dlp/tdl binaries are installed and in PATH
- **Cookie errors**: Update Twitter cookie file at `$base_dir/cookies/x.com/default.cookie`
- **Permission errors**: Check download directory permissions
- **Config not loading**: Check `~/.config/x-extract-go/config.yaml` exists (XDG path)
- **Orphaned processing**: Server auto-resets orphaned `processing` downloads to `queued` on startup
- **Server won't start**: Check if port is already in use; use `make kill-server` then retry

**Full Troubleshooting Guide**: See `docs/TROUBLESHOOTING.md`

---

## Project Status

✅ **Production Ready** (as of 2026-02-23)
- Core download features fully implemented (X/Twitter + Telegram)
- Per-platform parallel downloads with semaphores
- Next.js web dashboard embedded in binary
- CLI with auto-start server
- Daemon mode server
- Topic-based structured logging with WebSocket streaming
- Telegram channel/message caching
- Desktop notifications (macOS + Linux)
- Docker deployment ready (multi-platform)
- XDG-compliant configuration
- Unit tests for domain models

🔄 **Active Development**
- Additional platform support (future)
- Enhanced monitoring/metrics (future)
- Authentication/authorization (future)

---

## Quick Reference

### File Locations
- **System Config**: `~/.config/x-extract-go/config.yaml`
- **User Override**: `$base_dir/config/config.yaml`
- **Database**: `~/.config/x-extract-go/queue.db`
- **Downloads**: `$HOME/Downloads/x-download/completed/`
- **Cookies**: `$HOME/Downloads/x-download/cookies/`
- **Logs**: `$HOME/Downloads/x-download/logs/` (always topic-based)
- **Binaries**: `bin/x-extract-server`, `bin/x-extract-cli`, `bin/x-extract`

### Important Interfaces
- `domain.Downloader` - Platform downloader contract (with `DownloadProgressCallback`)
- `domain.DownloadRepository` - Download persistence contract
- `domain.TelegramChannelRepository` - Channel name caching contract
- `domain.TelegramMessageCacheRepository` - Message metadata caching contract

### Key Constants
- Default server port: `8080` (reference config: `9091`)
- Queue check interval: `10s`
- Max retries: `3`
- Retry delay: `30s`
- Per-platform semaphore limit: `1` (per platform)
- Auto-exit empty wait: `30s`
- Channel cache max age: `7 days`

---

**End of Project Context**

