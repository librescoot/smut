#!/bin/bash
set -euo pipefail

# Check arguments
if [ $# -lt 2 ] || [ $# -gt 3 ]; then
    echo "Usage: $0 <target-host> <target> [--skip-upload]"
    echo "target: mdb, dbc, or both"
    exit 1
fi

TARGET_HOST="$1"
INSTALL_TARGET="$2"
SKIP_UPLOAD="false" # Default

if [ $# -eq 3 ]; then
    if [ "$3" == "--skip-upload" ]; then
        SKIP_UPLOAD="true"
    else
        echo "Error: Invalid third argument. Usage: $0 <target-host> <target> [--skip-upload]"
        echo "target: mdb, dbc, or both"
        exit 1
    fi
fi


if [[ ! "$INSTALL_TARGET" =~ ^(mdb|dbc|both)$ ]]; then
    echo "Error: target must be 'mdb', 'dbc', or 'both'"
    exit 1
fi

SOURCE_BINARY="smut-arm-dist"
TARGET_BINARY="smut"
MDB_SERVICE="smut@mdb.service"
DBC_SERVICE="smut@dbc.service"

# Check if ARM binary exists and is executable
if [ ! -x "$SOURCE_BINARY" ]; then
    echo "Error: $SOURCE_BINARY not found or not executable. Please build for ARM first."
    exit 1
fi

echo "=== Preparing files for MDB ($TARGET_HOST) ==="

# Conditionally copy files to MDB
if [ "$SKIP_UPLOAD" != "true" ]; then
    echo "Copying files to MDB..."
    scp -CO "$SOURCE_BINARY" "$MDB_SERVICE" "$DBC_SERVICE" "$TARGET_HOST:/tmp/"
else
    echo "Skipping file upload. Assuming binary and service files are already in /tmp/ on $TARGET_HOST."
fi


# Configure everything in one SSH session
ssh "$TARGET_HOST" "
    set -euo pipefail
    
    # Install and enable MDB service if requested
    if [[ '$INSTALL_TARGET' =~ ^(mdb|both)$ ]]; then
        echo 'Configuring MDB...'
        systemctl stop smut@mdb || true
        cp /tmp/$SOURCE_BINARY /usr/local/bin/$TARGET_BINARY
        chmod +x /usr/local/bin/$TARGET_BINARY
        cp /tmp/$MDB_SERVICE /etc/systemd/system/$MDB_SERVICE
        systemctl daemon-reload
        systemctl enable --now smut@mdb
    fi

    # Handle DBC installation if requested
    if [[ '$INSTALL_TARGET' =~ ^(dbc|both)$ ]]; then
        echo 'Preparing DBC connection...'
        systemctl stop unu-vehicle || true
        echo 50 | tee /sys/class/gpio/export || true
        echo out | tee /sys/class/gpio/gpio50/direction || true
        echo 1 | tee /sys/class/gpio/gpio50/value || true

        # Wait for DBC to be accessible
        echo 'Waiting for DBC to respond...'
        while ! ping -c 1 -W 1 192.168.7.2 > /dev/null 2>&1; do
            sleep 1
        done

        echo 'DBC responded, waiting 3s for stability...'
        sleep 3

        redis-cli HSET dashboard ready false

        echo 'Configuring DBC...'
        # Copy files to DBC
        scp /tmp/$SOURCE_BINARY /tmp/$DBC_SERVICE root@192.168.7.2:/tmp/
        
        ssh root@192.168.7.2 '
            set -euo pipefail
            systemctl stop smut@dbc || true
            cp /tmp/$SOURCE_BINARY /usr/local/bin/$TARGET_BINARY
            chmod +x /usr/local/bin/$TARGET_BINARY
            cp /tmp/$DBC_SERVICE /etc/systemd/system/$DBC_SERVICE
            systemctl daemon-reload
            systemctl enable --now smut@dbc
            rm /tmp/$SOURCE_BINARY /tmp/$DBC_SERVICE
            
            # Verify service started correctly
            if ! systemctl is-active --quiet smut@dbc; then
                echo \"Error: smut@dbc failed to start on DBC\"
                exit 1
            fi
            echo \"smut@dbc started successfully on DBC\"
            exit
        '

        # Reset GPIO and restart unu-vehicle
        echo 'Resetting GPIO and restarting unu-vehicle...'
        sleep 1
        echo 0 | tee /sys/class/gpio/gpio50/value || true
        echo 50 | tee /sys/class/gpio/unexport || true
        sleep 1
        systemctl start unu-vehicle
    fi

    # Clean up temporary files
    rm -f /tmp/$SOURCE_BINARY /tmp/$MDB_SERVICE /tmp/$DBC_SERVICE
"

echo "=== Installation completed successfully ==="
