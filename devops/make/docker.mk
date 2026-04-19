# conba - Docker image build targets

.PHONY: docker/build

# Build the container image
docker/build:
	$(DOCKER_EXECUTABLE) build \
		-f Containerfile \
		--build-arg app_version=$(IMAGE_TAG) \
		--build-arg build_commit_sha=$(COMMIT_SHA) \
		--build-arg restic_version=$(RESTIC_VERSION) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) .
