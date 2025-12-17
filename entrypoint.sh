#!/bin/bash
set -e

echo "=== pg_backuper container starting ==="
echo "Config file: ${CONFIG_FILE}"
echo "Current time: $(date '+%Y-%m-%d %H:%M:%S')"

# Replace placeholder with actual config file path
sed -i "s|\$CONFIG_FILE|${CONFIG_FILE}|" /etc/cron.d/pg_backuper-cron

# Verify crontab was configured
echo "=== Crontab configuration ==="
cat /etc/cron.d/pg_backuper-cron
echo "=========================="

# Run initial backup if RUN_ON_STARTUP is set
if [ "${RUN_ON_STARTUP}" = "true" ]; then
    echo "=== Running initial backup (RUN_ON_STARTUP=true) ==="
    /usr/local/bin/pg_backuper "${CONFIG_FILE}" 2>&1
    echo "=== Initial backup completed ==="
fi

# Calculate next cron run time
current_minute=$(date '+%M')
minutes_until_next=$((60 - 10#$current_minute))
next_run=$(date -d "+${minutes_until_next} minutes" '+%Y-%m-%d %H:%M')
echo "=== Next scheduled backup: ${next_run} (in ${minutes_until_next} minutes) ==="

echo "=== Starting cron daemon ==="
# Start cron in foreground mode with max logging
cron -f -L 0
