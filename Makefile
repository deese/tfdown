# tfdown Makefile

# Variables
BINARY_NAME=tfdown
MAIN_PACKAGE=./src
VERSION?=1.0.0
BUILD_DIR=dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.DEFAULT_GOAL := build

# Help target
.PHONY: help
help:
	@echo "tfdown - Terraform Downloader"
	@echo ""
	@echo "Available targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-all     - Build for all platforms"
	@echo "  make build-linux   - Build for Linux (amd64, 386, arm64, arm)"
	@echo "  make build-windows - Build for Windows (amd64, 386)"
	@echo "  make build-darwin  - Build for macOS (amd64, arm64)"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deps          - Download dependencies"
	@echo "  make run           - Run the application"

# Download dependencies
.PHONY: deps
deps:
	cd $(MAIN_PACKAGE) && $(GOMOD) download && $(GOMOD) tidy

# Run tests
.PHONY: test
test:
	cd $(MAIN_PACKAGE) && $(GOTEST) -v ./...

# Build for current platform
.PHONY: build
build: deps
	@echo "Building for current platform..."
	cd $(MAIN_PACKAGE) && $(GOBUILD) $(LDFLAGS) -o ../$(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin
	@echo "All builds complete!"

# Build for Linux
.PHONY: build-linux
build-linux: deps
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	cd $(MAIN_PACKAGE) && GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	cd $(MAIN_PACKAGE) && GOOS=linux GOARCH=386 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-386 .
	cd $(MAIN_PACKAGE) && GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	cd $(MAIN_PACKAGE) && GOOS=linux GOARCH=arm $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-arm .
	@echo "Linux builds complete!"

# Build for Windows
.PHONY: build-windows
build-windows: deps
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	cd $(MAIN_PACKAGE) && GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	cd $(MAIN_PACKAGE) && GOOS=windows GOARCH=386 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-windows-386.exe .
	cd $(MAIN_PACKAGE) && GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe .
	@echo "Windows builds complete!"

# Build for macOS/Darwin
.PHONY: build-darwin
build-darwin: deps
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	cd $(MAIN_PACKAGE) && GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	cd $(MAIN_PACKAGE) && GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "macOS builds complete!"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME).exe
	rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME)

# Install to GOPATH/bin
.PHONY: install
install: deps
	@echo "Installing..."
	cd $(MAIN_PACKAGE) && $(GOCMD) install $(LDFLAGS) .
	@echo "Install complete!"
