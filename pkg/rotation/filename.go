package rotation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	// Date format constants
	DateFormatOld = "2006-01-02_15-04-05" // Old format with underscore separator
	DateFormatNew = "2006-01-02T15-04-05" // New format with T separator (ISO-8601 like)

	// Filename separators
	SeparatorOld = "_"  // Old separator (problematic with database names containing underscores)
	SeparatorNew = "--" // New separator (rarely used in database names)
)

// BackupFilenameComponents represents the parsed components of a backup filename
type BackupFilenameComponents struct {
	DatabaseName string
	Tier         string    // Empty string if no tier tag
	Timestamp    time.Time
	HasTier      bool      // True if filename contains tier tag
}

// ExtractDateFromFilename extracts the timestamp from a backup filename.
// Supports both old format (dbname_2024-12-17_15-04-05.backup)
// and new format (dbname--2024-12-17T15-04-05.backup).
// Returns an error instead of zero time if parsing fails.
// Deprecated: Use ParseBackupFilename for new code that needs tier information.
func ExtractDateFromFilename(filename string) (time.Time, error) {
	components, err := ParseBackupFilename(filename)
	if err != nil {
		return time.Time{}, err
	}
	return components.Timestamp, nil
}

// ParseBackupFilename parses a backup filename and extracts all components.
// Supports three formats:
// - New with tier: dbname--TIER--2024-12-17T15-04-05.backup
// - New without tier: dbname--2024-12-17T15-04-05.backup
// - Old format: dbname_2024-12-17_15-04-05.backup
func ParseBackupFilename(filename string) (BackupFilenameComponents, error) {
	base := filepath.Base(filename)

	// Try new format first (with -- separator)
	if strings.Contains(base, SeparatorNew) {
		return parseNewFormatWithTier(base)
	}

	// Fall back to old format (with _ separator)
	timestamp, err := parseOldFormat(base)
	if err != nil {
		return BackupFilenameComponents{}, err
	}

	// Extract database name from old format (everything before the last two underscores)
	parts := strings.Split(strings.TrimSuffix(base, filepath.Ext(base)), SeparatorOld)
	dbName := strings.Join(parts[:len(parts)-2], SeparatorOld)

	return BackupFilenameComponents{
		DatabaseName: dbName,
		Tier:         "",
		Timestamp:    timestamp,
		HasTier:      false,
	}, nil
}

// parseNewFormatWithTier parses filenames with optional tier tag.
// Supports: dbname--TIER--2024-12-17T15-04-05.backup and dbname--2024-12-17T15-04-05.backup
func parseNewFormatWithTier(filename string) (BackupFilenameComponents, error) {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Split by --
	parts := strings.Split(nameWithoutExt, SeparatorNew)

	if len(parts) == 3 {
		// Format: dbname--TIER--timestamp
		dbName := parts[0]
		tier := parts[1]
		timestampStr := parts[2]

		timestamp, err := time.Parse(DateFormatNew, timestampStr)
		if err != nil {
			return BackupFilenameComponents{}, fmt.Errorf("failed to parse timestamp '%s' from tier format: %w", timestampStr, err)
		}

		return BackupFilenameComponents{
			DatabaseName: dbName,
			Tier:         tier,
			Timestamp:    timestamp,
			HasTier:      true,
		}, nil
	} else if len(parts) == 2 {
		// Format: dbname--timestamp (no tier)
		dbName := parts[0]
		timestampStr := parts[1]

		timestamp, err := time.Parse(DateFormatNew, timestampStr)
		if err != nil {
			return BackupFilenameComponents{}, fmt.Errorf("failed to parse timestamp '%s' from new format: %w", timestampStr, err)
		}

		return BackupFilenameComponents{
			DatabaseName: dbName,
			Tier:         "",
			Timestamp:    timestamp,
			HasTier:      false,
		}, nil
	}

	return BackupFilenameComponents{}, fmt.Errorf("invalid new format: expected 2 or 3 parts separated by --, got %d in %s", len(parts), filename)
}

// parseNewFormat parses filenames like: dbname--2024-12-17T15-04-05.backup
// Deprecated: Use parseNewFormatWithTier for better tier support
func parseNewFormat(filename string) (time.Time, error) {
	components, err := parseNewFormatWithTier(filename)
	if err != nil {
		return time.Time{}, err
	}
	return components.Timestamp, nil
}

// parseOldFormat parses filenames like: dbname_2024-12-17_15-04-05.backup
// This has limitations with database names containing underscores.
// For simple database names (no underscores), it works fine.
// For database names with underscores, it will fail - those should use new format.
func parseOldFormat(filename string) (time.Time, error) {
	// Split by underscore
	parts := strings.Split(filename, SeparatorOld)
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("invalid old format: expected at least 3 parts separated by _, got %d in %s", len(parts), filename)
	}

	// The timestamp should be the last two parts (date_time)
	// For dbname_2024-12-17_15-04-05.backup:
	//   parts = [dbname, 2024-12-17, 15-04-05.backup]
	//   We want: 2024-12-17_15-04-05

	datePart := parts[len(parts)-2]
	timePart := parts[len(parts)-1]

	// Remove extension from time part
	timePart = strings.TrimSuffix(timePart, filepath.Ext(filename))

	// Reconstruct the timestamp string
	timestampStr := datePart + SeparatorOld + timePart

	// Parse the timestamp
	t, err := time.Parse(DateFormatOld, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse old format date from '%s': %w", timestampStr, err)
	}

	return t, nil
}

// GenerateBackupFilename creates a backup filename using the new format (without tier tag)
func GenerateBackupFilename(backupDir, dbName string, timestamp time.Time) string {
	dateStr := timestamp.Format(DateFormatNew)
	filename := fmt.Sprintf("%s%s%s.backup", dbName, SeparatorNew, dateStr)
	return filepath.Join(backupDir, filename)
}

// GenerateBackupFilenameWithTier creates a backup filename with tier tag
// Format: dbname--TIER--2024-12-17T15-04-05.backup
func GenerateBackupFilenameWithTier(backupDir, dbName, tier string, timestamp time.Time) string {
	dateStr := timestamp.Format(DateFormatNew)
	filename := fmt.Sprintf("%s%s%s%s%s.backup", dbName, SeparatorNew, tier, SeparatorNew, dateStr)
	return filepath.Join(backupDir, filename)
}

// GetBackupPattern returns the glob pattern to find all backups for a database
// Returns patterns for both old and new formats
func GetBackupPattern(backupDir, dbName string) string {
	// This pattern matches both formats:
	// - old: dbname_*.backup
	// - new: dbname--*.backup
	// We'll use a simple pattern that matches both
	return filepath.Join(backupDir, dbName+"*.backup")
}

// GetBackupPatternForTier returns the glob pattern to find backups for a specific tier
// Format: dbname--TIER--*.backup
func GetBackupPatternForTier(backupDir, dbName, tier string) string {
	pattern := fmt.Sprintf("%s%s%s%s*.backup", dbName, SeparatorNew, tier, SeparatorNew)
	return filepath.Join(backupDir, pattern)
}
