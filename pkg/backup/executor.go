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
	Database string
	Success  bool
	Error    error
	Duration time.Duration
}

// BackupDatabase performs a backup of a single database
func BackupDatabase(cfg *config.Config, db config.DatabaseConfig, timestamp time.Time, logger zerolog.Logger) Result {
	start := time.Now()
	result := Result{
		Database: db.Name,
		Success:  false,
	}

	dbLog := logger.With().Str("database", db.Name).Logger()

	// Generate backup filename
	backupFile := rotation.GenerateBackupFilename(cfg.BackupDir, db.Name, timestamp)
	port := db.GetPort(cfg.GlobalDefaults)

	dbLog.Info().
		Str("backup_file", backupFile).
		Str("host", db.Host).
		Int("port", port).
		Msg("starting backup")

	// Get .pgpass file path
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

	// Execute backup
	if err := cmd.Run(); err != nil {
		result.Error = fmt.Errorf("pg_dump failed: %w", err)
		result.Duration = time.Since(start)
		dbLog.Error().
			Err(err).
			Dur("duration", result.Duration).
			Msg("backup failed")
		return result
	}

	result.Duration = time.Since(start)
	dbLog.Info().
		Dur("duration", result.Duration).
		Msg("backup completed")

	// Perform rotation
	retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)
	if len(retentionTiers) == 0 {
		dbLog.Warn().Msg("no retention tiers configured, skipping rotation")
	} else {
		dbLog.Info().
			Int("tier_count", len(retentionTiers)).
			Msg("applying retention policy")

		if err := rotation.RotateBackups(cfg.BackupDir, db.Name, retentionTiers, dbLog); err != nil {
			dbLog.Error().Err(err).Msg("rotation failed")
			// Don't fail the backup operation if rotation fails
		}
	}

	result.Success = true
	return result
}
