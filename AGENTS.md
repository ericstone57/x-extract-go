# AGENTS.md

**Project-specific guidelines for AI coding agents**

---

## Build, Lint, and Test Commands

### Core Commands
```bash
make build              # Build server + CLI + Next.js dashboard
make build-dashboard    # Build Next.js dashboard only
make run-server         # Build and run server
make run-cli           # Build and run CLI
make test              # Run all tests with race detection and coverage
```

### Single Test
```bash
# Run specific test file
go test -v ./internal/domain/

# Run specific test function
go test -v ./internal/domain/ -run TestDownload_MarkProcessing

# Run tests in specific package with coverage
go test -v -cover ./api/handlers/
```

### Lint and Format
```bash
make lint              # Run go vet and check formatting
make fmt               # Format all Go code
make clean             # Clean build artifacts
```

### Docker
```bash
make docker-build      # Build Docker image
make docker-up         # Start Docker Compose services
make docker-logs       # View Docker logs
```

### Dependencies
```bash
make deps              # Download and tidy Go modules
make install-tools    # Install dev tools (golangci-lint)
```

---

## Code Style Guidelines

### Imports
- Use grouped imports with standard library first, then third-party
- Separate with blank line between groups
- Alphabetical within groups
```go
import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "go.uber.org/zap"
)
```

### Naming Conventions
- **Packages**: lowercase, short, descriptive (`api`, `domain`, `infra`)
- **Interfaces**: noun + er suffix (`Downloader`, `Repository`)
- **Structs**: PascalCase, clear domain meaning (`Download`, `QueueManager`)
- **Variables/Fields**: camelCase, avoid abbreviations
- **Constants**: SCREAMING_Snake_Case for values, camelCase for function-scoped
- **Error variables**: `Err` prefix (`ErrNotFound`)
- **Config structs**: `Config` suffix per domain

### Error Handling
- Return errors to caller; avoid logging at low levels
- Use `fmt.Errorf` with context: `fmt.Errorf("failed to %s: %w", action, err)`
- Wrap errors with `%w` for wrapping
- Check errors immediately with early return
```go
if err != nil {
    return fmt.Errorf("create download: %w", err)
}
```

### Types and Constants
- Use typed constants for status/enum values
- Define constants in domain package
- String constants for user-facing values
```go
type DownloadStatus string

const (
    StatusQueued     DownloadStatus = "queued"
    StatusProcessing DownloadStatus = "processing"
    StatusCompleted  DownloadStatus = "completed"
    StatusFailed     DownloadStatus = "failed"
)
```

### GORM Models
- JSON tags: `json:"field_name"` (camelCase)
- GORM tags: `gorm:"primaryKey"` for IDs, `gorm:"not null;index"` for constraints
- Use pointer types for nullable fields (`*time.Time`)
- JSON omitempty for optional fields

### Configuration
- Use viper with `mapstructure` tags
- Environment variables: `VARIABLE_NAME` in config.yaml
- Path expansion via `$HOME` in config
- Dynamic subdirectory paths computed from BaseDir

### Logging
- Use uber/zap for structured logging
- Create logger in `pkg/logger/`
- Request context logging via middleware
- Multi-logger for topic-based logs (download, queue, error)

### HTTP Handlers (Gin)
- Return JSON responses: `c.JSON(http.StatusOK, gin.H{"key": "value"}))
- Use `c.Param("id")` for path params, `c.Query("q")` for query params
- Bind and validate request bodies early
- Centralized error handling via middleware

### Web Dashboard (Next.js)
- Located in `web-dashboard/`
- Static export with `bun run build`
- Embedded via `embed.go` in Go binary
- API calls to `/api/v1/*` endpoints

---

## Project Structure

```
x-extract-go/
├── api/                    # HTTP handlers, router, middleware
├── cmd/
│   ├── server/main.go     # Server entrypoint
│   └── cli/               # CLI commands
├── configs/               # Config.yaml
├── deployments/docker/    # Dockerfile, docker-compose
├── internal/
│   ├── app/               # Services (QueueManager, DownloadManager)
│   ├── domain/            # Models, config, repository interfaces
│   └── infrastructure/    # Downloaders, SQLite repo
├── pkg/logger/            # Logging utilities
└── web-dashboard/        # Next.js frontend
```

---

## Key Patterns

1. **Per-platform download semaphores**: Each platform (X, Telegram) has limit=1, allowing parallel different-platform downloads
2. **Queue persistence**: SQLite with GORM for download queue
3. **Config cascade**: `configs/config.yaml` → `base_dir/config/local.yaml`
4. **Multi-logger**: Topic-based logs in `logs_dir/{topic}-YYYYMMDD.log`
5. **Embedded dashboard**: Next.js static export embedded via `go:embed`
