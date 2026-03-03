# X-Extract Go - Quick Cheatsheet

**Ultra-fast reference for common tasks**

---

## 🚀 Quick Commands

### Build & Run
```bash
make build              # Build dashboard + server + CLI (3 binaries)
make build-dashboard    # Build Next.js dashboard only
make run-server         # Build and run server
make deploy             # Build and copy binaries to ~/bin/
./bin/x-extract-server  # Run server directly
./bin/x-extract-cli     # Run CLI directly
./bin/x-extract         # Run CLI (alias)
```

### Testing
```bash
make test               # Run all tests with race detection
make test-coverage      # Generate HTML coverage report
make lint               # Run go vet + check formatting
make fmt                # Format code
```

### Docker
```bash
make docker-build       # Build multi-platform image (amd64+arm64)
make docker-build-local # Build for local platform only
make docker-up          # Start services
make docker-up-build    # Rebuild and start services
make docker-down        # Stop services
make docker-logs        # View logs
make docker-status      # Show container status
make docker-clean       # Remove Docker resources
```

### Server Management
```bash
make kill-server        # Kill running server
make restart-server     # Kill, rebuild, and restart
make clean              # Remove build artifacts
```

---

## 📡 API Quick Reference

### Base URL
```
http://localhost:9091/api/v1   # Reference config port (code default: 8080)
```

### Downloads
```bash
# Add download
curl -X POST http://localhost:9091/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{"url": "https://x.com/user/status/123", "platform": "x", "mode": "default"}'

# List downloads (with optional status filter)
curl http://localhost:9091/api/v1/downloads
curl http://localhost:9091/api/v1/downloads?status=completed

# Get stats
curl http://localhost:9091/api/v1/downloads/stats

# Get single download
curl http://localhost:9091/api/v1/downloads/{id}

# Cancel / Retry / Delete
curl -X POST http://localhost:9091/api/v1/downloads/{id}/cancel
curl -X POST http://localhost:9091/api/v1/downloads/{id}/retry
curl -X DELETE http://localhost:9091/api/v1/downloads/{id}
```

### Logs
```bash
# List categories
curl http://localhost:9091/api/v1/logs/categories

# Read logs (categories: download, stderr, queue, error)
curl http://localhost:9091/api/v1/logs/queue?limit=50&date=2026-02-23

# Search logs
curl http://localhost:9091/api/v1/logs/error/search?q=failed&limit=50

# Export log file
curl -O http://localhost:9091/api/v1/logs/download/export?date=2026-02-23
```

### Health Check
```bash
curl http://localhost:9091/health
```

---

## 💻 CLI Quick Reference

**Note**: CLI auto-starts server if not running. Use `--no-auto-start` to disable.

### Add Download
```bash
./bin/x-extract-cli add "https://x.com/user/status/123"
./bin/x-extract-cli add "https://t.me/channel/123" -m single -p telegram
```

### List Downloads
```bash
./bin/x-extract-cli list
./bin/x-extract-cli list -s completed
./bin/x-extract-cli list -s failed
```

### Get / Cancel / Retry
```bash
./bin/x-extract-cli get <download-id>
./bin/x-extract-cli cancel <download-id>
./bin/x-extract-cli retry <download-id>
```

### Stats & Logs
```bash
./bin/x-extract-cli stats
./bin/x-extract-cli logs <download-id>      # View download logs
./bin/x-extract-cli logs <download-id> -j   # JSON output
```

### Regenerate Metadata
```bash
./bin/x-extract-cli regenerate-metadata          # Fix missing Telegram metadata
./bin/x-extract-cli regenerate-metadata -n       # Dry run
./bin/x-extract-cli regenerate-metadata -d /path # Custom completed dir
```

### Custom Server
```bash
./bin/x-extract-cli --server http://localhost:9091 list
./bin/x-extract-cli --no-auto-start list
```

---

## 🗂️ File Locations

```
System Config:   ~/.config/x-extract-go/config.yaml (XDG)
User Override:   ~/Downloads/x-download/config/config.yaml (optional)
Database:        ~/.config/x-extract-go/queue.db
Completed Files: ~/Downloads/x-download/completed/
Incoming Files:  ~/Downloads/x-download/incoming/
Cookies:         ~/Downloads/x-download/cookies/
Logs:            ~/Downloads/x-download/logs/ (always topic-based)
Binaries:        bin/x-extract-server, bin/x-extract-cli, bin/x-extract
```

---

## 🔧 Configuration Quick Edit

```yaml
# ~/.config/x-extract-go/config.yaml

server:
  port: 9091                    # Server port (code default: 8080)

download:
  base_dir: ""                  # Empty = auto-resolve to ~/Downloads/x-download
  max_retries: 3                # Retry attempts
  # concurrent_limit: 3         # DEPRECATED - uses per-platform semaphores now
  auto_start_workers: true      # Auto-start queue

queue:
  check_interval: 10s           # Queue check frequency
  auto_exit_on_empty: true      # Exit when queue empty (default: true)
  empty_wait_time: 30s          # Wait before exit on empty queue

telegram:
  profile: default              # tdl profile name
  takeout: false                # Use takeout mode

logging:
  level: info                   # debug|info|warn|error
  format: console               # console|json
  output_path: stdout           # stdout|stderr|auto|<file path>
```

