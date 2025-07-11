.PHONY: all build run clean proto test deps dev fmt lint mocks run-local run-dev ui-build ui-dev ui-install setup prod-build help

# Variables
PROTO_DIR = grpc
PROTO_FILE = $(PROTO_DIR)/grpc.proto
GO_OUT_DIR = .
MAIN_FILE = grpc/cmd/main.go
BINARY_NAME = visionex

# Default target
all: proto build

# Generate Go code from proto files
proto:
	@echo "Generating Go code from proto files..."
	protoc --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)

# Build the binary
build:
	@echo "Building the binary..."
	go build -o $(BINARY_NAME) $(MAIN_FILE)

# Run the server
run: build
	@echo "Running the server..."
	./$(BINARY_NAME)

# Run with local configuration
run-local:
	@echo "Running with local configuration..."
	go run $(MAIN_FILE) --config=grpc/cmd/config.local.env

# Run with development configuration
run-dev:
	@echo "Running with development configuration..."
	go run $(MAIN_FILE) --config=grpc/cmd/config.dev.env

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	find . -name "*.pb.go" -type f -delete
	rm -rf ui/dist ui/dist_local ui/dist_dev

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Development server with hot reload
dev:
	@echo "Starting development server..."
	go run $(MAIN_FILE)

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Generate mocks
mocks:
	@echo "Generating mocks..."
	go generate ./...

# Frontend commands
ui-install:
	@echo "Installing frontend dependencies..."
	cd ui && npm install

ui-dev:
	@echo "Starting frontend development server..."
	cd ui && npm run dev

ui-build:
	@echo "Building frontend for production..."
	cd ui && npm run build

# Full development setup
setup: deps ui-install
	@echo "Setup complete! Run 'make dev' to start the backend or 'make ui-dev' for frontend"

# Production build
prod-build: proto build ui-build
	@echo "Production build complete!"

# Help
help:
	@echo "Available commands:"
	@echo "  build      - Build the Go binary"
	@echo "  run        - Build and run the server"
	@echo "  run-local  - Run with local configuration"
	@echo "  run-dev    - Run with development configuration"
	@echo "  dev        - Run in development mode"
	@echo "  test       - Run tests"
	@echo "  clean      - Clean build artifacts"
	@echo "  deps       - Install Go dependencies"
	@echo "  proto      - Generate protobuf code"
	@echo "  fmt        - Format Go code"
	@echo "  lint       - Lint Go code"
	@echo "  mocks      - Generate mocks"
	@echo "  ui-install - Install frontend dependencies"
	@echo "  ui-dev     - Start frontend development server"
	@echo "  ui-build   - Build frontend for production"
	@echo "  setup      - Full development setup"
	@echo "  prod-build - Production build"
	@echo "  help       - Show this help" 