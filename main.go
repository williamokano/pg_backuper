package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/williamokano/pg_backuper/pkg/backuper"
	"github.com/williamokano/pg_backuper/pkg/logger"
)

func main() {
	// Initialize logger with default settings for now
	logger.Init("info", "json")
	log := logger.Get()

	configFile := "./noop_config.json"

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	log.Info().Str("config_file", configFile).Msg("starting pg_backuper")

	backuper.Validate(configFile)
	config, err := backuper.ParseConfig(configFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse config file")
	}

	date := time.Now().Format("2006-01-02_15-04-05")

	// Ensure backup directory exists
	if err := os.MkdirAll(config.BackupDir, os.ModePerm); err != nil {
		log.Fatal().Err(err).Str("backup_dir", config.BackupDir).Msg("failed to create backup directory")
	}

	for _, db := range config.Databases {
		dbLog := log.With().Str("database", db.Name).Logger()

		backupFile := filepath.Join(config.BackupDir, db.Name+"_"+date+".backup")
		dbLog.Info().Str("backup_file", backupFile).Msg("starting backup")

		cmd := exec.Command("pg_dump", "-U", db.User, "-h", db.Host, "-F", "c", "-b", "-v", "-f", backupFile, db.Name)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
		if err := cmd.Run(); err != nil {
			dbLog.Error().Err(err).Msg("backup failed")
			continue
		}

		dbLog.Info().Msg("backup completed")

		// Rotate backups
		pattern := filepath.Join(config.BackupDir, db.Name+"_*.backup")
		files, err := filepath.Glob(pattern)
		if err != nil {
			dbLog.Error().Err(err).Msg("failed to list backup files")
			continue
		}

		// Parse and filter valid backup files
		type backupInfo struct {
			path string
			date time.Time
		}
		var validBackups []backupInfo
		for _, file := range files {
			date, err := backuper.ExtractDateFromFilename(file)
			if err != nil {
				dbLog.Warn().Err(err).Str("file", file).Msg("skipping file with invalid date format")
				continue
			}
			validBackups = append(validBackups, backupInfo{path: file, date: date})
		}

		// Sort files by date (oldest first)
		sort.Slice(validBackups, func(i, j int) bool {
			return validBackups[i].date.Before(validBackups[j].date)
		})

		// Delete old files beyond retention limit
		if len(validBackups) > config.Retention {
			filesToDelete := validBackups[:len(validBackups)-config.Retention]
			for _, bf := range filesToDelete {
				if err := os.Remove(bf.path); err != nil {
					dbLog.Error().Err(err).Str("file", bf.path).Msg("failed to delete old backup")
				} else {
					dbLog.Info().Str("file", bf.path).Msg("deleted old backup")
				}
			}
		} else {
			dbLog.Info().Int("count", len(validBackups)).Int("retention", config.Retention).Msg("within retention limit, no files to delete")
		}
	}

	log.Info().Msg("pg_backuper completed successfully")
}
