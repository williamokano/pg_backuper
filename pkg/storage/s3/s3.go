package s3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/williamokano/pg_backuper/pkg/storage"
)

type Backend struct {
	name     string
	client   *s3.Client
	bucket   string
	prefix   string
	uploader *manager.Uploader
}

func init() {
	storage.RegisterBackend("s3", func(ctx context.Context, cfg storage.Config) (storage.Backend, error) {
		return New(ctx, cfg)
	})
}

// New creates a new S3 backend
func New(ctx context.Context, cfg storage.Config) (*Backend, error) {
	// Extract S3 config from options
	s3Cfg, err := parseConfig(cfg.Options)
	if err != nil {
		return nil, err
	}

	// Build AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(s3Cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				s3Cfg.AccessKeyID,
				s3Cfg.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, storage.WrapError(cfg.Name, "init", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if s3Cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(s3Cfg.Endpoint)
		}
		o.UsePathStyle = s3Cfg.ForcePathStyle
	})

	// Test connection
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s3Cfg.Bucket),
	})
	if err != nil {
		return nil, storage.WrapError(cfg.Name, "connection test", storage.ErrConnFailed)
	}

	return &Backend{
		name:     cfg.Name,
		client:   client,
		bucket:   s3Cfg.Bucket,
		prefix:   strings.TrimPrefix(s3Cfg.Prefix, "/"),
		uploader: manager.NewUploader(client),
	}, nil
}

func (b *Backend) Name() string { return b.name }
func (b *Backend) Type() string { return "s3" }

// Write uploads a file to S3
func (b *Backend) Write(ctx context.Context, sourcePath, destPath string) error {
	return storage.WithRetry(ctx, storage.DefaultRetryConfig(), func() error {
		// Open source file
		file, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer file.Close()

		// Build S3 key
		key := path.Join(b.prefix, destPath)

		// Upload
		_, err = b.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(b.bucket),
			Key:    aws.String(key),
			Body:   file,
		})

		if err != nil {
			return storage.WrapError(b.name, "upload", err)
		}

		return nil
	})
}

// Delete removes an object from S3
func (b *Backend) Delete(ctx context.Context, objectPath string) error {
	key := path.Join(b.prefix, objectPath)

	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return storage.WrapError(b.name, "delete", err)
	}

	return nil
}

// List returns objects matching the pattern
func (b *Backend) List(ctx context.Context, pattern string) ([]storage.FileInfo, error) {
	// Convert glob pattern to prefix
	// For "dbname--*.backup", use "dbname--" as prefix
	prefix := extractPrefix(pattern)
	fullPrefix := path.Join(b.prefix, prefix)

	var files []storage.FileInfo

	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, storage.WrapError(b.name, "list", err)
		}

		for _, obj := range page.Contents {
			// Get relative path (remove prefix)
			relPath := strings.TrimPrefix(*obj.Key, b.prefix)
			relPath = strings.TrimPrefix(relPath, "/")

			// Filter by glob pattern
			if !matchesGlob(relPath, pattern) {
				continue
			}

			// Skip 0-byte objects
			if *obj.Size == 0 {
				continue
			}

			files = append(files, storage.FileInfo{
				Path:    relPath,
				Size:    *obj.Size,
				ModTime: *obj.LastModified,
			})
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// Stat returns metadata about an object
func (b *Backend) Stat(ctx context.Context, objectPath string) (*storage.FileInfo, error) {
	key := path.Join(b.prefix, objectPath)

	result, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, storage.WrapError(b.name, "stat", err)
	}

	return &storage.FileInfo{
		Path:    objectPath,
		Size:    *result.ContentLength,
		ModTime: *result.LastModified,
	}, nil
}

// Exists checks if an object exists
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

// Close is a no-op for S3
func (b *Backend) Close() error {
	return nil
}

// Helper functions

func parseConfig(options map[string]interface{}) (*Config, error) {
	cfg := &Config{
		UseSSL:         true, // Default
		ForcePathStyle: false,
	}

	// Extract each field from options map
	if v, ok := options["endpoint"].(string); ok {
		cfg.Endpoint = v
	}
	if v, ok := options["region"].(string); ok {
		cfg.Region = v
	} else {
		return nil, fmt.Errorf("missing required option: region")
	}
	if v, ok := options["bucket"].(string); ok {
		cfg.Bucket = v
	} else {
		return nil, fmt.Errorf("missing required option: bucket")
	}
	if v, ok := options["prefix"].(string); ok {
		cfg.Prefix = v
	}
	if v, ok := options["access_key_id"].(string); ok {
		cfg.AccessKeyID = v
	} else {
		return nil, fmt.Errorf("missing required option: access_key_id")
	}
	if v, ok := options["secret_access_key"].(string); ok {
		cfg.SecretAccessKey = v
	} else {
		return nil, fmt.Errorf("missing required option: secret_access_key")
	}
	if v, ok := options["use_ssl"].(bool); ok {
		cfg.UseSSL = v
	}
	if v, ok := options["force_path_style"].(bool); ok {
		cfg.ForcePathStyle = v
	}

	return cfg, nil
}

func extractPrefix(pattern string) string {
	// Extract prefix before first wildcard
	// "dbname--*.backup" -> "dbname--"
	// "dbname--hourly--*.backup" -> "dbname--hourly--"
	if idx := strings.Index(pattern, "*"); idx >= 0 {
		return pattern[:idx]
	}
	return pattern
}

func matchesGlob(path, pattern string) bool {
	// Simple glob matching for *.backup patterns
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
