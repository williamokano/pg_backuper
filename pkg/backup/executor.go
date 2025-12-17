package backup

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/rotation"
)

// Result represents the outcome of a backup operation
type Result struct {
	Database       string
	Success        bool
	Skipped        bool     // True if backup was skipped due to not being due
	TiersCompleted []string // List of tiers that were successfully backed up
	TiersFailed    []string // List of tiers that failed to backup
	Error          error
	Duration       time.Duration
}

// BackupDatabase performs backups for specified tiers of a single database
func BackupDatabase(cfg *config.Config, db config.DatabaseConfig, timestamp time.Time, dueTiers []string, logger zerolog.Logger) Result {
	start := time.Now()
	result := Result{
		Database:       db.Name,
		Success:        false,
		TiersCompleted: []string{},
		TiersFailed:    []string{},
	}

	dbLog := logger.With().Str("database", db.Name).Logger()
	port := db.GetPort(cfg.GlobalDefaults)

	// If no tiers specified, skip backup (backward compatibility with old behavior)
	if len(dueTiers) == 0 {
		dbLog.Debug().Msg("no tiers due, skipping backup")
		result.Skipped = true
		result.Success = true
		result.Duration = time.Since(start)
		return result
	}

	dbLog.Info().
		Strs("due_tiers", dueTiers).
		Str("host", db.Host).
		Int("port", port).
		Msg("starting backup for due tiers")

	// Get .pgpass file path (shared across all tier backups)
	pgpassPath, pgpassErr := GetPgpassPath(cfg.GetPgpassFile())
	if pgpassErr != nil {
		// Log warning but continue - pg_dump might work with trust auth or other methods
		dbLog.Warn().
			Err(pgpassErr).
			Msg(".pgpass file not found, pg_dump will use alternative authentication")
	} else {
		// Validate permissions
		if err := ValidatePgpassPermissions(pgpassPath); err != nil {
			dbLog.Warn().
				Err(err).
				Str("pgpass_path", pgpassPath).
				Msg(".pgpass file has incorrect permissions")
		} else {
			dbLog.Debug().
				Str("pgpass_path", pgpassPath).
				Msg("using .pgpass for authentication")
		}
	}

	// Create one backup per due tier
	for _, tier := range dueTiers {
		// Generate tier-specific backup filename
		backupFile := rotation.GenerateBackupFilenameWithTier(cfg.BackupDir, db.Name, tier, timestamp)

		tierLog := dbLog.With().Str("tier", tier).Logger()
		tierLog.Info().
			Str("backup_file", backupFile).
			Msg("creating tier-specific backup")

		// Build pg_dump command
		cmd := exec.Command("pg_dump",
			"-U", db.User,
			"-h", db.Host,
			"-p", fmt.Sprintf("%d", port),
			"-F", "c", // custom format (compressed)
			"-b",      // include blobs
			"-v",      // verbose
			"-f", backupFile,
			db.Name,
		)

		// Set PGPASSFILE environment variable if .pgpass was found
		if pgpassPath != "" {
			cmd.Env = append(os.Environ(), "PGPASSFILE="+pgpassPath)
		} else {
			cmd.Env = os.Environ()
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Execute backup for this tier
		if err := cmd.Run(); err != nil {
			tierLog.Error().
				Err(err).
				Msg("backup failed for tier")
			result.TiersFailed = append(result.TiersFailed, tier)

			// Clean up failed backup file (may be 0 bytes)
			if removeErr := os.Remove(backupFile); removeErr != nil {
				tierLog.Warn().
					Err(removeErr).
					Str("file", backupFile).
					Msg("failed to remove incomplete backup file")
			} else {
				tierLog.Info().
					Str("file", backupFile).
					Msg("removed incomplete backup file")
			}

			// Continue with other tiers even if this one fails
			continue
		}

		// Verify backup file was actually created and has content
		fileInfo, err := os.Stat(backupFile)
		if err != nil {
			tierLog.Error().
				Err(err).
				Str("file", backupFile).
				Msg("backup file not found after pg_dump")
			result.TiersFailed = append(result.TiersFailed, tier)
			continue
		}

		if fileInfo.Size() == 0 {
			tierLog.Error().
				Str("file", backupFile).
				Msg("backup file is empty (0 bytes)")
			result.TiersFailed = append(result.TiersFailed, tier)
			// Remove the empty file
			os.Remove(backupFile)
			continue
		}

		tierLog.Info().
			Int64("size_bytes", fileInfo.Size()).
			Msg("backup completed for tier")
		result.TiersCompleted = append(result.TiersCompleted, tier)
	}

	result.Duration = time.Since(start)

	// Check overall success
	if len(result.TiersFailed) > 0 {
		result.Error = fmt.Errorf("%d of %d tier backups failed", len(result.TiersFailed), len(dueTiers))
		dbLog.Error().
			Strs("completed_tiers", result.TiersCompleted).
			Strs("failed_tiers", result.TiersFailed).
			Dur("duration", result.Duration).
			Msg("backup completed with failures")
	} else {
		result.Success = true
		dbLog.Info().
			Strs("completed_tiers", result.TiersCompleted).
			Dur("duration", result.Duration).
			Msg("all tier backups completed successfully")
	}

	// Perform rotation only if at least one backup succeeded
	// CRITICAL: Never delete old backups if all new backups failed
	if len(result.TiersCompleted) > 0 {
		retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)
		if len(retentionTiers) == 0 {
			dbLog.Warn().Msg("no retention tiers configured, skipping rotation")
		} else {
			dbLog.Info().
				Int("tier_count", len(retentionTiers)).
				Strs("completed_tiers", result.TiersCompleted).
				Msg("applying retention policy")

			if err := rotation.RotateBackups(cfg.BackupDir, db.Name, retentionTiers, dbLog); err != nil {
				dbLog.Error().Err(err).Msg("rotation failed")
				// Don't fail the backup operation if rotation fails
			}
		}
	} else {
		dbLog.Warn().Msg("skipping rotation - no successful backups created")
	}

	return result
}
