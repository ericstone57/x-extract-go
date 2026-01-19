package infrastructure

import (
	"fmt"
	"os/exec"

	"github.com/yourusername/x-extract-go/internal/domain"
	"go.uber.org/zap"
)

// NotificationService handles sending notifications
type NotificationService struct {
	config *domain.NotificationConfig
	logger *zap.Logger
}

// NewNotificationService creates a new notification service
func NewNotificationService(config *domain.NotificationConfig, logger *zap.Logger) *NotificationService {
	return &NotificationService{
		config: config,
		logger: logger,
	}
}

// Send sends a notification
func (n *NotificationService) Send(title, message string) error {
	if !n.config.Enabled {
		n.logger.Debug("Notifications disabled, skipping",
			zap.String("title", title),
			zap.String("message", message))
		return nil
	}

	switch n.config.Method {
	case "osascript":
		return n.sendOSAScript(title, message)
	case "notify-send":
		return n.sendNotifySend(title, message)
	default:
		n.logger.Warn("Unknown notification method", zap.String("method", n.config.Method))
		return nil
	}
}

// sendOSAScript sends notification using macOS osascript
func (n *NotificationService) sendOSAScript(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	cmd := exec.Command("osascript", "-e", script)

	if err := cmd.Run(); err != nil {
		n.logger.Error("Failed to send notification",
			zap.String("method", "osascript"),
			zap.Error(err))
		return err
	}

	n.logger.Debug("Notification sent",
		zap.String("title", title),
		zap.String("message", message))

	return nil
}

// sendNotifySend sends notification using Linux notify-send
func (n *NotificationService) sendNotifySend(title, message string) error {
	cmd := exec.Command("notify-send", title, message)

	if err := cmd.Run(); err != nil {
		n.logger.Error("Failed to send notification",
			zap.String("method", "notify-send"),
			zap.Error(err))
		return err
	}

	n.logger.Debug("Notification sent",
		zap.String("title", title),
		zap.String("message", message))

	return nil
}

// NotifyDownloadQueued sends notification when download is queued
func (n *NotificationService) NotifyDownloadQueued(url string, platform domain.Platform) {
	title := "Download Queued"
	message := fmt.Sprintf("Added to queue: %s (%s)", truncateString(url, 30), platform)
	n.Send(title, message)
}

// NotifyDownloadStarted sends notification when download starts
func (n *NotificationService) NotifyDownloadStarted(url string, platform domain.Platform) {
	title := "Download Started"
	message := fmt.Sprintf("Processing: %s (%s)", truncateString(url, 30), platform)
	n.Send(title, message)
}

// NotifyDownloadCompleted sends notification when download completes
func (n *NotificationService) NotifyDownloadCompleted(url string, platform domain.Platform) {
	title := "Download Completed"
	message := fmt.Sprintf("Success: %s (%s)", truncateString(url, 30), platform)
	n.Send(title, message)
}

// NotifyDownloadFailed sends notification when download fails
func (n *NotificationService) NotifyDownloadFailed(url string, platform domain.Platform, err error) {
	title := "Download Failed"
	message := fmt.Sprintf("Failed: %s (%s)", truncateString(url, 30), platform)
	n.Send(title, message)
}

// NotifyQueueEmpty sends notification when queue is empty
func (n *NotificationService) NotifyQueueEmpty() {
	title := "Queue Empty"
	message := "All downloads completed"
	n.Send(title, message)
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

