package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/yourusername/x-extract-go/internal/app"
	"github.com/yourusername/x-extract-go/internal/domain"
	"github.com/yourusername/x-extract-go/internal/infrastructure"
	"github.com/yourusername/x-extract-go/internal/infrastructure/binmanager"
)

var (
	serverURL   string
	noAutoStart bool
	rootCmd     = &cobra.Command{
		Use:   "x-extract",
		Short: "X-Extract CLI - Download manager for X/Twitter and Telegram",
		Long:  `A command-line interface for managing downloads from X/Twitter and Telegram.`,
	}
)

// getDefaultServerURL loads the server URL from config
func getDefaultServerURL() string {
	config, err := app.LoadConfig()
	if err != nil {
		// Fallback to default if config loading fails
		return "http://localhost:9091"
	}
	return fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "Server URL (default: from config)")
	rootCmd.PersistentFlags().BoolVar(&noAutoStart, "no-auto-start", false, "Don't auto-start server if not running")

	// Set serverURL from config if not provided via flag
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if serverURL == "" {
			serverURL = getDefaultServerURL()
		}
	}

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(retryCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(regenerateMetadataCmd)
	rootCmd.AddCommand(eagleImportCmd)
	rootCmd.AddCommand(eagleRenameCmd)
	rootCmd.AddCommand(toolsCmd)
}

// ensureServer checks if server is running and starts it if needed (unless --no-auto-start)
func ensureServer() {
	if noAutoStart {
		return
	}
	if err := ensureServerRunning(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
}

var timelineFlag bool
var filterFlags []string

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a download to the queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()

		url := args[0]
		mode, _ := cmd.Flags().GetString("mode")
		explicitPlatform, _ := cmd.Flags().GetString("platform")

		xURLType := domain.DetectXURLType(url)

		// Reject --timeline on single tweet URLs (yt-dlp is always used for single tweets)
		if timelineFlag && xURLType == domain.XURLTypeSingle {
			fmt.Fprintf(os.Stderr, "Error: --timeline is only for account/media timeline URLs, not single tweets.\n")
			os.Exit(1)
		}

		// Resolve platform — explicit --platform wins; otherwise auto-detect
		platform := explicitPlatform
		if platform == "" {
			if xURLType == domain.XURLTypeTimeline || timelineFlag {
				platform = string(domain.PlatformGallery)
			} else {
				platform = string(domain.DetectPlatform(url))
			}
		}

		// Warn if user forced --platform x on a timeline (yt-dlp doesn't handle timelines well)
		if domain.Platform(platform) == domain.PlatformX && xURLType == domain.XURLTypeTimeline {
			fmt.Fprintf(os.Stderr, "Note: %s looks like an account timeline. gallery-dl may work better (use --timeline).\n", url)
		}

		payload := map[string]string{
			"url":      url,
			"platform": platform,
		}
		if mode != "" {
			payload["mode"] = mode
		}
		if len(filterFlags) > 0 {
			payload["filters"] = strings.Join(filterFlags, "|")
		}

		data, _ := json.Marshal(payload)
		resp, err := http.Post(serverURL+"/api/v1/downloads", "application/json", bytes.NewBuffer(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		fmt.Printf("Download added successfully!\n")
		fmt.Printf("ID: %s\n", result["id"])
		fmt.Printf("Status: %s\n", result["status"])
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all downloads",
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		status, _ := cmd.Flags().GetString("status")

		url := serverURL + "/api/v1/downloads"
		if status != "" {
			url += "?status=" + status
		}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var downloads []map[string]interface{}
		json.Unmarshal(body, &downloads)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tURL\tPLATFORM\tSTATUS\tCREATED")
		for _, d := range downloads {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				truncate(d["id"].(string), 8),
				truncate(d["url"].(string), 40),
				d["platform"],
				d["status"],
				d["created_at"])
		}
		w.Flush()
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show download statistics",
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		resp, err := http.Get(serverURL + "/api/v1/downloads/stats")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var stats map[string]interface{}
		json.Unmarshal(body, &stats)

		fmt.Println("Download Statistics:")
		fmt.Printf("  Total:      %v\n", stats["total"])
		fmt.Printf("  Queued:     %v\n", stats["queued"])
		fmt.Printf("  Processing: %v\n", stats["processing"])
		fmt.Printf("  Completed:  %v\n", stats["completed"])
		fmt.Printf("  Failed:     %v\n", stats["failed"])
		fmt.Printf("  Cancelled:  %v\n", stats["cancelled"])
	},
}

var getCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get download details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		id := args[0]
		resp, err := http.Get(serverURL + "/api/v1/downloads/" + id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var download map[string]interface{}
		json.Unmarshal(body, &download)

		fmt.Printf("Download Details:\n")
		fmt.Printf("  ID:       %s\n", download["id"])
		fmt.Printf("  URL:      %s\n", download["url"])
		fmt.Printf("  Platform: %s\n", download["platform"])
		fmt.Printf("  Status:   %s\n", download["status"])
		fmt.Printf("  Mode:     %s\n", download["mode"])
		fmt.Printf("  Created:  %s\n", download["created_at"])
		if download["file_path"] != nil {
			fmt.Printf("  File:     %s\n", download["file_path"])
		}
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel [id]",
	Short: "Cancel a download",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		id := args[0]
		resp, err := http.Post(serverURL+"/api/v1/downloads/"+id+"/cancel", "application/json", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		fmt.Println("Download cancelled successfully")
	},
}

