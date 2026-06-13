.PHONY: help test lint lint-fix build clean install coverage

# Default target
.DEFAULT_GOAL := help

# Add Go bin to PATH for all targets
GOPATH ?= $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

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
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "  Installing golangci-lint..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@command -v staticcheck >/dev/null 2>&1 || \
		(echo "  Installing staticcheck..." && \
		go install honnef.co/go/tools/cmd/staticcheck@latest)
	@command -v ineffassign >/dev/null 2>&1 || \
		(echo "  Installing ineffassign..." && \
		go install github.com/gordonklaus/ineffassign@latest)
	@command -v misspell >/dev/null 2>&1 || \
		(echo "  Installing misspell..." && \
		go install github.com/client9/misspell/cmd/misspell@latest)
	@command -v errcheck >/dev/null 2>&1 || \
		(echo "  Installing errcheck..." && \
		go install github.com/kisielk/errcheck@latest)
	@command -v gocyclo >/dev/null 2>&1 || \
		(echo "  Installing gocyclo..." && \
		go install github.com/fzipp/gocyclo/cmd/gocyclo@latest)
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

lint: ## Run all quality checks (gofmt, vet, staticcheck, misspell, gocyclo, errcheck)
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
