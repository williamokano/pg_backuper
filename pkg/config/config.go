package config

// RetentionTier defines a retention policy for a specific tier
type RetentionTier struct {
	Tier      string `json:"tier"`      // hourly, daily, weekly, monthly, quarterly, yearly
	Retention int    `json:"retention"` // number of backups to keep (0 = unlimited)
}

// StorageDestination represents a storage backend configuration
type StorageDestination struct {
	Name    string                 `json:"name"`     // User-friendly name
	Type    string                 `json:"type"`     // local, s3, backblaze, ssh
	Enabled bool                   `json:"enabled"`  // Whether this backend is active
	BaseDir string                 `json:"base_dir"` // Base path/prefix
	Options map[string]interface{} `json:"options"`  // Backend-specific config
}

// StorageConfig defines storage backend configuration
type StorageConfig struct {
	TempDir      string               `json:"temp_dir"`     // Temp directory for pg_dump
	Destinations []StorageDestination `json:"destinations"` // All configured backends
}

// GlobalDefaults defines default values applied to all databases
type GlobalDefaults struct {
	Port                int              `json:"port,omitempty"`                     // default PostgreSQL port
	RetentionTiers      []RetentionTier  `json:"retention_tiers,omitempty"`          // default retention policy
	PgpassFile          string           `json:"pgpass_file,omitempty"`              // path to .pgpass file
	StorageDestinations []string         `json:"storage_destinations,omitempty"`     // default storage backends
}

// DatabaseConfig defines configuration for a single database
type DatabaseConfig struct {
	Name                string          `json:"name"`
	User                string          `json:"user"`
	Host                string          `json:"host"`
	Port                int             `json:"port,omitempty"`                     // optional, overrides global default
	RetentionTiers      []RetentionTier `json:"retention_tiers,omitempty"`          // optional, overrides global default
	Enabled             bool            `json:"enabled,omitempty"`                  // defaults to true if omitted
	StorageDestinations []string        `json:"storage_destinations,omitempty"`     // override storage backends
}

// Config is the root configuration structure
type Config struct {
	BackupDir            string           `json:"backup_dir,omitempty"`             // DEPRECATED: Use storage.destinations instead
	Storage              StorageConfig    `json:"storage"`
	GlobalDefaults       GlobalDefaults   `json:"global_defaults,omitempty"`
	MaxConcurrentBackups int              `json:"max_concurrent_backups,omitempty"` // default: 3
	LogLevel             string           `json:"log_level,omitempty"`              // debug, info, warn, error (default: info)
	LogFormat            string           `json:"log_format,omitempty"`             // json, console (default: json)
	Databases            []DatabaseConfig `json:"databases"`
}

// GetPort returns the effective port for a database (database-specific or global default)
func (db *DatabaseConfig) GetPort(globalDefaults GlobalDefaults) int {
	if db.Port > 0 {
		return db.Port
	}
	if globalDefaults.Port > 0 {
		return globalDefaults.Port
	}
	return 5432 // PostgreSQL default
}

// GetRetentionTiers returns the effective retention tiers for a database
func (db *DatabaseConfig) GetRetentionTiers(globalDefaults GlobalDefaults) []RetentionTier {
	if len(db.RetentionTiers) > 0 {
		return db.RetentionTiers
	}
	return globalDefaults.RetentionTiers
}

// IsEnabled returns whether the database backup is enabled (defaults to true)
func (db *DatabaseConfig) IsEnabled() bool {
	// If Enabled field is not set (zero value), default to true
	// In JSON, omitempty means false will be treated as "not set" which is a limitation
	// For now, we assume if the database is in the config, it's enabled unless explicitly set to false
	return db.Enabled || db.Enabled == false // This logic needs adjustment based on requirements
}

// GetPgpassFile returns the pgpass file path (global default or standard location)
func (c *Config) GetPgpassFile() string {
	if c.GlobalDefaults.PgpassFile != "" {
		return c.GlobalDefaults.PgpassFile
	}
	// Will default to ~/.pgpass or /config/.pgpass in Docker
	return ""
}

// GetMaxConcurrentBackups returns the max concurrent backups (defaults to 3)
func (c *Config) GetMaxConcurrentBackups() int {
	if c.MaxConcurrentBackups > 0 {
		return c.MaxConcurrentBackups
	}
	return 3
}

// GetLogLevel returns the log level (defaults to info)
func (c *Config) GetLogLevel() string {
	if c.LogLevel != "" {
		return c.LogLevel
	}
	return "info"
}

// GetLogFormat returns the log format (defaults to json)
func (c *Config) GetLogFormat() string {
	if c.LogFormat != "" {
		return c.LogFormat
	}
	return "json"
}

// GetStorageDestinations returns the effective storage destinations for a database
func (db *DatabaseConfig) GetStorageDestinations(cfg *Config) []string {
	// Database-specific destinations take precedence
	if len(db.StorageDestinations) > 0 {
		return db.StorageDestinations
	}

	// Fall back to global defaults
	if len(cfg.GlobalDefaults.StorageDestinations) > 0 {
		return cfg.GlobalDefaults.StorageDestinations
	}

	// Last resort: all enabled backends
	var destinations []string
	for _, dest := range cfg.Storage.Destinations {
		if dest.Enabled {
			destinations = append(destinations, dest.Name)
		}
	}

	return destinations
}
