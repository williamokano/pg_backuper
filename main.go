package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/williamokano/pg_backuper/pkg/backup"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/logger"
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

	// Create context for cancellation support
	ctx := context.Background()

	// Execute parallel backups
	results, err := backup.BackupAllDatabases(ctx, cfg, timestamp, *log)
	if err != nil {
		log.Error().Err(err).Msg("backup execution failed")
		os.Exit(1)
	}

	// Count successes, skips, and failures
	successCount := 0
	skippedCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Skipped {
			skippedCount++
		} else if result.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	log.Info().
		Int("successful", successCount).
		Int("skipped", skippedCount).
		Int("failed", failureCount).
		Msg("pg_backuper v2.0 completed")

	if failureCount > 0 {
		os.Exit(1)
	}
}
