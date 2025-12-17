# pg_backuper v2.0

Automated PostgreSQL backup solution with multi-tier retention, parallel execution, and secure password management.

## ⚠️ Breaking Changes in v2.0

If you're upgrading from v1.0, see [MIGRATION.md](MIGRATION.md) for detailed upgrade instructions.

Key changes:
- New configuration format with multi-tier retention
- Passwords moved to `.pgpass` file (more secure)
- Parallel backup execution (faster)
- Structured JSON logging

## Features

- 🔄 **Multi-tier retention**: Hourly, daily, weekly, monthly, quarterly, yearly backup policies
- 🧠 **Smart scheduling**: Automatically determines if backup is due based on retention tiers
- ⚡ **Parallel execution**: Backup multiple databases concurrently with configurable limits
- 🔒 **Secure**: Passwords in `.pgpass` file (PostgreSQL standard), not in process list
- 📊 **Structured logging**: JSON logs for easy parsing and monitoring
- 🐳 **Docker-ready**: Designed for Docker/Portainer with automatic cron (runs hourly)
- ⚙️ **Flexible configuration**: Global defaults with per-database overrides
- 🔧 **Migration tool**: Automatic conversion from v1 to v2 config

## Quick Start

### 1. Create Configuration Files

**config.json**:
```json
{
  "backup_dir": "/backups",
  "global_defaults": {
    "port": 5432,
    "retention_tiers": [
      {"tier": "hourly", "retention": 6},
      {"tier": "daily", "retention": 7},
      {"tier": "weekly", "retention": 4},
      {"tier": "monthly", "retention": 3}
    ],
    "pgpass_file": "/config/.pgpass"
  },
  "max_concurrent_backups": 3,
  "log_level": "info",
  "log_format": "json",
  "databases": [
    {
      "name": "myapp",
      "user": "postgres",
      "host": "localhost"
    }
  ]
}
```

**.pgpass** (create with `chmod 600 .pgpass`):
```
# Format: hostname:port:database:username:password
localhost:5432:*:postgres:your_password_here
```

### 2. Docker Deployment

**Docker Compose**:
```yaml
version: '3.8'
services:
  pg_backuper:
    image: pg_backuper:v2.0
    volumes:
      - ./config:/config:ro        # Config folder with config.json and .pgpass
      - ./backups:/backups          # Backup storage
    environment:
      - CONFIG_FILE=/config/config.json
```

**Portainer**:
- Volume mappings:
  - `/path/to/config:/config` (read-only)
  - `/path/to/backups:/backups`
- Environment variables:
  - `CONFIG_FILE=/config/config.json`

### 3. Run

```bash
# Build
docker build -t pg_backuper:v2.0 .

# Run
docker-compose up -d

# Check logs
docker logs -f pg_backuper
```

## Configuration Reference

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `backup_dir` | string | ✅ | Directory where backups are stored |
| `global_defaults` | object | ❌ | Default values for all databases |
| `max_concurrent_backups` | integer | ❌ | Max parallel backups (default: 3) |
| `log_level` | string | ❌ | `debug`, `info`, `warn`, `error` (default: `info`) |
| `log_format` | string | ❌ | `json`, `console` (default: `json`) |
| `databases` | array | ✅ | List of databases to backup |

### Global Defaults

| Field | Type | Description |
|-------|------|-------------|
| `port` | integer | Default PostgreSQL port (default: 5432) |
| `retention_tiers` | array | Default retention policy |
| `pgpass_file` | string | Path to .pgpass file (default: auto-detect) |

### Database Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✅ | Database name |
| `user` | string | ✅ | PostgreSQL username |
| `host` | string | ✅ | Database host |
| `port` | integer | ❌ | Override global port |
| `retention_tiers` | array | ❌ | Override global retention |
| `enabled` | boolean | ❌ | Enable/disable (default: true) |

### Retention Tiers

| Field | Type | Description |
|-------|------|-------------|
| `tier` | string | `hourly`, `daily`, `weekly`, `monthly`, `quarterly`, `yearly` |
| `retention` | integer | Number to keep (0 = unlimited) |

## Configuration Examples

### Simple Setup

