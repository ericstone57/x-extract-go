# Codebase Map - X-Extract Go

**Quick reference for navigating the codebase**

---

## Core Domain Models

### Download Entity (`internal/domain/download.go`)
```go
type Download struct {
    ID           string
    URL          string
    Platform     Platform       // "x" or "telegram"
    Mode         DownloadMode   // "default", "single", "group"
    Status       DownloadStatus // "queued", "processing", "completed", "failed", "cancelled"
    Priority     int            // Higher = processed first (default 0)
    FilePath     string
    ErrorMessage string
    ProcessLog   string         // Process output (yt-dlp/tdl)
    RetryCount   int
    Metadata     string         // JSON metadata
    CreatedAt    time.Time
    UpdatedAt    time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
}

// Key Methods:
- NewDownload(url, platform, mode) *Download
- MarkProcessing()
- MarkCompleted(filePath)
- MarkFailed(err)
- IncrementRetry()
- CanRetry(maxRetries) bool
- IsPending() bool
- IsProcessing() bool
- IsTerminal() bool

// Helper Functions:
- DetectPlatform(url string) Platform     // URL pattern matching
- ValidatePlatform(platform string) error
- ValidateMode(mode string) error
```

### Downloader Interface (`internal/domain/downloader.go`)
```go
type DownloadProgressCallback func(output string, percent float64)

type Downloader interface {
    Download(download *Download, progressCallback DownloadProgressCallback) (*DownloadResult, error)
}

type DownloadResult struct {
    FilePath string
    Metadata string
    Error    error
}
```

### Repository Interfaces (`internal/domain/repository.go`)
```go
type DownloadRepository interface {
    Create(download *Download) error
    Update(download *Download) error
    FindByID(id string) (*Download, error)
    FindByURL(url string, excludeStatuses []DownloadStatus) (*Download, error)
    FindAll(status *DownloadStatus) ([]*Download, error)
    FindPending() ([]*Download, error)
    FindByStatus(status DownloadStatus) ([]*Download, error)
    CountActive() (int64, error)
    GetStats() (*DownloadStats, error)
    Delete(id string) error
    ResetOrphanedProcessing() error
}
```

### Telegram Models (`internal/domain/telegram_channel.go`, `telegram_message_cache.go`)
```go
type TelegramChannel struct {
    ChannelID   int64  // Primary key
    ChannelName string
    ChannelType string
    Username    string
}

type TelegramMessageCache struct {
    ChannelID int64  // Composite PK with MessageID
    MessageID int64
    Text      string
    Date      time.Time
    SenderID  int64
    SenderName string
    MediaType  string
    CachedAt   time.Time
}
```

### Configuration (`internal/domain/config.go`)
```go
type Config struct {
    Server       ServerConfig
    Download     DownloadConfig
    Queue        QueueConfig
    Telegram     TelegramConfig
    Twitter      TwitterConfig
    Notification NotificationConfig
    Logging      LoggingConfig
}

// XDG path helpers:
- DefaultConfigDir() string   // ~/.config/x-extract-go (or /app/config in Docker)
- DefaultConfigPath() string  // ~/.config/x-extract-go/config.yaml
- DefaultQueueDBPath() string // ~/.config/x-extract-go/queue.db
- IsDocker() bool
- DefaultConfig() *Config     // Returns hardcoded defaults
```

---

## Application Services

### DownloadManager (`internal/app/download_manager.go`)
**Responsibility**: Orchestrate download execution with per-platform concurrency

**Key Fields**:
```go
type DownloadManager struct {
    platformSemaphores map[domain.Platform]chan struct{} // limit=1 per platform
    // ... repo, downloaders, notifier, config, logger
}
```

**Key Methods**:
```go
func NewDownloadManager(
    repo domain.DownloadRepository,
    downloaders map[domain.Platform]domain.Downloader,
    notifier *infrastructure.NotificationService,
    config *domain.DownloadConfig,
    logger *zap.Logger,
) *DownloadManager

func (dm *DownloadManager) ProcessDownload(ctx context.Context, download *domain.Download) error
func (dm *DownloadManager) CancelDownload(id string) error
func (dm *DownloadManager) RetryDownload(ctx context.Context, id string) error
```

