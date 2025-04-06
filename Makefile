# Variables
BINARY_NAME=shareiscare
GO=go
TEMPL=templ
VERSION=1.0.0
BUILD_DIR=./bin
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Colors for output
YELLOW=\033[0;33m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

# Detect operating system
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    OS = darwin
else ifeq ($(UNAME_S),Linux)
    OS = linux
else
    OS = windows
endif

.PHONY: all build clean run help install generate cross-build build-linux build-windows build-mac build-raspberrypi build-raspberrypi-zero release test

all: help

# Generate Go code from templ templates
generate:
	@echo "$(YELLOW)Generating Go code from templ templates...$(NC)"
	@$(TEMPL) generate
	@echo "$(GREEN)✓ Code generated successfully$(NC)"

# Compile the main binary
build: generate
	@echo "$(YELLOW)Compiling $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Clean generated files and binaries
clean:
	@echo "$(YELLOW)Cleaning generated files...$(NC)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Cleanup completed$(NC)"

# Run the application
run: generate
	@echo "$(YELLOW)Running $(BINARY_NAME)...$(NC)"
	@$(GO) run main.go

# Run unit tests
test:
	@echo "$(YELLOW)Running unit tests...$(NC)"
	@# Cleanup of temporary files
	@if [ -f config.yaml.bak ]; then \
		mv config.yaml.bak config.yaml; \
	fi
	@# Run tests (excluding cross-compilation test)
	@$(GO) test -v ./... -short
	@# Verify compilation for Raspberry Pi (ARMv7)
	@echo "$(YELLOW)Verifying compilation for Raspberry Pi (ARMv7)...$(NC)"
	@GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 $(GO) build -o /tmp/shareiscare-arm-test main.go; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "$(GREEN)✓ Compilation for Raspberry Pi (ARMv7) is correct$(NC)"; \
		rm /tmp/shareiscare-arm-test; \
	else \
		echo "$(RED)✗ Error in compilation for Raspberry Pi (ARMv7)$(NC)"; \
		exit 1; \
	fi
	@# Verify compilation for Raspberry Pi Zero (ARMv6)
	@echo "$(YELLOW)Verifying compilation for Raspberry Pi Zero (ARMv6)...$(NC)"
	@GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 $(GO) build -o /tmp/shareiscare-arm6-test main.go; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "$(GREEN)✓ Compilation for Raspberry Pi Zero (ARMv6) is correct$(NC)"; \
		rm /tmp/shareiscare-arm6-test; \
	else \
		echo "$(RED)✗ Error in compilation for Raspberry Pi Zero (ARMv6)$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Tests completed successfully$(NC)"

# Update dependencies
deps:
	@echo "$(YELLOW)Updating dependencies...$(NC)"
	@$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(NC)"

# Install necessary tools
install:
	@echo "$(YELLOW)Installing necessary tools...$(NC)"
	@$(GO) install github.com/a-h/templ/cmd/templ@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"

# Compilation for different platforms
build-linux:
	@echo "$(YELLOW)Compiling for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

build-windows:
	@echo "$(YELLOW)Compiling for Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe$(NC)"

build-mac:
	@echo "$(YELLOW)Compiling for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64$(NC)"

build-raspberrypi:
	@echo "$(YELLOW)Compiling for Raspberry Pi (ARMv7)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7 main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7$(NC)"

build-raspberrypi-zero:
	@echo "$(YELLOW)Compiling for Raspberry Pi Zero (ARMv6)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv6 main.go
	@echo "$(GREEN)✓ Binary compiled at $(BUILD_DIR)/$(BINARY_NAME)-linux-armv6$(NC)"

# Compile for all platforms
cross-build: build-linux build-windows build-mac build-raspberrypi build-raspberrypi-zero
	@echo "$(GREEN)✓ Compilation completed for all platforms$(NC)"

# Start the application in development mode
dev: generate
	@echo "$(YELLOW)Starting in development mode...$(NC)"
	@$(GO) run main.go

# Generate configuration file
init-config:
	@echo "$(YELLOW)Generating configuration file...$(NC)"
	@$(GO) run main.go init
	@echo "$(GREEN)✓ Configuration file generated$(NC)"

# Create tag and release manually (aunque ahora los releases se generan automáticamente)
release:
	@if [ -z "$(v)" ]; then \
		echo "$(RED)Error: You must specify a version. Example: make release v=1.0.0$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Creating tag v$(v)...$(NC)"
	@git tag -a v$(v) -m "Version $(v)"
	@git push origin v$(v)
	@echo "$(GREEN)✓ Tag v$(v) created and pushed to GitHub$(NC)"
	@echo "$(YELLOW)GitHub Actions should be creating the release automatically...$(NC)"

# Merge a PR para crear un release automáticamente
merge-and-release:
	@if [ -z "$(pr)" ]; then \
		echo "$(RED)Error: You must specify a PR number. Example: make merge-and-release pr=123$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Merging PR #$(pr) into main branch...$(NC)"
	@git checkout main
	@git pull
	@git merge --no-ff -m "Merge PR #$(pr)" origin/PR-$(pr)
	@git push origin main
	@echo "$(GREEN)✓ PR #$(pr) merged into main$(NC)"
	@echo "$(YELLOW)El proceso de CI/CD creará un release automáticamente$(NC)"

# Show help
help:
	@echo "$(YELLOW)ShareIsCare - Available commands:$(NC)"
	@echo ""
	@echo "  $(GREEN)make build$(NC)        - Compile the binary"
	@echo "  $(GREEN)make run$(NC)          - Run the application"
	@echo "  $(GREEN)make dev$(NC)          - Start in development mode"
	@echo "  $(GREEN)make test$(NC)         - Run unit tests"
	@echo "  $(GREEN)make generate$(NC)     - Generate Go code from templ templates"
	@echo "  $(GREEN)make clean$(NC)        - Clean generated files"
	@echo "  $(GREEN)make deps$(NC)         - Update dependencies"
	@echo "  $(GREEN)make install$(NC)      - Install necessary tools"
	@echo "  $(GREEN)make init-config$(NC)  - Generate default configuration file"
	@echo "  $(GREEN)make cross-build$(NC)  - Compile for Linux, Windows, macOS and Raspberry Pi"
	@echo "  $(GREEN)make build-linux$(NC)  - Compile for Linux"
	@echo "  $(GREEN)make build-windows$(NC)- Compile for Windows"
	@echo "  $(GREEN)make build-mac$(NC)    - Compile for macOS"
	@echo "  $(GREEN)make build-raspberrypi$(NC) - Compile for Raspberry Pi (ARMv7)"
	@echo "  $(GREEN)make build-raspberrypi-zero$(NC) - Compile for Raspberry Pi Zero (ARMv6)"
	@echo "  $(GREEN)make release v=X.Y.Z$(NC) - Crear tag y release manual (método antiguo)"
	@echo "  $(GREEN)make merge-and-release pr=N$(NC) - Mergear PR y generar release automático"
	@echo ""
	@echo "Run 'make' or 'make help' to see this help"