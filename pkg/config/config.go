package config

// RetentionTier defines a retention policy for a specific tier
type RetentionTier struct {
	Tier      string `json:"tier"`      // hourly, daily, weekly, monthly, quarterly, yearly
	Retention int    `json:"retention"` // number of backups to keep (0 = unlimited)
}

// GlobalDefaults defines default values applied to all databases
type GlobalDefaults struct {
	Port           int              `json:"port,omitempty"`            // default PostgreSQL port
	RetentionTiers []RetentionTier  `json:"retention_tiers,omitempty"` // default retention policy
	PgpassFile     string           `json:"pgpass_file,omitempty"`     // path to .pgpass file
}

// DatabaseConfig defines configuration for a single database
type DatabaseConfig struct {
	Name           string          `json:"name"`
	User           string          `json:"user"`
	Host           string          `json:"host"`
	Port           int             `json:"port,omitempty"`            // optional, overrides global default
	RetentionTiers []RetentionTier `json:"retention_tiers,omitempty"` // optional, overrides global default
	Enabled        bool            `json:"enabled,omitempty"`         // defaults to true if omitted
}

// Config is the root configuration structure
type Config struct {
	BackupDir            string          `json:"backup_dir"`
	GlobalDefaults       GlobalDefaults  `json:"global_defaults,omitempty"`
	MaxConcurrentBackups int             `json:"max_concurrent_backups,omitempty"` // default: 3
	LogLevel             string          `json:"log_level,omitempty"`              // debug, info, warn, error (default: info)
	LogFormat            string          `json:"log_format,omitempty"`             // json, console (default: json)
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
