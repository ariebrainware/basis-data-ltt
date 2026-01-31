package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInitGeoIP_EmptyPath(t *testing.T) {
	// Should not error with empty path
	err := InitGeoIP("")
	if err != nil {
		t.Errorf("Expected no error with empty path, got %v", err)
	}
}

func TestInitGeoIP_NonExistentFile(t *testing.T) {
	err := InitGeoIP("/nonexistent/path/to/geoip.mmdb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestValidateGeoIP_NonExistentFile(t *testing.T) {
	err := ValidateGeoIP("/nonexistent/path/to/geoip.mmdb")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestGetIPLocation_EmptyIP(t *testing.T) {
	loc := GetIPLocation("")
	if loc.City != "" || loc.Country != "" {
		t.Errorf("Expected empty IPLocation for empty IP, got %+v", loc)
	}
}

func TestGetIPLocation_PrivateIPs(t *testing.T) {
	testCases := []string{
		"127.0.0.1",
		"::1",
		"10.0.0.1",
		"10.255.255.255",
		"192.168.1.1",
		"192.168.0.0",
		"::",
		"::ffff",
	}

	for _, ip := range testCases {
		loc := GetIPLocation(ip)
		if loc.City != "" || loc.Country != "" {
			t.Errorf("Expected empty IPLocation for private IP %s, got %+v", ip, loc)
		}
	}
}

func TestGetIPLocation_NoDB(t *testing.T) {
	// Ensure DB is nil
	geoipDB = nil
	geoipCache = nil

	loc := GetIPLocation("8.8.8.8")
	if loc.City != "" || loc.Country != "" {
		t.Errorf("Expected empty IPLocation when DB is nil, got %+v", loc)
	}
}

func TestGetGeoIPCacheMetrics_NoCache(t *testing.T) {
	geoipCache = nil
	hits, misses, size := GetGeoIPCacheMetrics()

	// Metrics should still be accessible even without cache
	_ = hits
	_ = misses
	if size != 0 {
		t.Errorf("Expected size 0 when cache is nil, got %d", size)
	}
}

func TestDownloadGeoIP_InvalidURL(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "geoip.mmdb")

	_, err := DownloadGeoIPWithRequest(ctx, DownloadRequest{URL: "http://invalid-url-that-does-not-exist-12345.com/file.mmdb", DestPath: destPath})
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestDownloadGeoIP_HTTPError(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "geoip.mmdb")

	_, err := DownloadGeoIPWithRequest(ctx, DownloadRequest{URL: server.URL, DestPath: destPath})
	if err == nil {
		t.Error("Expected error for HTTP 404")
	}
}

func TestDownloadGeoIP_Success(t *testing.T) {
	// Create a test server that returns mock data
	mockData := []byte("mock geoip database content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(mockData); err != nil {
			t.Fatalf("failed to write mock response: %v", err)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "geoip.mmdb")

	resultPath, err := DownloadGeoIPWithRequest(ctx, DownloadRequest{URL: server.URL, DestPath: destPath})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resultPath != destPath {
		t.Errorf("Expected result path %s, got %s", destPath, resultPath)
	}

	// Verify file was written
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(data) != string(mockData) {
		t.Errorf("Expected file content %q, got %q", mockData, data)
	}
}

func TestDownloadGeoIP_ContextCancellation(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "geoip.mmdb")

	_, err := DownloadGeoIPWithRequest(ctx, DownloadRequest{URL: server.URL, DestPath: destPath})
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
}

func TestCloseGeoIP(t *testing.T) {
	// Should not panic when DB is nil
	geoipDB = nil
	CloseGeoIP()

	// Verify it's still nil
	if geoipDB != nil {
		t.Error("Expected geoipDB to remain nil after CloseGeoIP")
	}
}

func TestGetIPLocation_InvalidIP(t *testing.T) {
	geoipDB = nil
	geoipCache = nil

	// Test with invalid IP format
	loc := GetIPLocation("not-an-ip")
	if loc.City != "" || loc.Country != "" {
		t.Errorf("Expected empty IPLocation for invalid IP, got %+v", loc)
	}
}

func TestDownloadGeoIP_TempFileCleanupOnError(t *testing.T) {
	// Create a test server that returns an invalid response after headers
	// This will cause an error during the copy phase, after temp file is created
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write some data then close connection abruptly
		w.Write([]byte("partial data"))
		// Force connection close by panicking (httptest will handle this)
		panic("simulated connection error")
	}))
	defer server.Close()

	ctx := context.Background()
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "geoip.mmdb")

	// Count files before download attempt
	beforeFiles, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	beforeCount := len(beforeFiles)

	// Attempt download - should fail
	_, err = DownloadGeoIPWithRequest(ctx, DownloadRequest{URL: server.URL, DestPath: destPath})
	if err == nil {
		t.Error("Expected error due to simulated connection error")
	}

	// Count files after download attempt - cleanup is synchronous in defer
	afterFiles, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}
	afterCount := len(afterFiles)

	// Should have no additional files (temp file should be cleaned up)
	if afterCount != beforeCount {
		t.Errorf("Expected no new files in temp dir after error, before=%d, after=%d", beforeCount, afterCount)
		// Log what files remain for debugging
		for _, f := range afterFiles {
			t.Logf("Remaining file: %s", f.Name())
		}
	}
}
