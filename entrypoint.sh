#!/bin/bash

# Replace placeholder with actual config file path
sed -i "s|\$CONFIG_FILE|${CONFIG_FILE}|" /etc/cron.d/pg_backuper-cron

# Start the cron daemon
cron

if [[ ! -f /var/log/pg_backuper.log ]]; then
  touch /var/log/pg_backuper.log
fi

# Keep the container running to allow cron to execute jobs
tail -f /var/log/pg_backuper.log
