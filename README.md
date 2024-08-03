# pg_backuper

It does backups periodically keeping the retention limit.

Configurable databases.

It doesn't support, yet, parallelism nor concurrency. Feel free to add support to it if it bothers you.

If you really really really need concurrent backups, just run it several times with different configuration files.

## Configuration file example

```json
{
  "databases": [
    {
      "name": "sonarr-main",
      "user": "sonarr",
      "password": "sonarr",
      "host": "192.168.0.65"
    },
    {
      "name": "sonarr-log",
      "user": "sonarr",
      "password": "sonarr",
      "host": "192.168.0.65"
    }
  ],
  "backup_dir": "/path/to/pgsql_backups",
  "retention": 1,
  "log_file": "/path/to/log_file.log"
}
```

## Note
Only tested in postgresql 16.2