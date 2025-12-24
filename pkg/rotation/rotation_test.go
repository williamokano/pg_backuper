package rotation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/williamokano/pg_backuper/pkg/config"
	"github.com/williamokano/pg_backuper/pkg/rotation"
	"github.com/williamokano/pg_backuper/pkg/storage"
	"github.com/williamokano/pg_backuper/pkg/storage/mocks"
)

func TestApplyRetentionWithBackend_DailyRotation(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// Simulate 10 existing daily backups
	now := time.Now()
	existingBackups := make([]storage.FileInfo, 10)
	for i := 0; i < 10; i++ {
		backupTime := now.Add(-time.Duration(i*24) * time.Hour)
		existingBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--daily--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    1024,
			ModTime: backupTime,
		}
	}

	// Expect List to be called
	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(existingBackups, nil).
		Once()

	// Expect Delete to be called for backups 7, 8, 9 (keep only 7)
	for i := 7; i < 10; i++ {
		mockBackend.On("Delete", ctx, existingBackups[i].Path).
			Return(nil).
			Once()
	}

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
	// Mock expectations verified automatically
}

func TestApplyRetentionWithBackend_HourlyRotation(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// Simulate 10 hourly backups
	now := time.Now()
	existingBackups := make([]storage.FileInfo, 10)
	for i := 0; i < 10; i++ {
		backupTime := now.Add(-time.Duration(i) * time.Hour)
		existingBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--hourly--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    512,
			ModTime: backupTime,
		}
	}

	// Expect List to be called
	mockBackend.On("List", ctx, "testdb--hourly--*.backup").
		Return(existingBackups, nil).
		Once()

	// Expect Delete to be called for backups 6-9 (keep only 6)
	for i := 6; i < 10; i++ {
		mockBackend.On("Delete", ctx, existingBackups[i].Path).
			Return(nil).
			Once()
	}

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "hourly", Retention: 6},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
}

func TestApplyRetentionWithBackend_MultipleTiers(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend").Maybe()
	mockBackend.On("Type").Return("mock").Maybe()

	now := time.Now()

	// Setup daily backups (10 backups, keep 7)
	dailyBackups := make([]storage.FileInfo, 10)
	for i := 0; i < 10; i++ {
		backupTime := now.Add(-time.Duration(i*24) * time.Hour)
		dailyBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--daily--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    1024,
			ModTime: backupTime,
		}
	}

	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(dailyBackups, nil).
		Once()

	for i := 7; i < 10; i++ {
		mockBackend.On("Delete", ctx, dailyBackups[i].Path).
			Return(nil).
			Once()
	}

	// Setup weekly backups (5 backups, keep 4)
	weeklyBackups := make([]storage.FileInfo, 5)
	for i := 0; i < 5; i++ {
		backupTime := now.Add(-time.Duration(i*7*24) * time.Hour)
		weeklyBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--weekly--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    2048,
			ModTime: backupTime,
		}
	}

	mockBackend.On("List", ctx, "testdb--weekly--*.backup").
		Return(weeklyBackups, nil).
		Once()

	mockBackend.On("Delete", ctx, weeklyBackups[4].Path).
		Return(nil).
		Once()

	// Execute rotation with multiple tiers
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
		{Tier: "weekly", Retention: 4},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
}

func TestApplyRetentionWithBackend_NoBackupsToDelete(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// Only 3 backups (less than retention of 7)
	now := time.Now()
	existingBackups := make([]storage.FileInfo, 3)
	for i := 0; i < 3; i++ {
		backupTime := now.Add(-time.Duration(i*24) * time.Hour)
		existingBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--daily--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    1024,
			ModTime: backupTime,
		}
	}

	// Expect List to be called
	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(existingBackups, nil).
		Once()

	// No Delete calls expected

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
}

func TestApplyRetentionWithBackend_EmptyBackupList(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// No backups found
	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return([]storage.FileInfo{}, nil).
		Once()

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
}

func TestApplyRetentionWithBackend_ListError(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// List returns error
	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(nil, storage.ErrConnFailed).
		Once()

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list files for tier daily")
}

func TestApplyRetentionWithBackend_DeleteError(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// 10 backups
	now := time.Now()
	existingBackups := make([]storage.FileInfo, 10)
	for i := 0; i < 10; i++ {
		backupTime := now.Add(-time.Duration(i*24) * time.Hour)
		existingBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--daily--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    1024,
			ModTime: backupTime,
		}
	}

	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(existingBackups, nil).
		Once()

	// First delete succeeds
	mockBackend.On("Delete", ctx, existingBackups[7].Path).
		Return(nil).
		Once()

	// Second delete fails
	mockBackend.On("Delete", ctx, existingBackups[8].Path).
		Return(storage.ErrPermissionDenied).
		Once()

	// Third delete succeeds
	mockBackend.On("Delete", ctx, existingBackups[9].Path).
		Return(nil).
		Once()

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions - function doesn't fail on individual delete errors
	assert.NoError(t, err, "ApplyRetentionWithBackend should not fail on individual delete errors")
}

func TestApplyRetentionWithBackend_ExactRetentionCount(t *testing.T) {
	ctx := context.Background()

	// Create mock backend
	mockBackend := mocks.NewMockBackend(t)
	mockBackend.On("Name").Return("test_backend")
	mockBackend.On("Type").Return("mock")

	// Exactly 7 backups (same as retention)
	now := time.Now()
	existingBackups := make([]storage.FileInfo, 7)
	for i := 0; i < 7; i++ {
		backupTime := now.Add(-time.Duration(i*24) * time.Hour)
		existingBackups[i] = storage.FileInfo{
			Path:    fmt.Sprintf("testdb--daily--%s.backup", backupTime.Format("2006-01-02T15-04-05")),
			Size:    1024,
			ModTime: backupTime,
		}
	}

	mockBackend.On("List", ctx, "testdb--daily--*.backup").
		Return(existingBackups, nil).
		Once()

	// No deletes expected

	// Execute rotation
	retentionTiers := []config.RetentionTier{
		{Tier: "daily", Retention: 7},
	}

	err := rotation.ApplyRetentionWithBackend(ctx, mockBackend, "testdb", retentionTiers, zerolog.Nop())

	// Assertions
	assert.NoError(t, err)
}
