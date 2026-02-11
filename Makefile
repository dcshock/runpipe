# runpipe - Makefile
# Go module: github.com/dcshock/runpipe

.PHONY: build clean test fmt lint deps sqlc-generate help

# Default target
all: build

# Build all packages (no binaries for this library)
build:
	go build ./...

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Run go vet
lint:
	go vet ./...

# Generate sqlc code for observer (requires sqlc: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)
sqlc-generate:
	cd observer && sqlc generate

# Tidy and verify dependencies
deps:
	go mod tidy
	go mod verify

# Remove build artifacts and test outputs
clean:
	go clean -cache -testcache
	rm -f coverage.out coverage.html
	rm -f *.test

# Show this help
help:
	@echo "runpipe Makefile targets:"
	@echo "  all           - build (default)"
	@echo "  build         - go build ./..."
	@echo "  test          - go test ./..."
	@echo "  test-race     - go test -race ./..."
	@echo "  test-coverage - go test with coverage report"
	@echo "  fmt           - go fmt ./..."
	@echo "  lint          - go vet ./..."
	@echo "  deps          - go mod tidy && go mod verify"
	@echo "  sqlc-generate - regenerate observer/repository from schema and queries"
	@echo "  clean         - remove cache and coverage artifacts"
	@echo "  help          - show this help"
