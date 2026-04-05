# conba — Docker image build targets

DOCKER_EXECUTABLE ?= docker
IMAGE_NAME        ?= conba
IMAGE_TAG         ?= edge
COMMIT_SHA        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
RESTIC_VERSION    ?= 0.18.1

.PHONY: docker/build

docker/build: ## Build the container image
	$(DOCKER_EXECUTABLE) build \
		-f Containerfile \
		--build-arg app_version=$(IMAGE_TAG) \
		--build-arg build_commit_sha=$(COMMIT_SHA) \
		--build-arg restic_version=$(RESTIC_VERSION) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) .
