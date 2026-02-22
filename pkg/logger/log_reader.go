package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogEntry represents a parsed log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Category  string                 `json:"category"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Raw       bool                   `json:"raw,omitempty"` // True if this is raw text (not JSON)
}

// LogReader provides functionality to read and stream log files
type LogReader struct {
	logsDir string
}

// NewLogReader creates a new log reader
func NewLogReader(logsDir string) *LogReader {
	return &LogReader{
		logsDir: logsDir,
	}
}

// GetLogPath returns the path to a category log file for a specific date
func (lr *LogReader) GetLogPath(category LogCategory, date time.Time) string {
	dateStr := date.Format("20060102")
	filename := fmt.Sprintf("%s-%s.log", category, dateStr)
	return filepath.Join(lr.logsDir, filename)
}

// GetTodayLogPath returns the path to today's log file for a category
func (lr *LogReader) GetTodayLogPath(category LogCategory) string {
	return lr.GetLogPath(category, time.Now())
}

// isRawTextCategory returns true if the category uses raw text format
// Note: "download" and "stderr" are raw text logs written directly by downloaders
func isRawTextCategory(category LogCategory) bool {
	return category == "download" || category == "stderr"
}

// ReadLogs reads log entries from a category log file
func (lr *LogReader) ReadLogs(category LogCategory, date time.Time, limit int) ([]LogEntry, error) {
	logPath := lr.GetLogPath(category, date)

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil // Return empty slice if file doesn't exist
		}
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)

	// Read all lines first
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Get last N lines if limit is specified
	startIdx := 0
	if limit > 0 && len(lines) > limit {
		startIdx = len(lines) - limit
	}

	// Parse log entries based on category format
	isRaw := isRawTextCategory(category)

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}

		var entry LogEntry

		if isRaw {
			// Raw text format (download logs)
			entry = lr.parseRawLogLine(line, category)
		} else {
			// JSON format (queue, error logs)
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				// Fallback for malformed JSON
				entry = LogEntry{
					Timestamp: "",
					Level:     "info",
					Message:   line,
					Category:  string(category),
				}
			}
			entry.Category = string(category)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parseRawLogLine parses a raw text log line into a LogEntry
func (lr *LogReader) parseRawLogLine(line string, category LogCategory) LogEntry {
	entry := LogEntry{
		Message:  line,
		Category: string(category),
		Raw:      true,
		Level:    "info",
	}

	// Try to extract timestamp from formatted lines like "[2006-01-02 15:04:05]"
	if strings.HasPrefix(line, "[") {
		if idx := strings.Index(line, "]"); idx > 0 {
			possibleTs := line[1:idx]
			if _, err := time.Parse("2006-01-02 15:04:05", possibleTs); err == nil {
				entry.Timestamp = possibleTs
				entry.Message = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	// Detect log level from content
	if strings.Contains(line, "[STDERR]") || strings.Contains(line, "ERROR") || strings.Contains(line, "FAILED") {
		entry.Level = "error"
	} else if strings.Contains(line, "WARNING") || strings.Contains(line, "WARN") {
		entry.Level = "warn"
	} else if strings.HasPrefix(line, "===") {
		entry.Level = "debug" // Section markers
	}

	return entry
}

// ReadTodayLogs reads today's log entries for a category
func (lr *LogReader) ReadTodayLogs(category LogCategory, limit int) ([]LogEntry, error) {
	return lr.ReadLogs(category, time.Now(), limit)
}

// SearchLogs searches for log entries matching a query
func (lr *LogReader) SearchLogs(category LogCategory, date time.Time, query string, limit int) ([]LogEntry, error) {
	entries, err := lr.ReadLogs(category, date, 0) // Read all
	if err != nil {
		return nil, err
	}

	var filtered []LogEntry
	query = strings.ToLower(query)

	for _, entry := range entries {
		// Search in message and level
		if strings.Contains(strings.ToLower(entry.Message), query) ||
			strings.Contains(strings.ToLower(entry.Level), query) {
			filtered = append(filtered, entry)
		}
	}

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	return filtered, nil
}

// TailLogs tails a log file and sends new entries to a channel
func (lr *LogReader) TailLogs(category LogCategory, entryChan chan<- LogEntry, stopChan <-chan struct{}) error {
	logPath := lr.GetTodayLogPath(category)
	isRaw := isRawTextCategory(category)

	// Open file
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Wait for file to be created
			time.Sleep(1 * time.Second)
			return lr.TailLogs(category, entryChan, stopChan)
		}
		return err
	}
	defer file.Close()

	// Seek to end of file
	file.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(file)

	for {
		select {
		case <-stopChan:
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// No more data, wait a bit
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return err
			}

			// Parse and send entry
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var entry LogEntry
			if isRaw {
				// Raw text format (download logs)
				entry = lr.parseRawLogLine(line, category)
			} else {
				// JSON format (queue, error logs)
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					entry = LogEntry{
						Timestamp: time.Now().Format(time.RFC3339),
						Level:     "info",
						Message:   line,
						Category:  string(category),
					}
				}
				entry.Category = string(category)
			}

			entryChan <- entry
		}
	}
}
