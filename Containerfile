# Global ARGs — single source of truth for base image versions
ARG go_version=1.26
ARG alpine_version=3.23
ARG restic_version=0.18.1

# Stage 0: Source the pinned restic binary
FROM docker.io/restic/restic:${restic_version} AS restic

# Stage 1: Test image — Debian-based for CGO (required by -race detector)
FROM docker.io/library/golang:${go_version} AS test
COPY --from=restic --link /usr/bin/restic /usr/bin/restic

# Stage 2: Build the conba binary
FROM docker.io/library/golang:${go_version} AS builder

ARG app_version=edge
ARG build_commit_sha=unknown
ARG restic_version

WORKDIR /build

COPY --link go.mod go.sum ./
RUN go mod download && go mod verify

COPY --link . .
RUN CGO_ENABLED=0 go build -buildvcs=false \
    -ldflags "-X github.com/lazybytez/conba/internal/build.Version=${app_version} -X github.com/lazybytez/conba/internal/build.CommitSHA=${build_commit_sha} -X github.com/lazybytez/conba/internal/build.ResticVersion=${restic_version}" \
    -o /build/conba ./cmd/conba

# Stage 3: Minimal runtime image
# Runs as root: conba needs access to the Docker socket (root:docker 660)
# and /var/lib/docker/volumes (root-owned) to snapshot container volumes.
FROM docker.io/library/alpine:${alpine_version} AS base

RUN apk add --no-cache tini && \
    rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app

COPY --from=builder --link --chmod=755 /build/conba ./conba
COPY --from=restic --link --chmod=755 /usr/bin/restic ./restic

LABEL org.opencontainers.image.title="conba"
LABEL org.opencontainers.image.description="A simple restic-based container volume backup tool"
LABEL org.opencontainers.image.vendor="Lazy Bytez"
LABEL org.opencontainers.image.source="https://github.com/lazybytez/conba"
LABEL org.opencontainers.image.licenses="MIT"

ENTRYPOINT ["/sbin/tini", "--", "/app/conba"]