var retryCmd = &cobra.Command{
	Use:   "retry [id]",
	Short: "Retry a failed download",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		id := args[0]
		resp, err := http.Post(serverURL+"/api/v1/downloads/"+id+"/retry", "application/json", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		fmt.Println("Download queued for retry")
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "View download process logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()
		id := args[0]
		jsonOutput, _ := cmd.Flags().GetBool("json")

		req, err := http.NewRequest("GET", serverURL+"/api/v1/downloads/"+id+"/logs", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if jsonOutput {
			req.Header.Set("Accept", "application/json")
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Error: %s\n", string(body))
			os.Exit(1)
		}

		if jsonOutput {
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			prettyJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(prettyJSON))
		} else {
			fmt.Print(string(body))
		}
	},
}

var regenerateMetadataCmd = &cobra.Command{
	Use:   "regenerate-metadata",
	Short: "Regenerate metadata JSON files for downloads with missing text",
	Long: `Regenerates metadata JSON files for Telegram downloads that have
empty descriptions. This command queries the message cache to find the
correct text for each downloaded file based on the message ID in the filename.
It uses grouped message resolution (media albums) and nearby message fallback
to find the correct text. Does NOT re-download any files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Note: This command doesn't need the server running
		// It reads the database and files directly
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		completedDir, _ := cmd.Flags().GetString("completed-dir")

		config, err := app.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		if completedDir == "" {
			completedDir = config.Download.CompletedDir()
		}

		dbPath := config.Queue.DatabasePath

		// Open database using repository interface
		repo, err := infrastructure.NewSQLiteDownloadRepository(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer repo.Close()

		// Phase 1: Update .info.json files in the completed directory
		fmt.Println("Scanning completed directory for Telegram .info.json files...")
		updated := 0
		files, err := os.ReadDir(completedDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading completed dir: %v\n", err)
			os.Exit(1)
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if !strings.HasSuffix(name, ".info.json") {
				continue
			}

			// Extract channel ID from filename (format: {channel_id}_{message_id}_{rest}.info.json)
			channelID := extractChannelIDFromFilename(name)
			if channelID == "" {
				continue
			}

			// Extract message ID from filename
			msgID := extractMessageIDFromFilename(name)
			if msgID == "" {
				continue
			}

			// Read the JSON file
			jsonPath := filepath.Join(completedDir, name)
			data, err := os.ReadFile(jsonPath)
			if err != nil {
				continue
			}

			var metadata map[string]interface{}
			if err := json.Unmarshal(data, &metadata); err != nil {
				continue
			}

			// Check if description is empty
			desc, _ := metadata["description"].(string)
			if desc != "" {
				continue // Already has description
			}

			// Resolve text using repository with grouped message resolution
			text := resolveMessageText(repo, channelID, msgID)

			// If not found by filename message ID, try the URL message ID from metadata
			if text == "" {
				if urlMsgID, ok := metadata["id"].(string); ok && urlMsgID != msgID {
					text = resolveMessageText(repo, channelID, urlMsgID)
				}
			}

			if text == "" {
				continue // No text found
			}

			// Update metadata
			metadata["description"] = text

			// Write back JSON file
			if !dryRun {
				newData, _ := json.MarshalIndent(metadata, "", "  ")
				os.WriteFile(jsonPath, newData, 0644)
			}
			fmt.Printf("Updated: %s (msg %s)\n", name, msgID)
			updated++
		}

		// Phase 2: Update database entries for completed Telegram downloads
		fmt.Printf("\nUpdating database entries...\n")
		dbUpdated := 0

		downloads, err := repo.FindAll(map[string]interface{}{
			"platform": domain.PlatformTelegram,
			"status":   domain.StatusCompleted,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying downloads: %v\n", err)
		} else {
			for _, dl := range downloads {
				if dl.Metadata == "" {
					continue
				}

				var metadata map[string]interface{}
				if err := json.Unmarshal([]byte(dl.Metadata), &metadata); err != nil {
					continue
				}

				// Check if description is empty
				desc, _ := metadata["description"].(string)
				if desc != "" {
					continue // Already has description
				}

				// Extract channel and message IDs from the download's files or URL
				channelID, msgID := extractIDsFromDownload(dl, metadata)
				if channelID == "" || msgID == "" {
					continue
				}

				// Resolve text using repository with grouped message resolution
				text := resolveMessageText(repo, channelID, msgID)
				if text == "" {
					continue
				}

				// Update metadata
				metadata["description"] = text
				newMetadataBytes, _ := json.Marshal(metadata)

				// Update database
				if !dryRun {
					dl.Metadata = string(newMetadataBytes)
					if err := repo.Update(dl); err != nil {
						fmt.Fprintf(os.Stderr, "Error updating download %s: %v\n", dl.ID[:8], err)
						continue
					}
				}
				fmt.Printf("Updated DB: %s (msg %s)\n", dl.ID[:8], msgID)
				dbUpdated++
			}
		}

		if dryRun {
			fmt.Printf("\nDry run: would update %d JSON files and %d DB entries\n", updated, dbUpdated)
		} else {
			fmt.Printf("\nUpdated %d JSON files and %d DB entries\n", updated, dbUpdated)
		}
	},
}

// extractChannelIDFromFilename extracts the channel ID from a Telegram filename.
// Format: {channel_id}_{message_id}_{media_id}.{ext}
// Returns empty string if the first part is not a numeric channel ID.
func extractChannelIDFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	// Handle .info.json double extension
	name = strings.TrimSuffix(name, ".info")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}
	// Validate that it's a numeric channel ID (Telegram private channels)
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return ""
	}
	return parts[0]
}

// resolveMessageText looks up message text from the cache repository,
// using grouped message resolution and nearby message fallback.
func resolveMessageText(repo *infrastructure.SQLiteDownloadRepository, channelID, messageID string) string {
	// First try direct lookup
	cached, err := repo.GetMessage(channelID, messageID)
	if err != nil {
		return ""
	}
	if cached != nil && cached.Text != "" {
		return cached.Text
	}

	// If message exists but has no text, try grouped message resolution
	if cached != nil && cached.GroupedID != "" {
		grouped, err := repo.GetMessagesByGroupedID(channelID, cached.GroupedID)
		if err == nil {
			for _, g := range grouped {
				if g.Text != "" {
					return g.Text
				}
			}
		}
	}

	// Fallback: search nearby message IDs (±3) for text
	nearby, err := repo.GetNearbyMessages(channelID, messageID, 3)
	if err == nil {
		for _, n := range nearby {
			if n.Text != "" {
				return n.Text
			}
		}
	}

	return ""
}

// extractIDsFromDownload extracts channel ID and message ID from a download record.
// Tries to extract from the files list first (filename), then from the URL.
func extractIDsFromDownload(dl *domain.Download, metadata map[string]interface{}) (channelID, msgID string) {
	// Try extracting from files list in metadata
	if filesRaw, ok := metadata["files"].([]interface{}); ok && len(filesRaw) > 0 {
		if filePath, ok := filesRaw[0].(string); ok {
			filename := filepath.Base(filePath)
			channelID = extractChannelIDFromFilename(filename)
			msgID = extractMessageIDFromFilename(filename)
			if channelID != "" && msgID != "" {
				return channelID, msgID
			}
		}
	}

	// Fallback: extract from URL (format: https://t.me/c/{channel_id}/{message_id})
	url := dl.URL
	parts := strings.Split(url, "/")
	if len(parts) >= 5 && parts[3] == "c" {
		// Private channel: https://t.me/c/1234567890/messageid
		return parts[4], parts[len(parts)-1]
	}

	return "", ""
}

// Format: {channel_id}_{message_id}_{media_id}.{ext}
func extractMessageIDFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

var eagleImportCmd = &cobra.Command{
	Use:   "eagle-import",
	Short: "Import completed downloads into Eagle App",
	Long: `Imports media files from the completed directory into Eagle App
using the Eagle API. Each media file's .info.json metadata is used to
populate Eagle item fields (name, tags, website, annotation).

Files are imported in batches via /api/item/addFromPaths for efficiency.
After successful import, files are moved to an 'imported' subdirectory
to prevent duplicate imports.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		completedDir, _ := cmd.Flags().GetString("completed-dir")

		config, err := app.LoadConfig()
		if err != nil {
			return fmt.Errorf("error loading config: %w", err)
		}

		if completedDir == "" {
			completedDir = config.Download.CompletedDir()
		}

		imported := 0
		failed := 0
		runID := newEagleImportRunID()

		var importLog *infrastructure.ImportLogger
		importLog, err = infrastructure.NewImportLogger(config.Download.LogsDir(), runID, completedDir, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open import log: %v\n", err)
		} else {
			defer func() {
				closeEagleImportLogger(importLog, imported, failed)
			}()
			writeEagleImportStdout(importLog, "Import log: %s\n", importLog.LogPath())
		}

		eagleCfg := config.Eagle

		// Check Eagle is reachable
		if !dryRun {
			if err := checkEagleRunning(eagleCfg.APIEndpoint); err != nil {
				if importLog != nil {
					importLog.Logf("Error: %v", err)
				}
				return err
			}
			writeEagleImportStdout(importLog, "Eagle App is running.\n")
		}

		// Scan completed directory for media files
		items, skipped := scanForEagleItems(completedDir)
		if len(items) == 0 {
			writeEagleImportStdout(importLog, "No media files found to import.\n")
			return nil
		}
		writeEagleImportStdout(importLog, "Found %d media files to import (%d skipped, no .info.json)\n", len(items), skipped)

		if dryRun {
			writeEagleImportStdout(importLog, "\nDry run — files that would be imported:\n")
			for _, item := range items {
				writeEagleImportStdout(importLog, "  %s → %s\n", filepath.Base(item.Path), item.Name)
			}
			return nil
		}

		// Import one file at a time for reliable tracking
		total := len(items)

		// Prepare imported dir if we'll move files
		var importedDir string
		if eagleCfg.MoveOnSuccess {
			importedDir = filepath.Join(completedDir, eagleCfg.ImportedSubdir)
			if err := os.MkdirAll(importedDir, 0755); err != nil {
				writeEagleImportStderr(importLog, "Warning: failed to create imported dir: %v\n", err)
				importedDir = "" // disable moving
			}
		}

		for i, item := range items {
			writeEagleImportStdout(importLog, "[%d/%d] Importing %s ...\n", i+1, total, filepath.Base(item.Path))

			itemID, err := eagleAddFromPath(eagleCfg.APIEndpoint, item, eagleCfg.FolderID, eagleCfg.MaxRetries)
			if err != nil {
				writeEagleImportStdout(importLog, "  ✗ Import failed: %v\n", err)
				failed++
				continue
			}

			// Verify import by polling item/info with timeout based on file size
			if verifyEagleImport(eagleCfg.APIEndpoint, itemID, item.Path, importLog) {
				writeEagleImportStdout(importLog, "  ✓ Imported\n")
				imported++
				// Wait 10s for Eagle to fully settle before moving file
				time.Sleep(10 * time.Second)
				if importedDir != "" {
					moveImportedFile(item.Path, importedDir, importLog)
				}
			} else {
				writeEagleImportStdout(importLog, "  ✗ Import verification failed (ID: %s)\n", itemID)
				failed++
			}
		}

		fmt.Printf("\nImport complete: %d imported, %d failed\n", imported, failed)
		if importLog != nil {
			importLog.Logf("Import complete: %d imported, %d failed", imported, failed)
		}

		return nil
	},
}

