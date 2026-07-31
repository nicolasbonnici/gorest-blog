.PHONY: help test lint lint-fix build clean install coverage

# Default target
.DEFAULT_GOAL := help

# Add Go bin to PATH for all targets
GOPATH ?= $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

# Kept in step with .github/workflows/ci.yml. The v2 config format is not
# readable by v1, so a stale v1 binary must be upgraded, not merely detected.
GOLANGCI_LINT_VERSION := v2.12.2

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies, dev tools, and git hooks
	@echo "[INFO] Installing development environment..."
	@echo ""
	@echo "[1/3] Installing Go dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies installed"
	@echo ""
	@echo "[2/3] Installing development tools..."
	@if ! golangci-lint --version 2>/dev/null | grep -qE 'version v?2\.'; then \
		echo "  Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		GOWORK=off go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@echo "✓ Development tools installed"
	@echo ""
	@echo "[3/3] Installing git hooks..."
	@bash .githooks/install.sh
	@echo ""
	@echo "✅ Installation complete! Ready to develop."
	@echo ""
	@echo "Next steps:"
	@echo "  • Run 'make test' to verify your setup"
	@echo "  • Run 'make lint' to check code quality"
	@echo "  • See 'make help' for all available commands"
test: ## Run tests with race detector
	@echo "Running tests..."
	@go test -v -race ./...

coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML coverage report, run:"
	@echo "  go tool cover -html=coverage.out"

lint: ## Run golangci-lint (bundles staticcheck, errcheck, govet, gocyclo, misspell)
	@echo "Running golangci-lint..."
	@GOWORK=off $$(go env GOPATH)/bin/golangci-lint run ./...

lint-fix: ## Run linter with auto-fix (CI parity: GOWORK=off)
	@echo "Running golangci-lint with auto-fix..."
	@GOWORK=off $$(go env GOPATH)/bin/golangci-lint run --fix ./...

build: ## Build verification
	@echo "Building plugin..."
	@go build -v ./...
	@echo "✓ Build successful"

build-import-cli: ## Build import CLI binary (requires main.go wrapper - see blog project)
	@echo "⚠️  Note: The import CLI requires a main.go wrapper to set up the service factory."
	@echo "    Build from the blog project instead: cd ../blog && go build -o bin/import ./cmd/import"
	@echo ""
	@echo "    Or use Docker: docker build -t blog:latest . (includes import binary)"

clean: ## Clean build artifacts and caches
	@echo "Cleaning..."
	@go clean -cache -testcache -modcache
	@rm -f coverage.out
	@echo "✓ Cleaned"

generate:
