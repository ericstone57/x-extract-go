# AGENTS.md - Development Guide for x-extract-go

This document provides guidelines for AI agents working on this codebase.

## Project Overview

x-extract-go is a Go application for downloading media from X/Twitter and Telegram. Features:
- REST API server (Gin framework)
- Download queue management with SQLite persistence
- Multi-platform support (X/Twitter via yt-dlp, Telegram via tdl)
- Topic-based logging system

## Build Commands

```bash
make build              # Build server + CLI (bin/x-extract-server, bin/x-extract-cli)
make run-server         # Build and run server
make run-cli            # Build and run CLI
make kill-server        # Kill running server
make restart-server-multi  # Kill and restart with multi-logger
```

## Test Commands

```bash
make test               # All tests with race detection and coverage
make test-coverage      # Generate HTML coverage report

# Single test: go test -v ./<package> -run <TestName>
go test -v ./internal/domain -run TestNewDownload

# Single test file: go test -v ./<package> -run TestFile
go test -v ./internal/domain -run TestDownload_MarkProcessing

go test -v ./...        # All tests verbose
go test -v -tags=integration ./test/integration/...  # Integration tests
```

## Linting & Formatting

```bash
make lint               # Run go vet + format check
make fmt                # Format all code (gofmt -w .)
make install-tools      # Install golangci-lint
make deps               # Download + tidy dependencies
```

## Code Style Guidelines

### Imports (3 groups, blank lines between)

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yourusername/x-extract-go/internal/domain"
)
```

### Types & Structs

- Use struct tags: `json:"field_name" gorm:"..."`
- Pointer types for optional fields: `*time.Time`
- Custom types for constants: `type DownloadStatus string`

### Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Packages | lowercase | `domain`, `infrastructure` |
| Variables | camelCase | `downloadManager` |
| Constants | camelCase or snake_case | `maxRetries` or `db_timeout` |
| Exported | PascalCase | `NewDownload`, `DownloadStatus` |
| Unexported | camelCase (no underscore) | `download` |
| Interfaces | Noun or -er | `Repository`, `Downloader` |
| Errors | Descriptive | `errNotFound` |

### Error Handling

```go
func FindByID(id string) (*Download, error) {
	var download Download
	err := db.First(&download, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("find download %s: %w", id, err)
	}
	return &download, nil
}
```
- Use `%w` for wrapped errors
- Check errors immediately
- Don't ignore with `_`

### Logging

Use `go.uber.org/zap` with structured fields:
```go
logger.Info("Processing download",
	zap.String("id", download.ID),
	zap.String("platform", string(download.Platform)),
	zap.Error(err))
```

### HTTP Handlers (Gin)

```go
func (h *Handler) CreateDownload(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}
```

## Project Structure

```
x-extract-go/
├── api/              # HTTP handlers and routing
├── cmd/              # Entry points (server, cli)
├── configs/          # Default configuration files
├── deployments/      # Docker configs
├── internal/
│   ├── app/          # Application services
│   ├── domain/       # Business models and interfaces
│   └── infrastructure/  # DB, downloader implementations
├── pkg/              # Shared packages (logger)
├── test/             # Integration tests
└── web/              # Web UI assets
```

## Key Patterns

- **Repository**: Define interfaces in `internal/domain/`, implement in `internal/infrastructure/`
- **Configuration**: Use Viper, prefix env vars with `XEXTRACT_`, defaults in `domain.DefaultConfig()`
- **Context**: Pass as first parameter, use `context.WithTimeout` for HTTP handlers
- **Database**: GORM with SQLite, UUID primary keys, auto-migration

## Testing

- Use `testify/assert` and `testify/require`
- Integration tests: `//go:build integration` tag
- Mock external dependencies
- Clean up temp directories after tests
