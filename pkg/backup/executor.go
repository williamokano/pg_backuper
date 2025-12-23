package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/rotation"
	"github.com/williamokano/pg_backuper/pkg/storage"

	// Import backends to register them
	_ "github.com/williamokano/pg_backuper/pkg/storage/backblaze"
	_ "github.com/williamokano/pg_backuper/pkg/storage/local"
	_ "github.com/williamokano/pg_backuper/pkg/storage/s3"
	_ "github.com/williamokano/pg_backuper/pkg/storage/ssh"
)

// Result represents the outcome of a backup operation
type Result struct {
	Database       string
	Success        bool
	Skipped        bool                          // True if backup was skipped due to not being due
	TiersCompleted []string                      // List of tiers that were successfully backed up
	TiersFailed    []string                      // List of tiers that failed to backup
	BackendResults map[string][]storage.Result   // Tier -> Backend results
	Error          error
	Duration       time.Duration
}

// BackupDatabase performs backups for specified tiers of a single database
func BackupDatabase(cfg *config.Config, db config.DatabaseConfig, timestamp time.Time, dueTiers []string, logger zerolog.Logger) Result {
	start := time.Now()
	ctx := context.Background()

	result := Result{
		Database:       db.Name,
		Success:        false,
		TiersCompleted: []string{},
		TiersFailed:    []string{},
		BackendResults: make(map[string][]storage.Result),
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
		// FAIL FAST: .pgpass file must exist for backup to succeed
		result.Error = fmt.Errorf(".pgpass file not found: %w", pgpassErr)
		result.Duration = time.Since(start)
		dbLog.Error().
			Err(pgpassErr).
			Msg("FATAL: .pgpass file not found - cannot authenticate to database")
		return result
	}

	// FAIL FAST: Validate permissions - PostgreSQL will refuse to use the file if permissions are wrong
	if err := ValidatePgpassPermissions(pgpassPath); err != nil {
		result.Error = fmt.Errorf(".pgpass file has incorrect permissions: %w", err)
		result.Duration = time.Since(start)
		dbLog.Error().
			Err(err).
			Str("pgpass_path", pgpassPath).
			Msg("FATAL: .pgpass file must have 0600 permissions - run: chmod 600 /config/.pgpass")
		return result
	}

	dbLog.Debug().
		Str("pgpass_path", pgpassPath).
		Msg("using .pgpass for authentication")

	// Initialize storage backends
	backends, err := initializeBackends(ctx, cfg, db, dbLog)
	if err != nil {
		result.Error = fmt.Errorf("failed to initialize storage backends: %w", err)
		result.Duration = time.Since(start)
		dbLog.Error().Err(err).Msg("FATAL: cannot initialize storage backends")
		return result
	}
	defer closeBackends(backends)

	dbLog.Info().
		Int("backend_count", len(backends)).
		Msg("initialized storage backends")

	// Create temp directory for pg_dump
	tempDir := cfg.Storage.TempDir
	if tempDir == "" {
		// Backward compatibility: if BackupDir is set, use it with .tmp subdirectory
		if cfg.BackupDir != "" {
			tempDir = filepath.Join(cfg.BackupDir, ".tmp")
		} else {
			tempDir = filepath.Join(os.TempDir(), "pg_backuper")
		}
	}
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		result.Duration = time.Since(start)
		dbLog.Error().Err(err).Str("temp_dir", tempDir).Msg("FATAL: cannot create temp directory")
		return result
	}

	// Initialize multi-uploader
	uploader := storage.NewMultiUploader(logger)

	// Create one backup per due tier
	for _, tier := range dueTiers {
		// Generate temp filename
		tempFile := filepath.Join(tempDir, fmt.Sprintf("%s--%s--%s.backup.tmp",
			db.Name, tier, timestamp.Format("2006-01-02T15-04-05")))

		// Generate final filename (without .tmp extension, without base directory)
		finalFilename := rotation.GenerateBackupFilenameWithTier("", db.Name, tier, timestamp)
		finalFilename = filepath.Base(finalFilename)

		tierLog := dbLog.With().Str("tier", tier).Logger()
		tierLog.Info().
			Str("temp_file", tempFile).
			Str("final_filename", finalFilename).
			Msg("creating tier-specific backup")

		// Build pg_dump command (writes to temp file)
		cmd := exec.Command("pg_dump",
			"-U", db.User,
			"-h", db.Host,
			"-p", fmt.Sprintf("%d", port),
			"-F", "c", // custom format (compressed)
			"-b",      // include blobs
			"-v",      // verbose
			"-f", tempFile,
			db.Name,
		)

		// Set PGPASSFILE environment variable if .pgpass was found
		if pgpassPath != "" {
			cmd.Env = append(os.Environ(), "PGPASSFILE="+pgpassPath)
		} else {
			cmd.Env = os.Environ()
		}

		// Create logs directory for pg_dump output
		logsDir := filepath.Join(tempDir, "logs")
		var logFile *os.File
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			tierLog.Warn().Err(err).Msg("failed to create logs directory, pg_dump output will go to stdout")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			// Create log file for this backup operation
			logFileName := filepath.Join(logsDir, fmt.Sprintf("%s--%s--%s.log", db.Name, tier, timestamp.Format("2006-01-02T15-04-05")))
			var err error
			logFile, err = os.Create(logFileName)
			if err != nil {
				tierLog.Warn().Err(err).Msg("failed to create log file, pg_dump output will go to stdout")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				cmd.Stdout = logFile
				cmd.Stderr = logFile
				tierLog.Debug().Str("log_file", logFileName).Msg("pg_dump output redirected to log file")
			}
		}

		// Execute backup for this tier
		cmdErr := cmd.Run()

		// Close log file if it was opened
		if logFile != nil {
			logFile.Close()
		}

		if cmdErr != nil {
			tierLog.Error().
				Err(cmdErr).
				Msg("backup failed for tier")
			result.TiersFailed = append(result.TiersFailed, tier)

			// Clean up failed temp backup file
			os.Remove(tempFile)

			// Continue with other tiers even if this one fails
			continue
		}

		// Verify backup file was actually created and has content
		fileInfo, err := os.Stat(tempFile)
		if err != nil {
			tierLog.Error().
				Err(err).
				Str("file", tempFile).
				Msg("backup file not found after pg_dump")
			result.TiersFailed = append(result.TiersFailed, tier)
			continue
		}

		if fileInfo.Size() == 0 {
			tierLog.Error().
				Str("file", tempFile).
				Msg("backup file is empty (0 bytes)")
			result.TiersFailed = append(result.TiersFailed, tier)
			// Remove the empty file
			os.Remove(tempFile)
			continue
		}

		tierLog.Info().
			Int64("size_bytes", fileInfo.Size()).
			Msg("backup created successfully, uploading to destinations")

		// Upload to all configured backends in parallel
		uploadResults := uploader.Upload(ctx, backends, tempFile, finalFilename)
		result.BackendResults[tier] = uploadResults

		// Check if at least one backend succeeded
		hasSuccess := false
		for _, ur := range uploadResults {
			if ur.Success {
				hasSuccess = true
				break
			}
		}

		if !hasSuccess {
			tierLog.Error().Msg("all backends failed to upload backup")
			result.TiersFailed = append(result.TiersFailed, tier)
			// Keep temp file for retry
		} else {
			tierLog.Info().Msg("backup uploaded to at least one destination")
			result.TiersCompleted = append(result.TiersCompleted, tier)
			// Delete temp file after successful upload
			os.Remove(tempFile)
		}
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

	// Perform rotation on each backend only if at least one backup succeeded
	// CRITICAL: Never delete old backups if all new backups failed
	if len(result.TiersCompleted) > 0 {
		retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)
		if len(retentionTiers) == 0 {
			dbLog.Warn().Msg("no retention tiers configured, skipping rotation")
		} else {
			dbLog.Info().
				Int("tier_count", len(retentionTiers)).
				Strs("completed_tiers", result.TiersCompleted).
				Int("backend_count", len(backends)).
				Msg("applying retention policy per backend")

			// Apply retention policy on each backend independently
			for _, backend := range backends {
				if err := rotation.ApplyRetentionWithBackend(ctx, backend, db.Name, retentionTiers, dbLog); err != nil {
					dbLog.Error().
						Err(err).
						Str("backend", backend.Name()).
						Msg("rotation failed for backend")
					// Don't fail the backup operation if rotation fails on one backend
				}
			}
		}
	} else {
		dbLog.Warn().Msg("skipping rotation - no successful backups created")
	}

	return result
}

