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

// ExtractDateFromFilename extracts the timestamp from a backup filename.
// Supports both old format (dbname_2024-12-17_15-04-05.backup)
// and new format (dbname--2024-12-17T15-04-05.backup).
// Returns an error instead of zero time if parsing fails.
func ExtractDateFromFilename(filename string) (time.Time, error) {
	base := filepath.Base(filename)

	// Try new format first (with -- separator)
	if strings.Contains(base, SeparatorNew) {
		return parseNewFormat(base)
	}

	// Fall back to old format (with _ separator)
	return parseOldFormat(base)
}

// parseNewFormat parses filenames like: dbname--2024-12-17T15-04-05.backup
func parseNewFormat(filename string) (time.Time, error) {
	// Split on the last occurrence of --
	lastIdx := strings.LastIndex(filename, SeparatorNew)
	if lastIdx == -1 {
		return time.Time{}, fmt.Errorf("invalid new format: no -- separator found in %s", filename)
	}

	// Extract the timestamp part (after --)
	timestampPart := filename[lastIdx+len(SeparatorNew):]

	// Remove extension if present
	timestampStr := strings.TrimSuffix(timestampPart, filepath.Ext(filename))

	// Parse the timestamp
	t, err := time.Parse(DateFormatNew, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse new format date from '%s': %w", timestampStr, err)
	}

	return t, nil
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

// GenerateBackupFilename creates a backup filename using the new format
func GenerateBackupFilename(backupDir, dbName string, timestamp time.Time) string {
	dateStr := timestamp.Format(DateFormatNew)
	filename := fmt.Sprintf("%s%s%s.backup", dbName, SeparatorNew, dateStr)
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
