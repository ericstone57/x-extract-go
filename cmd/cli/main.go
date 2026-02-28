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

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a download to the queue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ensureServer()

		url := args[0]
		mode, _ := cmd.Flags().GetString("mode")
		platform, _ := cmd.Flags().GetString("platform")

		payload := map[string]string{
			"url": url,
		}
		if mode != "" {
			payload["mode"] = mode
		}
		if platform != "" {
			payload["platform"] = platform
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
	listCmd.Flags().StringP("status", "s", "", "Filter by status")
	logsCmd.Flags().BoolP("json", "j", false, "Output in JSON format")
	regenerateMetadataCmd.Flags().BoolP("dry-run", "n", false, "Show what would be updated without making changes")
	regenerateMetadataCmd.Flags().StringP("completed-dir", "d", "", "Completed downloads directory (default: from config)")
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
