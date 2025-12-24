package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/williamokano/pg_backuper/pkg/storage"
	"github.com/williamokano/pg_backuper/pkg/storage/mocks"
)

func TestMultiUploader_Upload(t *testing.T) {
	t.Run("single_backend_success", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)
		mockBackend.On("Name").Return("backend1")
		mockBackend.On("Type").Return("s3")
		mockBackend.On("Write",
			mock.Anything,
			"/tmp/backup.tmp",
			"backup.backup",
		).Return(nil).Once()

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, []storage.Backend{mockBackend}, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 1)
		assert.True(t, results[0].Success)
		assert.Equal(t, "backend1", results[0].BackendName)
		assert.Equal(t, "s3", results[0].BackendType)
		assert.NoError(t, results[0].Error)
		assert.Greater(t, results[0].Duration, time.Duration(0))
	})

	t.Run("single_backend_failure", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)
		mockBackend.On("Name").Return("backend1")
		mockBackend.On("Type").Return("s3")
		mockBackend.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(storage.ErrConnFailed).Once()

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, []storage.Backend{mockBackend}, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 1)
		assert.False(t, results[0].Success)
		assert.Equal(t, "backend1", results[0].BackendName)
		assert.ErrorIs(t, results[0].Error, storage.ErrConnFailed)
	})

	t.Run("multiple_backends_all_succeed", func(t *testing.T) {
		// Create 3 mock backends
		backends := make([]storage.Backend, 3)
		for i := 0; i < 3; i++ {
			mockBackend := mocks.NewMockBackend(t)
			mockBackend.On("Name").Return("backend" + string(rune('1'+i)))
			mockBackend.On("Type").Return("mock")
			mockBackend.On("Write",
				mock.Anything,
				"/tmp/backup.tmp",
				"backup.backup",
			).Return(nil).Once()
			backends[i] = mockBackend
		}

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, backends, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 3)
		for _, result := range results {
			assert.True(t, result.Success, "Backend %s should succeed", result.BackendName)
			assert.NoError(t, result.Error)
		}
	})

	t.Run("multiple_backends_partial_failure", func(t *testing.T) {
		// Create 3 mock backends: 2 succeed, 1 fails
		backend1 := mocks.NewMockBackend(t)
		backend1.On("Name").Return("backend1")
		backend1.On("Type").Return("s3")
		backend1.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Once()

		backend2 := mocks.NewMockBackend(t)
		backend2.On("Name").Return("backend2")
		backend2.On("Type").Return("ssh")
		backend2.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(storage.ErrConnFailed).Once()

		backend3 := mocks.NewMockBackend(t)
		backend3.On("Name").Return("backend3")
		backend3.On("Type").Return("local")
		backend3.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil).Once()

		backends := []storage.Backend{backend1, backend2, backend3}

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, backends, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 3)

		// Count successes and failures
		successCount := 0
		failureCount := 0
		for _, result := range results {
			if result.Success {
				successCount++
			} else {
				failureCount++
			}
		}

		assert.Equal(t, 2, successCount, "Expected 2 backends to succeed")
		assert.Equal(t, 1, failureCount, "Expected 1 backend to fail")
	})

	t.Run("multiple_backends_all_fail", func(t *testing.T) {
		// Create 3 mock backends, all fail
		backends := make([]storage.Backend, 3)
		for i := 0; i < 3; i++ {
			mockBackend := mocks.NewMockBackend(t)
			mockBackend.On("Name").Return("backend" + string(rune('1'+i)))
			mockBackend.On("Type").Return("mock")
			mockBackend.On("Write",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(errors.New("write failed")).Once()
			backends[i] = mockBackend
		}

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, backends, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 3)
		for _, result := range results {
			assert.False(t, result.Success, "Backend %s should fail", result.BackendName)
			assert.Error(t, result.Error)
		}
	})

	t.Run("empty_backends_list", func(t *testing.T) {
		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute with empty backends
		ctx := context.Background()
		results := uploader.Upload(ctx, []storage.Backend{}, "/tmp/backup.tmp", "backup.backup")

		// Verify
		assert.Empty(t, results, "Should return empty results for empty backends")
	})

	t.Run("parallel_execution", func(t *testing.T) {
		// Create backends with artificial delays to test parallelism
		backend1 := mocks.NewMockBackend(t)
		backend1.On("Name").Return("slow1")
		backend1.On("Type").Return("mock")
		backend1.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).Return(nil).Once()

		backend2 := mocks.NewMockBackend(t)
		backend2.On("Name").Return("slow2")
		backend2.On("Type").Return("mock")
		backend2.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond)
		}).Return(nil).Once()

		backends := []storage.Backend{backend1, backend2}

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute and measure time
		ctx := context.Background()
		start := time.Now()
		results := uploader.Upload(ctx, backends, "/tmp/backup.tmp", "backup.backup")
		elapsed := time.Since(start)

		// Verify parallel execution: should take ~100ms, not ~200ms
		// Allow some overhead for goroutine scheduling
		assert.Less(t, elapsed, 150*time.Millisecond, "Uploads should run in parallel")
		require.Len(t, results, 2)
		for _, result := range results {
			assert.True(t, result.Success)
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		// Create mock backend that respects context
		mockBackend := mocks.NewMockBackend(t)
		mockBackend.On("Name").Return("backend1")
		mockBackend.On("Type").Return("mock")
		mockBackend.On("Write",
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(context.Canceled).Once()

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Create canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Execute
		results := uploader.Upload(ctx, []storage.Backend{mockBackend}, "/tmp/backup.tmp", "backup.backup")

		// Verify
		require.Len(t, results, 1)
		assert.False(t, results[0].Success)
		assert.ErrorIs(t, results[0].Error, context.Canceled)
	})

	t.Run("verify_result_fields", func(t *testing.T) {
		// Create mock backend
		mockBackend := mocks.NewMockBackend(t)
		mockBackend.On("Name").Return("test-backend")
		mockBackend.On("Type").Return("test-type")
		mockBackend.On("Write",
			mock.Anything,
			"/source/path.tmp",
			"dest/path.backup",
		).Return(nil).Once()

		// Create uploader
		uploader := storage.NewMultiUploader(zerolog.Nop())

		// Execute
		ctx := context.Background()
		results := uploader.Upload(ctx, []storage.Backend{mockBackend}, "/source/path.tmp", "dest/path.backup")

		// Verify all result fields
		require.Len(t, results, 1)
		result := results[0]

		assert.Equal(t, "test-backend", result.BackendName, "BackendName should match")
		assert.Equal(t, "test-type", result.BackendType, "BackendType should match")
		assert.True(t, result.Success, "Success should be true")
		assert.NoError(t, result.Error, "Error should be nil")
		assert.Greater(t, result.Duration, time.Duration(0), "Duration should be positive")
	})
}

func TestNewMultiUploader(t *testing.T) {
	t.Run("creates_uploader", func(t *testing.T) {
		logger := zerolog.Nop()
		uploader := storage.NewMultiUploader(logger)

		assert.NotNil(t, uploader)
	})
}