func newEagleImportRunID() string {
	return fmt.Sprintf("%s-%d", time.Now().Format("20060102-150405"), os.Getpid())
}

func closeEagleImportLogger(importLog *infrastructure.ImportLogger, imported, failed int) {
	if importLog == nil {
		return
	}

	if err := importLog.Close(imported, failed); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close import log: %v\n", err)
	}
}

func writeEagleImportStdout(importLog *infrastructure.ImportLogger, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Print(message)
	logEagleImportMessage(importLog, message)
}

func writeEagleImportStderr(importLog *infrastructure.ImportLogger, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Fprint(os.Stderr, message)
	logEagleImportMessage(importLog, message)
}

func logEagleImportMessage(importLog *infrastructure.ImportLogger, message string) {
	if importLog == nil {
		return
	}

	trimmed := strings.TrimRight(message, "\n")
	if trimmed == "" {
		return
	}

	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		importLog.Logf(line)
	}
}

// checkEagleRunning verifies that Eagle App's API server is accessible.
func checkEagleRunning(apiEndpoint string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiEndpoint + "/api/application/info")
	if err != nil {
		return fmt.Errorf("Eagle App is not running or not reachable at %s: %w", apiEndpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Eagle App returned status %d from %s", resp.StatusCode, apiEndpoint)
	}
	return nil
}

