# conba - Go build targets (Docker-based)
# All Go commands run inside containers; no local Go installation required.

.PHONY: go/build go/test go/test-image go/lint go/coverage go/fmt go/clean

# Build the conba binary with version injection
go/build:
	$(DOCKER_RUN) -e CGO_ENABLED=0 $(GO_IMAGE) \
		go build -buildvcs=false \
			-ldflags "-X $(MODULE)/internal/build.Version=$(VERSION) -X $(MODULE)/internal/build.CommitSHA=$(COMMIT_SHA) -X $(MODULE)/internal/build.ResticVersion=$(RESTIC_VERSION)" \
			-o bin/conba ./cmd/conba

# Build the test image containing Go toolchain and restic binary
go/test-image:
	$(DOCKER_EXECUTABLE) build \
		--target test \
		--build-arg restic_version=$(RESTIC_VERSION) \
		-t $(TEST_IMAGE) \
		-f Containerfile $$(mktemp -d)

# Run tests with race detector
go/test: go/test-image
	$(DOCKER_RUN) $(TEST_IMAGE) \
		go test -race -v ./...

# Run golangci-lint across the default build set and the e2e-tagged files
go/lint:
	$(DOCKER_RUN) $(LINT_IMAGE) \
		golangci-lint run --build-tags=e2e ./...

# Run tests with coverage report
go/coverage: go/test-image
	$(DOCKER_RUN) $(TEST_IMAGE) \
		sh -c 'go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

# Format code
go/fmt:
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'gofmt -w . && if command -v goimports > /dev/null 2>&1; then goimports -w .; fi'

# Remove build artifacts
go/clean:
	rm -rf bin/
	rm -f coverage.out
