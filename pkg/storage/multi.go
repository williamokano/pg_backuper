package storage

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// MultiUploader handles uploading to multiple backends in parallel
type MultiUploader struct {
	logger zerolog.Logger
}

// NewMultiUploader creates a new multi-uploader
func NewMultiUploader(logger zerolog.Logger) *MultiUploader {
	return &MultiUploader{logger: logger}
}

// Upload uploads a file to multiple backends concurrently
func (m *MultiUploader) Upload(ctx context.Context, backends []Backend, sourcePath, destPath string) []Result {
	var wg sync.WaitGroup
	resultsChan := make(chan Result, len(backends))

	// Upload to each backend in parallel
	for _, backend := range backends {
		wg.Add(1)

		go func(b Backend) {
			defer wg.Done()

			start := time.Now()

			m.logger.Debug().
				Str("backend", b.Name()).
				Str("type", b.Type()).
				Str("file", destPath).
				Msg("starting upload")

			err := b.Write(ctx, sourcePath, destPath)
			duration := time.Since(start)

			result := Result{
				BackendName: b.Name(),
				BackendType: b.Type(),
				Success:     err == nil,
				Error:       err,
				Duration:    duration,
			}

			if err != nil {
				m.logger.Error().
					Err(err).
					Str("backend", b.Name()).
					Dur("duration", duration).
					Msg("upload failed")
			} else {
				m.logger.Info().
					Str("backend", b.Name()).
					Dur("duration", duration).
					Msg("upload succeeded")
			}

			resultsChan <- result
		}(backend)
	}

	wg.Wait()
	close(resultsChan)

	// Collect results
	var results []Result
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// Delete deletes a file from multiple backends
func (m *MultiUploader) Delete(ctx context.Context, backends []Backend, path string) []Result {
	var wg sync.WaitGroup
	resultsChan := make(chan Result, len(backends))

	for _, backend := range backends {
		wg.Add(1)

		go func(b Backend) {
			defer wg.Done()

			start := time.Now()
			err := b.Delete(ctx, path)

			resultsChan <- Result{
				BackendName: b.Name(),
				BackendType: b.Type(),
				Success:     err == nil,
				Error:       err,
				Duration:    time.Since(start),
			}
		}(backend)
	}

	wg.Wait()
	close(resultsChan)

	var results []Result
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}
