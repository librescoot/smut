#!/bin/bash
# Test script for file:// URL support in Simple Mender Update Tool
# This script creates a dummy update file and pushes a file:// URL to Redis

set -e

# Check if redis-cli is available
if ! command -v redis-cli &> /dev/null; then
    echo "Error: redis-cli is not installed or not in PATH"
    exit 1
fi

# Check if Redis server is running
if ! redis-cli ping &> /dev/null; then
    echo "Error: Redis server is not running or not accessible"
    exit 1
fi

# Create a temporary directory for the test
TEMP_DIR=$(mktemp -d -t smut-test-XXXXXX)
echo "Created temporary directory: $TEMP_DIR"

# Create a dummy update file
UPDATE_FILE="$TEMP_DIR/test-update.mender"
echo "This is a test update file for SMUT file:// URL support" > "$UPDATE_FILE"
echo "Created test update file: $UPDATE_FILE"

# Create the file:// URL
FILE_URL="file://$UPDATE_FILE"
echo "File URL: $FILE_URL"

# Redis key for updates (use the default or specify your own)
REDIS_KEY=${1:-"mender/update/url"}
echo "Using Redis key: $REDIS_KEY"

# Push the file:// URL to Redis
redis-cli LPUSH "$REDIS_KEY" "$FILE_URL"
echo "Pushed file:// URL to Redis key '$REDIS_KEY'"
echo "The Simple Mender Update Tool should now detect this URL and use the local file directly."

# Wait for a while to allow the updater to process it
echo "Waiting for the updater to process the file..."
sleep 5

# Check the status in Redis
STATUS=$(redis-cli HGET ota status)
echo "Current status in Redis: $STATUS"

# Clean up
echo "Test completed. Cleaning up..."
rm -rf "$TEMP_DIR"
echo "Temporary directory removed."
