package backblaze

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/kurin/blazer/b2"

	"github.com/williamokano/pg_backuper/pkg/storage"
)

type Backend struct {
	name   string
	client *b2.Client
	bucket *b2.Bucket
	prefix string
}

func init() {
	storage.RegisterBackend("backblaze", func(ctx context.Context, cfg storage.Config) (storage.Backend, error) {
		return New(ctx, cfg)
	})
}

// New creates a new Backblaze B2 backend
func New(ctx context.Context, cfg storage.Config) (*Backend, error) {
	b2Cfg, err := parseConfig(cfg.Options)
	if err != nil {
		return nil, err
	}

	// Create B2 client
	client, err := b2.NewClient(ctx, b2Cfg.AccountID, b2Cfg.ApplicationKey)
	if err != nil {
		return nil, storage.WrapError(cfg.Name, "init", storage.ErrAuthFailed)
	}

	// Get bucket
	bucket, err := client.Bucket(ctx, b2Cfg.BucketName)
	if err != nil {
		return nil, storage.WrapError(cfg.Name, "get bucket", err)
	}

	return &Backend{
		name:   cfg.Name,
		client: client,
		bucket: bucket,
		prefix: strings.TrimPrefix(b2Cfg.Prefix, "/"),
	}, nil
}

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return "backblaze" }

// Write uploads a file to B2
func (b *Backend) Write(ctx context.Context, sourcePath, destPath string) error {
	return storage.WithRetry(ctx, storage.DefaultRetryConfig(), func() error {
		file, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer file.Close()

		key := path.Join(b.prefix, destPath)

		obj := b.bucket.Object(key)
		writer := obj.NewWriter(ctx)

		if _, err := io.Copy(writer, file); err != nil {
			writer.Close()
			return storage.WrapError(b.name, "upload", err)
		}

		if err := writer.Close(); err != nil {
			return storage.WrapError(b.name, "upload", err)
		}

		return nil
	})
}

// Delete removes a file from B2
func (b *Backend) Delete(ctx context.Context, objectPath string) error {
	key := path.Join(b.prefix, objectPath)
	obj := b.bucket.Object(key)

	if err := obj.Delete(ctx); err != nil {
		return storage.WrapError(b.name, "delete", err)
	}

	return nil
}

// List returns objects matching pattern
func (b *Backend) List(ctx context.Context, pattern string) ([]storage.FileInfo, error) {
	prefix := extractPrefix(pattern)
	fullPrefix := path.Join(b.prefix, prefix)

	var files []storage.FileInfo

	iter := b.bucket.List(ctx, b2.ListPrefix(fullPrefix))
	for iter.Next() {
		obj := iter.Object()

		// Get relative path
		relPath := strings.TrimPrefix(obj.Name(), b.prefix)
		relPath = strings.TrimPrefix(relPath, "/")

		// Filter by pattern
		if !matchesGlob(relPath, pattern) {
			continue
		}

		attrs, err := obj.Attrs(ctx)
		if err != nil {
			continue
		}

		// Skip 0-byte files
		if attrs.Size == 0 {
			continue
		}

		files = append(files, storage.FileInfo{
			Path:    relPath,
			Size:    attrs.Size,
			ModTime: attrs.UploadTimestamp,
		})
	}

	if err := iter.Err(); err != nil {
		return nil, storage.WrapError(b.name, "list", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// Stat returns file metadata
func (b *Backend) Stat(ctx context.Context, objectPath string) (*storage.FileInfo, error) {
	key := path.Join(b.prefix, objectPath)
	obj := b.bucket.Object(key)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, storage.WrapError(b.name, "stat", err)
	}

	return &storage.FileInfo{
		Path:    objectPath,
		Size:    attrs.Size,
		ModTime: attrs.UploadTimestamp,
	}, nil
}

// Exists checks if object exists
func (b *Backend) Exists(ctx context.Context, objectPath string) (bool, error) {
	_, err := b.Stat(ctx, objectPath)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Close releases resources
func (b *Backend) Close() error {
	return nil
}

func parseConfig(options map[string]interface{}) (*Config, error) {
	cfg := &Config{}

	if v, ok := options["account_id"].(string); ok {
		cfg.AccountID = v
	} else {
		return nil, fmt.Errorf("missing required option: account_id")
	}
	if v, ok := options["application_key"].(string); ok {
		cfg.ApplicationKey = v
	} else {
		return nil, fmt.Errorf("missing required option: application_key")
	}
	if v, ok := options["bucket_name"].(string); ok {
		cfg.BucketName = v
	} else {
		return nil, fmt.Errorf("missing required option: bucket_name")
	}
	if v, ok := options["bucket_id"].(string); ok {
		cfg.BucketID = v
	}
	if v, ok := options["prefix"].(string); ok {
		cfg.Prefix = v
	}

	return cfg, nil
}

func extractPrefix(pattern string) string {
	if idx := strings.Index(pattern, "*"); idx >= 0 {
		return pattern[:idx]
	}
	return pattern
}

func matchesGlob(path, pattern string) bool {
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(path, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
	}
	return path == pattern
}
