<div align="center">

# Conba

[![License][license-badge]][license-url]
[![CI][ci-badge]][ci-url]
[![Last Commit][commit-badge]][commit-url]

**Con**tainer **Ba**ckup — automated Docker volume backups powered by restic.

</div>

## Description

Conba is a Go CLI tool that wraps [restic](https://restic.net/) to provide automated,
configurable backups for Docker container volumes. It auto-discovers containers and their
volumes, applies filtering rules, executes the appropriate backup strategy per container,
and manages snapshot retention — all driven by a YAML config file with environment
variable overrides and optional container labels.

## Features

| Feature | Description |
|---------|-------------|
| Auto-discovery | Finds all running containers and their volume mounts via Docker API |
| Label-driven config | Per-container backup strategy, retention, and commands via Docker labels |
| Three backup strategies | Snapshot (default), Command + Snapshot (pre/post hooks), Stream (stdin pipe) |
| Flexible filtering | Include/exclude by name, ID, regex, or labels; opt-in-only mode |
| Retention management | Global policy with per-container overrides; wraps `restic forget --prune` |
| Tagged snapshots | Every snapshot tagged with container name, ID, volume name, and hostname |
| Environment overrides | All config values overridable via `CONBA_` prefixed env vars |
| Structured logging | Human-readable or JSON output at configurable levels |

## Requirements

- Docker (or compatible runtime with Docker socket)
- restic (installed separately for host binary; bundled in container image)

## Getting Started

Clone and build:

```sh
git clone https://github.com/lazybytez/conba.git
cd conba
make build
```

All Make targets run inside Docker containers — no local Go installation required.

Create a config file (`conba.yaml`):

```yaml
restic:
  repository: "s3:s3.amazonaws.com/my-bucket"
  password_file: "/run/secrets/restic-password"

runtime:
  type: docker
  docker:
    host: "unix:///var/run/docker.sock"

discovery:
  opt_in_only: false

retention:
  keep_daily: 7
  keep_weekly: 4
  keep_monthly: 6
  keep_yearly: 0

logging:
  level: "info"
  format: "human"
```

Run a backup:

```sh
./bin/conba backup
```

### Running the container image locally

After `make docker/build`, run the built image with your local (gitignored)
config bind-mounted in. This is the recommended way to smoke-test conba
against the host's Docker daemon without installing the binary:

```sh
docker run --rm -it \
  --hostname "$(hostname)" \
  -v "$PWD/conba-config.test.yaml:/app/conba.yaml:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /var/lib/docker/volumes:/var/lib/docker/volumes:ro \
  -v /tmp/conba-restic-test-repo:/tmp/conba-restic-test-repo \
  ghcr.io/lazybytez/conba:edge \
  backup --dry-run
```

Drop `--dry-run` to execute the backup. `--hostname "$(hostname)"` makes
snapshots carry the real host's name instead of a random container ID
(conba tags every snapshot with the hostname). The Docker socket mount
lets conba discover running containers; `/var/lib/docker/volumes` exposes
the actual volume contents so they can be read for snapshotting;
`/tmp/conba-restic-test-repo` is the writable local restic repository
(matching `restic.repository` in the test config); and the config is
mounted to `/app/conba.yaml`, the default lookup path inside the image's
working directory.

### Backing up bind mounts

Two things to know about bind mounts:

1. **Container labels match the destination path.** Use the
   container-side destination in `conba.exclude-mount-destinations`
   (and other label values), not the host source. Destinations are
   portable across hosts; sources are not.
2. **Conba opens the source path.** When conba runs in a container,
   the host source of every bind mount you want backed up must be
   visible inside conba's container — mount it at the same path.

Example: a service with `-v /srv/myapp/data:/var/lib/myapp/data` is
only backed up when conba's container also has `/srv/myapp/data`
mounted at `/srv/myapp/data`:

```sh
docker run --rm -it \
  ...existing mounts... \
  -v /srv/myapp/data:/srv/myapp/data:ro \
  ghcr.io/lazybytez/conba:edge backup
```

If the source isn't reachable, conba pre-flights, logs
`WARN: skipping <container>/<destination>: source unreadable (...)`,
and continues with the remaining targets.

## Container Labels

Configure per-container behavior with Docker labels:

| Label | Values | Default | Description |
|-------|--------|---------|-------------|
| `conba.enabled` | `true`, `false` | — | Override include/exclude filters |
| `conba.strategy` | `snapshot`, `command-snapshot`, `stream` | `snapshot` | Backup strategy |
| `conba.pre-command` | shell command | — | Pre-backup command (command-snapshot) |
| `conba.post-command` | shell command | — | Post-backup command (command-snapshot) |
| `conba.stream-command` | shell command | — | Stream command (stream strategy) |
| `conba.stdin-filename` | filename | `stdin` | Filename for `--stdin-filename` |
| `conba.retention` | `Nd,Nw,Nm,Ny` | global | Per-container retention override |
| `conba.exclude-volumes` | comma-separated | — | Comma-separated list matched against `Mount.Name`. For named volumes that's the volume name; for bind mounts it's the host source path (which is rarely portable across hosts — prefer `conba.exclude-mount-destinations` for bind mounts). |
| `conba.exclude-bind-mounts` | `true`, `false` | `false` | Set to `true` on a container to exclude all of its bind-mounted paths from backup. Named volumes on the same container are not affected. Default: false (bind mounts are eligible). |
| `conba.exclude-mount-destinations` | comma-separated | — | Comma-separated list of container-side destination paths. Any mount (bind or named volume) whose destination matches an entry exactly is excluded from backup. Example: `conba.exclude-mount-destinations: "/var/log,/etc/myapp/cache"`. |
| `conba.pre-backup.command` | shell command | — | Required to enable a pre-backup command for the container; the shell string executed inside the container, whose stdout is streamed into restic as the snapshot. Requires `pre_backup_commands.enabled: true` in config. |
| `conba.pre-backup.mode` | `replace`, `alongside` | `replace` | `replace` substitutes the stream snapshot for the container's volume snapshots; `alongside` produces the stream snapshot plus the volume snapshots. |
| `conba.pre-backup.container` | container name | labeled container | Override the exec target — the named container must already be running. Use for sidecar patterns where dump tools live in a separate admin container. |
| `conba.pre-backup.filename` | filename | labeled container name | Filename used for restic's `--stdin-filename` (e.g. `mysql.sql`). |

## Pre-backup commands

Stateful services like databases produce inconsistent on-disk files
unless quiesced or routed through the engine's own export tool. Conba
can run a shell command inside a container at backup time and stream
its stdout into restic as the snapshot — for example, `mysqldump`
piped straight into a restic snapshot tagged for the mysql container.

The feature is **off by default**. Label-driven command execution is a
qualitative change in conba's trust surface (anyone able to set labels
on a container can cause conba to execute arbitrary shell strings
inside it), so operators must opt in explicitly:

