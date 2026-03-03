# Development Notes - X-Extract Go

**Working notes and decisions for development**

---

## Current State (2026-02-23)

### Implemented Features ✅
- [x] Core download management system
- [x] Queue processing with background workers
- [x] Per-platform parallel downloads (semaphore per platform, limit=1)
- [x] X/Twitter support via yt-dlp
- [x] Telegram support via tdl (with channel/message caching)
- [x] REST API with Gin (including DELETE endpoint)
- [x] CLI tool with Cobra (auto-starts server)
- [x] Next.js web dashboard (embedded in binary via go:embed)
- [x] SQLite persistence (downloads, channels, message cache)
- [x] Desktop notifications (macOS osascript + Linux notify-send)
- [x] Retry logic with exponential backoff
- [x] Download cancellation support
- [x] Topic-based structured logging (MultiLogger)
- [x] WebSocket real-time log streaming
- [x] Log viewing/search/export API
- [x] Daemon mode server (auto-forks to background)
- [x] XDG Base Directory compliant configuration
- [x] Config cascade (defaults → system → user override)
- [x] Docker deployment (multi-platform)
- [x] Unit tests for domain layer
- [x] API documentation
- [x] Configuration via YAML (Viper)
- [x] Download priority support
- [x] Auto-exit when queue empties
- [x] Orphaned download recovery on startup
- [x] Telegram metadata regeneration command

### Known Limitations
- No authentication/authorization
- Single-machine deployment only
- No distributed queue support
- Limited error recovery for external binaries
- No bandwidth limiting
- No pause/resume for in-progress downloads

---

## Design Decisions

### Why Clean Architecture?
- **Testability**: Domain logic isolated from infrastructure
- **Flexibility**: Easy to swap implementations (e.g., different database)
- **Maintainability**: Clear separation of concerns
- **Scalability**: Can evolve each layer independently

### Why SQLite?
- **Simplicity**: No separate database server needed
- **Portability**: Single file, easy to backup
- **Performance**: Sufficient for single-machine use
- **Reliability**: ACID compliance
- **Future**: Can migrate to PostgreSQL if needed

### Why Gin Framework?
- **Performance**: Fast HTTP router
- **Middleware**: Rich ecosystem
- **Simplicity**: Easy to learn and use
- **Community**: Well-maintained, popular

### Why External Binaries (yt-dlp, tdl)?
- **Expertise**: These tools are battle-tested
- **Maintenance**: Don't reinvent the wheel
- **Features**: Rich feature sets (formats, quality, etc.)
- **Updates**: Platform changes handled by tool maintainers

### Per-Platform Concurrency Model
- **Per-platform semaphores**: Each platform (x, telegram) has a semaphore with limit=1
- **Cross-platform parallelism**: Different platforms download simultaneously
- **Same-platform serialization**: Same-platform downloads are serialized
- **Goroutines**: One per download in progress
- **Context**: Graceful shutdown support with cancellation checks
- **Channels**: Communication between queue and workers, auto-exit signaling

### XDG Configuration
- System config at `~/.config/x-extract-go/config.yaml` (standard location)
- Database at `~/.config/x-extract-go/queue.db` (separate from data dir)
- Docker override: `/app/config/` when `IsDocker()` detects container
- Config cascade: hardcoded → system → user override

### Daemon Mode
- Server starts as daemon by default (forks to background)
- Parent process starts child with `-server-mode` flag and exits
- Child detaches with `Setsid: true`, redirects I/O to `/dev/null`
- CLI auto-starts server if not running (locates binary, starts, waits for `/health`)

### Topic-Based Logging (MultiLogger)
- Always enabled (no flags needed)
- Categories: `queue` (JSON), `error` (JSON), `download` (raw text), `stderr` (raw text)
- Date-based files: `{category}-YYYYMMDD.log` in `$base_dir/logs/`
- `LoggerAdapter` wraps MultiLogger for backward compatibility with `*zap.Logger`

