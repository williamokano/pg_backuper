package rotation

import (
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
)

// RotateBackups performs backup rotation for a single database
func RotateBackups(backupDir string, dbName string, retentionTiers []config.RetentionTier, logger zerolog.Logger) error {
	dbLogger := logger.With().Str("database", dbName).Logger()

	// Find all backup files for this database
	pattern := GetBackupPattern(backupDir, dbName)
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	dbLogger.Debug().
		Int("file_count", len(files)).
		Str("pattern", pattern).
		Msg("found backup files")

	if len(files) == 0 {
		dbLogger.Debug().Msg("no backup files found, nothing to rotate")
		return nil
	}

	// Parse backup files and extract timestamps
	var backups []BackupFile
	for _, file := range files {
		timestamp, err := ExtractDateFromFilename(file)
		if err != nil {
			dbLogger.Warn().
				Err(err).
				Str("file", file).
				Msg("skipping file with invalid date format")
			continue
		}

		backups = append(backups, BackupFile{
			Path:      file,
			Timestamp: timestamp,
		})
	}

	dbLogger.Info().
		Int("valid_backups", len(backups)).
		Int("total_files", len(files)).
		Msg("parsed backup files")

	if len(backups) == 0 {
		dbLogger.Warn().Msg("no valid backup files found after parsing")
		return nil
	}

	// Apply retention policy
	filesToDelete, err := ApplyRetention(backups, retentionTiers, dbLogger)
	if err != nil {
		return fmt.Errorf("failed to apply retention policy: %w", err)
	}

	if len(filesToDelete) == 0 {
		dbLogger.Info().Msg("no backups to delete, within retention limits")
		return nil
	}

	// Delete files
	dbLogger.Info().
		Int("to_delete", len(filesToDelete)).
		Msg("applying retention policy")

	if err := DeleteFiles(filesToDelete, dbLogger); err != nil {
		return fmt.Errorf("failed to delete files: %w", err)
	}

	return nil
}
