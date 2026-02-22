package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yourusername/x-extract-go/internal/app"
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
		return "http://localhost:8080"
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
correct text for each downloaded file based on the message ID in the filename.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Note: This command doesn't need the server running
		// It reads the database and files directly
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		completedDir, _ := cmd.Flags().GetString("completed-dir")

		if completedDir == "" {
			// Get from config
			config, err := app.LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
			completedDir = config.Download.CompletedDir()
		}

		dbPath := ""
		{
			config, err := app.LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
			dbPath = config.Queue.DatabasePath
		}

		// Load cache into memory for fast lookup
		cache := make(map[string]map[string]string) // channel -> messageID -> text
		loadCache(dbPath, cache)

		updated := 0
		// Process all Telegram JSON files in completed directory
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

			// Check if it's a Telegram file (has numeric ID pattern)
			if !strings.HasPrefix(name, "3464638440_") {
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
			desc, ok := metadata["description"].(string)
			if ok && desc != "" {
				continue // Already has description
			}

			// Look up text in cache - try both filename message ID and URL message ID
			channel := "3464638440" // Hardcoded for now
			text := ""
			if channelCache, ok := cache[channel]; ok {
				// First try the filename message ID
				text = channelCache[msgID]
				// If not found, try the URL message ID from metadata
				if text == "" {
					urlMsgID, ok := metadata["id"].(string)
					if ok {
						text = channelCache[urlMsgID]
					}
				}
			}

			if text == "" {
				continue // No text in cache
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

		// Also update database entries
		fmt.Printf("\nUpdating database entries...\n")
		dbUpdated := 0
		{
			// Get all telegram downloads with empty descriptions
			output, err := exec.Command("sqlite3", dbPath,
				"SELECT id, metadata FROM downloads WHERE platform='telegram' AND status='completed';").Output()
			if err == nil {
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					// Parse: id|metadata
					parts := strings.SplitN(line, "|", 2)
					if len(parts) < 2 {
						continue
					}
					downloadID := parts[0]
					metadataStr := parts[1]

					var metadata map[string]interface{}
					if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
						continue
					}

					// Check if description is empty
					desc, ok := metadata["description"].(string)
					if ok && desc != "" {
						continue // Already has description
					}

					// Get files to find message ID
					files, ok := metadata["files"].([]interface{})
					if len(files) == 0 {
						continue
					}
					filePath, ok := files[0].(string)
					if !ok {
						continue
					}

					// Extract message ID from filename
					filename := filepath.Base(filePath)
					msgID := extractMessageIDFromFilename(filename)
					if msgID == "" {
						continue
					}

					// Look up text in cache - try both filename message ID and URL message ID
					channel := "3464638440"
					text := ""
					if channelCache, ok := cache[channel]; ok {
						// First try the filename message ID
						text = channelCache[msgID]
						// If not found, try the URL message ID from metadata
						if text == "" {
							urlMsgID, ok := metadata["id"].(string)
							if ok {
								text = channelCache[urlMsgID]
							}
						}
					}

					if text == "" {
						continue
					}

					// Update metadata
					metadata["description"] = text
					newMetadataStr, _ := json.Marshal(metadata)

					// Update database
					if !dryRun {
						exec.Command("sqlite3", dbPath,
							fmt.Sprintf("UPDATE downloads SET metadata='%s' WHERE id='%s';",
								strings.ReplaceAll(string(newMetadataStr), "'", "''"), downloadID)).Run()
					}
					fmt.Printf("Updated DB: %s (msg %s)\n", downloadID[:8], msgID)
					dbUpdated++
				}
			}
		}

		if dryRun {
			fmt.Printf("\nDry run: would update %d JSON files and %d DB entries\n", updated, dbUpdated)
		} else {
			fmt.Printf("\nUpdated %d JSON files and %d DB entries\n", updated, dbUpdated)
		}
	},
}

// loadCache loads all Telegram message cache from database into memory
func loadCache(dbPath string, cache map[string]map[string]string) {
	output, err := exec.Command("sqlite3", dbPath, "SELECT channel_id, message_id, text FROM telegram_message_cache;").Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Parse: channel_id|message_id|text
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		channelID := parts[0]
		msgID := parts[1]
		text := parts[2]

		if _, ok := cache[channelID]; !ok {
			cache[channelID] = make(map[string]string)
		}
		cache[channelID][msgID] = text
	}
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

func init() {
	addCmd.Flags().StringP("mode", "m", "", "Download mode (single, group, default)")
	addCmd.Flags().StringP("platform", "p", "", "Platform (x, telegram)")
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
