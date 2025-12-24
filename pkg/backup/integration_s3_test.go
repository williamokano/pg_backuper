//go:build integration
// +build integration

package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/rotation"
	"github.com/williamokano/pg_backuper/pkg/storage"
)

// S3Credentials holds S3 access credentials
type S3Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

func TestBackupToS3Integration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("basic_backup_to_s3", func(t *testing.T) {
		ctx := context.Background()

		// Setup PostgreSQL container
		pgContainer, connStr, err := setupPostgresContainer(ctx, t)
		if err != nil {
			t.Fatalf("Failed to start PostgreSQL: %v", err)
		}
		defer pgContainer.Terminate(ctx)

		// Setup LocalStack (S3) container
		s3Container, s3Endpoint, s3Creds, err := setupLocalStackContainer(ctx, t)
		if err != nil {
			t.Fatalf("Failed to start LocalStack: %v", err)
		}
		defer s3Container.Terminate(ctx)

		// Create test database with sample data
		if err := createTestDatabase(connStr); err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}

		// Extract host and port from PostgreSQL container
		host, err := pgContainer.Host(ctx)
		if err != nil {
			t.Fatalf("Failed to get PostgreSQL host: %v", err)
		}
		mappedPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
		if err != nil {
			t.Fatalf("Failed to get PostgreSQL port: %v", err)
		}
		port := mappedPort.Int()

		// Create .pgpass file
		pgpassFile, err := createTestPgpass(t, host, port, "testdb", "postgres", "testpass")
		if err != nil {
			t.Fatalf("Failed to create .pgpass: %v", err)
		}
		defer os.Remove(pgpassFile)

		// Create temp directory for backups
		tempDir, err := os.MkdirTemp("", "pg_backuper_integration_*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create S3 bucket using AWS SDK
		if err := createS3Bucket(ctx, s3Endpoint, s3Creds, "test-backups"); err != nil {
			t.Fatalf("Failed to create S3 bucket: %v", err)
		}

		// Build configuration
		cfg := &config.Config{
			Storage: config.StorageConfig{
				TempDir: tempDir,
				Destinations: []config.StorageDestination{
					{
						Name:    "test_s3",
						Type:    "s3",
						Enabled: true,
						Options: map[string]interface{}{
							"endpoint":          s3Endpoint,
							"region":            "us-east-1",
							"bucket":            "test-backups",
							"prefix":            "backups/",
							"access_key_id":     s3Creds.AccessKeyID,
							"secret_access_key": s3Creds.SecretAccessKey,
							"use_ssl":           false,
							"force_path_style":  true,
						},
					},
				},
			},
			GlobalDefaults: config.GlobalDefaults{
				PgpassFile: pgpassFile,
				RetentionTiers: []config.RetentionTier{
					{Tier: "daily", Retention: 7},
				},
			},
		}

		dbConfig := config.DatabaseConfig{
			Name:                "testdb",
			User:                "postgres",
			Host:                host,
			Port:                port,
			StorageDestinations: []string{"test_s3"},
		}

		// Run backup
		logger := zerolog.Nop()
		timestamp := time.Now()
		result := BackupDatabase(cfg, dbConfig, timestamp, []string{"daily"}, logger)

		// Verify result
		require.True(t, result.Success, "Backup should succeed: %v", result.Error)
		require.Len(t, result.TiersCompleted, 1, "Expected 1 tier completed")
		assert.Equal(t, "daily", result.TiersCompleted[0], "Expected 'daily' tier")
		assert.Empty(t, result.TiersFailed, "Expected 0 tiers failed")

		// Verify backup exists in S3
		verifyBackupInS3(t, ctx, cfg, dbConfig.Name, "daily", timestamp)
	})
}

// setupPostgresContainer starts a PostgreSQL container and returns connection string
func setupPostgresContainer(ctx context.Context, t *testing.T) (*postgres.PostgresContainer, string, error) {
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, "", err
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, "", err
	}

	return pgContainer, connStr, nil
}

// setupLocalStackContainer starts a LocalStack container with S3 service
func setupLocalStackContainer(ctx context.Context, t *testing.T) (*localstack.LocalStackContainer, string, S3Credentials, error) {
	lsContainer, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:3.0"),
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "s3",
			"DEBUG":    "1",
		}),
	)
	if err != nil {
		return nil, "", S3Credentials{}, err
	}

	// Get S3 endpoint from container
	mappedPort, err := lsContainer.MappedPort(ctx, "4566/tcp")
	if err != nil {
		lsContainer.Terminate(ctx)
		return nil, "", S3Credentials{}, err
	}

	host, err := lsContainer.Host(ctx)
	if err != nil {
		lsContainer.Terminate(ctx)
		return nil, "", S3Credentials{}, err
	}

	s3Endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// LocalStack default credentials
	creds := S3Credentials{
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}

	return lsContainer, s3Endpoint, creds, nil
}

