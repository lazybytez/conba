# Commands

All commands read configuration as described in [Configuration](configuration.md).

## Global flags

| Flag | Description |
|------|-------------|
| `-c, --config <path>` | Config file path (default `conba.yaml`). |
| `-o, --output text\|json` | Output format. Overrides `output.format`; defaults to `auto` (text on a terminal, JSON otherwise). |
| `--no-color` | Disable colored text output (also honors `NO_COLOR`). |

In **text** mode commands print human-readable lines and tables to stdout. In
**json** mode they emit a newline-delimited JSON event stream to stdout (the
diagnostic logger stays on stderr). Every command's events are listed with it
below; the schema and exit codes are documented in
[Automation](guides/automation.md).

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. |
| `1` | Configuration or precondition error (invalid/missing config, repository locked or not initialised). |
| `2` | Partial failure — at least one target succeeded and at least one failed (`backup`/`forget`/`run`). |
| `3` | Total failure, or any unclassified fatal error. |

| Command | Purpose | Dry-run |
|---------|---------|---------|
| [`init`](#init) | Initialise the restic repository | — |
| [`backup`](#backup) | Back up all discovered volume targets | ✅ |
| [`snapshots`](#snapshots) | List snapshots | n/a (read-only) |
| [`restore`](#restore) | Restore a volume or stream snapshot | ✅ |
| [`verify`](#verify) | Check repository integrity | n/a (read-only) |
| [`diff`](#diff) | Show changes between two snapshots | n/a (read-only) |
| [`forget`](#forget) | Apply retention / prune snapshots | ✅ |
| [`run`](#run) | One-shot init + backup + forget cycle | ✅ |
| [`inspect`](#inspect) | Preview which containers/volumes would be backed up | n/a (read-only) |
| [`status`](#status) | Show repository status | n/a (read-only) |
| [`unlock`](#unlock) | Remove stale repository locks | — |
| [`version`](#version) | Print version information | — |

Mutating commands (`backup`, `restore`, `forget`, `run`) support `--dry-run`,
which prints the planned actions and performs none of them. Read-only commands
have no dry-run because they never change state.

---

## init

```sh
conba init
```

Initialises the configured restic repository. Safe to run against an
already-initialised repository (reported, not an error).

## backup

```sh
conba backup [--dry-run]
```

Discovers running containers, applies discovery filters, and backs up each
eligible volume mount as a `kind=volume` snapshot. If
`pre_backup_commands.enabled` is true, containers carrying
`conba.pre-backup.command` instead (or additionally) produce a `kind=stream`
snapshot — see [Database dumps](guides/database-dumps.md).

| Flag | Description |
|------|-------------|
| `--dry-run` | Print what would be backed up; create no snapshots. |

## snapshots

```sh
conba snapshots [--container <name>] [--volume <name>] [--hostname <host>]
```

Lists snapshots, optionally filtered. Filters combine (AND).

| Flag | Description |
|------|-------------|
| `--container` | Only snapshots tagged `container=<name>`. |
| `--volume` | Only snapshots tagged `volume=<name>`. |
| `--hostname` | Only snapshots tagged `hostname=<host>`. |

## restore

```sh
conba restore --container <name> [--volume <name>] [--snapshot <id>] \
              [--to <dir>] [--to-command <cmd>] [--force] [--all-hosts] [--dry-run]
```

Restores the latest matching snapshot (or `--snapshot <id>`). The mode is
auto-detected from the snapshot's `kind` tag. See [Restoring](guides/restore.md).

| Flag | Description |
|------|-------------|
| `--container` | Container whose snapshot to restore (**required**). |
| `--volume` | Volume name; required when multiple volume snapshots match. |
| `--snapshot` | Restore a specific snapshot ID instead of the latest. |
| `--to` | Host directory to restore a **volume** snapshot into. |
| `--to-command` | In-container command to pipe a **stream** snapshot into. |
| `--force` | Overwrite a non-empty restore destination (volume mode). |
| `--all-hosts` | Consider snapshots from any hostname, not just this host. |
| `--dry-run` | Print the planned restore; touch nothing. |

## verify

```sh
conba verify [--read-data]
```

Wraps `restic check`. Exits non-zero if the repository is corrupt or missing.

| Flag | Description |
|------|-------------|
| `--read-data` | Read and verify every data blob (slow, exhaustive). Without it, only repository structure is checked. |

## diff

```sh
conba diff <snapshot-a> <snapshot-b>
```

Wraps `restic diff` and prints its output verbatim. Snapshot identifiers may be
full IDs, short IDs, or the literal `latest`.

## forget

```sh
conba forget [--dry-run] [--no-prune] [--all-hosts] \
             [--container <name>] [--volume <name>] [--tag <tag>]...
```

Applies the configured [retention](guides/retention-and-filtering.md) policy
(with per-container `conba.retention` overrides) and prunes by default.

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be forgotten; change nothing. |
| `--no-prune` | Forget snapshots but skip the (slow) prune step. |
| `--all-hosts` | Apply across all hostnames, not just this host. |
| `--container` | Restrict to one container. |
| `--volume` | Restrict to one volume. |
| `--tag` | Restrict to snapshots carrying the tag (repeatable). |

## run

```sh
conba run [--dry-run] [--all-hosts] [--no-forget]
```

The on-demand "do everything" command, intended for cron/CI: initialises the
repository if needed, runs a backup, then applies retention. This is the typical
scheduled entry point.

| Flag | Description |
|------|-------------|
| `--dry-run` | Plan the full cycle without changing anything. |
| `--all-hosts` | Pass through to the forget phase. |
| `--no-forget` | Skip the retention/forget phase. |

## inspect

```sh
conba inspect
```

Read-only preview of discovery: lists containers and volumes that **would** be
backed up (Included) and those that would not (Excluded, with reasons), plus any
pre-backup label details. Use it to validate filters before backing up.

## status

```sh
conba status
```

Prints repository status (initialised/ready, repository path, snapshot/stats
summary). Exits 0 even when the repository is uninitialised, printing a friendly
"run 'conba init'" hint.

## unlock

```sh
conba unlock
```

Removes stale locks left behind by an interrupted restic operation.

## version

```sh
conba version
```

Prints the conba version plus the Go and bundled restic versions. Requires no
configuration or repository.
