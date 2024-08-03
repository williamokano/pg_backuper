package backuper

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func ExtractDateFromFilename(filename string) time.Time {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) < 2 {
		return time.Time{}
	}

	dateTimeStr := strings.TrimSuffix(strings.Join(parts[1:], "_"), filepath.Ext(base))
	layout := "2006-01-02_15-04-05"

	t, err := time.Parse(layout, dateTimeStr)
	if err != nil {
		fmt.Printf("Failed to parse date from `%s`\n", dateTimeStr)
		return time.Time{}
	}

	return t
}
