.PHONY: all build build-cli build-server build-ui engine test clean install dev-ui dev-server dist

# Variables
BINARY_NAME=lorag
CLI_NAME=loragctl
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# VectixDB checkout (search engine, linked through cgo)
VECTIXDB ?= ../vectixdb
HOST_PLATFORM=$(shell go env GOOS)_$(shell go env GOARCH)
ENGINE_LIB=$(VECTIXDB)/bindings/go/lib/$(HOST_PLATFORM)/lib/libvectixdb.a

# Directories
BUILD_DIR=bin
WEB_DIR=ui
UI_DIR=ui

# Default target
all: build

# Build everything
build: build-ui build-server build-cli
	@echo "✓ Build complete!"

# Build the web UI
build-ui:
	@echo "Building UI..."
	cd $(WEB_DIR) && npm install && npm run build
	@mkdir -p $(UI_DIR)/dist
	@cp -r $(WEB_DIR)/dist/* $(UI_DIR)/dist/ 2>/dev/null || true
	@echo "✓ UI built"

# Build the VectixDB static library for the host (needs zig 0.16)
engine:
	$(VECTIXDB)/scripts/go-libs.sh

$(ENGINE_LIB):
	$(VECTIXDB)/scripts/go-libs.sh

# Build the server binary
build-server: $(ENGINE_LIB)
	@echo "Building server..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/lorag
	@echo "✓ Server built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build the CLI binary
build-cli: $(ENGINE_LIB)
	@echo "Building CLI..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME) ./cmd/loragctl
	@echo "✓ CLI built: $(BUILD_DIR)/$(CLI_NAME)"

# Development mode - run server with hot reload
dev-server: $(ENGINE_LIB)
	go run ./cmd/lorag server

# Development mode - run UI
dev-ui:
	cd $(WEB_DIR) && npm run dev

# Run tests
test: $(ENGINE_LIB)
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(UI_DIR)/dist
	@rm -rf $(WEB_DIR)/dist dist
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

# Install dependencies
install:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing UI dependencies..."
	cd $(WEB_DIR) && npm install
	@echo "✓ Dependencies installed"

# Run the server (data in ~/.lorag/data)
run: build-server
	@echo "Starting Lorag server..."
	./$(BUILD_DIR)/$(BINARY_NAME) server

# Docker build
docker-build:
	docker build --build-context vectixdb=$(VECTIXDB) -t lorag:$(VERSION) .

# Docker run
docker-run:
	docker run -p 8080:8080 -v lorag-data:/data lorag:$(VERSION)

# Release archives in dist/ (scripts/dist.sh): the host platform by default,
# e.g. make dist PLATFORMS="darwin_arm64 darwin_amd64 linux_amd64 linux_arm64"
PLATFORMS ?= $(HOST_PLATFORM)
dist: build-ui
	VECTIXDB=$(VECTIXDB) ./scripts/dist.sh $(VERSION) $(PLATFORMS)

# Install locally
install-local: build
	@echo "Installing locally..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/ 2>/dev/null || sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@cp $(BUILD_DIR)/$(CLI_NAME) /usr/local/bin/ 2>/dev/null || sudo cp $(BUILD_DIR)/$(CLI_NAME) /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/"

# Format code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Formatting complete"

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run ./...
	cd $(WEB_DIR) && npm run lint

# Show help
help:
	@echo "Lorag Makefile targets:"
	@echo ""
	@echo "  make build        - Build everything (UI + binaries)"
	@echo "  make build-ui     - Build the React UI"
	@echo "  make build-server - Build the server binary"
	@echo "  make build-cli    - Build the CLI binary"
	@echo "  make engine       - Rebuild the VectixDB library (VECTIXDB=../vectixdb)"
	@echo "  make dev-server   - Run server in development mode"
	@echo "  make dev-ui       - Run UI in development mode"
	@echo "  make test         - Run all tests"
	@echo "  make clean        - Clean all build artifacts"
	@echo "  make install      - Install dependencies"
	@echo "  make run          - Build and run the server"
	@echo "  make dist         - Release archives in dist/ (PLATFORMS=...)"
	@echo "  make docker-build - Build Docker image"
	@echo "  make help         - Show this help"