// scanForEagleItems scans the completed directory for media files with .info.json metadata.
// Returns the list of EagleItems and the count of media files skipped (no metadata).
func scanForEagleItems(completedDir string) ([]*domain.EagleItem, int) {
	files, err := os.ReadDir(completedDir)
	if err != nil {
		return nil, 0
	}

	var items []*domain.EagleItem
	skipped := 0

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filePath := filepath.Join(completedDir, f.Name())
		if !infrastructure.IsMediaFile(filePath) {
			continue
		}

		// Look for corresponding .info.json
		baseName := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		infoPath := filepath.Join(completedDir, baseName+".info.json")

		data, err := os.ReadFile(infoPath)
		if err != nil {
			skipped++
			continue
		}

		var meta domain.MediaMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			skipped++
			continue
		}

		item := meta.ToEagleItem(filePath)
		// Use filename as fallback name if metadata title is empty
		if item.Name == "" {
			item.Name = baseName
		}
		items = append(items, item)
	}

	return items, skipped
}

// eagleAddFromPath imports a single file to Eagle via /api/item/addFromPaths.
// Returns the created item ID on success. The API returns immediately after queuing
// the import — use waitForItemReady to verify the import is complete before moving files.
func eagleAddFromPath(apiEndpoint string, item *domain.EagleItem, folderID string, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	type addFromPathsRequest struct {
		Items    []*domain.EagleItem `json:"items"`
		FolderID string              `json:"folderId,omitempty"`
	}

	payload := addFromPathsRequest{
		Items:    []*domain.EagleItem{item},
		FolderID: folderID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.Post(apiEndpoint+"/api/item/addFromPaths", "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt, err)
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result struct {
				Status string   `json:"status"`
				Data   []string `json:"data"`
			}
			if err := json.Unmarshal(respBody, &result); err != nil {
				return "", fmt.Errorf("parse response: %w (body: %s)", err, string(respBody))
			}
			if result.Status == "success" && len(result.Data) > 0 {
				return result.Data[0], nil
			}
			if result.Status == "success" {
				return "", fmt.Errorf("no item ID returned: %s", string(respBody))
			}
			return "", fmt.Errorf("Eagle API error: %s", string(respBody))
		}

		lastErr = fmt.Errorf("attempt %d: status %d: %s", attempt, resp.StatusCode, string(respBody))
		time.Sleep(time.Duration(attempt*attempt) * time.Second)
	}

	return "", fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// verifyEagleImport polls /api/item/info in a loop at 10s intervals until
// status="success" is returned. Timeout is calculated from file size: 60s per 100MB,
// with a minimum of 30s. Returns true if verified, false if timed out.
func verifyEagleImport(apiEndpoint string, itemID string, filePath string, importLog *infrastructure.ImportLogger) bool {
	// Calculate timeout: 60s per 100MB, minimum 30s
	timeout := 30 * time.Second
	if info, err := os.Stat(filePath); err == nil {
		sizeMB := info.Size() / (1024 * 1024)
		sizeTimeout := time.Duration(sizeMB/100*60) * time.Second
		if sizeTimeout > timeout {
			timeout = sizeTimeout
		}
	}

	const pollInterval = 10 * time.Second
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	attempt := 0

	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		attempt++

		resp, err := client.Get(fmt.Sprintf("%s/api/item/info?id=%s", apiEndpoint, itemID))
		if err != nil {
			writeEagleImportStderr(importLog, "    [verify] attempt %d: request error: %v\n", attempt, err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Status string      `json:"status"`
			Data   interface{} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			writeEagleImportStderr(importLog, "    [verify] attempt %d: parse error: %v\n", attempt, err)
			continue
		}

		if result.Status == "success" && result.Data != nil {
			return true
		}

		writeEagleImportStderr(importLog, "    [verify] attempt %d: not ready yet (status=%q, timeout in %ds)\n",
			attempt, result.Status, int(time.Until(deadline).Seconds()))
	}

	return false
}

// moveImportedFile moves a media file and its associated metadata files to the imported directory.
func moveImportedFile(mediaPath, importedDir string, importLog *infrastructure.ImportLogger) {
	baseName := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	dir := filepath.Dir(mediaPath)

	// Move the media file itself plus any associated metadata files
	associatedFiles := []string{
		mediaPath, // media file
		filepath.Join(dir, baseName+".info.json"),  // yt-dlp metadata
		filepath.Join(dir, baseName+".eagle.json"), // eagle metadata
	}

	for _, src := range associatedFiles {
		if _, err := os.Stat(src); err != nil {
			continue // file doesn't exist
		}
		dst := filepath.Join(importedDir, filepath.Base(src))
		if err := infrastructure.MoveFile(src, dst); err != nil {
			writeEagleImportStderr(importLog, "  Warning: failed to move %s: %v\n", filepath.Base(src), err)
		}
	}
}

// eagleRenameCmd scans Eagle library for items with problematic names and renames them
var eagleRenameCmd = &cobra.Command{
	Use:   "eagle-rename",
	Short: "Find and fix problematic Eagle item names for filesystem compatibility",
	Long: `Scans Eagle library for items with names that may cause sync issues
due to illegal characters or excessive length.

Common issues:
- Names containing: < > : " / \ | ? *
- Names exceeding 255 bytes (filesystem limit)
- Names with trailing dots or spaces

Note: The limit is in BYTES, not characters. Chinese/UTF-8 characters
take 3 bytes each. Default limit is 180 bytes (conservative).

By default, this command only identifies problematic items. Use --apply
to actually rename items via Eagle's API.`,
	Run: func(cmd *cobra.Command, args []string) {
		maxLen, _ := cmd.Flags().GetInt("max-length")
		folderID, _ := cmd.Flags().GetString("folder-id")
		skipImages, _ := cmd.Flags().GetBool("skip-images")
		applyRenames, _ := cmd.Flags().GetBool("apply")
		idsFlag, _ := cmd.Flags().GetStringSlice("ids")

		config, err := app.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		eagleCfg := config.Eagle

		// Check Eagle is running
		if err := checkEagleRunning(eagleCfg.APIEndpoint); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Eagle App is running.")

		// Fetch all items from Eagle
		fmt.Println("\nFetching items from Eagle library...")
		items, err := listEagleItems(eagleCfg.APIEndpoint, folderID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching items: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d items in library.\n\n", len(items))

		// Skip images if requested
		if skipImages {
			var filtered []eagleItemInfo
			imageExts := map[string]bool{
				".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
				".webp": true, ".bmp": true, ".svg": true, ".ico": true,
			}
			for _, item := range items {
				ext := strings.ToLower(filepath.Ext(item.Name))
				if !imageExts[ext] {
					filtered = append(filtered, item)
				}
			}
			fmt.Printf("Filtered to %d non-image items.\n\n", len(filtered))
			items = filtered
		}

		// Analyze items for problems
		var problematic []renameItem
		seenNames := make(map[string]int) // Track proposed names to avoid collisions

		for _, item := range items {
			issues, proposed := analyzeItemName(item.Name, maxLen)
			if len(issues) > 0 {
				// Ensure unique proposed name
				proposed = ensureUniqueName(proposed, seenNames)
				seenNames[proposed]++
				problematic = append(problematic, renameItem{
					ID:       item.ID,
					Current:  item.Name,
					Proposed: proposed,
					Issues:   issues,
					ItemType: item.ItemType,
					FilePath: item.FilePath,
				})
			}
		}

		if len(problematic) == 0 {
			fmt.Println("No problematic items found. All names are valid!")
			return
		}

		// Display problematic items
		fmt.Printf("Problematic items found: %d\n\n", len(problematic))
		for i, item := range problematic {
			fmt.Printf("[%d] ID: %s\n", i+1, item.ID)
			fmt.Printf("    Current:  %s (%d bytes)\n", truncate(item.Current, 60), len(item.Current))
			fmt.Printf("    Proposed: %s (%d bytes)\n", truncate(item.Proposed, 60), len(item.Proposed))
			fmt.Printf("    Issues:   %s\n", strings.Join(item.Issues, ", "))
			if item.FilePath != "" {
				fmt.Printf("    Path:     %s\n", truncate(item.FilePath, 60))
			}
			fmt.Println()
		}

		// Check for output file flag
		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile != "" {
			// Export as JSON for use with Eagle plugin API
			type exportItem struct {
				ID       string   `json:"id"`
				Current  string   `json:"currentName"`
				Proposed string   `json:"proposedName"`
				Issues   []string `json:"issues"`
			}
			var exportData []exportItem
			for _, item := range problematic {
				exportData = append(exportData, exportItem{
					ID:       item.ID,
					Current:  item.Current,
					Proposed: item.Proposed,
					Issues:   item.Issues,
				})
			}
			data, err := json.MarshalIndent(exportData, "", "  ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(outputFile, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("\nExported %d items to: %s\n", len(problematic), outputFile)
		}

		fmt.Printf("\nFound %d items needing rename.\n", len(problematic))

		if applyRenames {
			// Get Eagle library path for direct metadata.json modification
			fmt.Println("\nFetching Eagle library path...")
			libraryPath, err := getEagleLibraryPath(eagleCfg.APIEndpoint)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting library path: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Library path: %s\n", libraryPath)

			// Filter to specific IDs if --ids is provided
			toRename := problematic
			if len(idsFlag) > 0 {
				idSet := make(map[string]bool, len(idsFlag))
				for _, id := range idsFlag {
					idSet[id] = true
				}
				var filtered []renameItem
				for _, item := range problematic {
					if idSet[item.ID] {
						filtered = append(filtered, item)
					}
				}
				toRename = filtered
				fmt.Printf("\nFiltered to %d items matching --ids.\n", len(toRename))
				if len(toRename) == 0 {
					fmt.Println("No matching items found. Check the IDs and try again.")
					return
				}
			}

			// Apply renames by modifying metadata.json directly
			fmt.Println("\nApplying renames...")
			renamed := 0
			renameFailed := 0
			for i, item := range toRename {
				fmt.Printf("[%d/%d] %s -> %s ... ", i+1, len(toRename), truncate(item.Current, 40), truncate(item.Proposed, 40))
				if err := updateEagleItemName(libraryPath, item.ID, item.Proposed); err != nil {
					fmt.Printf("✗ %v\n", err)
					renameFailed++
				} else {
					fmt.Printf("✓\n")
					renamed++
				}
			}
			fmt.Printf("\nRename complete: %d renamed, %d failed\n", renamed, renameFailed)
		} else {
			fmt.Println("Use --apply to rename items via metadata.json (direct file modification).")
			fmt.Println("Use --output to export list as JSON.")
		}
	},
}

// renameItem holds information about an item that needs renaming
type renameItem struct {
	ID       string
	Current  string
	Proposed string
	Issues   []string
	ItemType string
	FilePath string
}

// eagleItemInfo represents an item from Eagle API
type eagleItemInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ItemType   string `json:"type"`
	FilePath   string `json:"filePath,omitempty"`
	FolderID   string `json:"folderId,omitempty"`
	Website    string `json:"website,omitempty"`
	Tags       []any  `json:"tags,omitempty"`
	Annotation string `json:"annotation,omitempty"`
}

// listEagleItems fetches all items from Eagle library
func listEagleItems(apiEndpoint, folderID string) ([]eagleItemInfo, error) {
	// Eagle API uses page-based pagination: offset is a page number starting at 0.
	// limit=100 is used per page; incrementing offset by 1 moves to the next page.
	const limit = 100
	client := &http.Client{Timeout: 60 * time.Second}

	var allItems []eagleItemInfo
	page := 0

	for {
		reqURL := fmt.Sprintf("%s/api/item/list?limit=%d&offset=%d", apiEndpoint, limit, page)
		if folderID != "" {
			reqURL += "&folders=" + folderID
		}

		resp, err := client.Get(reqURL)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
		}

		var rawResp map[string]interface{}
		decodeErr := json.NewDecoder(resp.Body).Decode(&rawResp)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode response: %w", decodeErr)
		}

		status, _ := rawResp["status"].(string)
		if status != "success" {
			return nil, fmt.Errorf("API error: %s", status)
		}

		pageItems := parseEagleItems(rawResp["data"])

		// Stop when the API returns an empty data array — end of results.
		if len(pageItems) == 0 {
			break
		}

		allItems = append(allItems, pageItems...)
		fmt.Printf("  Fetched %d items (total: %d)...\n", len(pageItems), len(allItems))

		// Stop when the API returns fewer items than the page size — last page.
		if len(pageItems) < limit {
			break
		}

		page++
	}

	return allItems, nil
}

