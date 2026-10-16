.PHONY: all build build-cli build-server build-ui engine test clean install dev-ui dev-server release

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
DATA_DIR=data
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
	@rm -rf $(WEB_DIR)/dist
	@rm -rf $(DATA_DIR)
	@rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

# Install dependencies
install:
	@echo "Installing Go dependencies..."
	go mod download
	@echo "Installing UI dependencies..."
	cd $(WEB_DIR) && npm install
	@echo "✓ Dependencies installed"

# Create data directory
init-data:
	@mkdir -p $(DATA_DIR)

# Run the server (requires build)
run: build-server init-data
	@echo "Starting Lorag server..."
	./$(BUILD_DIR)/$(BINARY_NAME) server

# Quick launch with script
launch: build
	@./scripts/launch.sh

# Docker build
docker-build:
	docker build --build-context vectixdb=$(VECTIXDB) -t lorag:$(VERSION) .

# Docker run
docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/data lorag:$(VERSION)

# Release build for multiple platforms
release:
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/lorag
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-darwin-amd64 ./cmd/loragctl
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/lorag
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-darwin-arm64 ./cmd/loragctl
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/lorag
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-amd64 ./cmd/loragctl
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/lorag
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-linux-arm64 ./cmd/loragctl
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/lorag
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_NAME)-windows-amd64.exe ./cmd/loragctl
	
	@echo "✓ Release binaries built in $(BUILD_DIR)/"

# Package for distribution
package: build
	@echo "Creating distribution packages..."
	@mkdir -p dist
	
	# Create tar.gz for Unix systems
	tar -czf dist/lorag-$(VERSION)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m).tar.gz \
		-C $(BUILD_DIR) $(BINARY_NAME) $(CLI_NAME) \
		-C .. README.md USER_GUIDE.md QUICKSTART.md
	
	@echo "✓ Packages created in dist/"

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
	@echo "  make launch       - Quick launch with scripts/launch.sh"
	@echo "  make release      - Build binaries for all platforms"
	@echo "  make package      - Create distribution packages"
	@echo "  make docker-build - Build Docker image"
	@echo "  make help         - Show this help"
