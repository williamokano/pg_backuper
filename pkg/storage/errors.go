package storage

import (
	"errors"
	"fmt"
)

var (
	ErrAuthFailed       = errors.New("authentication failed")
	ErrConnFailed       = errors.New("connection failed")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound         = errors.New("file not found")
	ErrTimeout          = errors.New("operation timeout")
	ErrInvalidConfig    = errors.New("invalid configuration")
)

// IsRetryable returns true if error should trigger a retry
func IsRetryable(err error) bool {
	return errors.Is(err, ErrConnFailed) || errors.Is(err, ErrTimeout)
}

// IsCritical returns true if error should stop all operations
func IsCritical(err error) bool {
	return errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrInvalidConfig)
}

// WrapError adds context to an error
func WrapError(backend, operation string, err error) error {
	return fmt.Errorf("%s (%s): %w", operation, backend, err)
}
