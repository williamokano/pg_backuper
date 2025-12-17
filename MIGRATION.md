# Migration Guide: v1.0 → v2.0

This guide helps you migrate from pg_backuper v1.0 to v2.0.

## Overview of Changes

v2.0 introduces breaking changes to improve security, performance, and flexibility:

- **New config structure** with multi-tier retention support
- **Passwords moved to `.pgpass` file** (more secure)
- **Parallel backup execution** (faster)
- **Structured JSON logging** (better monitoring)

## Quick Migration (Recommended)

### Step 1: Backup Your Current Setup

```bash
# Backup your v1 config
cp /path/to/config.json config.v1.backup.json

# Backup your current backups (optional but recommended)
cp -r /path/to/backups /path/to/backups.backup
```

### Step 2: Build and Run Migration Tool

```bash
# Clone/update to v2.0
git checkout feature/v2-refactor  # or main after merge

# Build the migration tool
go build -o migrate ./cmd/migrate

# Run migration - creates both config.json and .pgpass
./migrate config.v1.backup.json /path/to/new-config-dir/

# Verify output
ls -la /path/to/new-config-dir/
# Should show:
#   config.json (new format)
#   .pgpass (with 0600 permissions)
```

### Step 3: Review Generated Files

**config.json** (new format):
```json
{
  "backup_dir": "/backups",
  "global_defaults": {
    "port": 5432,
    "retention_tiers": [
      {"tier": "daily", "retention": 7}
    ],
    "pgpass_file": "/config/.pgpass"
  },
  "max_concurrent_backups": 3,
  "log_level": "info",
  "log_format": "json",
  "databases": [
    {
      "name": "mydb",
      "user": "postgres",
      "host": "localhost",
      "enabled": true
    }
  ]
}
```

**.pgpass** (passwords extracted):
```
# PostgreSQL password file
# Format: hostname:port:database:username:password
localhost:5432:*:postgres:yourpassword
```

### Step 4: Deploy to Docker/Portainer

**Update your Docker configuration:**

```yaml
# Portainer or docker-compose
version: '3.8'
services:
  pg_backuper:
    image: pg_backuper:v2.0
    volumes:
      - /path/to/new-config-dir:/config:ro  # Mount entire config folder
      - /path/to/backups:/backups
    environment:
      - CONFIG_FILE=/config/config.json
      - CRON_SCHEDULE=0 3 * * *  # 3 AM daily
```

**Or in Portainer UI:**
- Volume mapping: `/path/to/new-config-dir:/config`
- Environment variable: `CONFIG_FILE=/config/config.json`

### Step 5: Test

```bash
# Test the new config
docker run --rm \
  -v /path/to/new-config-dir:/config:ro \
  -v /path/to/backups:/backups \
  -e CONFIG_FILE=/config/config.json \
  pg_backuper:v2.0 \
  /usr/local/bin/pg_backuper /config/config.json

# Check logs (JSON format)
docker logs <container_id>
```

## Manual Migration (Advanced)

If you prefer to create the config manually:

### 1. Create New Config Structure

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
      "name": "database1",
      "user": "user1",
      "host": "host1.example.com",
      "port": 5432,
      "enabled": true
    },
    {
      "name": "database2",
      "user": "user2",
      "host": "host2.example.com",
      "retention_tiers": [
        {"tier": "daily", "retention": 30}
      ]
    }
  ]
}
```

### 2. Create .pgpass File

```bash
# Create .pgpass with correct format
cat > .pgpass <<'EOF'
host1.example.com:5432:*:user1:password1
host2.example.com:5432:*:user2:password2
EOF