**Flow**:
1. Acquire **per-platform semaphore** (limit=1; different platforms run in parallel)
2. Check cancellation
3. Update status to "processing"
4. Send start notification
5. Select platform downloader
6. Execute download with `DownloadProgressCallback`
7. Check cancellation after download
8. Update status (completed/failed)
9. Send completion notification
10. Release platform semaphore

### QueueManager (`internal/app/queue_manager.go`)
**Responsibility**: Manage download queue lifecycle

**Key Methods**:
```go
func NewQueueManager(
    repo domain.DownloadRepository,
    downloadMgr *DownloadManager,
    config *domain.QueueConfig,
    multiLogger *logger.MultiLogger,  // Topic-based logger (not *zap.Logger)
) *QueueManager

func (qm *QueueManager) Start(ctx context.Context) error
func (qm *QueueManager) Stop() error
func (qm *QueueManager) AddDownload(url, platform, mode string) (*domain.Download, error)
func (qm *QueueManager) GetDownload(id string) (*domain.Download, error)
func (qm *QueueManager) ListDownloads(status *domain.DownloadStatus) ([]*domain.Download, error)
func (qm *QueueManager) GetStats() (*domain.DownloadStats, error)
func (qm *QueueManager) DeleteDownload(id string) error
func (qm *QueueManager) WaitForExit() <-chan struct{}  // Auto-exit signal
```

**Background Worker**:
- Runs `processQueue()` goroutine
- Checks every `check_interval` (10s default)
- Fetches pending downloads (ordered by priority, then creation time)
- Spawns goroutine for each pending download → `DownloadManager.ProcessDownload()`
- Resets orphaned `processing` downloads on startup
- Auto-exit: signals when queue empty for `empty_wait_time`

### ConfigLoader (`internal/app/config_loader.go`)
**Responsibility**: XDG-based configuration loading

**Key Functions**:
```go
func LoadConfig() (*domain.Config, error)           // Main loading function
func SaveConfig(config *domain.Config) error
func MigrateOldStructure(config *domain.Config) error
```

**Loading Cascade**:
1. `domain.DefaultConfig()` → hardcoded defaults
2. `~/.config/x-extract-go/config.yaml` → system config (or `/app/config` in Docker)
3. `$base_dir/config/config.yaml` → user override (if exists)

---

## Infrastructure Layer

### Platform Downloaders

#### TwitterDownloader (`internal/infrastructure/downloader_twitter.go`)
```go
type TwitterDownloader struct {
    config       *domain.TwitterConfig
    logger       *zap.Logger
    incomingDir  string
    completedDir string
    logsDir      string               // Topic-based log output
    multiLogger  *logger.MultiLogger
}

func (td *TwitterDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) (*domain.DownloadResult, error)
```
- Uses `yt-dlp` binary
- Cookie-based authentication
- Downloads to `incoming/`, moves to `completed/` on success
- Writes stdout to `download-YYYYMMDD.log`, stderr to `stderr-YYYYMMDD.log`

#### TelegramDownloader (`internal/infrastructure/downloader_telegram.go`)
```go
type TelegramDownloader struct {
    config       *domain.TelegramConfig
    logger       *zap.Logger
    incomingDir  string
    completedDir string
    logsDir      string
    multiLogger  *logger.MultiLogger
    channelRepo  domain.TelegramChannelRepository      // Channel name caching
    messageRepo  domain.TelegramMessageCacheRepository  // Message metadata caching
}

func (td *TelegramDownloader) Download(download *domain.Download, progressCallback domain.DownloadProgressCallback) (*domain.DownloadResult, error)
func (td *TelegramDownloader) SetChannelRepository(repo domain.TelegramChannelRepository)
func (td *TelegramDownloader) SetMessageCacheRepository(repo domain.TelegramMessageCacheRepository)
```
- Uses `tdl` binary
- Profile-based authentication
- Supports group downloads and takeout mode
- Channel name caching (7-day refresh)
- Message metadata caching for smart incremental exports

