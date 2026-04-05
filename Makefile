# conba — project root Makefile

include devops/make/go.mk
include devops/make/docker.mk

.DEFAULT_GOAL := help

.PHONY: build test lint help

build: go/build ## Alias for go/build
test: go/test ## Alias for go/test
lint: go/lint ## Alias for go/lint

help: ## Show available targets
	@echo "=== conba ==="
	@echo ""
	@echo "  Go targets:"
	@echo "    make go/build       Build the conba binary with version injection"
	@echo "    make go/test        Run tests with race detector"
	@echo "    make go/lint        Run golangci-lint"
	@echo "    make go/coverage    Run tests with coverage report"
	@echo "    make go/fmt         Format code"
	@echo "    make go/clean       Remove build artifacts"
	@echo ""
	@echo "  Docker targets:"
	@echo "    make docker/build   Build the container image"
	@echo ""
	@echo "  Aliases:"
	@echo "    make build          Alias for go/build"
	@echo "    make test           Alias for go/test"
	@echo "    make lint           Alias for go/lint"
