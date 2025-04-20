# Simple Mender Update Tool (SMUT)

A lightweight daemon for managing Mender updates on embedded Linux systems via Redis.

## Overview

SMUT runs on embedded Linux systems (MDB and DBC) and triggers Mender updates via Redis. It handles resumable downloads, checksum verification, and interacts with Mender.

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

The project includes an installer script that handles deployment to both MDB and DBC systems.

1. Build the ARM binary:
```bash
make build-arm
```

2. Run the installer:
```bash
./install.sh <target-host> <target>
```

Where:
- `target-host`: The hostname/IP of the MDB system
- `target`: One of:
  - `mdb`: Install only on MDB
  - `dbc`: Install only on DBC
  - `both`: Install on both MDB and DBC

The installer handles:
- Copying files to target systems
- Service installation and configuration
- GPIO management for DBC communication
- Service startup and verification

## Usage

### Command-Line Arguments

- `--redis-addr`: Redis server address (default: "localhost:6379")
- `--update-key`: Redis key for update URLs (default: "mender/update/url")
- `--checksum-key`: Redis key for checksums (default: "mender/update/checksum")
- `--failure-key`: Redis key to set on failure (default: "mender/update/last-failure")
- `--download-dir`: Directory to store downloaded update files (default: "/tmp")
- `--update-type`: Type of update ('blocking' for DBC, 'non-blocking' for MDB) (default: "non-blocking")

### Redis Usage

SMUT uses the `ota` Redis hash to report status and update type. The `status` field indicates the current state, and the `update-type` field indicates if the update is blocking or non-blocking.

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

Errors are reported by setting the configured failure key in Redis with the error message as a string. The `status` field in the `ota` hash will also be updated to reflect the error state.

## Monitoring

### Logs

When running as a systemd service, logs can be viewed with:

```bash
journalctl -u smut@<system> -f
```

Replace `<system>` with `mdb` or `dbc`.

### Status

Check the service status with:

```bash
systemctl status smut@<system>
```

Replace `<system>` with `mdb` or `dbc`.

## License

[MIT](LICENSE)