### Repository (`internal/infrastructure/repository_sqlite.go`)
```go
type SQLiteDownloadRepository struct {
    db *gorm.DB
}

// Implements domain.DownloadRepository, domain.TelegramChannelRepository,
// and domain.TelegramMessageCacheRepository interfaces
func NewSQLiteDownloadRepository(dbPath string) (*SQLiteDownloadRepository, error)
// Auto-migrates: Download, TelegramChannel, TelegramMessageCache tables
```

### Notification Service (`internal/infrastructure/notification.go`)
```go
type NotificationService struct {
    config *domain.NotificationConfig
    logger *zap.Logger
}

func (ns *NotificationService) Send(title, message string) error
func (ns *NotificationService) NotifyDownloadQueued(url string, platform domain.Platform)
func (ns *NotificationService) NotifyDownloadStarted(url string, platform domain.Platform)
func (ns *NotificationService) NotifyDownloadCompleted(url string, platform domain.Platform)
func (ns *NotificationService) NotifyDownloadFailed(url string, platform domain.Platform, err error)
func (ns *NotificationService) NotifyQueueEmpty()
```
- macOS: `osascript` display notification
- Linux: `notify-send`

### Shell Utilities (`internal/infrastructure/shell_utils.go`)
- `ShellEscape(s string) string` - Safe escaping for shell command logging

---

## API Layer

### Router (`api/router.go`)
```go
func SetupRouterWithMultiLogger(
    queueMgr *app.QueueManager,
    downloadMgr *app.DownloadManager,
    logAdapter *logger.LoggerAdapter,
    logsDir string,
) *gin.Engine
```

**Routes**:
- `GET /health` → `healthHandler.Health`
- `GET /ready` → `healthHandler.Ready`
- `POST /api/v1/downloads` → `downloadHandler.AddDownload`
- `GET /api/v1/downloads` → `downloadHandler.ListDownloads`
- `GET /api/v1/downloads/stats` → `downloadHandler.GetStats`
- `GET /api/v1/downloads/:id` → `downloadHandler.GetDownload`
- `POST /api/v1/downloads/:id/cancel` → `downloadHandler.CancelDownload`
- `POST /api/v1/downloads/:id/retry` → `downloadHandler.RetryDownload`
- `DELETE /api/v1/downloads/:id` → `downloadHandler.DeleteDownload`
- `GET /api/v1/logs/categories` → `logHandler.GetCategories`
- `GET /api/v1/logs/:category` → `logHandler.GetLogs`
- `GET /api/v1/logs/:category/search` → `logHandler.SearchLogs`
- `GET /api/v1/logs/:category/export` → `logHandler.ExportLogs`
- `GET /` → Embedded Next.js dashboard (SPA with fallback to index.html)
- `GET /_next/*` → Next.js static assets

### Handlers

#### DownloadHandler (`api/handlers/download_handler.go`)
```go
type DownloadHandler struct {
    queueMgr    *app.QueueManager
    downloadMgr *app.DownloadManager
    logger      *zap.Logger
}
// Methods: AddDownload, ListDownloads, GetDownload, GetStats,
//          CancelDownload, RetryDownload, DeleteDownload
```

#### HealthHandler (`api/handlers/health_handler.go`)
```go
type HealthHandler struct {
    queueMgr *app.QueueManager
}
```

#### LogHandler (`api/handlers/log_handler.go`)
```go
type LogHandler struct {
    logReader *logger.LogReader
}
// Methods: GetLogs, SearchLogs, GetCategories, ExportLogs
// Categories: download, stderr, queue, error
```

#### LogWebSocketHandler (`api/handlers/log_websocket.go`)
```go
type LogWebSocketHandler struct {
    logReader *logger.LogReader
}
// Real-time log streaming via gorilla/websocket
```

### Middleware (`api/middleware/`)
- `logging.go` - Request/response logging
- `cors.go` - CORS headers
- `recovery.go` - Panic recovery

---

## Utilities

### MultiLogger (`pkg/logger/multi_logger.go`)
```go
type LogCategory string
const (
    CategoryQueue LogCategory = "queue"
    CategoryError LogCategory = "error"
)

type MultiLogger struct { /* topic-based JSON loggers */ }
func NewMultiLogger(logsDir string) (*MultiLogger, error)
func (ml *MultiLogger) LogQueue(msg string, fields ...zap.Field)
func (ml *MultiLogger) LogError(msg string, fields ...zap.Field)
```

