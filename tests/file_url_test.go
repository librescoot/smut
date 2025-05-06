package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// TestFileURL tests the file:// URL support in the Simple Mender Update Tool.
// It creates a dummy update file and pushes a file:// URL to Redis.
//
// Note: This is an integration test that requires:
// 1. A running Redis server on localhost:6379
// 2. The Simple Mender Update Tool running and configured to use the same Redis server
//
// Run with: go test -v ./tests
func TestFileURL(t *testing.T) {
	// Default Redis key for updates
	redisKey := "mender/update/url"

	// Create a temporary file to simulate an update file
	tempDir, err := os.MkdirTemp("", "smut-test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	updateFile := filepath.Join(tempDir, "test-update.mender")

	// Write some dummy content to the file
	content := []byte("This is a test update file for SMUT file:// URL support")
	if err := os.WriteFile(updateFile, content, 0644); err != nil {
		t.Fatalf("Error writing test file: %v", err)
	}

	t.Logf("Created test update file: %s", updateFile)

	// Connect to Redis
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer rdb.Close()

	// Test Redis connection
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("Error connecting to Redis: %v", err)
	}

	// Create the file:// URL
	fileURL := fmt.Sprintf("file://%s", updateFile)

	// Push the file:// URL to Redis
	err = rdb.LPush(ctx, redisKey, fileURL).Err()
	if err != nil {
		t.Fatalf("Error pushing URL to Redis: %v", err)
	}

	t.Logf("Pushed file:// URL to Redis key '%s': %s", redisKey, fileURL)
	t.Log("The Simple Mender Update Tool should now detect this URL and use the local file directly.")

	// Keep the file around for a while to allow the updater to process it
	t.Log("Waiting for the updater to process the file...")
	time.Sleep(5 * time.Second)

	// Check if the updater has processed the file by checking Redis status
	// This is a simple check and might need to be adjusted based on your actual implementation
	status, err := rdb.HGet(ctx, "ota", "status").Result()
	if err != nil {
		t.Logf("Warning: Could not get status from Redis: %v", err)
	} else {
		t.Logf("Current status in Redis: %s", status)
	}

	t.Log("Test completed.")
}