Keep last 7 daily backups:

```json
{
  "backup_dir": "/backups",
  "global_defaults": {
    "retention_tiers": [
      {"tier": "daily", "retention": 7}
    ]
  },
  "databases": [
    {
      "name": "mydb",
      "user": "postgres",
      "host": "db.example.com"
    }
  ]
}
```

### Comprehensive Retention

```json
{
  "backup_dir": "/backups",
  "global_defaults": {
    "port": 5432,
    "retention_tiers": [
      {"tier": "hourly", "retention": 24},
      {"tier": "daily", "retention": 7},
      {"tier": "weekly", "retention": 4},
      {"tier": "monthly", "retention": 12},
      {"tier": "quarterly", "retention": 4},
      {"tier": "yearly", "retention": 3}
    ],
    "pgpass_file": "/config/.pgpass"
  },
  "max_concurrent_backups": 5,
  "log_level": "info",
  "log_format": "json",
  "databases": [
    {
      "name": "production",
      "user": "backup_user",
      "host": "prod.db.example.com"
    }
  ]
}
```

### Per-Database Overrides

```json
{
  "backup_dir": "/backups",
  "global_defaults": {
    "port": 5432,
    "retention_tiers": [
      {"tier": "daily", "retention": 7}
    ]
  },
  "databases": [
    {
      "name": "critical_db",
      "user": "postgres",
      "host": "critical.example.com",
      "port": 5433,
      "retention_tiers": [
        {"tier": "hourly", "retention": 24},
        {"tier": "daily", "retention": 30},
        {"tier": "monthly", "retention": 12}
      ]
    },
    {
      "name": "test_db",
      "user": "postgres",
      "host": "test.example.com",
      "enabled": false
    }
  ]
}
```

## Multi-Tier Retention

Backups are automatically categorized by age:

| Tier | Age Range | Use Case |
|------|-----------|----------|
| **hourly** | 0-24 hours | Recent recovery points |
| **daily** | 1-7 days | Last week's snapshots |
| **weekly** | 7-30 days | Last month's backups |
| **monthly** | 30-90 days | Quarterly archives |
| **quarterly** | 90-365 days | Annual archives |
| **yearly** | >365 days | Long-term storage |

**How it works:**
1. Tool analyzes each backup's age
2. Assigns to appropriate tier automatically
3. Applies retention policy per tier
4. Deletes oldest backups in each tier beyond limit

**Example:** With `[{"tier": "hourly", "retention": 6}, {"tier": "daily", "retention": 7}]`:
- Keeps last 6 backups that are <24h old
- Keeps last 7 backups that are 1-7 days old
- Other tiers: keeps all (no limit)

## Smart Scheduling

The tool intelligently determines when backups are needed based on your retention tier configuration.

**How it works:**
1. **Cron runs hourly**: Docker container has cron configured to run every hour
2. **Tool checks if backup is due**: For each database, examines existing backups
3. **Uses shortest tier**: Finds the shortest configured retention tier (e.g., if you have "hourly" and "daily", uses "hourly")
4. **Compares timestamps**: If enough time has passed since last backup, creates new backup
5. **Skips if not due**: If not enough time has passed, skips backup (logged as "skipped")

**Example scenarios:**

| Retention Tiers | Backup Frequency | Behavior |
|----------------|------------------|----------|
| `[{"tier": "hourly", "retention": 6}]` | Every hour | Backup created every hour |
| `[{"tier": "daily", "retention": 7}]` | Once per day | Backup created once per day, skipped on other hourly runs |
| `[{"tier": "hourly", ...}, {"tier": "daily", ...}]` | Every hour | Uses shortest tier (hourly) |

**Benefits:**
- **No cron misconfiguration**: You can't accidentally set cron to run daily when you want hourly backups
- **Automatic frequency detection**: Backup frequency is inferred from your retention policy
- **Safe defaults**: On errors, defaults to creating backup (fail-safe)
- **Clear logging**: Skipped backups are logged with reason

## Logging

### JSON Format (Default)

