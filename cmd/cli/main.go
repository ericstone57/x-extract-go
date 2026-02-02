package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
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

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "http://localhost:8080", "Server URL")
	rootCmd.PersistentFlags().BoolVar(&noAutoStart, "no-auto-start", false, "Don't auto-start server if not running")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(retryCmd)
	rootCmd.AddCommand(logsCmd)
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

func init() {
	addCmd.Flags().StringP("mode", "m", "", "Download mode (single, group, default)")
	addCmd.Flags().StringP("platform", "p", "", "Platform (x, telegram)")
	listCmd.Flags().StringP("status", "s", "", "Filter by status")
	logsCmd.Flags().BoolP("json", "j", false, "Output in JSON format")
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