### LoggerAdapter (`pkg/logger/adapter.go`)
```go
type LoggerAdapter struct { /* wraps MultiLogger */ }
func NewLoggerAdapter(ml *MultiLogger) *LoggerAdapter
func (la *LoggerAdapter) GetSingleLogger() *zap.Logger  // Returns error logger
```

### LogReader (`pkg/logger/log_reader.go`)
```go
type LogReader struct { logsDir string }
func (lr *LogReader) ReadLogs(category LogCategory, date time.Time, limit int) ([]LogEntry, error)
func (lr *LogReader) SearchLogs(category LogCategory, date time.Time, query string, limit int) ([]LogEntry, error)
func (lr *LogReader) TailLogs(category LogCategory, entryChan chan<- LogEntry, stopChan <-chan struct{}) error
```

### Base Logger (`pkg/logger/logger.go`)
```go
func NewLogger(config *domain.LoggingConfig) (*zap.Logger, error)
```

---

## Data Flow Examples

### Adding a Download (API → Database)
```
1. POST /api/v1/downloads {url, platform?, mode?}
   ↓
2. downloadHandler.AddDownload() - validates input
   ↓
3. queueMgr.AddDownload(url, platform, mode)
   ↓
4. DetectPlatform(url) if platform not specified
   ↓
5. domain.NewDownload() - creates entity with UUID, status=queued
   ↓
6. repo.Create(download) - saves to SQLite
   ↓
7. Returns download JSON to client
```

### Processing Queue (Background Worker)
```
1. queueMgr.processQueue() - runs every check_interval (10s)
   ↓
2. repo.FindPending() - fetch queued downloads (ordered by priority, created_at)
   ↓
3. For each download → spawn goroutine:
   ↓
4. downloadMgr.ProcessDownload(ctx, download)
   ↓
5. Acquire per-platform semaphore (limit=1)
   ↓
6. Check cancellation
   ↓
7. download.MarkProcessing() + repo.Update()
   ↓
8. downloader.Download(download, progressCallback)
   ↓
9. Check cancellation after download
   ↓
10. download.MarkCompleted()/MarkFailed() + repo.Update()
   ↓
11. notifier.NotifyDownloadCompleted/Failed()
   ↓
12. Release platform semaphore
```

---

## Testing Structure

### Unit Tests
- `internal/domain/download_test.go` - Download entity tests
- `internal/domain/config_test.go` - Config validation tests

### Running Tests
```bash
make test                          # All tests with race detection
go test -v ./internal/domain/      # Specific package
go test -v ./internal/domain/ -run TestDownload_MarkProcessing  # Specific test
```

---

## Configuration Loading

**Flow** (`internal/app/config_loader.go` → called by `cmd/server/main.go`):
```
1. domain.DefaultConfig() → hardcoded defaults
2. domain.DefaultConfigPath() → ~/.config/x-extract-go/config.yaml
3. If config file missing → createDefaultConfigFile()
4. viper.ReadInConfig() → merge system config
5. viper.Unmarshal(&config)
6. expandPaths() → resolve $HOME, ~ in paths
7. Check $base_dir/config/config.yaml → merge user override if exists
```

---

## Dependency Injection

**Server Initialization** (`cmd/server/main.go`):
```
LoadConfig() → MultiLogger → LoggerAdapter → SQLiteRepository
                                                    ↓
                                    NotificationService + Downloaders
                                                    ↓
                                    Set channel/message repos on TelegramDownloader
                                                    ↓
                                             DownloadManager (per-platform semaphores)
                                                    ↓
                                              QueueManager (with MultiLogger)
                                                    ↓
                                    SetupRouterWithMultiLogger (embedded dashboard)
                                                    ↓
                                              HTTP Server
                                                    ↓
                                    Wait: OS signal OR QueueManager.WaitForExit()
```

All dependencies injected via constructors (no globals).

---

**End of Codebase Map**