// parseEagleItems extracts eagleItemInfo from the API response data field.
func parseEagleItems(data interface{}) []eagleItemInfo {
	var rawItems []interface{}

	switch v := data.(type) {
	case []interface{}:
		rawItems = v
	case map[string]interface{}:
		if items, ok := v["items"].([]interface{}); ok {
			rawItems = items
		}
	}

	items := make([]eagleItemInfo, 0, len(rawItems))
	for _, item := range rawItems {
		if itemMap, ok := item.(map[string]interface{}); ok {
			eagleItem := eagleItemInfo{}
			if id, ok := itemMap["id"].(string); ok {
				eagleItem.ID = id
			}
			if name, ok := itemMap["name"].(string); ok {
				eagleItem.Name = name
			}
			if itemType, ok := itemMap["type"].(string); ok {
				eagleItem.ItemType = itemType
			}
			if fp, ok := itemMap["filePath"].(string); ok {
				eagleItem.FilePath = fp
			}
			if fid, ok := itemMap["folderId"].(string); ok {
				eagleItem.FolderID = fid
			}
			items = append(items, eagleItem)
		}
	}
	return items
}

// getEagleLibraryPath fetches the current Eagle library path via /api/library/info
func getEagleLibraryPath(apiEndpoint string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiEndpoint + "/api/library/info")
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var rawResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	status, _ := rawResp["status"].(string)
	if status != "success" {
		return "", fmt.Errorf("API error: %s", status)
	}

	data, ok := rawResp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format: missing data")
	}

	// The library info contains a "library" object with a "path" field,
	// or the path may be directly in data. Try both.
	if lib, ok := data["library"].(map[string]interface{}); ok {
		if p, ok := lib["path"].(string); ok && p != "" {
			return p, nil
		}
	}
	if p, ok := data["path"].(string); ok && p != "" {
		return p, nil
	}

	return "", fmt.Errorf("library path not found in API response")
}

