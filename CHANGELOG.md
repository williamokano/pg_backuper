# Changelog

All notable changes to pg_backuper will be documented in this file.

## [2.0.0] - 2025-12-17

### ⚠️ BREAKING CHANGES

- **New configuration format**: Complete redesign of config structure with multi-tier retention support
- **Password management**: Passwords moved from config to `.pgpass` file for security
- **Filename format**: New backups use `dbname--YYYY-MM-DDTHH-MM-SS.backup` format (old format still supported for rotation)
- **Logging**: Log output now structured JSON by default (configurable to console format)

### ✨ Added

#### Multi-Tier Retention System
- Automatic age-based categorization of backups into tiers (hourly/daily/weekly/monthly/quarterly/yearly)
- Per-tier retention policies with independent limits
- Global and per-database retention tier configuration
- Graceful handling of unlimited retention (retention: 0)

#### Parallel Backup Execution
- Concurrent backup operations with configurable concurrency limits (`max_concurrent_backups`)
- Resource management via semaphore to prevent overwhelming the system
- Fail-fast error handling with context cancellation
- Per-database timing and result tracking

#### Enhanced Configuration
- Global defaults with per-database overrides for port and retention
- Configurable log level (`debug`, `info`, `warn`, `error`)
- Configurable log format (`json`, `console`)
- Database enable/disable flag
- Structured validation with JSON schema

#### Security Improvements
- `.pgpass` file support for password management (PostgreSQL standard)
- Passwords no longer visible in process list
- Automatic permission validation (requires 0600)
- Separation of secrets from configuration

#### Developer Experience
- Structured JSON logging with zerolog
- Comprehensive error messages with context
- Migration tool for automatic v1→v2 config conversion
- Extensive unit tests for core functionality

### 🔧 Fixed

- **Critical bug**: Fixed zero-time return in date parsing that could cause wrong backups to be deleted
- **Bug**: Improved filename parsing to handle database names with underscores correctly
- **Bug**: Better error handling throughout - no more silent failures

### 🏗️ Changed

- Complete rewrite of core architecture
- New package structure: `pkg/config`, `pkg/backup`, `pkg/rotation`, `pkg/logger`
- Backup filename format now uses `--` separator and ISO-8601-like timestamps
- Log output defaults to structured JSON instead of plain text
- Process exits with code 1 if any backup fails (was continuing silently)

### 🚀 Migration

A migration tool is provided to automatically convert v1 configs to v2 format:

```bash
# Build migration tool
go build ./cmd/migrate

# Run migration (creates config.json and .pgpass)
./migrate old_config.json /path/to/output-dir/

# Mount in Docker/Portainer
# Volume: /path/to/output-dir:/config
# Env: CONFIG_FILE=/config/config.json
```

See [MIGRATION.md](MIGRATION.md) for detailed migration instructions.

### 📦 Dependencies

- Added: `github.com/rs/zerolog` v1.31.0 - Structured logging
- Added: `golang.org/x/sync` v0.19.0 - Concurrency primitives (errgroup, semaphore)
- Kept: `github.com/xeipuuv/gojsonschema` v1.2.0 - JSON schema validation

### 🔍 Internal

- Refactored date extraction to return errors instead of zero values
- Extracted backup logic into reusable components
- Improved test coverage with comprehensive unit tests
- Better separation of concerns across packages

---

## [1.0.0] - Initial Release

### Features

- Basic PostgreSQL backup with pg_dump
- Simple retention policy (keep last N backups)
- Configurable databases
- Docker support with cron scheduling
- Sequential backup execution

### Configuration

- JSON-based configuration
- Embedded passwords in config
- Simple retention count
- Fixed port (5432)

---

For upgrade instructions, see [MIGRATION.md](MIGRATION.md).
For usage examples, see [README.md](README.md).
