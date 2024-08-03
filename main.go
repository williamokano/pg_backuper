package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/williamokano/pg_backuper/pkg/backuper"
)

func main() {
	configFile := "./noop_config.json"
	schemaFile := "./db_config_schema.json"

	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	backuper.Validate(schemaFile, configFile)
	config, err := backuper.ParseConfig(configFile)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	date := time.Now().Format("2006-01-02_15-04-05")

	// Ensure backup directory exists
	if err := os.MkdirAll(config.BackupDir, os.ModePerm); err != nil {
		log.Fatalf("Error creating backup directory: %v", err)
	}

	for _, db := range config.Databases {
		backupFile := filepath.Join(config.BackupDir, fmt.Sprintf("%s_%s.backup", db.Name, date))
		cmd := exec.Command("pg_dump", "-U", db.User, "-h", db.Host, "-F", "c", "-b", "-v", "-f", backupFile, db.Name)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+db.Password)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error backuping database %s: %v\n", db.Name, err)
			continue
		}

		fmt.Printf("Backuping database %s to %s\n", db.Name, backupFile)

		// Rotate backups
		files, err := filepath.Glob(filepath.Join(config.BackupDir, fmt.Sprintf("%s_*.backup", db.Name)))
		if err != nil {
			fmt.Printf("Error listing database backups: %v\n", err)
			continue
		}

		// Sort files
		sort.Slice(files, func(i, j int) bool {
			dateI := backuper.ExtractDateFromFilename(files[i])
			dateJ := backuper.ExtractDateFromFilename(files[j])
			return dateI.Before(dateJ)
		})

		// Delete old files beyond retention limit
		if len(files) > config.Retention {
			filesToDelete := files[:len(files)-config.Retention]
			for _, file := range filesToDelete {
				if err := os.Remove(file); err != nil {
					fmt.Printf("Error deleting file %s: %v\n", file, err)
				} else {
					fmt.Printf("Deleted old backup file: %s\n", file)
				}
			}
		} else {
			fmt.Println("No files to delete, within retention limit.")
		}
	}
}
