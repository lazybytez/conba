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
volumes, applies filtering rules, snapshots each volume (optionally running a pre-backup
command and streaming its output instead), and manages snapshot retention — all driven by
a YAML config file with environment variable overrides and optional container labels.

Conba runs **on demand** — there is no built-in scheduler. Invoke it from cron, a systemd
timer, or CI/CD (the `run` command does a full init + backup + forget cycle in one shot).

## Features

| Feature | Description |
|---------|-------------|
| Auto-discovery | Finds all running containers and their volume mounts via Docker API |
| Label-driven config | Per-container filtering, retention, and pre-backup commands via Docker labels |
| Pre-backup commands | Run a command in a container and stream its stdout into a snapshot — replacing or running alongside volume backups (opt-in) |
| Flexible filtering | Include/exclude by name, ID, regex, or labels; opt-in-only mode |
| Retention management | Global policy with per-container overrides; wraps `restic forget --prune` |
| Tagged snapshots | Every snapshot tagged with container name, ID, volume name, and hostname |
| Environment overrides | All config values overridable via `CONBA_` prefixed env vars |
| Human + machine output | Text for terminals, NDJSON event stream for automation (auto-detected), plus differentiated exit codes |

## Requirements

- Docker (or compatible runtime with Docker socket)
- restic (installed separately for the host binary; bundled in the container image)

## Quick start

Build the binary (all Make targets run inside Docker — no local Go needed) or use the
container image; see [Installation](docs/installation.md) for both paths.

```sh
git clone https://github.com/lazybytez/conba.git
cd conba
make build        # -> ./bin/conba
```

Create a minimal `conba.yaml`:

```yaml
restic:
  repository: "s3:s3.amazonaws.com/my-bucket"
  password_file: "/run/secrets/restic-password"

retention:
  keep_daily: 7
  keep_weekly: 4
```

Then:

```sh
conba init                 # initialise the repository
conba inspect              # preview which containers/volumes will be backed up
conba backup --dry-run     # confirm, then run for real:
conba backup
conba snapshots            # list what you have
```

For scheduled backups, run `conba run` (init + backup + forget) from cron/CI. A full
walkthrough is in [Getting started](docs/guides/getting-started.md).

## Documentation

Full documentation lives in [`docs/`](docs/README.md):

- **[Installation](docs/installation.md)** — host binary or container image.
- **[Getting started](docs/guides/getting-started.md)** — first backup, end to end.
- **[Configuration](docs/configuration.md)** — `conba.yaml` and `CONBA_*` env overrides.
- **[Commands](docs/commands.md)** — every subcommand, flags, and dry-run support.
- **[Container labels](docs/container-labels.md)** — per-container filtering, retention, and pre-backup behaviour.
- **[Database dumps](docs/guides/database-dumps.md)** — consistent DB backups via `mysqldump` and friends.
- **[Restoring](docs/guides/restore.md)** — restore volume and stream snapshots.
- **[Retention and filtering](docs/guides/retention-and-filtering.md)** — scope and snapshot lifetime.
- **[Automation](docs/guides/automation.md)** — JSON output, exit codes, and the event schema for Ansible/cron/monitoring.

## Commands

| Command | Purpose |
|---------|---------|
| `init` | Initialise the restic repository |
| `backup` | Back up all discovered volume targets (`--dry-run` supported) |
| `snapshots` | List snapshots (filter by container/volume/hostname) |
| `restore` | Restore a volume or stream snapshot (`--dry-run` supported) |
| `verify` | Check repository integrity (`--read-data` for a full scan) |
| `diff` | Show changes between two snapshots |
| `forget` | Apply retention / prune (`--dry-run` supported) |
| `run` | One-shot init + backup + forget cycle (for cron/CI) |
| `inspect` | Preview which containers/volumes would be backed up |
| `status` | Show repository status |
| `unlock` | Remove stale repository locks |
| `version` | Print version information |

See [Commands](docs/commands.md) for full flag details.

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

The `test/e2e/` package exercises the compiled `conba` binary against a real Docker daemon
and a real restic filesystem repository, using a small Docker Compose fixture
(`test/e2e/compose.yaml`). Run the full suite with:

```sh
make e2e
```

The target builds the test image, brings the fixture up, runs every scenario, then
unconditionally tears the fixture down. For an iterative loop: `make go/test-e2e/up` once,
then `make go/test-e2e/run` repeatedly. CI runs the same target on every PR.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the branching model
(`main`, `feature/*`, `fix/*`) and the conventional-commit format enforced by commitlint.

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
