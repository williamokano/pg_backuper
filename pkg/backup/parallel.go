package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/williamokano/pg_backuper/pkg/config"
)

// BackupAllDatabases performs backups of all enabled databases in parallel
// with concurrency control via semaphore
func BackupAllDatabases(ctx context.Context, cfg *config.Config, timestamp time.Time, logger zerolog.Logger) ([]Result, error) {
	// Filter enabled databases
	var enabledDBs []config.DatabaseConfig
	for _, db := range cfg.Databases {
		if db.IsEnabled() {
			enabledDBs = append(enabledDBs, db)
		} else {
			logger.Info().Str("database", db.Name).Msg("skipping disabled database")
		}
	}

	if len(enabledDBs) == 0 {
		logger.Warn().Msg("no enabled databases to backup")
		return nil, nil
	}

	maxConcurrent := cfg.GetMaxConcurrentBackups()
	logger.Info().
		Int("total_databases", len(enabledDBs)).
		Int("max_concurrent", maxConcurrent).
		Msg("starting parallel backup execution")

	// Create semaphore for concurrency control
	sem := semaphore.NewWeighted(int64(maxConcurrent))

	// Create errgroup for structured concurrency
	g, gCtx := errgroup.WithContext(ctx)

	// Results channel
	resultsChan := make(chan Result, len(enabledDBs))

	// Launch backup goroutine for each database
	for _, db := range enabledDBs {
		db := db // capture loop variable

		g.Go(func() error {
			// Acquire semaphore slot
			if err := sem.Acquire(gCtx, 1); err != nil {
				return fmt.Errorf("failed to acquire semaphore: %w", err)
			}
			defer sem.Release(1)

			// Check if context was cancelled
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			// Check if backup is due
			isDue, err := IsBackupDue(cfg, db, timestamp, logger)
			if err != nil {
				logger.Warn().
					Err(err).
					Str("database", db.Name).
					Msg("error checking if backup is due, proceeding with backup")
				isDue = true // On error, proceed with backup to be safe
			}

			if !isDue {
				logger.Info().
					Str("database", db.Name).
					Msg("backup not due yet, skipping")
				// Return a success result for skipped backup
				resultsChan <- Result{
					Database: db.Name,
					Success:  true,
					Skipped:  true,
					Duration: 0,
				}
				return nil
			}

			// Perform backup
			result := BackupDatabase(cfg, db, timestamp, logger)
			resultsChan <- result

			// If backup failed, return error (will cancel other operations)
			if !result.Success {
				return fmt.Errorf("backup failed for database %s: %w", db.Name, result.Error)
			}

			return nil
		})
	}

	// Wait for all backups to complete
	waitErr := g.Wait()
	close(resultsChan)

	// Collect results
	var results []Result
	for result := range resultsChan {
		results = append(results, result)
	}

	// Log summary
	successCount := 0
	failureCount := 0
	var totalDuration time.Duration

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
		totalDuration += result.Duration
	}

	logger.Info().
		Int("successful", successCount).
		Int("failed", failureCount).
		Dur("total_duration", totalDuration).
		Msg("parallel backup execution completed")

	return results, waitErr
}