```json
{"level":"info","time":"2025-12-17T03:00:00Z","message":"starting pg_backuper v2.0","config_file":"/config/config.json"}
{"level":"info","database":"mydb","message":"starting backup","backup_file":"/backups/mydb--2025-12-17T03-00-00.backup"}
{"level":"info","database":"mydb","message":"backup completed","duration":1250}
{"level":"info","database":"mydb","tier":"hourly","message":"deleted backup file","file":"/backups/mydb--2025-12-16T03-00-00.backup"}
```

### Console Format

Set `log_format: "console"`:
```
3:00AM INF starting pg_backuper v2.0 config_file=/config/config.json
3:00AM INF starting backup database=mydb backup_file=/backups/mydb--2025-12-17T03-00-00.backup
3:01AM INF backup completed database=mydb duration=1250
3:01AM INF deleted backup file database=mydb tier=hourly file=/backups/mydb--2025-12-16T03-00-00.backup
```

## Security

### .pgpass File

PostgreSQL's standard password file format:

```
# hostname:port:database:username:password
prod-db:5432:*:backup_user:secret123
staging-db:5433:*:backup_user:staging_pass
```

**Key points:**
- Must have `0600` permissions (owner read/write only)
- Supports wildcards (`*`) for flexible matching
- Passwords never appear in process list
- Standard PostgreSQL authentication method

**Setup:**
```bash
# Create .pgpass
touch .pgpass
chmod 600 .pgpass

# Add entries
echo "localhost:5432:*:postgres:mypassword" >> .pgpass

# Verify
ls -l .pgpass  # Should show -rw-------
```

## Parallel Execution

Backups run concurrently with configurable limits:

```json
{
  "max_concurrent_backups": 3
}
```

**How it works:**
- Uses semaphore for resource control
- Fail-fast on first error (cancels remaining)
- Per-database timing tracked
- Optimal resource utilization

**Tuning:**
- **Disk I/O bound**: Keep low (2-4)
- **Network bound**: Can increase (5-10)
- **Default**: 3 (balanced)

## Building

```bash
# Build main application
go build -o pg_backuper .

# Build migration tool
go build -o migrate ./cmd/migrate

# Build Docker image
docker build -t pg_backuper:v2.0 .

# Run tests
go test ./...
```

## Migration from v1.0

See [MIGRATION.md](MIGRATION.md) for detailed instructions.

**Quick migration:**
```bash
# Build migration tool
go build -o migrate ./cmd/migrate

# Run migration (creates config.json and .pgpass)
./migrate old_config.json /path/to/new-config-dir/

# Deploy with new config
docker run -v /path/to/new-config-dir:/config pg_backuper:v2.0
```

## Troubleshooting

### Authentication Errors

**"password authentication failed"**
- Check `.pgpass` exists and has correct entries
- Verify permissions: `chmod 600 .pgpass`
- Test manually: `PGPASSFILE=/path/to/.pgpass pg_dump ...`

### Backup Failures

**"pg_dump: command not found"**
- Ensure postgresql-client is installed
- Check `POSTGRES_VERSION` build arg in Dockerfile

**"permission denied" on backup_dir**
- Ensure directory exists and is writable
- Check Docker volume mappings

### Configuration Issues

**"configuration validation failed"**
- Run `pg_backuper /path/to/config.json` to see detailed errors
- Check JSON syntax
- Verify required fields present

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_FILE` | `/app/noop_config.json` | Path to config file |
| `POSTGRES_VERSION` | `16` | PostgreSQL client version (build arg) |

**Note:** Cron schedule is fixed to run every hour (`0 * * * *`). The tool's smart scheduling determines if backups are actually needed based on retention tiers.

## File Formats

### Backup Filenames

**New format (v2.0):**
```
dbname--2025-12-17T03-00-00.backup
```

**Old format (v1.0, still supported):**
```
dbname_2025-12-17_03-00-00.backup
```

Tool automatically detects and handles both formats during rotation.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and breaking changes.

## License

MIT License - see [LICENSE](LICENSE) file

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## Support

- Issues: https://github.com/williamokano/pg_backuper/issues
- Documentation: This README and [MIGRATION.md](MIGRATION.md)

## Tested With

- PostgreSQL 16.2 (primary)
- Docker / Portainer
- Go 1.22+