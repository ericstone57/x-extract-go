package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	serverStartTimeout = 10 * time.Second
	serverPollInterval = 200 * time.Millisecond
)

// isServerRunning checks if the server is responding to health checks
func isServerRunning() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// findServerBinary locates the x-extract-server binary
func findServerBinary() (string, error) {
	// 1. Check same directory as CLI binary
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		serverPath := filepath.Join(execDir, "x-extract-server")
		if _, err := os.Stat(serverPath); err == nil {
			return serverPath, nil
		}
	}

	// 2. Check PATH
	serverPath, err := exec.LookPath("x-extract-server")
	if err == nil {
		return serverPath, nil
	}

	// 3. Check common locations
	commonPaths := []string{
		"/usr/local/bin/x-extract-server",
		"/usr/bin/x-extract-server",
		filepath.Join(os.Getenv("HOME"), "go/bin/x-extract-server"),
		filepath.Join(os.Getenv("HOME"), ".local/bin/x-extract-server"),
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("x-extract-server binary not found")
}

// startServerBackground starts the server as a detached background process
func startServerBackground() error {
	serverPath, err := findServerBinary()
	if err != nil {
		return err
	}

	// Start server as detached process
	cmd := exec.Command(serverPath)

	// Detach from parent process
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Set process group to detach from terminal
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Don't wait for the process - let it run in background
	go func() {
		cmd.Wait()
	}()

	return nil
}

// waitForServerReady polls the server until it's ready or timeout
func waitForServerReady() error {
	deadline := time.Now().Add(serverStartTimeout)

	for time.Now().Before(deadline) {
		if isServerRunning() {
			return nil
		}
		time.Sleep(serverPollInterval)
	}

	return fmt.Errorf("server did not start within %v", serverStartTimeout)
}

// ensureServerRunning checks if server is running, starts it if not
func ensureServerRunning() error {
	if isServerRunning() {
		return nil
	}

	fmt.Println("Server not running, starting...")

	if err := startServerBackground(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	if err := waitForServerReady(); err != nil {
		return err
	}

	fmt.Println("Server started successfully")
	return nil
}
