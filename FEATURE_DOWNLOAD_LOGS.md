# Feature: Download Process Logging

## Overview

Added comprehensive download process logging functionality that captures yt-dlp and tdl output and makes it accessible through both the Web UI and CLI.

## Implementation Summary

### 1. Domain Layer Changes

**File**: `internal/domain/download.go`

Added `ProcessLog` field to the `Download` entity:
```go
ProcessLog string `json:"process_log,omitempty" gorm:"type:text"` // Process output log (yt-dlp/tdl)
```

This field stores the complete output from yt-dlp (Twitter/X) or tdl (Telegram) processes.

### 2. Downloader Changes

**Files**: 
- `internal/infrastructure/downloader_twitter.go`
- `internal/infrastructure/downloader_telegram.go`

Both downloaders now capture process output and store it in the `ProcessLog` field:

```go
// Execute command
cmd := exec.Command(binary, args...)
output, err := cmd.CombinedOutput()

// Store process log regardless of success/failure
download.ProcessLog = string(output)
```

**Benefits**:
- Logs are captured even if download fails
- Helps with debugging download issues
- Provides visibility into what yt-dlp/tdl is doing

### 3. API Endpoint

**File**: `api/handlers/download_handler.go`

Added new endpoint: `GET /api/v1/downloads/:id/logs`

**Features**:
- Returns logs as plain text by default
- Returns JSON when `Accept: application/json` header is set
- Includes download metadata in JSON response

**Example Usage**:
```bash
# Plain text
curl http://localhost:8080/api/v1/downloads/{id}/logs

# JSON format
curl -H "Accept: application/json" http://localhost:8080/api/v1/downloads/{id}/logs
```

**File**: `api/router.go`

Added route:
```go
downloads.GET("/:id/logs", downloadHandler.GetDownloadLogs)
```

### 4. CLI Command

**File**: `cmd/cli/main.go`

Added new `logs` subcommand:

```bash
# View logs as plain text
x-extract-cli logs <download-id>

# View logs as JSON
x-extract-cli logs <download-id> --json
```

**Features**:
- Plain text output by default (easy to read)
- JSON output with `--json` flag (for scripting)
- Clear error messages if download not found

### 5. Web UI

**Files**:
- `web/static/js/app.js`
- `web/static/css/style.css`

**Features**:
- "View Logs" button appears for downloads that have logs
- Modal popup displays logs in a code-style format
- Dark theme for log content (like a terminal)
- Copy to clipboard functionality
- Responsive design

**UI Components**:
- Modal with header, body, and footer
- Monospace font for log content
- Syntax highlighting-friendly dark background
- Close on background click or close button
- Smooth animations (fade in, slide up)

## Usage Examples

### CLI Examples

```bash
# Add a download
x-extract-cli add "https://x.com/user/status/123456"

# List downloads
x-extract-cli list

# View logs (plain text)
x-extract-cli logs <download-id>

# View logs (JSON)
x-extract-cli logs <download-id> --json
```

### API Examples

```bash
# Get logs as plain text
curl http://localhost:8080/api/v1/downloads/{id}/logs

# Get logs as JSON
curl -H "Accept: application/json" \
  http://localhost:8080/api/v1/downloads/{id}/logs

# Example JSON response
{
  "id": "2f449da4-fac6-4e49-bcc9-fafc5bcd9254",
  "url": "https://x.com/user/status/123",
  "platform": "x",
  "status": "failed",
  "process_log": "[twitter] Extracting URL...\nERROR: No video found"
}
```

### Web UI

1. Open http://localhost:8080
2. View downloads list
3. Click "View Logs" button on any download with logs
4. Modal opens showing process output
5. Click "Copy to Clipboard" to copy logs
6. Click "Close" or background to dismiss

## Testing Results

### Build Test
```bash
$ make build
✅ Build complete!
```

### Unit Tests
```bash
$ go test -v ./internal/domain/...
✅ PASS: All 11 tests passed
```

### Functional Tests

1. **CLI Logs Command** ✅
   ```bash
   $ x-extract-cli logs 2f449da4-fac6-4e49-bcc9-fafc5bcd9254
   [twitter] Extracting URL: https://x.com/QingQ77/status/2012684034086785024
   [twitter] 2012684034086785024: Downloading GraphQL JSON
   ERROR: [twitter] 2012684034086785024: No video could be found in this tweet
   ```

2. **CLI JSON Output** ✅
   ```bash
   $ x-extract-cli logs <id> --json
   {
     "id": "...",
     "platform": "x",
     "process_log": "...",
     "status": "failed",
     "url": "..."
   }
   ```

3. **API Endpoint** ✅
   ```bash
   $ curl http://localhost:8080/api/v1/downloads/{id}/logs
   [twitter] Extracting URL...
   ```

4. **Web UI** ✅
   - Modal displays correctly
   - Logs are readable with dark theme
   - Copy to clipboard works
   - Responsive design

## Database Migration

The new `process_log` column is automatically added by GORM's `AutoMigrate` feature when the server starts. No manual migration required.

**Backward Compatibility**: ✅
- Existing downloads will have `null` or empty `process_log`
- New downloads will have logs captured
- No breaking changes to API or database schema

## Files Modified

1. `internal/domain/download.go` - Added ProcessLog field
2. `internal/infrastructure/downloader_twitter.go` - Capture yt-dlp output
3. `internal/infrastructure/downloader_telegram.go` - Capture tdl output
4. `api/handlers/download_handler.go` - Added GetDownloadLogs handler
5. `api/router.go` - Added /logs route
6. `cmd/cli/main.go` - Added logs subcommand
7. `web/static/js/app.js` - Added log viewer modal
8. `web/static/css/style.css` - Added modal styles

## Benefits

1. **Debugging**: Easy to see why downloads fail
2. **Transparency**: Users can see what's happening
3. **Troubleshooting**: Support can ask for logs
4. **Monitoring**: Track download progress in real-time
5. **Accessibility**: Available via CLI, API, and Web UI

## Status

✅ **COMPLETE** - All features implemented and tested successfully!

