// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"x-extract-go/api"
	"x-extract-go/internal/app"
	"x-extract-go/internal/domain"
	"x-extract-go/internal/infrastructure/persistence"
)

func setupTestServer(t *testing.T) (*httptest.Server, *app.DownloadService) {
	// Create in-memory database
	repo, err := persistence.NewSQLiteRepository(":memory:")
	require.NoError(t, err)

	// Create mock downloaders
	mockTwitter := &MockDownloader{}
	mockTelegram := &MockDownloader{}

	// Create service
	config := domain.DefaultConfig()
	service := app.NewDownloadService(repo, mockTwitter, mockTelegram, config)

	// Create router
	router := api.NewRouter(service)

	// Create test server
	server := httptest.NewServer(router)
	
	return server, service
}

type MockDownloader struct{}

func (m *MockDownloader) Download(download *domain.Download) error {
	// Simulate download
	time.Sleep(100 * time.Millisecond)
	download.MarkCompleted("/tmp/test.mp4")
	return nil
}

func TestAPI_AddDownload(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	payload := map[string]string{
		"url": "https://x.com/user/status/123",
	}
	data, _ := json.Marshal(payload)

	resp, err := http.Post(server.URL+"/api/v1/downloads", "application/json", bytes.NewBuffer(data))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["id"])
	assert.Equal(t, "https://x.com/user/status/123", result["url"])
	assert.Equal(t, "x", result["platform"])
	assert.Equal(t, "queued", result["status"])
}

func TestAPI_ListDownloads(t *testing.T) {
	server, service := setupTestServer(t)
	defer server.Close()

	// Add some downloads
	download1 := domain.NewDownload("https://x.com/test1", domain.PlatformX, domain.ModeDefault)
	download2 := domain.NewDownload("https://t.me/test2", domain.PlatformTelegram, domain.ModeDefault)
	
	service.AddDownload(download1)
	service.AddDownload(download2)

	resp, err := http.Get(server.URL + "/api/v1/downloads")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var downloads []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&downloads)
	require.NoError(t, err)

	assert.Len(t, downloads, 2)
}

func TestAPI_GetDownload(t *testing.T) {
	server, service := setupTestServer(t)
	defer server.Close()

	// Add a download
	download := domain.NewDownload("https://x.com/test", domain.PlatformX, domain.ModeDefault)
	err := service.AddDownload(download)
	require.NoError(t, err)

	resp, err := http.Get(server.URL + "/api/v1/downloads/" + download.ID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, download.ID, result["id"])
	assert.Equal(t, download.URL, result["url"])
}

func TestAPI_GetStats(t *testing.T) {
	server, service := setupTestServer(t)
	defer server.Close()

	// Add downloads with different statuses
	d1 := domain.NewDownload("https://x.com/test1", domain.PlatformX, domain.ModeDefault)
	d2 := domain.NewDownload("https://x.com/test2", domain.PlatformX, domain.ModeDefault)
	d2.MarkCompleted("/tmp/test.mp4")
	d3 := domain.NewDownload("https://x.com/test3", domain.PlatformX, domain.ModeDefault)
	d3.MarkFailed(assert.AnError)

	service.AddDownload(d1)
	service.AddDownload(d2)
	service.AddDownload(d3)

	resp, err := http.Get(server.URL + "/api/v1/downloads/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(t, err)

	assert.Equal(t, float64(3), stats["total"])
	assert.Equal(t, float64(1), stats["queued"])
	assert.Equal(t, float64(1), stats["completed"])
	assert.Equal(t, float64(1), stats["failed"])
}

func TestAPI_CancelDownload(t *testing.T) {
	server, service := setupTestServer(t)
	defer server.Close()

	// Add a download
	download := domain.NewDownload("https://x.com/test", domain.PlatformX, domain.ModeDefault)
	err := service.AddDownload(download)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/api/v1/downloads/"+download.ID+"/cancel", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify download was cancelled
	updated, err := service.GetDownload(download.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, updated.Status)
}

