# Global ARGs — single source of truth for base image versions
ARG go_version=1.26
ARG alpine_version=3.23
ARG restic_version=0.18.1

# Stage 0: Source the pinned restic binary
FROM docker.io/restic/restic:${restic_version} AS restic

# Stage 1: Build the conba binary
FROM docker.io/library/golang:${go_version}-alpine AS builder

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

# Stage 2: Minimal runtime image
FROM docker.io/library/alpine:${alpine_version} AS base

ARG container_uid=1000
ARG container_gid=1000

RUN addgroup -g "${container_gid}" conba && \
    adduser -u "${container_uid}" -G conba -h /home/conba -s /bin/sh -S conba && \
    apk add --no-cache tini && \
    rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app

COPY --from=builder --link --chmod=755 /build/conba ./conba
COPY --from=restic --link --chmod=755 /usr/bin/restic ./restic

LABEL org.opencontainers.image.title="conba"
LABEL org.opencontainers.image.description="A simple restic-based container volume backup tool"
LABEL org.opencontainers.image.vendor="Lazy Bytez"
LABEL org.opencontainers.image.source="https://github.com/lazybytez/conba"
LABEL org.opencontainers.image.licenses="MIT"

USER conba

ENTRYPOINT ["/sbin/tini", "--", "/app/conba"]
