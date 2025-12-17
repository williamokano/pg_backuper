package rotation

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
)

// BackupFile represents a backup file with its metadata
type BackupFile struct {
	Path         string
	Timestamp    time.Time
	Tier         TierName // Age-based tier (calculated during retention)
	CreationTier string   // Tier tag from filename (empty if untagged)
}

// ApplyRetention applies the retention policy to a set of backup files
// Returns the list of files that should be deleted
func ApplyRetention(backups []BackupFile, retentionTiers []config.RetentionTier, logger zerolog.Logger) ([]string, error) {
	if len(backups) == 0 {
		return nil, nil
	}

	now := time.Now()

	// Categorize backups by tier
	tierMap := make(map[TierName][]BackupFile)
	for _, backup := range backups {
		tier := CategorizeTier(backup.Timestamp, now)
		backup.Tier = tier
		tierMap[tier] = append(tierMap[tier], backup)
	}

	// Build retention policy map
	retentionMap := make(map[TierName]int)
	for _, rt := range retentionTiers {
		retentionMap[TierName(rt.Tier)] = rt.Retention
	}

	// Collect files to delete
	var filesToDelete []string

	// Process each tier
	for _, tierName := range []TierName{TierHourly, TierDaily, TierWeekly, TierMonthly, TierQuarterly, TierYearly} {
		tieredBackups := tierMap[tierName]
		if len(tieredBackups) == 0 {
			continue
		}

		retention, hasRetention := retentionMap[tierName]
		if !hasRetention {
			// No retention policy for this tier, keep all
			logger.Debug().
				Str("tier", string(tierName)).
				Int("count", len(tieredBackups)).
				Msg("no retention policy for tier, keeping all backups")
			continue
		}

		if retention == 0 {
			// Retention = 0 means unlimited, keep all
			logger.Debug().
				Str("tier", string(tierName)).
				Int("count", len(tieredBackups)).
				Msg("unlimited retention for tier, keeping all backups")
			continue
		}

		// Sort backups by timestamp (oldest first)
		sort.Slice(tieredBackups, func(i, j int) bool {
			return tieredBackups[i].Timestamp.Before(tieredBackups[j].Timestamp)
		})

		// If we have more backups than retention limit, delete oldest
		if len(tieredBackups) > retention {
			toDelete := tieredBackups[:len(tieredBackups)-retention]
			for _, backup := range toDelete {
				filesToDelete = append(filesToDelete, backup.Path)
				logger.Info().
					Str("tier", string(tierName)).
					Str("file", backup.Path).
					Time("timestamp", backup.Timestamp).
					Msg("marking backup for deletion")
			}
		} else {
			logger.Debug().
				Str("tier", string(tierName)).
				Int("count", len(tieredBackups)).
				Int("retention", retention).
				Msg("within retention limit")
		}
	}

	return filesToDelete, nil
}

// DeleteFiles deletes the specified files and logs the results
func DeleteFiles(files []string, logger zerolog.Logger) error {
	deletedCount := 0
	errorCount := 0

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			logger.Error().
				Err(err).
				Str("file", file).
				Msg("failed to delete backup file")
			errorCount++
		} else {
			logger.Info().
				Str("file", file).
				Msg("deleted backup file")
			deletedCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("failed to delete %d out of %d files", errorCount, len(files))
	}

	logger.Info().
		Int("deleted", deletedCount).
		Msg("backup rotation completed")

	return nil
}
