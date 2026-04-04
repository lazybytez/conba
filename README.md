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
| `conba.exclude-volumes` | comma-separated | — | Volume names to skip |

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
