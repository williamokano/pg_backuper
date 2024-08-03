#!/bin/bash

# Replace placeholder with actual cron schedule
sed -i "s|\$CRON_SCHEDULE|${CRON_SCHEDULE}|" /etc/cron.d/pg_backuper-cron
sed -i "s|\$CONFIG_FILE|${CONFIG_FILE}|" /etc/cron.d/pg_backuper-cron

# Start the cron daemon
cron

if [[ ! -f /var/log/pg_backuper.log ]]; then
  touch /var/log/pg_backuper.log
fi

# Keep the container running to allow cron to execute jobs
tail -f /var/log/pg_backuper.log
