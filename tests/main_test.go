package tests

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/librescoot/smut/pkg/config"
	"github.com/librescoot/smut/pkg/download"
)

// MockRedisClient is a mock implementation of the Redis client for testing
type MockRedisClient struct {
	status      string
	updateType  string
	failureMsg  string
	checksumVal string
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		status:     "initializing",
		updateType: "non-blocking",
	}
}

func (m *MockRedisClient) SetStatus(ctx context.Context, status string) error {
	m.status = status
	return nil
}

func (m *MockRedisClient) SetUpdateType(ctx context.Context, updateType string) error {
	m.updateType = updateType
	return nil
}

func (m *MockRedisClient) SetFailure(ctx context.Context, key, message string) error {
	m.failureMsg = message
	return nil
}

func (m *MockRedisClient) GetChecksum(ctx context.Context, key string) (string, error) {
	return m.checksumVal, nil
}

func (m *MockRedisClient) Close() error {
	return nil
}

// MockMenderClient is a mock implementation of the Mender client for testing
type MockMenderClient struct {
	installCalled bool
	commitCalled  bool
	needsCommit   bool
	installErr    error
	commitErr     error
}

func NewMockMenderClient() *MockMenderClient {
	return &MockMenderClient{
		needsCommit: false,
	}
}

func (m *MockMenderClient) Install(filePath string) error {
	m.installCalled = true
	return m.installErr
}

func (m *MockMenderClient) Commit() error {
	m.commitCalled = true
	return m.commitErr
}

func (m *MockMenderClient) NeedsCommit() (bool, error) {
	return m.needsCommit, nil
}

// This test is disabled because it requires mocking the actual handleUpdate function from main.go
// It's included here as an example of how to test the file:// URL support
// DISABLED_TestHandleUpdate tests the handleUpdate function with different URL types
func DISABLED_TestHandleUpdate(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "smut-handle-update-test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file to use with file:// URL
	testContent := []byte("This is a test file for handleUpdate testing")
	testFilePath := filepath.Join(tempDir, "test-update.mender")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Create a download directory
	downloadDir := filepath.Join(tempDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		t.Fatalf("Error creating download directory: %v", err)
	}

	// Test cases
	testCases := []struct {
		name          string
		url           string
		checksum      string
		expectInstall bool
		expectError   bool
	}{
		{
			name:          "File URL",
			url:           "file://" + testFilePath,
			checksum:      "",
			expectInstall: true,
			expectError:   false,
		},
		{
			name:          "File URL with checksum",
			url:           "file://" + testFilePath,
			checksum:      "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // Incorrect checksum
			expectInstall: false,
			expectError:   true,
		},
		// Add more test cases as needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the components
			downloadManager := download.NewManager(downloadDir)
			mockRedisClient := NewMockRedisClient()
			mockRedisClient.checksumVal = tc.checksum
			mockMenderClient := NewMockMenderClient()

			// Create a config
			cfg := &config.Config{
				RedisAddr:   "localhost:6379",
				UpdateKey:   "mender/update/url",
				ChecksumKey: "mender/update/checksum",
				FailureKey:  "mender/update/last-failure",
				UpdateType:  "non-blocking",
				DownloadDir: downloadDir,
			}

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Call handleUpdate
			err := handleUpdate(ctx, tc.url, downloadManager, mockMenderClient, mockRedisClient, cfg)

			// Check results
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if mockMenderClient.installCalled != tc.expectInstall {
				t.Errorf("Expected install called: %v, got: %v", tc.expectInstall, mockMenderClient.installCalled)
			}

			// Check status
			if err == nil {
				if mockRedisClient.status != "installation-complete-waiting-reboot" {
					t.Errorf("Expected status 'installation-complete-waiting-reboot', got: %s", mockRedisClient.status)
				}
			} else {
				if mockRedisClient.failureMsg == "" {
					t.Errorf("Expected failure message to be set")
				}
			}
		})
	}
}

// MenderClientInterface defines the interface for the Mender client
type MenderClientInterface interface {
	Install(filePath string) error
	Commit() error
	NeedsCommit() (bool, error)
}

// handleUpdate is a copy of the function from main.go for testing
// This should be kept in sync with the actual implementation
func handleUpdate(
	ctx context.Context,
	url string,
	downloadManager *download.Manager,
	menderClient MenderClientInterface,
	redisClient interface {
		SetStatus(ctx context.Context, status string) error
		GetChecksum(ctx context.Context, key string) (string, error)
		SetFailure(ctx context.Context, key, message string) error
	},
	cfg *config.Config,
) error {
	var downloadPath string
	var err error
	
	// Check if this is a file:// URL
	if strings.HasPrefix(url, "file://") {
		// For file:// URLs, extract the path and skip downloading
		filePath := strings.TrimPrefix(url, "file://")
		log.Printf("Using local file: %s", filePath)
		downloadPath = filePath
	} else {
		// Set status to downloading-updates for non-file URLs
		if err := redisClient.SetStatus(ctx, "downloading-updates"); err != nil {
			log.Printf("Error setting status to downloading-updates in Redis: %v", err)
		}

		downloadPath, err = downloadManager.Download(ctx, url)
		if err != nil {
			// Set status to downloading-update-error on download error
			if err := redisClient.SetStatus(ctx, "downloading-update-error"); err != nil {
				log.Printf("Error setting status to downloading-update-error in Redis: %v", err)
			}
			return fmt.Errorf("error downloading update: %w", err)
		}
		log.Printf("Downloaded update to: %s", downloadPath)
	}

	checksum, err := redisClient.GetChecksum(ctx, cfg.ChecksumKey)
	if err != nil {
		log.Printf("Warning: Could not retrieve checksum from Redis: %v", err)
	}

	if checksum != "" {
		log.Printf("Verifying checksum: %s", checksum)
		if err := downloadManager.VerifyChecksum(downloadPath, checksum); err != nil {
			os.Remove(downloadPath)
			// Set status to downloading-update-error on checksum mismatch
			if err := redisClient.SetStatus(ctx, "downloading-update-error"); err != nil {
				log.Printf("Error setting status to downloading-update-error in Redis: %v", err)
			}
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		log.Println("Checksum verification successful")
	} else {
		log.Println("No checksum provided, skipping verification")
	}

	log.Println("Installing update...")
	// Set status to installing-updates
	if err := redisClient.SetStatus(ctx, "installing-updates"); err != nil {
		log.Printf("Error setting status to installing-updates in Redis: %v", err)
	}

	if err := menderClient.Install(downloadPath); err != nil {
		os.Remove(downloadPath)
		// Set status to installing-update-error on install error
		if err := redisClient.SetStatus(ctx, "installing-update-error"); err != nil {
			log.Printf("Error setting status to installing-update-error in Redis: %v", err)
		}
		return fmt.Errorf("error installing update: %w", err)
	}
	log.Println("Update installed successfully")

	// Only remove the file if it was downloaded (not a file:// URL)
	if !strings.HasPrefix(url, "file://") {
		if err := os.Remove(downloadPath); err != nil {
			log.Printf("Warning: Failed to remove downloaded file %s: %v", downloadPath, err)
		}
	}

	// Set final success status based on update type
	successStatus := "installation-complete-waiting-reboot" // Default for non-blocking
	if cfg.UpdateType == "blocking" {
		successStatus = "installation-complete-waiting-dashboard-reboot"
	}
	if err := redisClient.SetStatus(ctx, successStatus); err != nil {
		log.Printf("Error setting final success status in Redis: %v", err)
	}

	return nil
}
