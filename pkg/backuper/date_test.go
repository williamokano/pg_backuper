package backuper

import (
	"testing"
	"time"
)

func TestExtractDateFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     time.Time
		wantErr  bool
	}{
		{
			name:     "valid simple database name",
			filename: "mydb_2024-12-17_03-00-00.backup",
			want:     time.Date(2024, 12, 17, 3, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "valid with full path",
			filename: "/backups/database_2024-01-01_00-00-00.backup",
			want:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "invalid - no underscore separator",
			filename: "database.backup",
			wantErr:  true,
		},
		{
			name:     "invalid - wrong date format",
			filename: "mydb_invalid-date.backup",
			wantErr:  true,
		},
		{
			name:     "invalid - incomplete date",
			filename: "mydb_2024-12.backup",
			wantErr:  true,
		},
		{
			name:     "invalid - no extension",
			filename: "mydb_2024-12-17_03-00-00",
			wantErr:  false, // Extension is optional
			want:     time.Date(2024, 12, 17, 3, 0, 0, 0, time.UTC),
		},
		// Note: Database names with underscores will be fixed in Phase 2
		// with the new filename format (using -- separator)
		{
			name:     "database name with underscores - currently fails (will be fixed in Phase 2)",
			filename: "my_prod_db_2024-12-17_14-30-45.backup",
			wantErr:  true, // Currently expected to fail, will work in Phase 2 with new format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDateFromFilename(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractDateFromFilename() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ExtractDateFromFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}
