# conba — Go build targets (Docker-based)
# All Go commands run inside containers; no local Go installation required.

.PHONY: go/build go/test go/lint go/coverage go/fmt go/clean

# Build the conba binary with version injection
go/build:
	$(DOCKER_RUN) -e CGO_ENABLED=0 $(GO_IMAGE) \
		go build -buildvcs=false \
			-ldflags "-X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.CommitSHA=$(COMMIT_SHA) -X $(MODULE)/internal/build.ResticVersion=$(RESTIC_VERSION)" \
			-o bin/conba ./cmd/conba

# Run tests with race detector
go/test:
	$(DOCKER_RUN) $(GO_IMAGE) \
		go test -race -v ./...

# Run golangci-lint
go/lint:
	$(DOCKER_RUN) $(LINT_IMAGE) \
		golangci-lint run ./...

# Run tests with coverage report
go/coverage:
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

# Format code
go/fmt:
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'gofmt -w . && if command -v goimports > /dev/null 2>&1; then goimports -w .; fi'

# Remove build artifacts
go/clean:
	rm -rf bin/
	rm -f coverage.out
