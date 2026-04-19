# conba — End-to-end test targets (Docker-based, requires Docker socket access)
# All e2e commands run inside containers; the test image must be built first.

.PHONY: go/test-e2e go/test-e2e/up go/test-e2e/down go/test-e2e/run e2e

E2E_COMPOSE := $(DOCKER_EXECUTABLE) compose -f test/e2e/compose.yaml

E2E_RUN := $(DOCKER_EXECUTABLE) run --rm \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-v /var/lib/docker/volumes:/var/lib/docker/volumes \
	-v $(CURDIR):/app -w /app \
	-e CONBA_BINARY=/app/bin/conba

E2E_JUNIT := test/e2e/junit.xml

# Bring up the e2e compose fixture and wait for all services to be healthy.
# --build rebuilds any service with a build: context (currently the custom
# mysql image that bakes in init.sql) when its Containerfile changes.
go/test-e2e/up:
	$(E2E_COMPOSE) up -d --build --wait --wait-timeout 120

# Tear down the e2e compose fixture and remove volumes (safe to call even when nothing is up)
go/test-e2e/down:
	-$(E2E_COMPOSE) down -v --remove-orphans

# Run e2e tests against the current fixture (does not manage compose lifecycle)
go/test-e2e/run:
	$(E2E_RUN) $(TEST_IMAGE) \
		gotestsum --junitfile $(E2E_JUNIT) --format testname --rerun-fails=0 \
			-- -tags=e2e -p 1 -count=1 ./test/e2e/...

# Full e2e: build conba + test image, bring fixture up, run tests, tear down (always, even on failure).
# Prereqs run in declaration order under serial make (the default); we do not use -j.
go/test-e2e: go/build go/test-image go/test-e2e/up
	@trap '$(MAKE) --no-print-directory go/test-e2e/down' EXIT; \
		$(MAKE) --no-print-directory go/test-e2e/run

# Alias for go/test-e2e
e2e: go/test-e2e
