# conba — Go build targets (Docker-based)
# All Go commands run inside containers; no local Go installation required.

GO_IMAGE    ?= golang:1.26
LINT_IMAGE  ?= golangci/golangci-lint:v2.11.4
MODULE      ?= github.com/lazybytez/conba
VERSION        ?= edge
COMMIT_SHA     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
RESTIC_VERSION ?= 0.18.1

DOCKER_RUN  ?= docker run --rm \
	-v $(CURDIR):/app \
	-v conba-gomod:/go/pkg/mod \
	-v conba-gobuild:/root/.cache/go-build \
	-w /app

.PHONY: go/build go/test go/lint go/coverage go/fmt go/clean

go/build: ## Build the conba binary with version injection
	$(DOCKER_RUN) -e CGO_ENABLED=0 $(GO_IMAGE) \
		go build -buildvcs=false \
			-ldflags "-X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.CommitSHA=$(COMMIT_SHA) -X $(MODULE)/internal/build.ResticVersion=$(RESTIC_VERSION)" \
			-o bin/conba ./cmd/conba

go/test: ## Run tests with race detector
	$(DOCKER_RUN) $(GO_IMAGE) \
		go test -race -v ./...

go/lint: ## Run golangci-lint
	$(DOCKER_RUN) $(LINT_IMAGE) \
		golangci-lint run ./...

go/coverage: ## Run tests with coverage report
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

go/fmt: ## Format code
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'gofmt -w . && if command -v goimports > /dev/null 2>&1; then goimports -w .; fi'

go/clean: ## Remove build artifacts
	rm -rf bin/
	rm -f coverage.out
