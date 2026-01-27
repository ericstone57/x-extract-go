# X-Extract Go - Quick Cheatsheet

**Ultra-fast reference for common tasks**

---

## 🚀 Quick Commands

### Build & Run
```bash
make build              # Build both server and CLI
make run-server         # Build and run server
./bin/x-extract-server  # Run server directly
./bin/x-extract-cli     # Run CLI directly
```

### Testing
```bash
make test               # Run all tests
make test-coverage      # Generate coverage report
make lint               # Run linters
make fmt                # Format code
```

### Docker
```bash
make docker-build       # Build image
make docker-up          # Start services
make docker-down        # Stop services
make docker-logs        # View logs
```

### Cleanup
```bash
make clean              # Remove build artifacts
```

---

## 📡 API Quick Reference

### Base URL
```
http://localhost:8080/api/v1
```

### Add Download
```bash
curl -X POST http://localhost:8080/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://x.com/user/status/123",
    "platform": "x",
    "mode": "default"
  }'
```

### List Downloads
```bash
curl http://localhost:8080/api/v1/downloads
```

### Get Stats
```bash
curl http://localhost:8080/api/v1/downloads/stats
```

### Get Download
```bash
curl http://localhost:8080/api/v1/downloads/{id}
```

### Cancel Download
```bash
curl -X POST http://localhost:8080/api/v1/downloads/{id}/cancel
```

### Retry Download
```bash
curl -X POST http://localhost:8080/api/v1/downloads/{id}/retry
```

### Health Check
```bash
curl http://localhost:8080/health
```

---

## 💻 CLI Quick Reference

### Add Download
```bash
./bin/x-extract-cli add "https://x.com/user/status/123"
./bin/x-extract-cli add "https://t.me/channel/123" --mode single
```

### List Downloads
```bash
./bin/x-extract-cli list
./bin/x-extract-cli list --status completed
./bin/x-extract-cli list --status failed
```

### Get Download
```bash
./bin/x-extract-cli get <download-id>
```

### Stats
```bash
./bin/x-extract-cli stats
```

### Cancel
```bash
./bin/x-extract-cli cancel <download-id>
```

### Retry
```bash
./bin/x-extract-cli retry <download-id>
```

### Custom Server
```bash
./bin/x-extract-cli --server http://localhost:9090 list
```

---

## 🗂️ File Locations

```
Config:          configs/config.yaml
Local Config:    ~/Downloads/x-download/config/local.yaml (optional)
Database:        ~/Downloads/x-download/config/queue.db
Completed Files: ~/Downloads/x-download/completed/
Incoming Files:  ~/Downloads/x-download/incoming/
Cookies:         ~/Downloads/x-download/cookies/
Logs:            ~/Downloads/x-download/logs/ (when output_path: auto)
Binaries:        bin/x-extract-server, bin/x-extract-cli
```

---

## 🔧 Configuration Quick Edit

```yaml
# configs/config.yaml

server:
  port: 8080                    # Change server port

download:
  base_dir: $HOME/Downloads/x-download  # Base directory
  completed_dir: .../completed  # Completed downloads
  incoming_dir: .../incoming    # Downloads in progress
  max_retries: 3                # Retry attempts
  concurrent_limit: 1           # Parallel downloads
  auto_start_workers: true      # Auto-start queue

queue:
  check_interval: 10s           # Queue check frequency
  auto_exit_on_empty: false     # Exit when queue empty

logging:
  level: info                   # debug|info|warn|error
  format: console               # console|json
  output_path: auto             # stdout|auto|<file path>
```

**Logging Modes**:
- `stdout`: Console only (default)
- `auto`: Date-based logs (`YYYYMMDD.log`)
- `--multi-logger`: Topic-based logs (run server with flag)

---

## 🐛 Quick Debugging

### Check Queue Status
```bash
# Via CLI
./bin/x-extract-cli stats

# Via SQLite
sqlite3 ~/Downloads/x-download/config/queue.db \
  "SELECT id, url, status, error_message FROM downloads;"
```

### Enable Debug Logging
```yaml
# configs/config.yaml
logging:
  level: debug
```

### Test External Binaries
```bash
# Test yt-dlp
yt-dlp --version
yt-dlp "https://x.com/..."

# Test tdl
tdl version
tdl dl -u "https://t.me/..."
```

### Common Issues

**Queue not processing?**
```yaml
download:
  auto_start_workers: true  # Must be true
```

**Download fails?**
```bash
# Check binary exists
which yt-dlp
which tdl

# Check logs
./bin/x-extract-server  # Watch output
```

**Cookie errors?**
```bash
# Export fresh cookies from browser
# Place in cookies/x.com/ directory
twitter:
  cookie_file: $HOME/Downloads/x-download/cookies/x.com/default.cookie
```

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
internal/domain/download.go
internal/domain/config.go

# Services
internal/app/download_manager.go
internal/app/queue_manager.go

# Downloaders
internal/infrastructure/downloader_twitter.go
internal/infrastructure/downloader_telegram.go

# API
api/router.go
api/handlers/download_handler.go
api/handlers/log_handler.go

# Entry points
cmd/server/main.go
cmd/cli/main.go
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
Web UI:     http://localhost:8080
API:        http://localhost:8080/api/v1
Health:     http://localhost:8080/health
Ready:      http://localhost:8080/ready
```

---

## 🎯 Environment Variables

```bash
# Override config values
export X_EXTRACT_SERVER_PORT=9090
export X_EXTRACT_LOGGING_LEVEL=debug
export X_EXTRACT_DOWNLOAD_CONCURRENT_LIMIT=3

# Run server
./bin/x-extract-server
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

