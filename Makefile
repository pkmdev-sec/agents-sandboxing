# ASO - Agent Sandbox Orchestrator
# Makefile for building and managing the project

# Build variables
BINARY_NAME := aso
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/agent-sandbox-orchestrator/aso/internal/cli.Version=$(VERSION) \
	-X github.com/agent-sandbox-orchestrator/aso/internal/cli.GitCommit=$(GIT_COMMIT) \
	-X github.com/agent-sandbox-orchestrator/aso/internal/cli.BuildDate=$(BUILD_DATE)"

# Go settings
GO := go
GOFLAGS := -v

# Docker/Container settings
RUNTIME_IMAGE := ghcr.io/agent-sandbox-orchestrator/agent-runtime
RUNTIME_TAG := latest

.PHONY: all build clean test install uninstall deps runtime-image help setup mcp-config

# Default target
all: deps build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/aso

# Build for release (optimized)
build-release:
	@echo "Building $(BINARY_NAME) for release..."
	CGO_ENABLED=0 $(GO) build -trimpath $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/aso

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod tidy
	$(GO) mod download

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp bin/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed successfully!"

# Uninstall from system
uninstall:
	@echo "Removing $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalled successfully!"

# Build the agent runtime container image
runtime-image:
	@echo "Building agent runtime image..."
	docker build -t $(RUNTIME_IMAGE):$(RUNTIME_TAG) ./runtime

# Push runtime image (requires authentication)
push-runtime-image: runtime-image
	@echo "Pushing agent runtime image..."
	docker push $(RUNTIME_IMAGE):$(RUNTIME_TAG)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Run the binary (for development)
run: build
	./bin/$(BINARY_NAME)

# Development: watch for changes and rebuild
dev:
	@echo "Starting development mode..."
	@which watchexec > /dev/null || (echo "Install watchexec: cargo install watchexec-cli" && exit 1)
	watchexec -e go -r make build

# Full setup: build, install, runtime image, and MCP config
setup: install runtime-image mcp-config
	@echo ""
	@echo "=== ASO Setup Complete ==="
	@echo "Binary installed to /usr/local/bin/aso"
	@echo "Runtime image built: $(RUNTIME_IMAGE):$(RUNTIME_TAG)"
	@echo ""
	@echo "To use with Conductor, restart it and the 'aso' MCP server will be available."
	@echo "To use standalone: aso mcp serve"

# Generate MCP config for Conductor
mcp-config:
	@echo "Setting up MCP configuration for Conductor..."
	@mkdir -p ~/.claude
	@if [ -f ~/.claude/claude_desktop_config.json ]; then \
		echo "Backing up existing config to ~/.claude/claude_desktop_config.json.bak"; \
		cp ~/.claude/claude_desktop_config.json ~/.claude/claude_desktop_config.json.bak; \
		jq '.mcpServers.aso = {"command": "/usr/local/bin/aso", "args": ["mcp", "serve"]}' \
			~/.claude/claude_desktop_config.json > ~/.claude/claude_desktop_config.json.tmp && \
			mv ~/.claude/claude_desktop_config.json.tmp ~/.claude/claude_desktop_config.json; \
	else \
		echo '{"mcpServers": {"aso": {"command": "/usr/local/bin/aso", "args": ["mcp", "serve"]}}}' > ~/.claude/claude_desktop_config.json; \
	fi
	@echo "MCP config updated at ~/.claude/claude_desktop_config.json"

# Show help
help:
	@echo "ASO - Agent Sandbox Orchestrator"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build the project (default)"
	@echo "  make build        Build the binary"
	@echo "  make build-release Build optimized release binary"
	@echo "  make deps         Install Go dependencies"
	@echo "  make test         Run tests"
	@echo "  make test-coverage Run tests with coverage report"
	@echo "  make install      Install to /usr/local/bin"
	@echo "  make uninstall    Remove from /usr/local/bin"
	@echo "  make runtime-image Build agent runtime Docker image"
	@echo "  make setup        Full setup: build, install, runtime image, MCP config"
	@echo "  make mcp-config   Add ASO to Conductor's MCP configuration"
	@echo "  make clean        Remove build artifacts"
	@echo "  make fmt          Format code"
	@echo "  make lint         Run linter"
	@echo "  make help         Show this help"
