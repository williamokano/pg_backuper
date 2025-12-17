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

// TierSchedule represents which tiers are due for backup
type TierSchedule struct {
	Due  []string              // List of tier names that are due (e.g., ["hourly", "daily"])
	Next map[string]time.Time  // When each configured tier becomes due next
}

// GetDueTiers checks which tiers are due for backup for a database.
// Returns a TierSchedule with the list of due tiers and next backup times.
func GetDueTiers(cfg *config.Config, db config.DatabaseConfig, now time.Time, logger zerolog.Logger) (TierSchedule, error) {
	// Get retention tiers for this database (with fallback to global)
	retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)

	schedule := TierSchedule{
		Due:  []string{},
		Next: make(map[string]time.Time),
	}

	if len(retentionTiers) == 0 {
		// No retention tiers configured, backup all tiers (backward compatibility)
		logger.Debug().Str("database", db.Name).Msg("no retention tiers configured, backup is due")
		schedule.Due = append(schedule.Due, "default")
		return schedule, nil
	}

	// Check each configured tier independently
	for _, retentionTier := range retentionTiers {
		tierName := retentionTier.Tier
		interval, ok := TierInterval[tierName]
		if !ok {
			logger.Warn().
				Str("database", db.Name).
				Str("tier", tierName).
				Msg("unknown tier name, skipping")
			continue
		}

		// Find last backup for this specific tier
		lastBackupTime, err := findLastBackupTimeByTier(cfg.BackupDir, db.Name, tierName)
		if err != nil {
			logger.Warn().
				Err(err).
				Str("database", db.Name).
				Str("tier", tierName).
				Msg("error finding last backup for tier, assuming due")
			schedule.Due = append(schedule.Due, tierName)
			schedule.Next[tierName] = now
			continue
		}

		// Special handling for yearly tier: also check if ANY backup is older than 365 days
		if tierName == "yearly" && !lastBackupTime.IsZero() {
			oldestBackup, err := findOldestBackup(cfg.BackupDir, db.Name)
			if err == nil && !oldestBackup.IsZero() {
				ageOfOldest := now.Sub(oldestBackup)
				if ageOfOldest >= interval {
					// We have a backup that's old enough to satisfy yearly retention
					logger.Debug().
						Str("database", db.Name).
						Time("oldest_backup", oldestBackup).
						Dur("age", ageOfOldest).
						Msg("yearly tier satisfied by aged backup")
					nextYearly := oldestBackup.Add(interval)
					schedule.Next[tierName] = nextYearly
					continue
				}
			}
		}

		if lastBackupTime.IsZero() {
			// No previous backup found for this tier, backup is due
			logger.Info().
				Str("database", db.Name).
				Str("tier", tierName).
				Msg("no previous backup found for tier, backup is due")
			schedule.Due = append(schedule.Due, tierName)
			schedule.Next[tierName] = now
			continue
		}

		// Check if enough time has passed for this tier
		timeSinceLastBackup := now.Sub(lastBackupTime)
		isDue := timeSinceLastBackup >= interval

		if isDue {
			logger.Info().
				Str("database", db.Name).
				Str("tier", tierName).
				Dur("interval", interval).
				Time("last_backup", lastBackupTime).
				Dur("time_since_last", timeSinceLastBackup).
				Msg("backup is due for tier")
			schedule.Due = append(schedule.Due, tierName)
			schedule.Next[tierName] = now
		} else {
			timeUntilDue := interval - timeSinceLastBackup
			nextBackupTime := lastBackupTime.Add(interval)
			logger.Info().
				Str("database", db.Name).
				Str("tier", tierName).
				Dur("interval", interval).
				Time("last_backup", lastBackupTime).
				Dur("time_since_last", timeSinceLastBackup).
				Dur("time_until_due", timeUntilDue).
				Time("next_backup", nextBackupTime).
				Msg("backup not due yet for tier")
			schedule.Next[tierName] = nextBackupTime
		}
	}

	return schedule, nil
}

// IsBackupDue checks if a backup is due for a database based on retention tiers
// and existing backups. Returns true if a backup should be created.
// Deprecated: Use GetDueTiers for more detailed tier information.
func IsBackupDue(cfg *config.Config, db config.DatabaseConfig, now time.Time, logger zerolog.Logger) (bool, error) {
	schedule, err := GetDueTiers(cfg, db, now, logger)
	if err != nil {
		return false, err
	}
	return len(schedule.Due) > 0, nil
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

// findLastBackupTimeByTier finds the most recent backup timestamp for a specific tier
func findLastBackupTimeByTier(backupDir, dbName, tierName string) (time.Time, error) {
	// Use tier-specific pattern: dbname--TIER--*.backup
	pattern := rotation.GetBackupPatternForTier(backupDir, dbName, tierName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to list backups for tier %s: %w", tierName, err)
	}

	if len(matches) == 0 {
		// No backups found for this tier
		return time.Time{}, nil
	}

	var mostRecent time.Time

	for _, match := range matches {
		// Parse filename to extract timestamp and tier
		components, err := rotation.ParseBackupFilename(match)
		if err != nil {
			// Skip files with invalid format
			continue
		}

		// Double-check tier matches (pattern should ensure this, but be safe)
		if components.Tier != tierName {
			continue
		}

		// CRITICAL: Validate file exists and has content (skip 0-byte failed backups)
		fileInfo, err := os.Stat(match)
		if err != nil || fileInfo.Size() == 0 {
			// Skip files that don't exist or are empty
			continue
		}

		if components.Timestamp.After(mostRecent) {
			mostRecent = components.Timestamp
		}
	}

	return mostRecent, nil
}

// findOldestBackup finds the oldest backup timestamp for a database (regardless of tier)
func findOldestBackup(backupDir, dbName string) (time.Time, error) {
	// List all backup files for this database
	pattern := rotation.GetBackupPattern(backupDir, dbName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to list backups: %w", err)
	}

	if len(matches) == 0 {
		return time.Time{}, nil
	}

	var oldest time.Time

	for _, match := range matches {
		// Extract just the filename
		filename := filepath.Base(match)

		// Check if this file actually belongs to this database
		if !isBackupForDatabase(filename, dbName) {
			continue
		}

		// Extract timestamp from filename
		timestamp, err := rotation.ExtractDateFromFilename(filename)
		if err != nil {
			// Skip files with invalid timestamps
			continue
		}

		if oldest.IsZero() || timestamp.Before(oldest) {
			oldest = timestamp
		}
	}

	return oldest, nil
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