---

## Code Patterns

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process download: %w", err)
}

// Log before returning
if err := someOperation(); err != nil {
    logger.Error("Operation failed", zap.Error(err))
    return err
}
```

### Logging
```go
// Use structured logging
logger.Info("Download started",
    zap.String("id", download.ID),
    zap.String("url", download.URL),
    zap.String("platform", string(download.Platform)))

// Log levels:
// - Debug: Verbose internal state
// - Info: Normal operations
// - Warn: Recoverable issues
// - Error: Operation failures
// - Fatal: Unrecoverable errors (app exit)
```

### Repository Pattern
```go
// Interface in domain layer
type DownloadRepository interface {
    Create(*Download) error
    Update(*Download) error
    FindByID(string) (*Download, error)
    FindByURL(string, []DownloadStatus) (*Download, error)
    FindPending() ([]*Download, error)
    ResetOrphanedProcessing() error
    // ...
}

// Implementation in infrastructure layer
type SQLiteDownloadRepository struct {
    db *gorm.DB
}
// Also implements TelegramChannelRepository and TelegramMessageCacheRepository

// Injected via constructor
func NewQueueManager(repo domain.DownloadRepository, ..., multiLogger *logger.MultiLogger) *QueueManager
```

### Configuration
```go
// XDG-based loading via app.LoadConfig()
// 1. domain.DefaultConfig() → hardcoded defaults
// 2. domain.DefaultConfigPath() → ~/.config/x-extract-go/config.yaml
// 3. If config file missing → createDefaultConfigFile()
// 4. viper.ReadInConfig() → merge system config
// 5. viper.Unmarshal(&config)
// 6. expandPaths() → resolve $HOME, ~ in paths
// 7. Check $base_dir/config/config.yaml → merge user override if exists
```

### Directory Structure
```
~/.config/x-extract-go/           # XDG config directory
├── config.yaml                    # System config
└── queue.db                       # SQLite database

$HOME/Downloads/x-download/        # Data directory
├── cookies/                       # Authentication files
│   ├── x.com/                    # Twitter/X cookies
│   └── telegram/default/         # Telegram storage (tdl profiles)
├── completed/                     # Successfully downloaded files
├── incoming/                      # Files being downloaded (temp)
├── logs/                          # Topic-based log files
│   ├── queue-YYYYMMDD.log        # Queue lifecycle (JSON)
│   ├── error-YYYYMMDD.log        # Application errors (JSON)
│   ├── download-YYYYMMDD.log     # Raw downloader output
│   └── stderr-YYYYMMDD.log       # Raw downloader stderr
└── config/                        # User override configuration
    └── config.yaml                # Optional overrides
```

**Migration**: Old installations are automatically migrated via `app.MigrateOldStructure()`.
- Media files → `completed/`
- Cookie files → `cookies/x.com/`
- `tdl-*` dirs → `cookies/telegram/`

### Git Commit Messages
**Follow Conventional Commits standard**

**Format**: `<type>(<scope>): <subject>`

**Types**:
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation
- `style` - Formatting
- `refactor` - Code restructuring
- `perf` - Performance
- `test` - Tests
- `chore` - Maintenance

**Rules**:
- ≤ 50 characters for subject
- Imperative mood ("add" not "added")
- No period at end
- Capitalize first letter
- Atomic commits (one logical change)

**Good Examples**:
```
feat(api): add pause download endpoint
fix(queue): prevent duplicate processing
docs(readme): update installation steps
refactor(downloader): simplify retry logic
test(domain): add download state tests
chore(deps): update gin to v1.9.1
perf(queue): optimize pending query
```

**Bad Examples** (avoid):
```
❌ Updated stuff
❌ Fixed bug
❌ WIP
❌ Fixed the issue where downloads were failing
```

---

## Common Development Tasks

### Adding a New Download Platform

1. **Add Platform Constant** (`internal/domain/download.go`)
```go
const (
    PlatformX        Platform = "x"
    PlatformTelegram Platform = "telegram"
    PlatformYouTube  Platform = "youtube" // NEW
)
```

2. **Create Downloader** (`internal/infrastructure/downloader_youtube.go`)
   Must implement `domain.Downloader` interface (Download, Platform, Validate):
```go
type YouTubeDownloader struct {
    config       *domain.YouTubeConfig
    logsDir      string
    multiLogger  *logger.MultiLogger
    incomingDir  string
    completedDir string
}

