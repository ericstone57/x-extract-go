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
- IsTerminal() bool
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

// Loaded via Viper from configs/config.yaml
```

---

## Application Services

### DownloadManager (`internal/app/download_manager.go`)
**Responsibility**: Orchestrate download execution

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
```

**Flow**:
1. Acquire semaphore (concurrency control)
2. Update status to "processing"
3. Send start notification
4. Select platform downloader
5. Execute download with retry logic
6. Update status (completed/failed)
7. Send completion notification

### QueueManager (`internal/app/queue_manager.go`)
**Responsibility**: Manage download queue lifecycle

**Key Methods**:
```go
func NewQueueManager(
    repo domain.DownloadRepository,
    downloadMgr *DownloadManager,
    config *domain.QueueConfig,
    logger *zap.Logger,
) *QueueManager

func (qm *QueueManager) Start(ctx context.Context) error
func (qm *QueueManager) Stop() error
func (qm *QueueManager) AddDownload(url, platform, mode) (*domain.Download, error)
func (qm *QueueManager) GetDownload(id string) (*domain.Download, error)
func (qm *QueueManager) ListDownloads(filters) ([]*domain.Download, error)
func (qm *QueueManager) GetStats() (*domain.DownloadStats, error)
func (qm *QueueManager) CancelDownload(id string) error
func (qm *QueueManager) RetryDownload(id string) error
```

**Background Worker**:
- Runs `processQueue()` goroutine
- Checks every `check_interval` (10s default)
- Fetches pending downloads
- Passes to DownloadManager

---

## Infrastructure Layer

### Platform Downloaders

#### TwitterDownloader (`internal/infrastructure/downloader_twitter.go`)
```go
type TwitterDownloader struct {
    config     *domain.TwitterConfig
    logger     *zap.Logger
    incomingDir string
    completedDir string
}

func (td *TwitterDownloader) Download(download *domain.Download) error
```
- Uses `yt-dlp` binary
- Cookie-based authentication
- Downloads to `incoming/`, moves to `completed/` on success
- Captures process output in `download.ProcessLog`

#### TelegramDownloader (`internal/infrastructure/downloader_telegram.go`)
```go
type TelegramDownloader struct {
    config     *domain.TelegramConfig
    logger     *zap.Logger
    incomingDir string
    completedDir string
}

func (td *TelegramDownloader) Download(download *domain.Download) error
```
- Uses `tdl` binary
- Profile-based authentication
- Supports group downloads
- Captures process output in `download.ProcessLog`

### Repository (`internal/infrastructure/repository_sqlite.go`)
```go
type SQLiteRepository struct {
    db *gorm.DB
}

// Implements domain.DownloadRepository interface
func (r *SQLiteRepository) Create(download *domain.Download) error
func (r *SQLiteRepository) Update(download *domain.Download) error
func (r *SQLiteRepository) FindByID(id string) (*domain.Download, error)
func (r *SQLiteRepository) FindAll(filters) ([]*domain.Download, error)
func (r *SQLiteRepository) FindPending() ([]*domain.Download, error)
func (r *SQLiteRepository) GetStats() (*domain.DownloadStats, error)
func (r *SQLiteRepository) Delete(id string) error
```

### Notification Service (`internal/infrastructure/notification.go`)
```go
type NotificationService struct {
    config *domain.NotificationConfig
    logger *zap.Logger
}

func (ns *NotificationService) NotifyDownloadStarted(url, platform)
func (ns *NotificationService) NotifyDownloadCompleted(url, platform)
func (ns *NotificationService) NotifyDownloadFailed(url, platform, err)
```
- Uses macOS `osascript` for notifications
- Configurable sound alerts

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
- `GET /api/v1/logs/categories` → `logHandler.GetCategories`
- `GET /api/v1/logs/:category` → `logHandler.GetLogs`
- `GET /api/v1/logs/:category/search` → `logHandler.SearchLogs`
- `GET /api/v1/logs/:category/export` → `logHandler.ExportLogs`
- `GET /` → Web UI (index.html)
- `GET /logs` → Log viewer page
- `GET /static/*` → Static assets

### Handlers

#### DownloadHandler (`api/handlers/download_handler.go`)
```go
type DownloadHandler struct {
    queueMgr    *app.QueueManager
    downloadMgr *app.DownloadManager
    logger      *zap.Logger
}

// HTTP handlers for download operations
```

#### HealthHandler (`api/handlers/health_handler.go`)
```go
type HealthHandler struct {
    queueMgr *app.QueueManager
}

// Health check endpoints
```

#### LogHandler (`api/handlers/log_handler.go`)
```go
type LogHandler struct {
    logsDir string
}

// HTTP handlers for log viewing and export
```

### Middleware (`api/middleware/`)
- `logging.go` - Request/response logging
- `cors.go` - CORS headers
- `recovery.go` - Panic recovery

---

## Utilities

### Logger (`pkg/logger/logger.go`)
```go
func NewLogger(config *domain.LoggingConfig) (*zap.Logger, error)
```
- Supports JSON and console formats
- Configurable log levels
- Output to file or stdout

### Validator (`pkg/validator/validator.go`)
```go
func ValidateURL(url string) error
func ValidatePlatform(platform string) error
func ValidateMode(mode string) error
```

---

## Data Flow Examples

### Adding a Download (API → Database)
```
1. POST /api/v1/downloads
   ↓
2. downloadHandler.AddDownload()
   ↓
3. queueMgr.AddDownload(url, platform, mode)
   ↓
4. domain.NewDownload() - creates entity
   ↓
5. repo.Create(download) - saves to SQLite
   ↓
6. Returns download ID to client
```

### Processing Queue (Background Worker)
```
1. queueMgr.processQueue() - runs every 10s
   ↓
2. repo.FindPending() - fetch queued downloads
   ↓
3. For each download:
   ↓
4. downloadMgr.ProcessDownload(download)
   ↓
5. Acquire semaphore slot
   ↓
6. repo.Update(status="processing")
   ↓
7. downloader.Download(download) - execute yt-dlp/tdl
   ↓
8. repo.Update(status="completed"/"failed")
   ↓
9. notifier.Notify(result)
```

---

## Testing Structure

### Unit Tests
- `internal/domain/download_test.go` - Download entity tests
- `internal/domain/config_test.go` - Config validation tests
- `internal/app/*_test.go` - Service layer tests

### Integration Tests
- `test/integration/` - End-to-end tests
- `test/fixtures/` - Test data

### Running Tests
```bash
make test              # All tests
make test-coverage     # With coverage report
```

---

## Configuration Loading

**Flow** (`cmd/server/main.go`):
```
1. viper.SetConfigName("config")
2. viper.AddConfigPath("./configs")
3. viper.AutomaticEnv() - environment variables
4. viper.ReadInConfig()
5. viper.Unmarshal(&config)
6. Expand environment variables ($HOME, etc.)
```

**Environment Variable Override**:
- Format: `X_EXTRACT_<SECTION>_<KEY>`
- Example: `X_EXTRACT_SERVER_PORT=9090`

---

## Dependency Injection

**Server Initialization** (`cmd/server/main.go`):
```
Config → Logger → Repository → Downloaders → Notifier
                                     ↓
                              DownloadManager
                                     ↓
                               QueueManager
                                     ↓
                                  Router
                                     ↓
                                HTTP Server
```

All dependencies injected via constructors (no globals).

---

**End of Codebase Map**

