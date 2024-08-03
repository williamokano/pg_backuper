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
  "backup_dir": "/backups",
  "retention": 1,
  "log_file": "/backups"
}
```

## Easiest way to configure
- Create the config file and map it to `/config/dbs.json` following the above config
- Create a mapping `/backups` in the docker
- Set the env var `CONFIG_FILE="/config/dbs.json`
- Set the env var `CRON_SCHEDULE=""` to your liking. Default is `0 3 * * *`. Run 3AM everyday. 🤷‍

## Maybe useful commands
### Build
`docker build -t pg_backuper:local .`

### Shell into docker
`docker run --network host --name pg_backuper_container -v /Users/w.dos/pgsql_backups:/backups -v $(pwd):/cwdd -e CONFIG_FILE="/cwdd/test_config.json" -e CRON_SCHEDULE="*/1 * * * *" --rm -it --entrypoint /bin/bash pg_backuper:local`

It won't start the `entrypoint.sh`, must do manually, otherwise cron won't run

## Note
Only tested in postgresql 16.2