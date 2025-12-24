package rotation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDateFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     time.Time
		wantErr  bool
	}{
		// Old format tests
		{
			name:     "old format - simple database name",
			filename: "mydb_2024-12-17_03-00-00.backup",
			want:     time.Date(2024, 12, 17, 3, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "old format - with full path",
			filename: "/backups/database_2024-01-01_00-00-00.backup",
			want:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "old format - database with underscore (works with last-two-parts approach)",
			filename: "my_prod_db_2024-12-17_14-30-45.backup",
			want:     time.Date(2024, 12, 17, 14, 30, 45, 0, time.UTC),
			wantErr:  false, // Actually works because we take the last two parts
		},

		// New format tests
		{
			name:     "new format - simple database name",
			filename: "mydb--2024-12-17T03-00-00.backup",
			want:     time.Date(2024, 12, 17, 3, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "new format - database with underscores (WORKS)",
			filename: "my_prod_db--2024-12-17T14-30-45.backup",
			want:     time.Date(2024, 12, 17, 14, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "new format - database with multiple underscores",
			filename: "my_very_long_db_name--2024-12-17T14-30-45.backup",
			want:     time.Date(2024, 12, 17, 14, 30, 45, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "new format - with full path",
			filename: "/backups/prod_db--2024-01-01T00-00-00.backup",
			want:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "new format - database with dashes",
			filename: "my-prod-db--2024-12-17T14-30-45.backup",
			want:     time.Date(2024, 12, 17, 14, 30, 45, 0, time.UTC),
			wantErr:  false,
		},

		// Invalid formats
		{
			name:     "invalid - no separator",
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
			name:     "new format - invalid date",
			filename: "mydb--invalid-date.backup",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractDateFromFilename(tt.filename)
			if tt.wantErr {
				assert.Error(t, err, "Expected an error but got none")
			} else {
				require.NoError(t, err)
				assert.True(t, got.Equal(tt.want), "ExtractDateFromFilename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateBackupFilename(t *testing.T) {
	timestamp := time.Date(2024, 12, 17, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name      string
		backupDir string
		dbName    string
		want      string
	}{
		{
			name:      "simple database name",
			backupDir: "/backups",
			dbName:    "mydb",
			want:      "/backups/mydb--2024-12-17T14-30-45.backup",
		},
		{
			name:      "database with underscores",
			backupDir: "/backups",
			dbName:    "my_prod_db",
			want:      "/backups/my_prod_db--2024-12-17T14-30-45.backup",
		},
		{
			name:      "database with dashes",
			backupDir: "/backups",
			dbName:    "my-prod-db",
			want:      "/backups/my-prod-db--2024-12-17T14-30-45.backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateBackupFilename(tt.backupDir, tt.dbName, timestamp)
			assert.Equal(t, tt.want, got, "GenerateBackupFilename() mismatch")

			// Verify the generated filename can be parsed back
			parsedTime, err := ExtractDateFromFilename(got)
			require.NoError(t, err, "Generated filename cannot be parsed")
			assert.True(t, parsedTime.Equal(timestamp), "Parsed time %v doesn't match original %v", parsedTime, timestamp)
		})
	}
}