// updateEagleItemName updates an item's name by directly modifying the metadata.json
// file in Eagle's library folder. The Eagle API /api/item/update does NOT support
// the "name" field, so we must edit the file directly.
// libraryPath should be the .library folder path (e.g. /path/to/MyLibrary.library)
func updateEagleItemName(libraryPath, itemID, newName string) error {
	infoDir := filepath.Join(libraryPath, "images", itemID+".info")
	metadataPath := filepath.Join(infoDir, "metadata.json")

	// Read existing metadata as raw bytes to preserve original formatting
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}

	// Parse JSON only to extract old name and ext for file renaming
	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}

	oldName, _ := metadata["name"].(string)
	ext, _ := metadata["ext"].(string)

	// Rename the actual file in the .info folder if it exists
	if oldName != "" && ext != "" {
		oldFilePath := filepath.Join(infoDir, oldName+"."+ext)
		newFilePath := filepath.Join(infoDir, newName+"."+ext)
		if oldFilePath != newFilePath {
			if _, err := os.Stat(oldFilePath); err == nil {
				if err := os.Rename(oldFilePath, newFilePath); err != nil {
					return fmt.Errorf("rename file %q -> %q: %w", filepath.Base(oldFilePath), filepath.Base(newFilePath), err)
				}
			}
		}
	}

	// Rename the thumbnail file if it exists
	if oldName != "" {
		oldThumb := filepath.Join(infoDir, oldName+"_thumbnail.png")
		newThumb := filepath.Join(infoDir, newName+"_thumbnail.png")
		if oldThumb != newThumb {
			if _, err := os.Stat(oldThumb); err == nil {
				if err := os.Rename(oldThumb, newThumb); err != nil {
					return fmt.Errorf("rename thumbnail %q -> %q: %w", filepath.Base(oldThumb), filepath.Base(newThumb), err)
				}
			}
		}
	}

	// Replace the "name" field value in the raw JSON without re-serializing.
	// This preserves the original file formatting, key order, etc.
	oldNameJSON, _ := json.Marshal(oldName)
	newNameJSON, _ := json.Marshal(newName)
	// Match `"name": "old value"` or `"name":"old value"` (with optional whitespace)
	oldPattern := []byte(`"name"`)
	idx := bytes.Index(data, oldPattern)
	if idx == -1 {
		return fmt.Errorf("could not find \"name\" field in metadata.json")
	}
	// Find the colon after "name"
	colonIdx := idx + len(oldPattern)
	for colonIdx < len(data) && data[colonIdx] != ':' {
		colonIdx++
	}
	if colonIdx >= len(data) {
		return fmt.Errorf("malformed metadata.json: no colon after \"name\"")
	}
	// Skip colon and whitespace to find the value
	valueStart := colonIdx + 1
	for valueStart < len(data) && (data[valueStart] == ' ' || data[valueStart] == '\t') {
		valueStart++
	}
	// The value should be the old name JSON string; find its end
	oldValueEnd := valueStart + len(oldNameJSON)
	if oldValueEnd > len(data) || !bytes.Equal(data[valueStart:oldValueEnd], oldNameJSON) {
		// Fallback: find the closing quote of the JSON string value
		if data[valueStart] != '"' {
			return fmt.Errorf("unexpected value type for \"name\" field")
		}
		oldValueEnd = valueStart + 1
		for oldValueEnd < len(data) {
			if data[oldValueEnd] == '\\' {
				oldValueEnd += 2 // skip escaped char
				continue
			}
			if data[oldValueEnd] == '"' {
				oldValueEnd++ // include closing quote
				break
			}
			oldValueEnd++
		}
	}

	// Build new file content: everything before value + new value + everything after
	var buf bytes.Buffer
	buf.Write(data[:valueStart])
	buf.Write(newNameJSON)
	buf.Write(data[oldValueEnd:])

	if err := os.WriteFile(metadataPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// illegalChars contains characters that are problematic for filesystems
var illegalChars = []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*'}

// analyzeItemName checks if a name has issues and returns the proposed sanitized name
func analyzeItemName(name string, maxLen int) ([]string, string) {
	var issues []string
	proposed := name

	// Check for empty name
	if strings.TrimSpace(name) == "" {
		issues = append(issues, "empty name")
		return issues, "unnamed_item"
	}

	// Remove and track illegal characters
	for _, c := range illegalChars {
		if strings.ContainsRune(proposed, c) {
			issues = append(issues, fmt.Sprintf("illegal char '%c'", c))
			proposed = strings.ReplaceAll(proposed, string(c), "-")
		}
	}

	// Check for trailing dots and spaces
	if strings.HasSuffix(proposed, ".") || strings.HasSuffix(proposed, " ") {
		issues = append(issues, "trailing dot/space")
		proposed = strings.TrimRight(proposed, ". ")
	}

	// Check leading spaces
	if strings.HasPrefix(proposed, " ") {
		issues = append(issues, "leading space")
		proposed = strings.TrimLeft(proposed, " ")
	}

	// Check length (using byte length, not rune count)
	if len(proposed) > maxLen {
		issues = append(issues, fmt.Sprintf("too long (%d bytes)", len(proposed)))
		// Try to preserve extension but need to truncate at byte level
		ext := filepath.Ext(proposed)
		// Use "…" (ellipsis char, 3 bytes) instead of "..." to avoid triggering trailing dot detection
		ellipsis := "…"
		ellipsisLen := len(ellipsis) // 3 bytes in UTF-8
		if ext != "" {
			extLen := len(ext)
			// Calculate safe byte length that won't cut UTF-8 characters
			maxBaseLen := maxLen - extLen - ellipsisLen
			if maxBaseLen < 1 {
				maxBaseLen = 1
			}
			// Truncate the base name at byte level
			baseName := strings.TrimSuffix(proposed, ext)
			if len(baseName) > maxBaseLen {
				// Find a safe truncation point - back up until we hit a valid UTF-8 rune
				truncated := baseName[:maxBaseLen]
				for len(truncated) > 0 {
					if utf8.ValidString(truncated) {
						break
					}
					truncated = truncated[:len(truncated)-1]
				}
				proposed = truncated + ellipsis + ext
			} else {
				proposed = baseName + ellipsis + ext
			}
		} else {
			// No extension - truncate at safe byte boundary
			truncated := proposed[:maxLen-ellipsisLen]
			for len(truncated) > 0 {
				if utf8.ValidString(truncated) {
					break
				}
				truncated = truncated[:len(truncated)-1]
			}
			proposed = truncated + ellipsis
		}
	}

	// Check for reserved Windows names
	reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5",
		"COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
	base := strings.Split(proposed, ".")[0]
	for _, r := range reserved {
		if strings.EqualFold(base, r) {
			issues = append(issues, fmt.Sprintf("reserved Windows name '%s'", r))
			proposed = "_" + proposed
			break
		}
	}

	// Final cleanup - ensure not empty
	if strings.TrimSpace(proposed) == "" {
		proposed = "unnamed_item"
		issues = []string{"name became empty after sanitization"}
	}

	// If sanitization produced the same name, no rename needed
	if proposed == name {
		return nil, name
	}

	return issues, proposed
}

// ensureUniqueName ensures the proposed name is unique by adding a counter suffix if needed
func ensureUniqueName(name string, seen map[string]int) string {
	if seen[name] == 0 {
		return name
	}

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	counter := 2
	for {
		candidate := fmt.Sprintf("%s_%d%s", base, counter, ext)
		if seen[candidate] == 0 {
			return candidate
		}
		counter++
	}
}

// toolsCmd is the parent command for managing external tools
var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Manage external tools (yt-dlp, tdl, gallery-dl)",
	Long:  "Check status, install, or update external download tools.",
}

var toolsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of external tools",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := app.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		binDir := config.Download.BinDirectory()
		tools := []struct {
			name       string
			configPath string
			version    string
		}{
			{"yt-dlp", config.Twitter.YTDLPBinary, config.Download.YTDLPVersion},
			{"tdl", config.Telegram.TDLBinary, config.Download.TDLVersion},
			{"gallery-dl", config.GalleryDL.GalleryDLBinary, config.Download.GalleryDLVersion},
		}

		fmt.Printf("Managed binary dir: %s\n", binDir)
		fmt.Printf("Auto-install: %v\n\n", config.Download.AutoInstall)

		for _, t := range tools {
			resolved, err := binmanager.ResolveBinary(t.name, t.configPath, binDir, false)
			if err != nil {
				fmt.Printf("%-8s  ✗ not found (%s)\n", t.name, err)
			} else {
				fmt.Printf("%-8s  ✓ %s\n", t.name, resolved)
			}
			fmt.Printf("         version pin: %s\n", t.version)
		}
	},
}

var toolsInstallCmd = &cobra.Command{
	Use:   "install [tool]",
	Short: "Install or reinstall an external tool",
	Long:  "Install a specific tool (yt-dlp, tdl, gallery-dl) or all tools if no argument given.",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := app.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		binDir := config.Download.BinDirectory()
		tools := []struct {
			name    string
			version string
		}{
			{"yt-dlp", config.Download.YTDLPVersion},
			{"tdl", config.Download.TDLVersion},
			{"gallery-dl", config.Download.GalleryDLVersion},
		}

		// Filter to specific tool if argument given
		if len(args) > 0 {
			toolName := args[0]
			found := false
			for _, t := range tools {
				if t.name == toolName {
					tools = []struct {
						name    string
						version string
					}{t}
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "Unknown tool: %s (available: yt-dlp, tdl, gallery-dl)\n", toolName)
				os.Exit(1)
			}
		}

		for _, t := range tools {
			spec, ok := binmanager.KnownTools[t.name]
			if !ok {
				fmt.Fprintf(os.Stderr, "Unknown tool spec: %s\n", t.name)
				continue
			}
			fmt.Printf("Installing %s (version: %s)...\n", t.name, t.version)
			path, err := binmanager.DownloadTool(spec, t.version, binDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed: %v\n", err)
				continue
			}
			fmt.Printf("  ✓ Installed to %s\n", path)
		}
	},
}

