# conba — Docker-based build targets for Go project
# All Go commands run inside containers; no local Go installation required.

GO_IMAGE    ?= golang:1.26
LINT_IMAGE  ?= golangci/golangci-lint:v2.11.4

DOCKER_RUN  := docker run --rm \
	-v $(CURDIR):/app \
	-v conba-gomod:/go/pkg/mod \
	-v conba-gobuild:/root/.cache/go-build \
	-w /app

.PHONY: build test lint coverage fmt clean

build:
	$(DOCKER_RUN) -e CGO_ENABLED=0 $(GO_IMAGE) \
		go build -buildvcs=false -o bin/conba ./cmd/conba

test:
	$(DOCKER_RUN) $(GO_IMAGE) \
		go test -race -v ./...

lint:
	$(DOCKER_RUN) $(LINT_IMAGE) \
		golangci-lint run ./...

coverage:
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

fmt:
	$(DOCKER_RUN) $(GO_IMAGE) \
		sh -c 'gofmt -w . && if command -v goimports > /dev/null 2>&1; then goimports -w .; fi'

clean:
	rm -rf bin/
	rm -f coverage.out