func (yd *YouTubeDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) error {
    // Implementation - call progressCallback with output lines
}
func (yd *YouTubeDownloader) Platform() domain.Platform { return domain.PlatformYouTube }
func (yd *YouTubeDownloader) Validate(url string) error { /* validate URL */ }
```

3. **Add Config** (`internal/domain/config.go`)
```go
type Config struct {
    // ...
    YouTube YouTubeConfig `mapstructure:"youtube"`
}
```

4. **Register Downloader** (`cmd/server/main.go`)
   The platform semaphore is auto-registered in `NewDownloadManager` for each entry:
```go
downloaders := map[domain.Platform]domain.Downloader{
    domain.PlatformX:        twitterDownloader,
    domain.PlatformTelegram: telegramDownloader,
    domain.PlatformYouTube:  youtubeDownloader, // NEW → auto gets semaphore(limit=1)
}
```

5. **Update Platform Detection** (`internal/domain/download.go`)
```go
func DetectPlatform(url string) Platform {
    if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
        return PlatformYouTube
    }
    // ...
}
```

6. **Update Platform Validation** (`internal/domain/download.go`)
```go
func ValidatePlatform(platform string) (Platform, error) {
    // Add "youtube" case
}
```

### Adding a New API Endpoint

1. **Add Handler Method** (`api/handlers/download_handler.go`)
```go
func (h *DownloadHandler) PauseDownload(c *gin.Context) {
    id := c.Param("id")
    // Implementation
    c.JSON(200, gin.H{"status": "paused"})
}
```

2. **Register Route** (`api/router.go`)
```go
downloads.POST("/:id/pause", downloadHandler.PauseDownload)
```

3. **Update Documentation** (`docs/API.md`)

### Modifying Download Entity

1. **Update Struct** (`internal/domain/download.go`)
```go
type Download struct {
    // ...
    Priority int `json:"priority"` // NEW
}
```

2. **Update Constructor**
```go
func NewDownload(url string, platform Platform, mode DownloadMode) *Download {
    return &Download{
        // ...
        Priority: 0, // NEW
    }
}
```

3. **Update Repository** (if schema changes)
```go
// GORM auto-migrates, but verify
db.AutoMigrate(&domain.Download{})
```

4. **Update Tests**

---

## Testing Guidelines

### Unit Test Structure
```go
func TestDownload_MarkCompleted(t *testing.T) {
    // Arrange
    download := domain.NewDownload("https://x.com/test", domain.PlatformX, domain.ModeDefault)
    
    // Act
    download.MarkCompleted("/path/to/file.mp4")
    
    // Assert
    assert.Equal(t, domain.StatusCompleted, download.Status)
    assert.Equal(t, "/path/to/file.mp4", download.FilePath)
    assert.NotNil(t, download.CompletedAt)
}
```

### Mock Repository
```go
type MockRepository struct {
    mock.Mock
}

func (m *MockRepository) Create(download *domain.Download) error {
    args := m.Called(download)
    return args.Error(0)
}

// Usage in tests
repo := new(MockRepository)
repo.On("Create", mock.Anything).Return(nil)
```

### Integration Test Pattern
```go
func TestDownloadFlow(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Create real components
    repo := persistence.NewSQLiteRepository(db)
    // ...
    
    // Test full flow
    download, err := queueMgr.AddDownload(testURL, platform, mode)
    assert.NoError(t, err)
    
    // Verify in database
    found, err := repo.FindByID(download.ID)
    assert.NoError(t, err)
    assert.Equal(t, download.ID, found.ID)
}
```

---

## Debugging Tips

### Enable Debug Logging
```yaml
# ~/.config/x-extract-go/config.yaml
logging:
  level: debug  # Change from info
