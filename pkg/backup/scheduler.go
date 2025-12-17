package backup

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/rotation"
)

// TierInterval maps tier names to their durations
var TierInterval = map[string]time.Duration{
	"hourly":    1 * time.Hour,
	"daily":     24 * time.Hour,
	"weekly":    7 * 24 * time.Hour,
	"monthly":   30 * 24 * time.Hour,
	"quarterly": 90 * 24 * time.Hour,
	"yearly":    365 * 24 * time.Hour,
}

// TierOrder defines the priority order of tiers (shortest first)
var TierOrder = []string{"hourly", "daily", "weekly", "monthly", "quarterly", "yearly"}

// IsBackupDue checks if a backup is due for a database based on retention tiers
// and existing backups. Returns true if a backup should be created.
func IsBackupDue(cfg *config.Config, db config.DatabaseConfig, now time.Time, logger zerolog.Logger) (bool, error) {
	// Get retention tiers for this database (with fallback to global)
	retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)

	if len(retentionTiers) == 0 {
		// No retention tiers configured, always backup
		logger.Debug().Str("database", db.Name).Msg("no retention tiers configured, backup is due")
		return true, nil
	}

	// Find the shortest configured tier interval
	shortestInterval, tierName := findShortestTierInterval(retentionTiers)
	if shortestInterval == 0 {
		// No valid tier found, always backup
		logger.Debug().Str("database", db.Name).Msg("no valid tier interval found, backup is due")
		return true, nil
	}

	// Get the most recent backup for this database
	lastBackupTime, err := findLastBackupTime(cfg.BackupDir, db.Name)
	if err != nil {
		// Error finding last backup, assume backup is due
		logger.Warn().
			Err(err).
			Str("database", db.Name).
			Msg("error finding last backup, assuming backup is due")
		return true, nil
	}

	if lastBackupTime.IsZero() {
		// No previous backup found, backup is due
		logger.Info().
			Str("database", db.Name).
			Msg("no previous backup found, backup is due")
		return true, nil
	}

	// Check if enough time has passed
	timeSinceLastBackup := now.Sub(lastBackupTime)
	isDue := timeSinceLastBackup >= shortestInterval

	logger.Debug().
		Str("database", db.Name).
		Str("shortest_tier", tierName).
		Dur("shortest_interval", shortestInterval).
		Time("last_backup", lastBackupTime).
		Dur("time_since_last", timeSinceLastBackup).
		Bool("is_due", isDue).
		Msg("checked if backup is due")

	return isDue, nil
}

// findShortestTierInterval returns the shortest interval from configured retention tiers
func findShortestTierInterval(retentionTiers []config.RetentionTier) (time.Duration, string) {
	for _, tierName := range TierOrder {
		// Check if this tier is configured
		for _, rt := range retentionTiers {
			if rt.Tier == tierName {
				if interval, ok := TierInterval[tierName]; ok {
					return interval, tierName
				}
			}
		}
	}
	return 0, ""
}

// findLastBackupTime finds the most recent backup timestamp for a database
func findLastBackupTime(backupDir, dbName string) (time.Time, error) {
	// List all backup files for this database
	pattern := filepath.Join(backupDir, fmt.Sprintf("%s*.backup", dbName))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to list backups: %w", err)
	}

	if len(matches) == 0 {
		return time.Time{}, nil
	}

	var mostRecent time.Time

	for _, match := range matches {
		// Extract just the filename
		filename := filepath.Base(match)

		// Check if this file actually belongs to this database
		// (avoid matching "db" when looking for "db_test")
		if !isBackupForDatabase(filename, dbName) {
			continue
		}

		// Extract timestamp from filename
		timestamp, err := rotation.ExtractDateFromFilename(filename)
		if err != nil {
			// Skip files with invalid timestamps
			continue
		}

		if timestamp.After(mostRecent) {
			mostRecent = timestamp
		}
	}

	return mostRecent, nil
}

// isBackupForDatabase checks if a backup filename belongs to the specified database
func isBackupForDatabase(filename, dbName string) bool {
	// Remove .backup extension
	nameWithoutExt := strings.TrimSuffix(filename, ".backup")

	// Check for new format: dbname--timestamp
	if strings.HasPrefix(nameWithoutExt, dbName+"--") {
		return true
	}

	// Check for old format: dbname_date_time
	// Split by underscore and check first part
	parts := strings.Split(nameWithoutExt, "_")
	if len(parts) >= 3 {
		// For old format, everything before the last two underscores is the db name
		dbNamePart := strings.Join(parts[:len(parts)-2], "_")
		return dbNamePart == dbName
	}

	return false
}
