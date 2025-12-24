.PHONY: help build build-migrate build-docker test test-integration test-all mocks clean

# Default target
help:
	@echo "pg_backuper Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make build              - Build main application binary"
	@echo "  make build-migrate      - Build migration tool binary"
	@echo "  make build-docker       - Build Docker image"
	@echo "  make test               - Run unit tests only (fast)"
	@echo "  make test-integration   - Run integration tests only (requires Docker)"
	@echo "  make test-all           - Run all tests (unit + integration)"
	@echo "  make mocks              - Regenerate mocks (requires mockery)"
	@echo "  make clean              - Remove build artifacts"

# Build targets
build:
	@echo "Building pg_backuper..."
	go build -o pg_backuper .

build-migrate:
	@echo "Building migration tool..."
	go build -o migrate ./cmd/migrate

build-docker:
	@echo "Building Docker image..."
	docker build -t pg_backuper:v2.0 .

# Test targets
test:
	@echo "Running unit tests..."
	go test -short -v ./...

test-integration:
	@echo "Running integration tests (this may take 5-10 minutes)..."
	go test -tags=integration -v ./pkg/backup -timeout 10m

test-all:
	@echo "Running all tests (unit + integration)..."
	go test -tags=integration -v ./... -timeout 10m

# Mock generation
mocks:
	@echo "Regenerating mocks..."
	@if command -v mockery >/dev/null 2>&1; then \
		mockery; \
	else \
		echo "Error: mockery not found. Install with: go install github.com/vektra/mockery/v2@v2.43.0"; \
		exit 1; \
	fi

# Cleanup
clean:
	@echo "Cleaning build artifacts..."
	rm -f pg_backuper migrate
	@echo "Done."
