package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/williamokano/pg_backuper/pkg/config"
)

func TestFindShortestTierInterval(t *testing.T) {
	tests := []struct {
		name          string
		tiers         []config.RetentionTier
		wantDuration  time.Duration
		wantTierName  string
	}{
		{
			name: "hourly is shortest",
			tiers: []config.RetentionTier{
				{Tier: "daily", Retention: 7},
				{Tier: "hourly", Retention: 6},
			},
			wantDuration: 1 * time.Hour,
			wantTierName: "hourly",
		},
		{
			name: "daily is shortest",
			tiers: []config.RetentionTier{
				{Tier: "weekly", Retention: 4},
				{Tier: "daily", Retention: 7},
			},
			wantDuration: 24 * time.Hour,
			wantTierName: "daily",
		},
		{
			name: "only yearly",
			tiers: []config.RetentionTier{
				{Tier: "yearly", Retention: 3},
			},
			wantDuration: 365 * 24 * time.Hour,
			wantTierName: "yearly",
		},
		{
			name:         "empty tiers",
			tiers:        []config.RetentionTier{},
			wantDuration: 0,
			wantTierName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDuration, gotTierName := findShortestTierInterval(tt.tiers)
			if gotDuration != tt.wantDuration {
				t.Errorf("findShortestTierInterval() duration = %v, want %v", gotDuration, tt.wantDuration)
			}
			if gotTierName != tt.wantTierName {
				t.Errorf("findShortestTierInterval() tierName = %v, want %v", gotTierName, tt.wantTierName)
			}
		})
	}
}

func TestIsBackupForDatabase(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		dbName   string
		want     bool
	}{
		{
			name:     "new format exact match",
			filename: "mydb--2025-12-17T03-00-00.backup",
			dbName:   "mydb",
			want:     true,
		},
		{
			name:     "new format with underscore",
			filename: "my_prod_db--2025-12-17T03-00-00.backup",
			dbName:   "my_prod_db",
			want:     true,
		},
		{
			name:     "old format exact match",
			filename: "mydb_2025-12-17_03-00-00.backup",
			dbName:   "mydb",
			want:     true,
		},
		{
			name:     "old format with underscore",
			filename: "my_prod_db_2025-12-17_03-00-00.backup",
			dbName:   "my_prod_db",
			want:     true,
		},
		{
			name:     "new format wrong db",
			filename: "otherdb--2025-12-17T03-00-00.backup",
			dbName:   "mydb",
			want:     false,
		},
		{
			name:     "old format wrong db",
			filename: "otherdb_2025-12-17_03-00-00.backup",
			dbName:   "mydb",
			want:     false,
		},
		{
			name:     "prefix match but not exact (should not match)",
			filename: "mydb_test--2025-12-17T03-00-00.backup",
			dbName:   "mydb",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBackupForDatabase(tt.filename, tt.dbName)
			if got != tt.want {
				t.Errorf("isBackupForDatabase(%q, %q) = %v, want %v", tt.filename, tt.dbName, got, tt.want)
			}
		})
	}
}

func TestFindLastBackupTime(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test backup files
	testFiles := []struct {
		filename string
		isValid  bool
	}{
		{"mydb--2025-12-17T01-00-00.backup", true},
		{"mydb--2025-12-17T02-00-00.backup", true},
		{"mydb--2025-12-17T03-00-00.backup", true}, // Most recent
		{"otherdb--2025-12-17T04-00-00.backup", false}, // Wrong database
		{"mydb--invalid.backup", false}, // Invalid timestamp
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tmpDir, tf.filename)
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Test finding last backup
	lastTime, err := findLastBackupTime(tmpDir, "mydb")
	if err != nil {
		t.Fatalf("findLastBackupTime() error = %v", err)
	}

	// Expected time: 2025-12-17T03:00:00
	expectedTime := time.Date(2025, 12, 17, 3, 0, 0, 0, time.UTC)
	if !lastTime.Equal(expectedTime) {
		t.Errorf("findLastBackupTime() = %v, want %v", lastTime, expectedTime)
	}

	// Test with non-existent database
	lastTime, err = findLastBackupTime(tmpDir, "nonexistent")
	if err != nil {
		t.Fatalf("findLastBackupTime() error = %v", err)
	}
	if !lastTime.IsZero() {
		t.Errorf("findLastBackupTime() for nonexistent db = %v, want zero time", lastTime)
	}
}

func TestIsBackupDue(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	tests := []struct {
		name    string
		tiers   []config.RetentionTier
		wantDue bool
	}{
		{
			name: "hourly tier - should be due (2 hours passed)",
			tiers: []config.RetentionTier{
				{Tier: "hourly", Retention: 6},
			},
			wantDue: true,
		},
		{
			name: "daily tier - should not be due (only 2 hours passed)",
			tiers: []config.RetentionTier{
				{Tier: "daily", Retention: 7},
			},
			wantDue: false,
		},
		{
			name: "hourly and daily - should be due (hourly is shortest)",
			tiers: []config.RetentionTier{
				{Tier: "hourly", Retention: 6},
				{Tier: "daily", Retention: 7},
			},
			wantDue: true,
		},
		{
			name:    "no tiers - always due",
			tiers:   []config.RetentionTier{},
			wantDue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tier-specific backup files for each tier in the test
			for _, tier := range tt.tiers {
				filename := twoHoursAgo.Format(fmt.Sprintf("mydb--%s--2006-01-02T15-04-05.backup", tier.Tier))
				filePath := filepath.Join(tmpDir, filename)
				if err := os.WriteFile(filePath, []byte("test backup data"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			cfg := &config.Config{
				BackupDir: tmpDir,
				GlobalDefaults: config.GlobalDefaults{
					RetentionTiers: tt.tiers,
				},
			}

			db := config.DatabaseConfig{
				Name: "mydb",
				User: "postgres",
				Host: "localhost",
			}

			isDue, err := IsBackupDue(cfg, db, now, logger)
			if err != nil {
				t.Fatalf("IsBackupDue() error = %v", err)
			}

			if isDue != tt.wantDue {
				t.Errorf("IsBackupDue() = %v, want %v", isDue, tt.wantDue)
			}

			// Clean up test files for next iteration
			for _, tier := range tt.tiers {
				filename := twoHoursAgo.Format(fmt.Sprintf("mydb--%s--2006-01-02T15-04-05.backup", tier.Tier))
				filePath := filepath.Join(tmpDir, filename)
				os.Remove(filePath)
			}
		})
	}
}

func TestIsBackupDue_NoExistingBackup(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "backup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := zerolog.Nop()
	now := time.Now()

	cfg := &config.Config{
		BackupDir: tmpDir,
		GlobalDefaults: config.GlobalDefaults{
			RetentionTiers: []config.RetentionTier{
				{Tier: "daily", Retention: 7},
			},
		},
	}

	db := config.DatabaseConfig{
		Name: "newdb",
		User: "postgres",
		Host: "localhost",
	}

	// With no existing backup, should always be due
	isDue, err := IsBackupDue(cfg, db, now, logger)
	if err != nil {
		t.Fatalf("IsBackupDue() error = %v", err)
	}

	if !isDue {
		t.Errorf("IsBackupDue() with no existing backup = %v, want true", isDue)
	}
}
