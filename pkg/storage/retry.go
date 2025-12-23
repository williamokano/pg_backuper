package storage

import (
	"context"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

// WithRetry executes operation with retry logic
func WithRetry(ctx context.Context, cfg RetryConfig, op func() error) error {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry critical errors
		if IsCritical(err) {
			return err
		}

		// Don't retry non-retryable errors
		if !IsRetryable(err) {
			return err
		}

		// Last attempt, don't wait
		if attempt == cfg.MaxAttempts {
			break
		}

		// Wait before retry
		select {
		case <-time.After(delay):
			delay = time.Duration(float64(delay) * cfg.BackoffFactor)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}
