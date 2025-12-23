package storage

import (
	"context"
	"time"
)

// Backend represents a storage backend that can store and manage backup files
type Backend interface {
	// Name returns a human-readable name for this backend (e.g., "local_primary", "s3_offsite")
	Name() string

	// Type returns the backend type (local, s3, backblaze, ssh)
	Type() string

	// Write uploads a file from local filesystem to the backend
	// sourcePath: absolute path to local file
	// destPath: relative path in backend (e.g., "dbname--hourly--2024-12-19.backup")
	Write(ctx context.Context, sourcePath string, destPath string) error

	// Delete removes a file from the backend
	// path: relative path in backend
	Delete(ctx context.Context, path string) error

	// List returns all files matching the pattern
	// pattern: glob pattern (e.g., "mydb*.backup", "mydb--hourly--*.backup")
	// Returns files sorted by modification time (newest first)
	List(ctx context.Context, pattern string) ([]FileInfo, error)

	// Stat returns metadata about a specific file
	// path: relative path in backend
	Stat(ctx context.Context, path string) (*FileInfo, error)

	// Exists checks if a file exists in the backend
	Exists(ctx context.Context, path string) (bool, error)

	// Close releases resources (connections, sessions)
	Close() error
}

// FileInfo represents metadata about a stored file
type FileInfo struct {
	Path    string    // Relative path in backend
	Size    int64     // Size in bytes
	ModTime time.Time // Last modification time
}

// Config represents storage backend configuration
type Config struct {
	Name    string                 `json:"name"`    // User-friendly name (e.g., "s3_primary")
	Type    string                 `json:"type"`    // Backend type: local, s3, backblaze, ssh
	Enabled bool                   `json:"enabled"` // Whether this backend is active
	BaseDir string                 `json:"base_dir"` // Base directory/prefix for backups
	Options map[string]interface{} `json:"options"` // Backend-specific options
}

// Result represents outcome of a storage operation
type Result struct {
	BackendName string
	BackendType string
	Success     bool
	Error       error
	Duration    time.Duration
}