```yaml
pre_backup_commands:
  enabled: true
```

When `pre_backup_commands.enabled` is `false` or absent (the default),
all `conba.pre-backup.*` labels are ignored and volume backups proceed
as usual.

### Example: consistent MySQL backups via mysqldump

Label the mysql container with the dump command and (optionally) a
filename for the stream:

```yaml
# compose.yaml
services:
  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: "${MYSQL_ROOT_PASSWORD}"
    volumes:
      - mysql-data:/var/lib/mysql
    labels:
      conba.pre-backup.command: 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump --all-databases -uroot'
      conba.pre-backup.filename: "mysql.sql"

volumes:
  mysql-data:
```

The `MYSQL_PWD` env var is preferred over `-p<password>` because the
`-p<password>` form puts the password on the argv where any `ps`
invocation in the container's PID namespace can read it; `MYSQL_PWD`
keeps it in the env.

Enable the feature in `conba.yaml`:

```yaml
pre_backup_commands:
  enabled: true
```

At backup time, conba runs `mysqldump` inside the mysql container via
`docker exec` and streams its stdout into a single restic snapshot
tagged `container=mysql` and `kind=stream` (an internal tag conba
writes to distinguish stream snapshots from volume snapshots — not a
label you set). In the default `replace` mode, the on-disk
`mysql-data` volume is **not** backed up as a separate snapshot —
the dump is the canonical representation of the database's state, so
the inconsistent at-rest files are skipped. Switch to
`conba.pre-backup.mode: alongside` if the container also holds
volumes you want backed up directly (e.g. an uploads directory next
to the database).

