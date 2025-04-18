# Simple Updater

A lightweight daemon for managing Mender updates on embedded Linux systems via Redis.

## Overview

Simple Updater runs on embedded Linux systems (MDB and DBC) and triggers Mender updates via Redis. It handles resumable downloads, checksum verification, and interacts with Mender.

## Features

- Redis BLPOP for update requests
- Resumable downloads with retry
- Checksum verification (SHA256)
- Automatic Mender update install and commit
- Error reporting via Redis
- Cross-compilation for armv7l
- No CGO

## Requirements

- Go 1.16+ (for building)
- Redis server
- Mender update client (`mender-update` in PATH)
- Linux (armv7l target)

## Building

### For Local Development

```bash
make
```

### For ARM Target (armv7l)

```bash
make build-arm
```

### Clean Build

```bash
make clean build
```

## Installation

1. Copy the binary to the target system:

```bash
scp simple-updater user@target:/tmp/
```

2. Move to a permanent location:

```bash
ssh user@target "sudo mv /tmp/simple-updater /usr/local/bin/ && sudo chmod +x /usr/local/bin/simple-updater"
```

3. Install the systemd service template:

```bash
scp simple-updater@.service user@target:/tmp/
ssh user@target "sudo mv /tmp/simple-updater@.service /etc/systemd/system/ && sudo systemctl daemon-reload"
```

4. Create the download directory:

```bash
ssh user@target "sudo mkdir -p /var/lib/mender/download && sudo chmod 755 /var/lib/mender/download"
```

5. Enable and start the service for a specific system (e.g., mdb):

```bash
ssh user@target "sudo systemctl enable simple-updater@mdb && sudo systemctl start simple-updater@mdb"
```

Replace `mdb` with `dbc` for the DBC system.

## Usage

### Command-Line Arguments

- `--redis-addr`: Redis server address (default: "localhost:6379")
- `--update-key`: Redis key for update URLs (default: "mender/update/url")
- `--checksum-key`: Redis key for checksums (default: "mender/update/checksum")
- `--failure-key`: Redis key to set on failure (default: "mender/update/last-failure")
- `--download-dir`: Directory to store downloaded update files (default: "/tmp")

### Redis Usage

To trigger an update, push the URL to the update key using LPUSH:

```bash
# For MDB system
redis-cli LPUSH mender/update/mdb/url "http://example.com/path/to/update.mender"

# For DBC system
redis-cli LPUSH mender/update/dbc/url "http://example.com/path/to/update.mender"
```

To set a checksum (optional):

```bash
# For MDB system
redis-cli SET mender/update/mdb/checksum "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

# For DBC system
redis-cli SET mender/update/dbc/checksum "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
```

The `checksum` is optional. If provided, it should be in the format `algorithm:hash`. Currently, only SHA256 is supported.

### Error Reporting

Errors are reported by setting the configured failure key in Redis with the error message as a string.

## Monitoring

### Logs

When running as a systemd service, logs can be viewed with:

```bash
journalctl -u simple-updater@<system> -f
```

Replace `<system>` with `mdb` or `dbc`.

### Status

Check the service status with:

```bash
systemctl status simple-updater@<system>
```

Replace `<system>` with `mdb` or `dbc`.

## License

[MIT](LICENSE)
