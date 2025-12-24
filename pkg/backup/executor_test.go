package backup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/williamokano/pg_backuper/pkg/storage"
	"github.com/williamokano/pg_backuper/pkg/storage/mocks"
)

// TestBackendMocking demonstrates how to use mocked backends
// Note: Full integration testing of BackupDatabase requires refactoring
// to support dependency injection of backends
func TestBackendMocking(t *testing.T) {
	t.Run("mock_backend_write_success", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)
		mockBackend.On("Name").Return("test_backend")
		mockBackend.On("Type").Return("mock")

		// Setup expectation: Write should be called once
		mockBackend.On("Write",
			mock.Anything, // ctx
			"/tmp/backup.tmp", // source
			"backup.backup", // dest
		).Return(nil).Once()

		// Execute
		ctx := context.Background()
		err := mockBackend.Write(ctx, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.NoError(t, err)
		assert.Equal(t, "test_backend", mockBackend.Name())
		assert.Equal(t, "mock", mockBackend.Type())
	})

	t.Run("mock_backend_write_failure", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: Write should fail
		mockBackend.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(storage.ErrConnFailed).Once()

		// Execute
		ctx := context.Background()
		err := mockBackend.Write(ctx, "/tmp/backup.tmp", "backup.backup")

		// Verify
		assert.ErrorIs(t, err, storage.ErrConnFailed)
	})

	t.Run("mock_backend_list", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: List should return files
		expectedFiles := []storage.FileInfo{
			{
				Path:    "db--daily--2025-12-20T10-00-00.backup",
				Size:    1024,
				ModTime: time.Now(),
			},
			{
				Path:    "db--daily--2025-12-19T10-00-00.backup",
				Size:    2048,
				ModTime: time.Now().Add(-24 * time.Hour),
			},
		}

		mockBackend.On("List",
			mock.Anything,
			"db--daily--*.backup",
		).Return(expectedFiles, nil).Once()

		// Execute
		ctx := context.Background()
		files, err := mockBackend.List(ctx, "db--daily--*.backup")

		// Verify
		require.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Equal(t, "db--daily--2025-12-20T10-00-00.backup", files[0].Path)
		assert.Equal(t, int64(1024), files[0].Size)
	})

	t.Run("mock_backend_delete", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: Delete should be called for old backups
		mockBackend.On("Delete",
			mock.Anything,
			"db--daily--2025-11-01T10-00-00.backup",
		).Return(nil).Once()

		// Execute
		ctx := context.Background()
		err := mockBackend.Delete(ctx, "db--daily--2025-11-01T10-00-00.backup")

		// Verify
		require.NoError(t, err)
	})

	t.Run("mock_backend_stat", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: Stat should return file info
		expectedInfo := &storage.FileInfo{
			Path:    "db--daily--2025-12-20T10-00-00.backup",
			Size:    2922,
			ModTime: time.Now(),
		}

		mockBackend.On("Stat",
			mock.Anything,
			"db--daily--2025-12-20T10-00-00.backup",
		).Return(expectedInfo, nil).Once()

		// Execute
		ctx := context.Background()
		info, err := mockBackend.Stat(ctx, "db--daily--2025-12-20T10-00-00.backup")

		// Verify
		require.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, int64(2922), info.Size)
		assert.Equal(t, "db--daily--2025-12-20T10-00-00.backup", info.Path)
	})

	t.Run("mock_backend_exists", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: File exists
		mockBackend.On("Exists",
			mock.Anything,
			"db--daily--2025-12-20T10-00-00.backup",
		).Return(true, nil).Once()

		// Execute
		ctx := context.Background()
		exists, err := mockBackend.Exists(ctx, "db--daily--2025-12-20T10-00-00.backup")

		// Verify
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("mock_backend_close", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)

		// Setup expectation: Close should be called
		mockBackend.On("Close").Return(nil).Once()

		// Execute
		err := mockBackend.Close()

		// Verify
		require.NoError(t, err)
	})
}

// TestBackupResult tests the Result struct
func TestBackupResult(t *testing.T) {
	t.Run("successful_result", func(t *testing.T) {
		result := Result{
			Database:       "testdb",
			Success:        true,
			Skipped:        false,
			TiersCompleted: []string{"daily", "hourly"},
			TiersFailed:    []string{},
			BackendResults: make(map[string][]storage.Result),
			Error:          nil,
			Duration:       5 * time.Second,
		}

		assert.True(t, result.Success)
		assert.False(t, result.Skipped)
		assert.Len(t, result.TiersCompleted, 2)
		assert.Empty(t, result.TiersFailed)
		assert.NoError(t, result.Error)
		assert.Equal(t, 5*time.Second, result.Duration)
	})

	t.Run("failed_result", func(t *testing.T) {
		result := Result{
			Database:       "testdb",
			Success:        false,
			Skipped:        false,
			TiersCompleted: []string{"daily"},
			TiersFailed:    []string{"hourly"},
			BackendResults: make(map[string][]storage.Result),
			Error:          assert.AnError,
			Duration:       3 * time.Second,
		}

		assert.False(t, result.Success)
		assert.False(t, result.Skipped)
		assert.Len(t, result.TiersCompleted, 1)
		assert.Len(t, result.TiersFailed, 1)
		assert.Error(t, result.Error)
	})

	t.Run("skipped_result", func(t *testing.T) {
		result := Result{
			Database:       "testdb",
			Success:        true,
			Skipped:        true,
			TiersCompleted: []string{},
			TiersFailed:    []string{},
			BackendResults: make(map[string][]storage.Result),
			Error:          nil,
			Duration:       100 * time.Millisecond,
		}

		assert.True(t, result.Success)
		assert.True(t, result.Skipped)
		assert.Empty(t, result.TiersCompleted)
		assert.Empty(t, result.TiersFailed)
		assert.NoError(t, result.Error)
	})
}

// TestInitializeBackends tests backend initialization logic
// Note: This tests the current implementation with config-based initialization
func TestInitializeBackends(t *testing.T) {
	t.Run("no_destinations_with_backup_dir", func(t *testing.T) {
		// This test would require actual filesystem and is better suited for integration tests
		// Skipping for unit test suite
		t.Skip("Requires filesystem access - use integration test instead")
	})

	t.Run("no_destinations_no_backup_dir", func(t *testing.T) {
		// This would test error handling but requires refactoring initializeBackends
		// to be testable without side effects
		t.Skip("Requires refactoring for testability")
	})
}

// Note: Full unit testing of BackupDatabase requires refactoring to support
// dependency injection. Current implementation couples pg_dump execution,
// file I/O, and backend initialization, making it difficult to unit test
// without significant changes.
//
// Recommended refactoring for Phase 3+:
// 1. Extract pg_dump execution into an interface (Dumper)
// 2. Inject storage backends instead of initializing them internally
// 3. Extract file I/O operations into testable helpers
//
// Example refactored signature:
// func BackupDatabase(
//     dumper Dumper,
//     backends []storage.Backend,
//     cfg *config.Config,
//     db config.DatabaseConfig,
//     timestamp time.Time,
//     dueTiers []string,
//     logger zerolog.Logger,
// ) Result
