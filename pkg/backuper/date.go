package backuper

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ExtractDateFromFilename extracts the timestamp from a backup filename.
// Returns an error instead of zero time if parsing fails.
func ExtractDateFromFilename(filename string) (time.Time, error) {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid filename format: %s", filename)
	}

	dateTimeStr := strings.TrimSuffix(strings.Join(parts[1:], "_"), filepath.Ext(base))
	layout := "2006-01-02_15-04-05"

	t, err := time.Parse(layout, dateTimeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse date from '%s': %w", dateTimeStr, err)
	}

	return t, nil
}
