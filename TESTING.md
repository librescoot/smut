# Simple Mender Update Tool Tests

## Test Files

- `download_test.go`: Tests for the download manager functionality
- `file_url_support_test.go`: Tests for the file:// URL support
- `file_url_test.go`: Integration test for file:// URL support with Redis
- `main_test.go`: Tests for the main application logic (currently disabled)

## Running Tests

### Go Tests

To run all Go tests:

```bash
go test -v
```

To run a specific Go test:

```bash
go test -v -run TestDownloadManager
go test -v -run TestFileURLSupport
go test -v -run TestChecksumVerification
```

### Shell Script Tests

The repository also includes a shell script for testing the file:// URL support with a running instance of the application:

```bash
# Make sure the script is executable
chmod +x tests/test_file_url.sh

# Run the test with the default Redis key (mender/update/url)
./tests/test_file_url.sh

# Or specify a custom Redis key
./tests/test_file_url.sh mender/update/mdb/url
```

This script:
1. Creates a dummy update file
2. Pushes a file:// URL to Redis
3. Waits for the application to process it
4. Checks the status in Redis
5. Cleans up temporary files

## Test Requirements

Some tests require specific setup:

1. `file_url_test.go` requires:
   - A running Redis server on localhost:6379
   - The Simple Mender Update Tool running and configured to use the same Redis server

2. `download_test.go` tests HTTP downloads and requires:
   - Network connectivity
   - Sufficient permissions to create temporary directories and files

## Test Coverage

The tests cover:

1. Download functionality:
   - Basic HTTP downloads
   - Download resumption
   - Error handling

2. Checksum verification:
   - SHA256 checksum verification
   - Error handling for invalid checksums

3. File URL support:
   - Handling file:// URLs
   - Skipping download for local files
   - Checksum verification for local files

## Adding New Tests

When adding new tests:

1. Follow Go testing best practices
2. Use descriptive test names
3. Add proper documentation
4. Ensure tests are independent and can run in isolation
5. Clean up any temporary files or resources created during tests