```

### Check Queue Status
```bash
# Via CLI
./bin/x-extract-cli stats

# Via API
curl http://localhost:9091/api/v1/downloads/stats

# Via SQLite
sqlite3 ~/.config/x-extract-go/queue.db "SELECT * FROM downloads;"
```

### Monitor Logs
```bash
# Topic-based logs (always enabled)
tail -f ~/Downloads/x-download/logs/queue-$(date +%Y%m%d).log
tail -f ~/Downloads/x-download/logs/error-$(date +%Y%m%d).log

# Via API
curl http://localhost:9091/api/v1/logs/error?limit=20

# Docker logs
make docker-logs
```

### Test External Binaries
```bash
yt-dlp --version
yt-dlp --cookies ~/Downloads/x-download/cookies/x.com/default.cookie "https://x.com/..."

tdl version
tdl dl -u "https://t.me/..."
```

### Common Issues

**Queue not processing**:
- Check `auto_start_workers: true` in config
- Verify queue manager started: check queue logs for "Starting queue manager"
- Check database: `sqlite3 ~/.config/x-extract-go/queue.db "SELECT * FROM downloads WHERE status='queued';"`

**Download fails immediately**:
- Verify binary exists: `which yt-dlp` or `which tdl`
- Test binary manually with same URL
- Check error logs: `curl http://localhost:9091/api/v1/logs/error?limit=10`

**Cookie authentication fails**:
- Export fresh cookies from browser
- Verify cookie file path in config
- Check cookie file format (Netscape format)

**Server not starting**:
- Kill existing: `make kill-server`
- Check config: `cat ~/.config/x-extract-go/config.yaml`
- Run in foreground: `./bin/x-extract-server -server-mode` (skips daemon fork)

---

## Performance Considerations

### Concurrency Model
- `concurrent_limit` config is **deprecated** — per-platform semaphores (limit=1 each) are now used
- Each platform downloads independently (X and Telegram in parallel)
- Same-platform downloads are serialized to avoid rate limiting
- To increase parallelism for a platform, modify semaphore buffer size in `NewDownloadManager`

### Queue Check Interval
```yaml
queue:
  check_interval: 5s  # Decrease for faster response
```
- Lower = more responsive, but more CPU usage
- 10s is good balance for most cases

### Auto-Exit Behavior
```yaml
queue:
  auto_exit_on_empty: true   # Default: true
  empty_wait_time: 30s       # Wait before exit
```
- Server exits when queue empties and no new downloads arrive within `empty_wait_time`
- CLI auto-restarts server on next command

### Database Optimization
- SQLite is fast for single-machine use
- Consider WAL mode for better concurrency
- Regular VACUUM for maintenance

---

## Future Enhancements (Ideas)

### High Priority
- [x] ~~Progress tracking during download~~ ✅ (DownloadProgressCallback)
- [ ] Pause/resume functionality
- [x] ~~Download priority queue~~ ✅ (Priority field)
- [ ] Bandwidth limiting
- [x] ~~Cross-platform notifications~~ ✅ (macOS + Linux)

### Medium Priority
- [ ] Authentication/authorization
- [ ] User management
- [ ] Download history cleanup
- [ ] Scheduled downloads
- [ ] Webhook notifications

### Low Priority
- [ ] Distributed queue (Redis/RabbitMQ)
- [ ] Metrics/monitoring (Prometheus)
- [x] ~~Admin dashboard~~ ✅ (Next.js embedded dashboard)
- [ ] Plugin system for custom downloaders
- [ ] Cloud storage integration (S3, etc.)

---

**End of Development Notes**

