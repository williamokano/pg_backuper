package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/williamokano/pg_backuper/pkg/storage"
)

type Backend struct {
	name     string
	basePath string
}

func init() {
	storage.RegisterBackend("local", func(ctx context.Context, cfg storage.Config) (storage.Backend, error) {
		return New(cfg)
	})
}

// New creates a new local filesystem backend
func New(cfg storage.Config) (*Backend, error) {
	// Extract path from options
	pathVal, ok := cfg.Options["path"]
	if !ok {
		return nil, fmt.Errorf("missing required option: path")
	}

	path, ok := pathVal.(string)
	if !ok {
		return nil, fmt.Errorf("path must be a string")
	}

	// Use base_dir if path not in options
	if path == "" {
		path = cfg.BaseDir
	}

	// Ensure directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &Backend{
		name:     cfg.Name,
		basePath: path,
	}, nil
}

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return "local" }

// Write copies a file to the backend
func (b *Backend) Write(ctx context.Context, sourcePath, destPath string) error {
	destFullPath := filepath.Join(b.basePath, destPath)

	// Ensure destination directory exists
	destDir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return storage.WrapError(b.name, "write", err)
	}

	// Open source file
	source, err := os.Open(sourcePath)
	if err != nil {
		return storage.WrapError(b.name, "write", err)
	}
	defer source.Close()

	// Create destination file
	dest, err := os.Create(destFullPath)
	if err != nil {
		return storage.WrapError(b.name, "write", err)
	}
	defer dest.Close()

	// Copy data
	if _, err := io.Copy(dest, source); err != nil {
		os.Remove(destFullPath) // Clean up partial file
		return storage.WrapError(b.name, "write", err)
	}

	return nil
}

// Delete removes a file from the backend
func (b *Backend) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(b.basePath, path)
	if err := os.Remove(fullPath); err != nil {
		return storage.WrapError(b.name, "delete", err)
	}
	return nil
}

// List returns files matching the pattern
func (b *Backend) List(ctx context.Context, pattern string) ([]storage.FileInfo, error) {
	globPattern := filepath.Join(b.basePath, pattern)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, storage.WrapError(b.name, "list", err)
	}

	var files []storage.FileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we can't stat
		}

		// Skip 0-byte files (failed backups)
		if info.Size() == 0 {
			continue
		}

		// Get relative path
		relPath, err := filepath.Rel(b.basePath, match)
		if err != nil {
			relPath = filepath.Base(match)
		}

		files = append(files, storage.FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// Stat returns metadata about a file
func (b *Backend) Stat(ctx context.Context, path string) (*storage.FileInfo, error) {
	fullPath := filepath.Join(b.basePath, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, storage.ErrNotFound
		}
		return nil, storage.WrapError(b.name, "stat", err)
	}

	return &storage.FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// Exists checks if a file exists
func (b *Backend) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(b.basePath, path)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, storage.WrapError(b.name, "exists", err)
	}
	return true, nil
}

// Close is a no-op for local backend
func (b *Backend) Close() error {
	return nil
}
