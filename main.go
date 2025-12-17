package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/logger"
	"github.com/williamokano/pg_backuper/pkg/rotation"
)

func main() {
	configFile := "./noop_config.json"

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	// Validate and parse config
	if err := config.Validate(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.ParseConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with config settings
	logger.Init(cfg.GetLogLevel(), cfg.GetLogFormat())
	log := logger.Get()

	log.Info().Str("config_file", configFile).Msg("starting pg_backuper v2.0")

	// Ensure backup directory exists
	if err := os.MkdirAll(cfg.BackupDir, os.ModePerm); err != nil {
		log.Fatal().Err(err).Str("backup_dir", cfg.BackupDir).Msg("failed to create backup directory")
	}

	timestamp := time.Now()

	// Process each database
	successCount := 0
	failureCount := 0

	for _, db := range cfg.Databases {
		if !db.IsEnabled() {
			log.Info().Str("database", db.Name).Msg("skipping disabled database")
			continue
		}

		dbLog := log.With().Str("database", db.Name).Logger()

		// Perform backup
		backupFile := rotation.GenerateBackupFilename(cfg.BackupDir, db.Name, timestamp)
		port := db.GetPort(cfg.GlobalDefaults)

		dbLog.Info().
			Str("backup_file", backupFile).
			Str("host", db.Host).
			Int("port", port).
			Msg("starting backup")

		cmd := exec.Command("pg_dump",
			"-U", db.User,
			"-h", db.Host,
			"-p", fmt.Sprintf("%d", port),
			"-F", "c", // custom format
			"-b",      // include blobs
			"-v",      // verbose
			"-f", backupFile,
			db.Name,
		)

		// TODO: This will be replaced with .pgpass in Phase 5
		// For now, keep using PGPASSWORD for backward compatibility during transition
		cmd.Env = append(os.Environ(), "PGPASSWORD=")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			dbLog.Error().Err(err).Msg("backup failed")
			failureCount++
			continue
		}

		dbLog.Info().Msg("backup completed")

		// Perform rotation with multi-tier retention
		retentionTiers := db.GetRetentionTiers(cfg.GlobalDefaults)
		if len(retentionTiers) == 0 {
			dbLog.Warn().Msg("no retention tiers configured, skipping rotation")
		} else {
			dbLog.Info().
				Int("tier_count", len(retentionTiers)).
				Msg("applying retention policy")

			if err := rotation.RotateBackups(cfg.BackupDir, db.Name, retentionTiers, dbLog); err != nil {
				dbLog.Error().Err(err).Msg("rotation failed")
				// Don't count as failure - backup succeeded
			}
		}

		successCount++
	}

	log.Info().
		Int("successful", successCount).
		Int("failed", failureCount).
		Msg("pg_backuper v2.0 completed")

	if failureCount > 0 {
		os.Exit(1)
	}
}
