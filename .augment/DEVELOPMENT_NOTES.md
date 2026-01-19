# Development Notes - X-Extract Go

**Working notes and decisions for development**

---

## Current State (2026-01-19)

### Implemented Features ✅
- [x] Core download management system
- [x] Queue processing with background workers
- [x] X/Twitter support via yt-dlp
- [x] Telegram support via tdl
- [x] REST API with Gin
- [x] CLI tool with Cobra
- [x] Web UI (basic)
- [x] SQLite persistence
- [x] macOS notifications
- [x] Retry logic with exponential backoff
- [x] Concurrent download limits
- [x] Structured logging
- [x] Docker deployment
- [x] Unit tests for domain/app layers
- [x] API documentation
- [x] Configuration via YAML

### Known Limitations
- macOS-only notifications (osascript)
- No authentication/authorization
- Single-machine deployment only
- No distributed queue support
- Limited error recovery for external binaries
- No progress tracking during download
- No bandwidth limiting

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

### Concurrency Model
- **Semaphore**: Limits concurrent downloads
- **Goroutines**: One per download in progress
- **Context**: Graceful shutdown support
- **Channels**: Communication between queue and workers

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
    // ...
}

// Implementation in infrastructure layer
type SQLiteRepository struct {
    db *gorm.DB
}

// Injected via constructor
func NewQueueManager(repo domain.DownloadRepository, ...) *QueueManager
```

### Configuration
```go
// Use Viper for flexibility
viper.SetConfigName("config")
viper.AddConfigPath("./configs")
viper.AutomaticEnv()

// Support environment variables
// XEXTRACT_SERVER_PORT overrides server.port

// Expand variables in config
os.ExpandEnv(config.Download.BaseDir) // $HOME/Downloads

// Configuration cascade:
// 1. Load configs/config.yaml
// 2. Merge config/local.yaml if exists
// 3. Apply environment variables
```

### Directory Structure
```
$HOME/Downloads/x-download/
├── cookies/              # Authentication files
│   ├── x.com/           # Twitter/X cookies
│   └── telegram/        # Telegram storage (tdl profiles)
├── completed/           # Successfully downloaded files
├── incoming/            # Files being downloaded (temp)
├── logs/                # Date-based log files (YYYYMMDD.log)
└── config/              # Configuration and database
    ├── queue.db         # SQLite database
    └── local.yaml       # Optional local config overrides
```

**Migration**: Old installations are automatically migrated on first run.
- Media files → `completed/`
- Cookie files → `cookies/x.com/`
- `queue.db` → `config/`
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

2. **Create Downloader** (`internal/infrastructure/downloaders/youtube.go`)
```go
type YouTubeDownloader struct {
    config *domain.YouTubeConfig
    logger *zap.Logger
}

func (yd *YouTubeDownloader) Download(download *domain.Download) error {
    // Implementation
}
```

3. **Add Config** (`internal/domain/config.go`)
```go
type Config struct {
    // ...
    YouTube YouTubeConfig `mapstructure:"youtube"`
}

type YouTubeConfig struct {
    Binary string `mapstructure:"binary"`
    // ...
}
```

4. **Register Downloader** (`cmd/server/main.go`)
```go
downloaders := map[domain.Platform]domain.Downloader{
    domain.PlatformX:        twitterDownloader,
    domain.PlatformTelegram: telegramDownloader,
    domain.PlatformYouTube:  youtubeDownloader, // NEW
}
```

5. **Update Platform Detection** (if needed)
```go
func DetectPlatform(url string) Platform {
    if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
        return PlatformYouTube
    }
    // ...
}
```

### Adding a New API Endpoint

1. **Add Handler Method** (`api/handlers/download.go`)
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
# configs/config.yaml
logging:
  level: debug  # Change from info
```

### Check Queue Status
```bash
# Via CLI
./bin/x-extract-cli stats

# Via API
curl http://localhost:8080/api/v1/downloads/stats

# Via SQLite
sqlite3 ~/Downloads/x-download/queue.db "SELECT * FROM downloads;"
```

### Monitor Logs
```bash
# Server logs (if running in terminal)
./bin/x-extract-server

# Docker logs
make docker-logs
```

### Test External Binaries
```bash
# Test yt-dlp
yt-dlp --version
yt-dlp --cookies ~/Downloads/x-download/x.com.cookie "https://x.com/..."

# Test tdl
tdl version
tdl dl -u "https://t.me/..."
```

### Common Issues

**Queue not processing**:
- Check `auto_start_workers: true` in config
- Verify queue manager started: check logs for "Starting queue manager"
- Check database: `sqlite3 queue.db "SELECT * FROM downloads WHERE status='queued';"`

**Download fails immediately**:
- Verify binary exists: `which yt-dlp` or `which tdl`
- Check binary permissions: `ls -l $(which yt-dlp)`
- Test binary manually with same URL
- Check logs for exact error message

**Cookie authentication fails**:
- Export fresh cookies from browser
- Verify cookie file path in config
- Check cookie file format (Netscape format)

---

## Performance Considerations

### Concurrency Tuning
```yaml
download:
  concurrent_limit: 3  # Increase for more parallelism
```
- Higher = faster overall, but more resource usage
- Consider network bandwidth
- Consider disk I/O

### Queue Check Interval
```yaml
queue:
  check_interval: 5s  # Decrease for faster response
```
- Lower = more responsive, but more CPU usage
- 10s is good balance for most cases

### Database Optimization
- SQLite is fast for single-machine use
- Consider WAL mode for better concurrency
- Regular VACUUM for maintenance

---

## Future Enhancements (Ideas)

### High Priority
- [ ] Progress tracking during download
- [ ] Pause/resume functionality
- [ ] Download priority queue
- [ ] Bandwidth limiting
- [ ] Cross-platform notifications (Linux, Windows)

### Medium Priority
- [ ] Authentication/authorization
- [ ] User management
- [ ] Download history cleanup
- [ ] Scheduled downloads
- [ ] Webhook notifications

### Low Priority
- [ ] Distributed queue (Redis/RabbitMQ)
- [ ] Metrics/monitoring (Prometheus)
- [ ] Admin dashboard
- [ ] Plugin system for custom downloaders
- [ ] Cloud storage integration (S3, etc.)

---

**End of Development Notes**

