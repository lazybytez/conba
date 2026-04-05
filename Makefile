# conba — project root Makefile

# Variables
MODULE            ?= github.com/lazybytez/conba
VERSION           ?= edge
COMMIT_SHA        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
RESTIC_VERSION    ?= 0.18.1
GO_IMAGE          ?= golang:1.26
LINT_IMAGE        ?= golangci/golangci-lint:v2.11.4
DOCKER_EXECUTABLE ?= docker
IMAGE_NAME        ?= ghcr.io/lazybytez/conba
IMAGE_TAG         ?= edge

DOCKER_RUN ?= docker run --rm \
	-v $(CURDIR):/app \
	-w /app

include devops/make/go.mk
include devops/make/docker.mk

.DEFAULT_GOAL := help

.PHONY: build test lint help

# Alias for go/build
build: go/build
# Alias for go/test
test: go/test
# Alias for go/lint
lint: go/lint

# Show available targets
help:
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