var toolsUpdateCmd = &cobra.Command{
	Use:   "update [tool]",
	Short: "Update an external tool to the latest version",
	Long:  "Update a specific tool (yt-dlp, tdl, gallery-dl) or all tools if no argument given.",
	Run:   toolsInstallCmd.Run, // Same logic — always downloads the specified/latest version
}

func init() {
	toolsCmd.AddCommand(toolsStatusCmd)
	toolsCmd.AddCommand(toolsInstallCmd)
	toolsCmd.AddCommand(toolsUpdateCmd)

	addCmd.Flags().StringP("mode", "m", "", "Download mode (single, group, default)")
	addCmd.Flags().StringP("platform", "p", "", "Platform (x, telegram, gallery)")
	addCmd.Flags().BoolVar(&timelineFlag, "timeline", false, "Use gallery-dl for account/media timeline URLs (auto-detected if omitted)")
	addCmd.Flags().StringArrayVar(&filterFlags, "filter", nil, "gallery-dl option in key=value form, e.g. --filter retweets=false (can repeat)")
	listCmd.Flags().StringP("status", "s", "", "Filter by status")
	logsCmd.Flags().BoolP("json", "j", false, "Output in JSON format")
	regenerateMetadataCmd.Flags().BoolP("dry-run", "n", false, "Show what would be updated without making changes")
	regenerateMetadataCmd.Flags().StringP("completed-dir", "d", "", "Completed downloads directory (default: from config)")
	eagleImportCmd.Flags().BoolP("dry-run", "n", false, "Preview what would be imported without making changes")
	eagleImportCmd.Flags().StringP("completed-dir", "d", "", "Completed downloads directory (default: from config)")
	eagleRenameCmd.Flags().IntP("max-length", "m", 180, "Maximum name length in bytes (default: 180)")
	eagleRenameCmd.Flags().StringP("folder-id", "f", "", "Only process items in specific folder (optional)")
	eagleRenameCmd.Flags().BoolP("skip-images", "i", false, "Skip image files, only process videos")
	eagleRenameCmd.Flags().StringP("output", "o", "", "Export problematic items as JSON for Eagle plugin API")
	eagleRenameCmd.Flags().BoolP("apply", "a", false, "Actually rename items via Eagle's API")
	eagleRenameCmd.Flags().StringSlice("ids", nil, "Only apply to specific item IDs (comma-separated)")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