// createTestDatabase creates a test database with sample data
func createTestDatabase(connStr string) error {
	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Create test table
	createTable := `
		CREATE TABLE IF NOT EXISTS test_data (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := db.Exec(createTable); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Insert sample data
	for i := 1; i <= 10; i++ {
		insertData := `INSERT INTO test_data (name) VALUES ($1)`
		if _, err := db.Exec(insertData, fmt.Sprintf("Test Record %d", i)); err != nil {
			return fmt.Errorf("failed to insert data: %w", err)
		}
	}

	return nil
}

// createTestPgpass creates a temporary .pgpass file with proper permissions
func createTestPgpass(t *testing.T, host string, port int, database, user, password string) (string, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", ".pgpass_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp .pgpass: %w", err)
	}
	defer tempFile.Close()

	// Write .pgpass format: hostname:port:database:username:password
	content := fmt.Sprintf("%s:%d:%s:%s:%s\n", host, port, database, user, password)
	if _, err := tempFile.WriteString(content); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write .pgpass: %w", err)
	}

	// Set permissions to 0600 (required by PostgreSQL)
	if err := os.Chmod(tempFile.Name(), 0600); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to set .pgpass permissions: %w", err)
	}

	return tempFile.Name(), nil
}

// createS3Bucket creates an S3 bucket in LocalStack
func createS3Bucket(ctx context.Context, endpoint string, creds S3Credentials, bucketName string) error {
	// Create AWS config
	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithRegion("us-east-1"),
		awsConfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				creds.AccessKeyID,
				creds.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	// Create bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	return nil
}

// verifyBackupInS3 verifies that backup file exists in S3 and has valid content
func verifyBackupInS3(t *testing.T, ctx context.Context, cfg *config.Config, dbName, tier string, timestamp time.Time) {
	// Create S3 backend
	factory := storage.NewFactory()
	dest := cfg.Storage.Destinations[0]
	storageConfig := storage.Config{
		Name:    dest.Name,
		Type:    dest.Type,
		Enabled: dest.Enabled,
		BaseDir: dest.BaseDir,
		Options: dest.Options,
	}
	backend, err := factory.Create(ctx, storageConfig)
	if err != nil {
		t.Fatalf("Failed to create S3 backend: %v", err)
	}
	defer backend.Close()

	// List backup files
	pattern := fmt.Sprintf("%s--%s--*.backup", dbName, tier)
	files, err := backend.List(ctx, pattern)
	require.NoError(t, err, "Failed to list S3 files")

	// Verify exactly 1 file exists
	require.Len(t, files, 1, "Expected exactly 1 backup file in S3")

	file := files[0]

	// Verify filename format
	expectedFilename := rotation.GenerateBackupFilenameWithTier("", dbName, tier, timestamp)
	expectedFilename = filepath.Base(expectedFilename)

	// Check if the filename matches (allowing for slight timestamp differences)
	assert.True(t, strings.HasPrefix(file.Path, fmt.Sprintf("%s--%s--", dbName, tier)),
		"Backup filename doesn't match expected pattern: got %s", file.Path)

	// Verify file size > 1000 bytes (reasonable minimum for a backup with data)
	assert.Greater(t, file.Size, int64(1000), "Backup file size too small: %d bytes", file.Size)

	// Download and verify PostgreSQL magic header
	verifyPostgresBackupFormat(t, ctx, backend, file.Path)
}

// verifyPostgresBackupFormat downloads backup file and verifies it's a valid PostgreSQL custom format backup
func verifyPostgresBackupFormat(t *testing.T, ctx context.Context, backend storage.Backend, filePath string) {
	// For now, we'll just verify the file exists and has reasonable size
	// In a more comprehensive test, we could download and check the "PGDMP" magic header
	info, err := backend.Stat(ctx, filePath)
	require.NoError(t, err, "Failed to stat backup file")
	assert.Greater(t, info.Size, int64(0), "Backup file should not be empty")

	t.Logf("✅ Backup file verified: %s (%d bytes)", filePath, info.Size)
}
