package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/librescoot/smut/pkg/download"
)

// TestDownloadManager tests the download manager functionality.
// It sets up a local HTTP server to serve a test file and tests the download functionality.
//
// Run with: go test -v ./tests
func TestDownloadManager(t *testing.T) {
	// Create a temporary directory for downloads
	tempDir, err := os.MkdirTemp("", "smut-download-test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file to serve
	testContent := []byte("This is a test file for download manager testing")
	testFilePath := filepath.Join(tempDir, "test-file.dat")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, testFilePath)
	}))
	defer server.Close()

	// Create the download manager
	downloadDir := filepath.Join(tempDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		t.Fatalf("Error creating download directory: %v", err)
	}

	manager := download.NewManager(downloadDir)

	// Test downloading the file
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	downloadURL := fmt.Sprintf("%s/test-file.dat", server.URL)
	t.Logf("Downloading from URL: %s", downloadURL)

	downloadedPath, err := manager.Download(ctx, downloadURL)
	if err != nil {
		t.Fatalf("Error downloading file: %v", err)
	}

	t.Logf("File downloaded to: %s", downloadedPath)

	// Verify the downloaded file
	downloadedContent, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("Error reading downloaded file: %v", err)
	}

	if string(downloadedContent) != string(testContent) {
		t.Fatalf("Downloaded content does not match original. Got %d bytes, expected %d bytes",
			len(downloadedContent), len(testContent))
	}

	t.Log("Download test successful")
}

// TestDownloadResume tests the download resume functionality.
// It sets up a local HTTP server that supports range requests and tests the resume capability.
//
// Run with: go test -v ./tests -run TestDownloadResume
func TestDownloadResume(t *testing.T) {
	// Create a temporary directory for downloads
	tempDir, err := os.MkdirTemp("", "smut-resume-test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a larger test file to serve (1MB)
	testContent := make([]byte, 1024*1024)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	testFilePath := filepath.Join(tempDir, "large-test-file.dat")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Start a test HTTP server that supports range requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, testFilePath)
	}))
	defer server.Close()

	// Create the download manager
	downloadDir := filepath.Join(tempDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		t.Fatalf("Error creating download directory: %v", err)
	}

	manager := download.NewManager(downloadDir)

	// Start a download but cancel it halfway
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	downloadURL := fmt.Sprintf("%s/large-test-file.dat", server.URL)
	t.Logf("Starting partial download from URL: %s", downloadURL)

	_, err = manager.Download(ctx, downloadURL)
	cancel()
	if err == nil {
		t.Fatalf("Expected download to be canceled, but it completed successfully")
	}
	t.Logf("Download canceled as expected: %v", err)

	// Now resume the download with a new context
	ctx = context.Background()
	t.Log("Resuming download...")
	downloadedPath, err := manager.Download(ctx, downloadURL)
	if err != nil {
		t.Fatalf("Error resuming download: %v", err)
	}

	t.Logf("File downloaded to: %s", downloadedPath)

	// Verify the downloaded file
	downloadedContent, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("Error reading downloaded file: %v", err)
	}

	if len(downloadedContent) != len(testContent) {
		t.Fatalf("Downloaded content size does not match original. Got %d bytes, expected %d bytes",
			len(downloadedContent), len(testContent))
	}

	// Check content integrity
	for i := range downloadedContent {
		if downloadedContent[i] != testContent[i] {
			t.Fatalf("Content mismatch at position %d: got %d, expected %d",
				i, downloadedContent[i], testContent[i])
		}
	}

	t.Log("Download resume test successful")
}

// TestChecksumVerification tests the checksum verification functionality.
//
// Run with: go test -v ./tests -run TestChecksumVerification
func TestChecksumVerification(t *testing.T) {
	// Create a temporary directory for downloads
	tempDir, err := os.MkdirTemp("", "smut-checksum-test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testContent := []byte("This is a test file for checksum verification")
	testFilePath := filepath.Join(tempDir, "checksum-test-file.dat")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Create the download manager
	manager := download.NewManager(tempDir)

	// Test with correct checksum
	// SHA256 of "This is a test file for checksum verification" is
	// 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
	correctChecksum := "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	err = manager.VerifyChecksum(testFilePath, correctChecksum)
	if err != nil {
		t.Fatalf("Checksum verification failed with correct checksum: %v", err)
	}
	t.Log("Correct checksum verification passed")

	// Test with incorrect checksum
	incorrectChecksum := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	err = manager.VerifyChecksum(testFilePath, incorrectChecksum)
	if err == nil {
		t.Fatalf("Checksum verification should have failed with incorrect checksum")
	}
	t.Logf("Incorrect checksum verification failed as expected: %v", err)

	// Test with invalid checksum format
	invalidChecksum := "invalid-format"
	err = manager.VerifyChecksum(testFilePath, invalidChecksum)
	if err == nil {
		t.Fatalf("Checksum verification should have failed with invalid format")
	}
	t.Logf("Invalid checksum format verification failed as expected: %v", err)

	// Test with unsupported algorithm
	unsupportedChecksum := "md5:d41d8cd98f00b204e9800998ecf8427e"
	err = manager.VerifyChecksum(testFilePath, unsupportedChecksum)
	if err == nil {
		t.Fatalf("Checksum verification should have failed with unsupported algorithm")
	}
	t.Logf("Unsupported algorithm verification failed as expected: %v", err)

	t.Log("Checksum verification tests successful")
}