## CLI Commands

```
conba backup              # Discover, filter, and backup all matching volumes
conba backup --dry-run    # Show what would be backed up without executing
conba forget              # Apply retention policies and prune
conba snapshots           # List snapshots
conba version             # Print version info
```

## Development

All build operations run inside Docker containers via Make:

```sh
make build       # Build the binary
make test        # Run tests with race detector
make lint        # Run golangci-lint
make coverage    # Run tests with coverage report
make fmt         # Format code
make clean       # Remove build artifacts
```

### End-to-end tests

The `test/e2e/` package exercises the compiled `conba` binary against a real
Docker daemon and a real restic filesystem repository. A small Docker Compose
stack (`test/e2e/compose.yaml`) provides MySQL plus two Alpine services as
backup targets. Run the full suite with:

```sh
make e2e
```

The target builds the test image, brings the compose fixture up, runs every
scenario inside the test image (with `/var/run/docker.sock` and
`/var/lib/docker/volumes` mounted), then unconditionally tears the fixture
down. Iterative loop: `make go/test-e2e/up` once, then `make go/test-e2e/run`
repeatedly. CI runs the same target on every PR via `.github/workflows/e2e.yml`
and publishes per-scenario pass/fail.

### Branching

| Branch | Purpose |
|--------|---------|
| `main` | Stable — all PRs target here |
| `feature/*` | New features |
| `fix/*` | Bug fixes |

### Commit Messages

Conventional commits enforced via [commitlint](https://commitlint.js.org/):

```
prefix(scope): subject
```

Prefixes: `feat`, `fix`, `build`, `chore`, `ci`, `docs`, `perf`, `refactor`, `revert`, `style`, `test`, `sec`

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Useful Links

[License][license-url] -
[Contributing](CONTRIBUTING.md) -
[Code of Conduct][codeofconduct-url] -
[Security](SECURITY.md) -
[Issues][issues-url] -
[Pull Requests][pulls-url]

<hr>

###### Copyright (c) [Lazy Bytez][team-url]. All rights reserved | Licensed under the MIT license.

<!-- Badges -->

[license-badge]: https://img.shields.io/github/license/lazybytez/conba?style=for-the-badge&colorA=302D41&colorB=a6e3a1
[ci-badge]: https://img.shields.io/github/actions/workflow/status/lazybytez/conba/go.yml?style=for-the-badge&colorA=302D41&colorB=89b4fa&label=CI
[commit-badge]: https://img.shields.io/github/last-commit/lazybytez/conba?style=for-the-badge&colorA=302D41&colorB=cba6f7

<!-- Links -->

[license-url]: https://github.com/lazybytez/conba/blob/main/LICENSE
[ci-url]: https://github.com/lazybytez/conba/actions/workflows/go.yml
[commit-url]: https://github.com/lazybytez/conba/commits/main
[codeofconduct-url]: https://github.com/lazybytez/.github/blob/main/docs/CODE_OF_CONDUCT.md
[issues-url]: https://github.com/lazybytez/conba/issues
[pulls-url]: https://github.com/lazybytez/conba/pulls
[team-url]: https://github.com/lazybytez