**Logging**: Always topic-based via MultiLogger (queue, error, download, stderr) in `$base_dir/logs/`

---

## 🐛 Quick Debugging

### Check Queue Status
```bash
# Via CLI
./bin/x-extract-cli stats

# Via SQLite
sqlite3 ~/.config/x-extract-go/queue.db \
  "SELECT id, url, status, error_message FROM downloads;"
```

### View Logs
```bash
# Via API
curl http://localhost:9091/api/v1/logs/error?limit=20
curl http://localhost:9091/api/v1/logs/queue/search?q=failed

# Via files
tail -f ~/Downloads/x-download/logs/queue-$(date +%Y%m%d).log
tail -f ~/Downloads/x-download/logs/error-$(date +%Y%m%d).log
```

### Enable Debug Logging
```yaml
# ~/.config/x-extract-go/config.yaml
logging:
  level: debug
```

### Test External Binaries
```bash
yt-dlp --version
tdl version
```

### Common Issues

**Queue not processing?** → Set `download.auto_start_workers: true`

**Download fails?** → Check `which yt-dlp` / `which tdl` exist in PATH

**Cookie errors?** → Update cookie file at `~/Downloads/x-download/cookies/x.com/default.cookie`

**Config not loading?** → Check `~/.config/x-extract-go/config.yaml` exists

**Server won't start?** → `make kill-server` then retry

---

## 📊 Key Enums

### Platforms
```
x          # X/Twitter
telegram   # Telegram
```

### Download Modes
```
default    # Platform default behavior
single     # Single item
group      # Group/collection
```

### Status Values
```
queued      # Waiting to process
processing  # Currently downloading
completed   # Successfully downloaded
failed      # Download failed
cancelled   # Download cancelled
```

---

## 🏗️ Code Snippets

### Create Download Entity
```go
download := domain.NewDownload(
    "https://x.com/user/status/123",
    domain.PlatformX,
    domain.ModeDefault,
)
```

### Add to Queue
```go
download, err := queueMgr.AddDownload(url, platform, mode)
if err != nil {
    return err
}
```

### Update Status
```go
download.MarkProcessing()
download.MarkCompleted("/path/to/file.mp4")
download.MarkFailed(err)
```

### Structured Logging
```go
logger.Info("Download started",
    zap.String("id", download.ID),
    zap.String("url", download.URL),
    zap.String("platform", string(download.Platform)),
)
```

---

## 🔍 Quick Navigation

### Find Code
```bash
# Domain models
internal/domain/download.go           # Download entity, platform detection
internal/domain/config.go             # Config models, XDG paths
internal/domain/telegram_channel.go   # Channel caching model
internal/domain/telegram_message_cache.go  # Message caching model

# Services
internal/app/download_manager.go      # Per-platform download execution
internal/app/queue_manager.go         # Queue processing, auto-exit
internal/app/config_loader.go         # XDG config loading

# Downloaders
internal/infrastructure/downloader_twitter.go    # yt-dlp wrapper
internal/infrastructure/downloader_telegram.go   # tdl wrapper

# API
api/router.go                         # Routes + embedded dashboard
api/handlers/download_handler.go      # Download CRUD
api/handlers/log_handler.go           # Log viewing/search/export
api/handlers/log_websocket.go         # WebSocket log streaming

# Logger
pkg/logger/multi_logger.go           # Topic-based logging
pkg/logger/log_reader.go             # Log file reading

# Entry points
cmd/server/main.go                    # Server with daemon mode
cmd/cli/main.go                       # CLI with auto-start server
```

---

## 🧪 Testing Patterns

### Unit Test
```go
func TestDownload_MarkCompleted(t *testing.T) {
    download := domain.NewDownload("url", platform, mode)
    download.MarkCompleted("/path/file.mp4")
    assert.Equal(t, domain.StatusCompleted, download.Status)
}
```

### Run Specific Test
```bash
go test -v ./internal/domain -run TestDownload_MarkCompleted
```

---

## 📦 Dependencies

### Install External Tools
```bash
# macOS
brew install yt-dlp
brew install tdl

# Or download binaries
# yt-dlp: https://github.com/yt-dlp/yt-dlp
# tdl: https://github.com/iyear/tdl
```

### Go Dependencies
```bash
go mod download
go mod tidy
```

---

## 🌐 URLs

```
Dashboard:  http://localhost:9091          # Embedded Next.js SPA
API:        http://localhost:9091/api/v1
Health:     http://localhost:9091/health
Ready:      http://localhost:9091/ready
```

---

## 📝 Git Commit Messages

### Format
```
<type>(<scope>): <subject>
```

### Types
```
feat      # New feature
fix       # Bug fix
docs      # Documentation
style     # Formatting
refactor  # Code restructuring
perf      # Performance
test      # Tests
chore     # Maintenance
ci        # CI/CD
```

### Examples
```bash
git commit -m "feat(api): add pause download endpoint"
git commit -m "fix(queue): prevent duplicate processing"
git commit -m "docs(readme): update installation steps"
git commit -m "refactor(downloader): simplify retry logic"
git commit -m "test(domain): add download state tests"
git commit -m "chore(deps): update gin to v1.9.1"
```

### Rules
- ≤ 50 characters
- Imperative mood
- No period at end
- Capitalize first letter
- One logical change per commit

---

**End of Cheatsheet**

