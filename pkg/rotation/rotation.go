package rotation

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/storage"
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

	// Parse backup files and extract timestamps and tier tags
	var backups []BackupFile
	for _, file := range files {
		components, err := ParseBackupFilename(file)
		if err != nil {
			dbLogger.Warn().
				Err(err).
				Str("file", file).
				Msg("skipping file with invalid format")
			continue
		}

		backups = append(backups, BackupFile{
			Path:         file,
			Timestamp:    components.Timestamp,
			CreationTier: components.Tier, // May be empty for old untagged backups
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

// ApplyRetentionWithBackend applies retention policy using storage backend
func ApplyRetentionWithBackend(ctx context.Context, backend storage.Backend, dbName string, retentionTiers []config.RetentionTier, logger zerolog.Logger) error {
	backendLog := logger.With().
		Str("backend", backend.Name()).
		Str("backend_type", backend.Type()).
		Str("database", dbName).
		Logger()

	backendLog.Debug().Msg("starting retention check")

	// For each tier, apply retention
	for _, tier := range retentionTiers {
		pattern := fmt.Sprintf("%s--%s--*.backup", dbName, tier.Tier)

		// List files for this tier using backend
		files, err := backend.List(ctx, pattern)
		if err != nil {
			return fmt.Errorf("failed to list files for tier %s: %w", tier.Tier, err)
		}

		tierLog := backendLog.With().Str("tier", tier.Tier).Logger()
		tierLog.Debug().Int("count", len(files)).Msg("found backups for tier")

		// Files are already sorted by modtime (newest first) from backend.List()
		if len(files) <= tier.Retention {
			tierLog.Debug().
				Int("found", len(files)).
				Int("retention", tier.Retention).
				Msg("retention not exceeded, no files to delete")
			continue
		}

		// Delete old files (beyond retention count)
		filesToDelete := files[tier.Retention:]

		tierLog.Info().
			Int("total", len(files)).
			Int("retention", tier.Retention).
			Int("to_delete", len(filesToDelete)).
			Msg("applying retention policy")

		for _, file := range filesToDelete {
			if err := backend.Delete(ctx, file.Path); err != nil {
				tierLog.Error().
					Err(err).
					Str("file", file.Path).
					Msg("failed to delete old backup")
			} else {
				tierLog.Info().
					Str("file", file.Path).
					Int64("size", file.Size).
					Msg("deleted old backup")
			}
		}
	}

	return nil
}
