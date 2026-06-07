# Conba Documentation

Conba is a Go CLI that wraps [restic](https://restic.net/) to back up Docker
container volumes — either as raw volume snapshots or by streaming a command's
output (e.g. `mysqldump`) straight into a snapshot. It is invoked **on demand**
(there is no built-in scheduler); run it from cron, a systemd timer, or a CI/CD
pipeline.

## Start here

- [Installation](installation.md) — build from source or run the container image.
- [Getting started](guides/getting-started.md) — your first backup, end to end.

## Reference

- [Configuration](configuration.md) — the `conba.yaml` file and `CONBA_*`
  environment overrides, field by field.
- [Commands](commands.md) — every subcommand, its flags, and dry-run support.
- [Container labels](container-labels.md) — per-container filtering, retention,
  and pre-backup/restore behaviour driven by Docker labels.

## Guides

- [Database dumps (streaming backups)](guides/database-dumps.md) — back up
  MySQL/PostgreSQL/etc. with a consistent dump instead of raw volume files.
- [Restoring](guides/restore.md) — restore volume and stream snapshots.
- [Retention and filtering](guides/retention-and-filtering.md) — choose what
  gets backed up and how long snapshots are kept.
- [Automation](guides/automation.md) — JSON output, exit codes, and the event
  schema for Ansible, cron, and log monitoring.

## Concepts at a glance

| Term | Meaning |
|------|---------|
| **Target** | A single container volume mount eligible for backup. |
| **Volume snapshot** | A restic snapshot of a volume's files (`kind=volume`). |
| **Stream snapshot** | A restic snapshot of a command's stdout (`kind=stream`), e.g. a `mysqldump`. |
| **Pre-backup command** | A command run inside a container at backup time whose stdout becomes a stream snapshot. |
| **Discovery** | Listing running containers and their mounts via the Docker API. |
| **Filter** | Include/exclude rules (config + labels) deciding which targets are backed up. |

Every snapshot is tagged with `container=`, `volume=` (volume snapshots),
`hostname=`, and `kind=`, which is how `snapshots`, `restore`, and `forget`
locate and scope snapshots.
