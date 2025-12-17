#!/bin/bash
set -e

echo "=== pg_backuper container starting ==="
echo "Config file: ${CONFIG_FILE}"

# Replace placeholder with actual config file path
sed -i "s|\$CONFIG_FILE|${CONFIG_FILE}|" /etc/cron.d/pg_backuper-cron

# Create log directory and symlink to stdout/stderr for Docker logging
mkdir -p /var/log
ln -sf /proc/1/fd/1 /var/log/pg_backuper.log
ln -sf /proc/1/fd/2 /var/log/pg_backuper.err

echo "=== Starting cron daemon ==="
# Start cron in foreground mode with logging to stderr (which goes to Docker logs)
cron -f -L 2