# Set correct permissions (REQUIRED)
chmod 600 .pgpass
```

### 3. Validate Configuration

```bash
# Check JSON schema
pg_backuper /path/to/config.json
# Will validate and show errors if invalid
```

## Configuration Mapping

### v1.0 → v2.0 Field Mapping

| v1.0 Field | v2.0 Field | Notes |
|------------|------------|-------|
| `backup_dir` | `backup_dir` | Unchanged |
| `retention` | `global_defaults.retention_tiers[].retention` | Now supports multiple tiers |
| `log_file` | Removed | Now logs to stdout/stderr with structured format |
| `databases[].name` | `databases[].name` | Unchanged |
| `databases[].user` | `databases[].user` | Unchanged |
| `databases[].password` | Moved to `.pgpass` | No longer in config file |
| `databases[].host` | `databases[].host` | Unchanged |
| N/A | `databases[].port` | **New**: Per-database port override |
| N/A | `databases[].retention_tiers` | **New**: Per-database retention override |
| N/A | `databases[].enabled` | **New**: Enable/disable individual databases |
| N/A | `global_defaults.port` | **New**: Global default port |
| N/A | `global_defaults.pgpass_file` | **New**: Path to .pgpass file |
| N/A | `max_concurrent_backups` | **New**: Concurrency limit |
| N/A | `log_level` | **New**: debug/info/warn/error |
| N/A | `log_format` | **New**: json/console |

## Multi-Tier Retention Examples

### Example 1: Simple (like v1.0)

Keep last 7 daily backups:

```json
"retention_tiers": [
  {"tier": "daily", "retention": 7}
]
```

### Example 2: Comprehensive

```json
"retention_tiers": [
  {"tier": "hourly", "retention": 6},     // Last 6 hours
  {"tier": "daily", "retention": 7},      // Last 7 days
  {"tier": "weekly", "retention": 4},     // Last 4 weeks
  {"tier": "monthly", "retention": 12},   // Last 12 months
  {"tier": "quarterly", "retention": 4},  // Last 4 quarters
  {"tier": "yearly", "retention": 3}      // Last 3 years
]
```

### Example 3: Per-Database Override

```json
{
  "global_defaults": {
    "retention_tiers": [
      {"tier": "daily", "retention": 7}
    ]
  },
  "databases": [
    {
      "name": "production_db",
      "retention_tiers": [
        {"tier": "hourly", "retention": 24},
        {"tier": "daily", "retention": 30},
        {"tier": "monthly", "retention": 12}
      ]
    },
    {
      "name": "staging_db"
      // Uses global retention (7 daily)
    }
  ]
}
```

## Tier Categorization

Backups are automatically categorized based on age:

| Tier | Age Range | Example Use Case |
|------|-----------|------------------|
| **hourly** | 0-24 hours | Recent point-in-time recovery |
| **daily** | 1-7 days | Last week's daily backups |
| **weekly** | 7-30 days | Last month's weekly snapshots |
| **monthly** | 30-90 days | Quarterly monthly backups |
| **quarterly** | 90-365 days | Yearly quarterly backups |
| **yearly** | >365 days | Long-term archives |

## Troubleshooting

### Migration Tool Errors

**Error: "failed to open config file"**
- Check that the old config file path is correct
- Ensure you have read permissions

**Error: "failed to create output directory"**
- Check that you have write permissions to the output directory
- Create the directory manually if needed: `mkdir -p /path/to/output`

### .pgpass Permission Errors

**Error: ".pgpass file has incorrect permissions"**
```bash
# Fix permissions
chmod 600 /path/to/.pgpass
```

**Error: "no .pgpass file found"**
- Check that migration tool created it
- Verify the path in `global_defaults.pgpass_file`
- Check Docker volume mappings

### Backup Authentication Errors

**Error: "pg_dump: FATAL: password authentication failed"**
- Verify `.pgpass` has correct entries
- Check format: `hostname:port:database:username:password`
- Ensure permissions are 0600
- Test manually: `PGPASSFILE=/path/to/.pgpass pg_dump ...`

### Configuration Validation Errors

Run validation to see detailed errors:
```bash
pg_backuper /path/to/config.json
```

Common issues:
- Missing required fields (`backup_dir`, `databases`)
- Invalid tier names (must be: hourly/daily/weekly/monthly/quarterly/yearly)
- Invalid retention values (must be >= 0)
- Invalid port numbers (must be 1-65535)

## Rollback Plan

If you need to rollback to v1.0:

```bash
# Stop v2.0 container
docker stop pg_backuper_container

# Restore v1.0 config
cp config.v1.backup.json /path/to/config.json

# Deploy v1.0 image
docker run ... pg_backuper:v1.0
```

Your existing backups will work with both versions (backward compatible filename parsing).

## Support

If you encounter issues:

1. Check this migration guide
2. Review [README.md](README.md) for configuration examples
3. Check logs: `docker logs <container>`
4. Report issues: https://github.com/williamokano/pg_backuper/issues

## Next Steps

After successful migration:

1. Monitor first few backup runs
2. Verify rotation is working correctly
3. Test restore from backups
4. Update documentation/runbooks for your team
5. Consider additional retention tiers for your needs
