package storage

import (
	"context"
	"fmt"
)

// BackendConstructor is a function that creates a backend instance
type BackendConstructor func(ctx context.Context, cfg Config) (Backend, error)

var backendRegistry = make(map[string]BackendConstructor)

// RegisterBackend registers a backend constructor
func RegisterBackend(backendType string, constructor BackendConstructor) {
	backendRegistry[backendType] = constructor
}

// Factory creates storage backends from configuration
type Factory struct{}

// NewFactory creates a new factory instance
func NewFactory() *Factory {
	return &Factory{}
}

// Create instantiates a backend from config
func (f *Factory) Create(ctx context.Context, cfg Config) (Backend, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("backend %s is disabled", cfg.Name)
	}

	constructor, ok := backendRegistry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	}

	return constructor(ctx, cfg)
}

// CreateAll creates all enabled backends from slice of configs
func (f *Factory) CreateAll(ctx context.Context, configs []Config) ([]Backend, error) {
	backends := make([]Backend, 0, len(configs))

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		backend, err := f.Create(ctx, cfg)
		if err != nil {
			// Close already created backends
			for _, b := range backends {
				b.Close()
			}
			return nil, fmt.Errorf("failed to create backend %s: %w", cfg.Name, err)
		}

		backends = append(backends, backend)
	}

	return backends, nil
}