// initializeBackends creates backend instances from config
func initializeBackends(ctx context.Context, cfg *config.Config, db config.DatabaseConfig, logger zerolog.Logger) ([]storage.Backend, error) {
	// Backward compatibility: if no storage config but BackupDir is set, create default local backend
	if len(cfg.Storage.Destinations) == 0 && cfg.BackupDir != "" {
		logger.Debug().Str("backup_dir", cfg.BackupDir).Msg("no storage config, creating default local backend")
		storageConfig := storage.Config{
			Name:    "default_local",
			Type:    "local",
			Enabled: true,
			BaseDir: cfg.BackupDir,
			Options: map[string]interface{}{
				"path": cfg.BackupDir,
			},
		}

		factory := storage.NewFactory()
		backend, err := factory.Create(ctx, storageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create default local backend: %w", err)
		}

		return []storage.Backend{backend}, nil
	}

	// Get destination names for this database
	destNames := db.GetStorageDestinations(cfg)

	if len(destNames) == 0 {
		return nil, fmt.Errorf("no storage destinations configured for database %s", db.Name)
	}

	// Build storage configs for requested destinations
	var storageConfigs []storage.Config
	for _, destName := range destNames {
		for _, dest := range cfg.Storage.Destinations {
			if dest.Name == destName && dest.Enabled {
				storageConfigs = append(storageConfigs, storage.Config{
					Name:    dest.Name,
					Type:    dest.Type,
					Enabled: dest.Enabled,
					BaseDir: dest.BaseDir,
					Options: dest.Options,
				})
				break
			}
		}
	}

	if len(storageConfigs) == 0 {
		return nil, fmt.Errorf("no enabled storage destinations found")
	}

	// Create backends using factory
	factory := storage.NewFactory()
	backends, err := factory.CreateAll(ctx, storageConfigs)
	if err != nil {
		return nil, err
	}

	logger.Info().
		Int("count", len(backends)).
		Strs("destinations", destNames).
		Msg("initialized storage backends")

	return backends, nil
}

// closeBackends safely closes all backends
func closeBackends(backends []storage.Backend) {
	for _, backend := range backends {
		backend.Close()
	}
}
